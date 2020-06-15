package stream

type Topic int32

// TODO: remove underscores
// TODO: type string instead of int?
const (
	Topic_ServiceHealth        Topic = 0
	Topic_ServiceHealthConnect Topic = 1
	Topic_ACLTokens            Topic = 2
	Topic_ACLPolicies          Topic = 3
	Topic_ACLRoles             Topic = 4
)

// TODO:
type Event struct {
	Topic   Topic
	Key     string
	Index   uint64
	Payload interface{}
}

func (e Event) IsEndOfSnapshot() bool {
	return e.Payload == endOfSnapshot{}
}

func (e Event) IsResumeStream() bool {
	return e.Payload == ResumeStream{}
}

type endOfSnapshot struct{}

type ResumeStream struct{}
