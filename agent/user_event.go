package agent

import (
	"bytes"
	"fmt"
	"regexp"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	// userEventMaxVersion is the maximum protocol version we understand
	userEventMaxVersion = 1

	// remoteExecName is the event name for a remote exec command
	remoteExecName = "_rexec"
)

// UserEventParam is used to parameterize a user event
type UserEvent struct {
	// ID of the user event. Automatically generated.
	ID string

	// Name of the event
	Name string `codec:"n"`

	// Optional payload
	Payload []byte `codec:"p,omitempty"`

	// NodeFilter is a regular expression to filter on nodes
	NodeFilter string `codec:"nf,omitempty"`

	// ServiceFilter is a regular expression to filter on services
	ServiceFilter string `codec:"sf,omitempty"`

	// TagFilter is a regular expression to filter on tags of a service,
	// must be provided with ServiceFilter
	TagFilter string `codec:"tf,omitempty"`

	// Version of the user event. Automatically generated.
	Version int `codec:"v"`

	// LTime is the lamport time. Automatically generated.
	LTime uint64 `codec:"-"`
}

// validateUserEventParams is used to sanity check the inputs
func validateUserEventParams(params *UserEvent) error {
	// Validate the inputs
	if params.Name == "" {
		return fmt.Errorf("User event missing name")
	}
	if params.TagFilter != "" && params.ServiceFilter == "" {
		return fmt.Errorf("Cannot provide tag filter without service filter")
	}
	if params.NodeFilter != "" {
		if _, err := regexp.Compile(params.NodeFilter); err != nil {
			return fmt.Errorf("Invalid node filter: %v", err)
		}
	}
	if params.ServiceFilter != "" {
		if _, err := regexp.Compile(params.ServiceFilter); err != nil {
			return fmt.Errorf("Invalid service filter: %v", err)
		}
	}
	if params.TagFilter != "" {
		if _, err := regexp.Compile(params.TagFilter); err != nil {
			return fmt.Errorf("Invalid tag filter: %v", err)
		}
	}
	return nil
}

// UserEvent is used to fire an event via the Serf layer on the LAN
// TODO: move to another file.
func (a *Agent) UserEvent(dc, token string, params *UserEvent) error {
	// Validate the params
	if err := validateUserEventParams(params); err != nil {
		return err
	}

	// Format message
	var err error
	if params.ID, err = uuid.GenerateUUID(); err != nil {
		return fmt.Errorf("UUID generation failed: %v", err)
	}
	params.Version = userEventMaxVersion
	payload, err := encodeMsgPackUserEvent(&params)
	if err != nil {
		return fmt.Errorf("UserEvent encoding failed: %v", err)
	}

	// Service the event fire over RPC. This ensures that we authorize
	// the request against the token first.
	args := structs.EventFireRequest{
		Datacenter:   dc,
		Name:         params.Name,
		Payload:      payload,
		QueryOptions: structs.QueryOptions{Token: token},
	}

	// Any server can process in the remote DC, since the
	// gossip will take over anyways
	args.AllowStale = true
	var out structs.EventFireResponse
	return a.RPC("Internal.EventFire", &args, &out)
}

// TODO: rename
type userEventHandler struct {
	// TODO: does this need to be reloadable?
	config  UserEventHandlerConfig
	eventCh chan serf.UserEvent

	// TODO: use context or Done() chan instead?
	shutdownCh chan struct{}
	// TODO: move to config
	logger hclog.Logger

	// TODO: document what this locks
	eventLock sync.RWMutex

	// eventBuf stores the most recent events in a ring buffer
	// using eventIndex as the next index to insert into. This
	// is guarded by eventLock. When an insert happens, the
	// eventNotify group is notified.
	eventBuf   []*UserEvent
	eventIndex int
}

type UserEventHandlerConfig struct {
	NodeName          string
	State             ServiceLister // TODO: rename field
	DisableRemoteExec bool
	HandleRemoteExec  func(*UserEvent)
	Notifier          *NotifyGroup // TODO: more descriptive name for field?
}

type ServiceLister interface {
	Services(entMeta *structs.EnterpriseMeta) map[structs.ServiceID]*structs.NodeService
}

func newUserEventHandler(cfg UserEventHandlerConfig, logger hclog.Logger) *userEventHandler {
	return &userEventHandler{
		config: cfg,
		logger: logger,
		// TODO: set shutdownCh
		eventCh:  make(chan serf.UserEvent, 1024),
		eventBuf: make([]*UserEvent, 256),
	}
}

func (u *userEventHandler) Submit(e serf.UserEvent) {
	select {
	case u.eventCh <- e:
	case <-u.shutdownCh:
	}
}

// Start is used to process incoming user events
func (u *userEventHandler) Start() {
	for {
		select {
		case e := <-u.eventCh:
			// Decode the event
			msg := new(UserEvent)
			if err := decodeMsgPackUserEvent(e.Payload, msg); err != nil {
				u.logger.Error("Failed to decode event", "error", err)
				continue
			}
			msg.LTime = uint64(e.LTime)

			// Skip if we don't pass filtering
			if !u.shouldProcessUserEvent(msg) {
				continue
			}

			// Ingest the event
			u.ingestUserEvent(msg)

		case <-u.shutdownCh:
			return
		}
	}
}

