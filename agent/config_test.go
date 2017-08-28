package agent

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/pascaldekloe/goe/verify"
)

func TestCheckDefinitionToCheckType(t *testing.T) {
	t.Parallel()
	got := &structs.CheckDefinition{
		ID:     "id",
		Name:   "name",
		Status: "green",
		Notes:  "notes",

		ServiceID:         "svcid",
		Token:             "tok",
		Script:            "/bin/foo",
		HTTP:              "someurl",
		TCP:               "host:port",
		Interval:          1 * time.Second,
		DockerContainerID: "abc123",
		Shell:             "/bin/ksh",
		TLSSkipVerify:     true,
		Timeout:           2 * time.Second,
		TTL:               3 * time.Second,
		DeregisterCriticalServiceAfter: 4 * time.Second,
	}
	want := &structs.CheckType{
		CheckID: "id",
		Name:    "name",
		Status:  "green",
		Notes:   "notes",

		Script:            "/bin/foo",
		HTTP:              "someurl",
		TCP:               "host:port",
		Interval:          1 * time.Second,
		DockerContainerID: "abc123",
		Shell:             "/bin/ksh",
		TLSSkipVerify:     true,
		Timeout:           2 * time.Second,
		TTL:               3 * time.Second,
		DeregisterCriticalServiceAfter: 4 * time.Second,
	}
	verify.Values(t, "", got.CheckType(), want)
}
