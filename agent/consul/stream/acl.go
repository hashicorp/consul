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
		if health.ServiceNode != nil {
			if health.ServiceNode.Node != nil {
				rules = append(rules, &ACLRule{
					Resource: ACLResource_NodeACL,
					Segment:  health.ServiceNode.Node.Node,
				})
			}
			if health.ServiceNode.Service != nil {
				rules = append(rules, &ACLRule{
					Resource: ACLResource_ServiceACL,
					Segment:  health.ServiceNode.Service.Service,
				})
			}
		}
		e.RequiredACLs = rules
	default:
	}
}
