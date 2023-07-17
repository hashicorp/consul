package stream

// StringSubject can be used as a Subject for Events sent to the EventPublisher
type StringSubject string

func (s StringSubject) String() string { return string(s) }

// StringTopic can be used as a Topic for Events sent to the EventPublisher
type StringTopic string

func (s StringTopic) String() string { return string(s) }
