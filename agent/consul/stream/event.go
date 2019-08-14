package stream

// FilterObject returns the object in the event to use for boolean
// expression filtering.
func (e *Event) FilterObject() interface{} {
	if e == nil || e.Payload == nil {
		return nil
	}

	switch e.Payload.(type) {
	case *Event_ServiceHealth:
		return e.GetServiceHealth().CheckServiceNode
	default:
		return nil
	}
}
