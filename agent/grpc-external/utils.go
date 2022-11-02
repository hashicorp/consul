package external

import "github.com/hashicorp/go-uuid"

// We tag logs with a unique identifier to ease debugging. In the future this
// should probably be a real Open Telemetry trace ID.
func TraceID() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		return ""
	}
	return id
}
