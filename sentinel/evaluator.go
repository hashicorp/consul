package sentinel

// Evaluator wraps the Sentinel evaluator from the HashiCorp Sentinel policy
// engine.
type Evaluator interface {
	Compile(policy string) error
	Execute(policy string, enforcementLevel string, data map[string]interface{}) bool
	Close()
}
