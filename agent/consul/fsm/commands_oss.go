//go:build !consulent

package fsm

import (
	"github.com/hashicorp/consul/agent/structs"
)

func registerEnterpriseCommands(_ map[structs.MessageType]command, _ *FSM) {}
