package fsm

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-raftchunking"
	"github.com/hashicorp/raft"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
)

// command is a command method on the FSM.
type command func(buf []byte, index uint64) interface{}

// unboundCommand is a command method on the FSM, not yet bound to an FSM
// instance.
type unboundCommand func(c *FSM, buf []byte, index uint64) interface{}

// commands is a map from message type to unbound command.
var commands map[structs.MessageType]unboundCommand

// registerCommand registers a new command with the FSM, which should be done
// at package init() time.
func registerCommand(msg structs.MessageType, fn unboundCommand) {
	if commands == nil {
		commands = make(map[structs.MessageType]unboundCommand)
	}
	if commands[msg] != nil {
		panic(fmt.Errorf("Message %d is already registered", msg))
	}
	commands[msg] = fn
}

// FSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type FSM struct {
	deps    Deps
	logger  hclog.Logger
	chunker *raftchunking.ChunkingFSM

	// apply is built off the commands global and is used to route apply
	// operations to their appropriate handlers.
	apply map[structs.MessageType]command

	// stateLock is only used to protect outside callers to State() from
	// racing with Restore(), which is called by Raft (it puts in a totally
	// new state store). Everything internal here is synchronized by the
	// Raft side, so doesn't need to lock this.
	stateLock sync.RWMutex
	state     *state.Store

	publisher *stream.EventPublisher
}

// New is used to construct a new FSM with a blank state.
//
// Deprecated: use NewFromDeps.
func New(gc *state.TombstoneGC, logger hclog.Logger) (*FSM, error) {
	newStateStore := func() *state.Store {
		return state.NewStateStore(gc)
	}
	return NewFromDeps(Deps{Logger: logger, NewStateStore: newStateStore}), nil
}

// Deps are dependencies used to construct the FSM.
type Deps struct {
	// Logger used to emit log messages
	Logger hclog.Logger
	// NewStateStore returns a state.Store which the FSM will use to make changes
	// to the state.
	// NewStateStore will be called once when the FSM is created and again any
	// time Restore() is called.
	NewStateStore func() *state.Store

	Publisher *stream.EventPublisher
}

// NewFromDeps creates a new FSM from its dependencies.
func NewFromDeps(deps Deps) *FSM {
	if deps.Logger == nil {
		deps.Logger = hclog.New(&hclog.LoggerOptions{})
	}

	fsm := &FSM{
		deps:   deps,
		logger: deps.Logger.Named(logging.FSM),
		apply:  make(map[structs.MessageType]command),
		state:  deps.NewStateStore(),
	}

	// Build out the apply dispatch table based on the registered commands.
	for msg, fn := range commands {
		thisFn := fn
		fsm.apply[msg] = func(buf []byte, index uint64) interface{} {
			return thisFn(fsm, buf, index)
		}
	}

	fsm.chunker = raftchunking.NewChunkingFSM(fsm, nil)

	// register the streaming snapshot handlers if an event publisher was provided.
	fsm.registerStreamSnapshotHandlers()

	return fsm
}

func (c *FSM) ChunkingFSM() *raftchunking.ChunkingFSM {
	return c.chunker
}

// State is used to return a handle to the current state
func (c *FSM) State() *state.Store {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.state
}

func (c *FSM) Apply(log *raft.Log) interface{} {
	buf := log.Data
	msgType := structs.MessageType(buf[0])

	// Check if this message type should be ignored when unknown. This is
	// used so that new commands can be added with developer control if older
	// versions can safely ignore the command, or if they should crash.
	ignoreUnknown := false
	if msgType&structs.IgnoreUnknownTypeFlag == structs.IgnoreUnknownTypeFlag {
		msgType &= ^structs.IgnoreUnknownTypeFlag
		ignoreUnknown = true
	}

	// Apply based on the dispatch table, if possible.
	if fn := c.apply[msgType]; fn != nil {
		return fn(buf[1:], log.Index)
	}

	// Otherwise, see if it's safe to ignore. If not, we have to panic so
	// that we crash and our state doesn't diverge.
	if ignoreUnknown {
		c.logger.Warn("ignoring unknown message type, upgrade to newer version", "type", msgType)
		return nil
	}
	panic(fmt.Errorf("failed to apply request: %#v", buf))
}

func (c *FSM) Snapshot() (raft.FSMSnapshot, error) {
	defer func(start time.Time) {
		c.logger.Info("snapshot created", "duration", time.Since(start).String())
	}(time.Now())

	chunkState, err := c.chunker.CurrentState()
	if err != nil {
		return nil, err
	}

	return &snapshot{
		state:      c.state.Snapshot(),
		chunkState: chunkState,
	}, nil
}

