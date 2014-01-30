package agent

// ServiceDefinition is used to JSON decode the Service definitions
type ServiceDefinition struct {
	ID    string
	Name  string
	Tag   string
	Port  int
	Check *CheckType
}

// ChecKDefinition is used to JSON decode the Check definitions
type CheckDefinition struct {
	ID    string
	Name  string
	Notes string
	CheckType
}

// UnionDefinition is used to decode when we don't know if
// we are being given a ServiceDefinition or a CheckDefinition
type UnionDefinition struct {
	Service *ServiceDefinition
	Check   *CheckDefinition
}
