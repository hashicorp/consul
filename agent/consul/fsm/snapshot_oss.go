//go:build !consulent

package fsm

import (
	"github.com/hashicorp/consul/agent/structs"
)

func registerEnterpriseRestorers(_ map[structs.MessageType]restorer) {}

func registerEnterprisePersisters(_ []persister, _ *snapshot) {}