// Restore streams in the snapshot and replaces the current state store with a
// new one based on the snapshot if all goes OK during the restore.
func (c *FSM) Restore(old io.ReadCloser) error {
	defer old.Close()

	stateNew := c.deps.NewStateStore()

	// Set up a new restore transaction
	restore := stateNew.Restore()
	defer restore.Abort()

	handler := func(header *SnapshotHeader, msg structs.MessageType, dec *codec.Decoder) error {
		switch {
		case msg == structs.ChunkingStateType:
			chunkState := &raftchunking.State{
				ChunkMap: make(raftchunking.ChunkMap),
			}
			if err := dec.Decode(chunkState); err != nil {
				return err
			}
			if err := c.chunker.RestoreState(chunkState); err != nil {
				return err
			}
		case restorers[msg] != nil:
			fn := restorers[msg]
			if err := fn(header, restore, dec); err != nil {
				return err
			}
		default:
			if msg >= 64 {
				return fmt.Errorf("msg type <%d> is a Consul Enterprise log entry. Consul OSS cannot restore it", msg)
			} else {
				return fmt.Errorf("Unrecognized msg type %d", msg)
			}
		}
		return nil
	}
	if err := ReadSnapshot(old, handler); err != nil {
		return err
	}

	if err := restore.Commit(); err != nil {
		return err
	}

	// External code might be calling State(), so we need to synchronize
	// here to make sure we swap in the new state store atomically.
	c.stateLock.Lock()
	stateOld := c.state
	c.state = stateNew

	// Tell the EventPublisher to cycle anything watching these topics. Replacement
	// of the state store means that indexes could have gone backwards and data changed.
	//
	// This needs to happen while holding the state lock to ensure its not racey. If we
	// did this outside of the locked section closer to where we abandon the old store
	// then there would be a possibility for new streams to be opened that would get
	// a snapshot from the cache sourced from old data but would be receiving events
	// for new data. To prevent that inconsistency we refresh the topics while holding
	// the lock which ensures that any subscriptions to topics for FSM generated events
	if c.deps.Publisher != nil {
		c.deps.Publisher.RefreshTopic(state.EventTopicServiceHealth)
		c.deps.Publisher.RefreshTopic(state.EventTopicServiceHealthConnect)
		c.deps.Publisher.RefreshTopic(state.EventTopicCARoots)
	}
	c.stateLock.Unlock()

	// Signal that the old state store has been abandoned. This is required
	// because we don't operate on it any more, we just throw it away, so
	// blocking queries won't see any changes and need to be woken up.
	stateOld.Abandon()

	return nil
}

// ReadSnapshot decodes each message type and utilizes the handler function to
// process each message type individually
func ReadSnapshot(r io.Reader, handler func(header *SnapshotHeader, msg structs.MessageType, dec *codec.Decoder) error) error {
	// Create a decoder
	dec := codec.NewDecoder(r, structs.MsgpackHandle)

	// Read in the header
	var header SnapshotHeader
	if err := dec.Decode(&header); err != nil {
		return err
	}

	// Populate the new state
	msgType := make([]byte, 1)
	for {
		// Read the message type
		_, err := r.Read(msgType)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		// Decode
		msg := structs.MessageType(msgType[0])

		if err := handler(&header, msg, dec); err != nil {
			return err
		}
	}
}

func (c *FSM) registerStreamSnapshotHandlers() {
	if c.deps.Publisher == nil {
		return
	}

	err := c.deps.Publisher.RegisterHandler(state.EventTopicServiceHealth, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().ServiceHealthSnapshot(req, buf)
	}, false)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}

	err = c.deps.Publisher.RegisterHandler(state.EventTopicServiceHealthConnect, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().ServiceHealthSnapshot(req, buf)
	}, false)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}

	err = c.deps.Publisher.RegisterHandler(state.EventTopicCARoots, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().CARootsSnapshot(req, buf)
	}, false)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}

	err = c.deps.Publisher.RegisterHandler(state.EventTopicMeshConfig, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().MeshConfigSnapshot(req, buf)
	}, true)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}

	err = c.deps.Publisher.RegisterHandler(state.EventTopicServiceResolver, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().ServiceResolverSnapshot(req, buf)
	}, true)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}

	err = c.deps.Publisher.RegisterHandler(state.EventTopicIngressGateway, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().IngressGatewaySnapshot(req, buf)
	}, true)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}

	err = c.deps.Publisher.RegisterHandler(state.EventTopicServiceIntentions, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().ServiceIntentionsSnapshot(req, buf)
	}, true)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}

	err = c.deps.Publisher.RegisterHandler(state.EventTopicServiceList, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().ServiceListSnapshot(req, buf)
	}, true)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}

	err = c.deps.Publisher.RegisterHandler(state.EventTopicServiceDefaults, func(req stream.SubscribeRequest, buf stream.SnapshotAppender) (uint64, error) {
		return c.State().ServiceDefaultsSnapshot(req, buf)
	}, true)
	if err != nil {
		panic(fmt.Errorf("fatal error encountered registering streaming snapshot handlers: %w", err))
	}
}
