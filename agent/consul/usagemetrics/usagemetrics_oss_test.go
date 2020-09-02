// +build !consulent

package usagemetrics

import "github.com/hashicorp/consul/agent/consul/state"

func newStateStore() (*state.Store, error) {
	return state.NewStateStore(nil)
}