// shouldProcessUserEvent checks if an event makes it through our filters
func (u *userEventHandler) shouldProcessUserEvent(msg *UserEvent) bool {
	// Check the version
	if msg.Version > userEventMaxVersion {
		u.logger.Warn("Event version may have unsupported features",
			"version", msg.Version,
			"event", msg.Name,
		)
	}

	// Apply the filters
	if msg.NodeFilter != "" {
		re, err := regexp.Compile(msg.NodeFilter)
		if err != nil {
			u.logger.Error("Failed to parse node filter for event",
				"filter", msg.NodeFilter,
				"event", msg.Name,
				"error", err,
			)
			return false
		}
		if !re.MatchString(u.config.NodeName) {
			return false
		}
	}

	if msg.ServiceFilter != "" {
		re, err := regexp.Compile(msg.ServiceFilter)
		if err != nil {
			u.logger.Error("Failed to parse service filter for event",
				"filter", msg.ServiceFilter,
				"event", msg.Name,
				"error", err,
			)
			return false
		}

		var tagRe *regexp.Regexp
		if msg.TagFilter != "" {
			re, err := regexp.Compile(msg.TagFilter)
			if err != nil {
				u.logger.Error("Failed to parse tag filter for event",
					"filter", msg.TagFilter,
					"event", msg.Name,
					"error", err,
				)
				return false
			}
			tagRe = re
		}

		// Scan for a match
		services := u.config.State.Services(structs.DefaultEnterpriseMeta())
		found := false
	OUTER:
		for name, info := range services {
			// Check the service name
			if !re.MatchString(name.String()) {
				continue
			}
			if tagRe == nil {
				found = true
				break
			}

			// Look for a matching tag
			for _, tag := range info.Tags {
				if !tagRe.MatchString(tag) {
					continue
				}
				found = true
				break OUTER
			}
		}

		// No matching services
		if !found {
			return false
		}
	}
	return true
}

// ingestUserEvent is used to process an event that passes filtering
func (u *userEventHandler) ingestUserEvent(msg *UserEvent) {
	// Special handling for internal events
	switch msg.Name {
	case remoteExecName:
		if u.config.DisableRemoteExec {
			u.logger.Info("ignoring remote exec event, disabled.",
				"event_name", msg.Name,
				"event_id", msg.ID,
			)
		} else {
			go u.config.HandleRemoteExec(msg)
		}
		return
	default:
		u.logger.Debug("new event",
			"event_name", msg.Name,
			"event_id", msg.ID,
		)
	}

	u.eventLock.Lock()
	defer func() {
		u.eventLock.Unlock()
		u.config.Notifier.Notify()
	}()

	idx := u.eventIndex
	u.eventBuf[idx] = msg
	u.eventIndex = (idx + 1) % len(u.eventBuf)
}

// UserEvents is used to return a slice of the most recent
// user events.
func (u *userEventHandler) UserEvents() []*UserEvent {
	n := len(u.eventBuf)
	out := make([]*UserEvent, n)
	u.eventLock.RLock()
	defer u.eventLock.RUnlock()

	// Check if the buffer is full
	if u.eventBuf[u.eventIndex] != nil {
		if u.eventIndex == 0 {
			copy(out, u.eventBuf)
		} else {
			copy(out, u.eventBuf[u.eventIndex:])
			copy(out[n-u.eventIndex:], u.eventBuf[:u.eventIndex])
		}
	} else {
		// We haven't filled the buffer yet
		copy(out, u.eventBuf[:u.eventIndex])
		out = out[:u.eventIndex]
	}
	return out
}

// LastUserEvent is used to return the last user event.
// This will return nil if there is no recent event.
// TODO: remove? not used
func (u *userEventHandler) lastUserEvent() *UserEvent {
	u.eventLock.RLock()
	defer u.eventLock.RUnlock()
	n := len(u.eventBuf)
	idx := (((u.eventIndex - 1) % n) + n) % n
	return u.eventBuf[idx]
}

// msgpackHandleUserEvent is a shared handle for encoding/decoding of
// messages for user events
var msgpackHandleUserEvent = &codec.MsgpackHandle{
	RawToString: true,
	WriteExt:    true,
}

// decodeMsgPackUserEvent is used to decode a MsgPack encoded object
func decodeMsgPackUserEvent(buf []byte, out interface{}) error {
	return codec.NewDecoder(bytes.NewReader(buf), msgpackHandleUserEvent).Decode(out)
}

// encodeMsgPackUserEvent is used to encode an object with msgpack
func encodeMsgPackUserEvent(msg interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := codec.NewEncoder(&buf, msgpackHandleUserEvent).Encode(msg)
	return buf.Bytes(), err
}
