package stream

type Topic int32

// TODO: remove underscores
// TODO: type string instead of int?
// TODO: define non-internal topics in state package?
const (
	TopicInternal              Topic = 0
	Topic_ServiceHealth        Topic = 1
	Topic_ServiceHealthConnect Topic = 2
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

// TODO: unexport once EventPublisher is in stream package
type UnsubscribePayload struct {
	TokensSecretIDs []string
}

// NewUnsubscribeEvent returns a special Event that is handled by the
// stream package, and is never sent to subscribers. It results in any subscriptions
// which match any of the TokenSecretIDs to be unsubscribed.
func NewUnsubscribeEvent(tokenSecretIDs []string) Event {
	return Event{Payload: UnsubscribePayload{TokensSecretIDs: tokenSecretIDs}}
}
