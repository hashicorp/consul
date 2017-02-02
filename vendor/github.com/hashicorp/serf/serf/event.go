package serf

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

// EventType are all the types of events that may occur and be sent
// along the Serf channel.
type EventType int

const (
	EventMemberJoin EventType = iota
	EventMemberLeave
	EventMemberFailed
	EventMemberUpdate
	EventMemberReap
	EventUser
	EventQuery
)

func (t EventType) String() string {
	switch t {
	case EventMemberJoin:
		return "member-join"
	case EventMemberLeave:
		return "member-leave"
	case EventMemberFailed:
		return "member-failed"
	case EventMemberUpdate:
		return "member-update"
	case EventMemberReap:
		return "member-reap"
	case EventUser:
		return "user"
	case EventQuery:
		return "query"
	default:
		panic(fmt.Sprintf("unknown event type: %d", t))
	}
}

// Event is a generic interface for exposing Serf events
// Clients will usually need to use a type switches to get
// to a more useful type
type Event interface {
	EventType() EventType
	String() string
}

// MemberEvent is the struct used for member related events
// Because Serf coalesces events, an event may contain multiple members.
type MemberEvent struct {
	Type    EventType
	Members []Member
}

func (m MemberEvent) EventType() EventType {
	return m.Type
}

func (m MemberEvent) String() string {
	switch m.Type {
	case EventMemberJoin:
		return "member-join"
	case EventMemberLeave:
		return "member-leave"
	case EventMemberFailed:
		return "member-failed"
	case EventMemberUpdate:
		return "member-update"
	case EventMemberReap:
		return "member-reap"
	default:
		panic(fmt.Sprintf("unknown event type: %d", m.Type))
	}
}

// UserEvent is the struct used for events that are triggered
// by the user and are not related to members
type UserEvent struct {
	LTime    LamportTime
	Name     string
	Payload  []byte
	Coalesce bool
}

func (u UserEvent) EventType() EventType {
	return EventUser
}

func (u UserEvent) String() string {
	return fmt.Sprintf("user-event: %s", u.Name)
}

// Query is the struct used by EventQuery type events
type Query struct {
	LTime   LamportTime
	Name    string
	Payload []byte

	serf        *Serf
	id          uint32    // ID is not exported, since it may change
	addr        []byte    // Address to respond to
	port        uint16    // Port to respond to
	deadline    time.Time // Must respond by this deadline
	relayFactor uint8     // Number of duplicate responses to relay back to sender
	respLock    sync.Mutex
}

func (q *Query) EventType() EventType {
	return EventQuery
}

func (q *Query) String() string {
	return fmt.Sprintf("query: %s", q.Name)
}

// Deadline returns the time by which a response must be sent
func (q *Query) Deadline() time.Time {
	return q.deadline
}

// Respond is used to send a response to the user query
func (q *Query) Respond(buf []byte) error {
	q.respLock.Lock()
	defer q.respLock.Unlock()

	// Check if we've already responded
	if q.deadline.IsZero() {
		return fmt.Errorf("Response already sent")
	}

	// Ensure we aren't past our response deadline
	if time.Now().After(q.deadline) {
		return fmt.Errorf("Response is past the deadline")
	}

	// Create response
	resp := messageQueryResponse{
		LTime:   q.LTime,
		ID:      q.id,
		From:    q.serf.config.NodeName,
		Payload: buf,
	}

	// Send a direct response
	{
		raw, err := encodeMessage(messageQueryResponseType, &resp)
		if err != nil {
			return fmt.Errorf("Failed to format response: %v", err)
		}

		// Check the size limit
		if len(raw) > q.serf.config.QueryResponseSizeLimit {
			return fmt.Errorf("response exceeds limit of %d bytes", q.serf.config.QueryResponseSizeLimit)
		}

		addr := net.UDPAddr{IP: q.addr, Port: int(q.port)}
		if err := q.serf.memberlist.SendTo(&addr, raw); err != nil {
			return err
		}
	}

	// Relay the response through up to relayFactor other nodes
	members := q.serf.Members()
	if len(members) > 2 {
		addr := net.UDPAddr{IP: q.addr, Port: int(q.port)}
		raw, err := encodeRelayMessage(messageQueryResponseType, addr, &resp)
		if err != nil {
			return fmt.Errorf("Failed to format relayed response: %v", err)
		}

		// Check the size limit
		if len(raw) > q.serf.config.QueryResponseSizeLimit {
			return fmt.Errorf("relayed response exceeds limit of %d bytes", q.serf.config.QueryResponseSizeLimit)
		}

		relayMembers := kRandomMembers(int(q.relayFactor), members, func(m Member) bool {
			return m.Status != StatusAlive || m.ProtocolMax < 5 || m.Name == q.serf.LocalMember().Name
		})

		for _, m := range relayMembers {
			relayAddr := net.UDPAddr{IP: m.Addr, Port: int(m.Port)}
			if err := q.serf.memberlist.SendTo(&relayAddr, raw); err != nil {
				return err
			}
		}
	}

	// Clear the deadline, responses sent
	q.deadline = time.Time{}
	return nil
}

// kRandomMembers selects up to k members from a given list, optionally
// filtering by the given filterFunc
func kRandomMembers(k int, members []Member, filterFunc func(Member) bool) []Member {
	n := len(members)
	kMembers := make([]Member, 0, k)
OUTER:
	// Probe up to 3*n times, with large n this is not necessary
	// since k << n, but with small n we want search to be
	// exhaustive
	for i := 0; i < 3*n && len(kMembers) < k; i++ {
		// Get random member
		idx := rand.Intn(n)
		member := members[idx]

		// Give the filter a shot at it.
		if filterFunc != nil && filterFunc(member) {
			continue OUTER
		}

		// Check if we have this member already
		for j := 0; j < len(kMembers); j++ {
			if member.Name == kMembers[j].Name {
				continue OUTER
			}
		}

		// Append the member
		kMembers = append(kMembers, member)
	}

	return kMembers
}
