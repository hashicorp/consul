package fsm

import (
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
)

// msgpackHandle is a shared handle for encoding/decoding msgpack payloads
var msgpackHandle = &codec.MsgpackHandle{}

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
	logOutput io.Writer
	logger    *log.Logger
	path      string

	// apply is built off the commands global and is used to route apply
	// operations to their appropriate handlers.
	apply map[structs.MessageType]command

	// stateLock is only used to protect outside callers to State() from
	// racing with Restore(), which is called by Raft (it puts in a totally
	// new state store). Everything internal here is synchronized by the
	// Raft side, so doesn't need to lock this.
	stateLock sync.RWMutex
	state     *state.Store

	gc *state.TombstoneGC
}

// New is used to construct a new FSM with a blank state.
func New(gc *state.TombstoneGC, logOutput io.Writer) (*FSM, error) {
	stateNew, err := state.NewStateStore(gc)
	if err != nil {
		return nil, err
	}

	fsm := &FSM{
		logOutput: logOutput,
		logger:    log.New(logOutput, "", log.LstdFlags),
		apply:     make(map[structs.MessageType]command),
		state:     stateNew,
		gc:        gc,
	}

	// Build out the apply dispatch table based on the registered commands.
	for msg, fn := range commands {
		thisFn := fn
		fsm.apply[msg] = func(buf []byte, index uint64) interface{} {
			return thisFn(fsm, buf, index)
		}
	}

	return fsm, nil
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
		c.logger.Printf("[WARN] consul.fsm: ignoring unknown message type (%d), upgrade to newer version", msgType)
		return nil
	}
	panic(fmt.Errorf("failed to apply request: %#v", buf))
}

func (c *FSM) Snapshot() (raft.FSMSnapshot, error) {
	defer func(start time.Time) {
		c.logger.Printf("[INFO] consul.fsm: snapshot created in %v", time.Since(start))
	}(time.Now())

	return &snapshot{c.state.Snapshot()}, nil
}

// Restore streams in the snapshot and replaces the current state store with a
// new one based on the snapshot if all goes OK during the restore.
func (c *FSM) Restore(old io.ReadCloser) error {
	defer old.Close()

	// Create a new state store.
	stateNew, err := state.NewStateStore(c.gc)
	if err != nil {
		return err
	}

	// Set up a new restore transaction
	restore := stateNew.Restore()
	defer restore.Abort()

	// Create a decoder
	dec := codec.NewDecoder(old, msgpackHandle)

	// Read in the header
	var header snapshotHeader
	if err := dec.Decode(&header); err != nil {
		return err
	}

	// Populate the new state
	msgType := make([]byte, 1)
	for {
		// Read the message type
		_, err := old.Read(msgType)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// Decode
		msg := structs.MessageType(msgType[0])
		if fn := restorers[msg]; fn != nil {
			if err := fn(&header, restore, dec); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("Unrecognized msg type %d", msg)
		}
	}
	restore.Commit()

	// External code might be calling State(), so we need to synchronize
	// here to make sure we swap in the new state store atomically.
	c.stateLock.Lock()
	stateOld := c.state
	c.state = stateNew
	c.stateLock.Unlock()

	// Signal that the old state store has been abandoned. This is required
	// because we don't operate on it any more, we just throw it away, so
	// blocking queries won't see any changes and need to be woken up.
	stateOld.Abandon()
	return nil
}
