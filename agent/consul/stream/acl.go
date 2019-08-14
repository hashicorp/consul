package stream

// SetACLRules sets the required ACL permissions for the Event.
func (e *Event) SetACLRules() {
	var rules []*ACLRule

	switch e.Topic {
	case Topic_ServiceHealth:
		health := e.GetServiceHealth()
		if health == nil {
			return
		}
		if health.CheckServiceNode != nil {
			if health.CheckServiceNode.Node != nil {
				rules = append(rules, &ACLRule{
					Resource: ACLResource_NodeACL,
					Segment:  health.CheckServiceNode.Node.Node,
				})
			}
			if health.CheckServiceNode.Service != nil {
				rules = append(rules, &ACLRule{
					Resource: ACLResource_ServiceACL,
					Segment:  health.CheckServiceNode.Service.Service,
				})
			}
		}
		e.RequiredACLs = rules
	default:
	}
}
