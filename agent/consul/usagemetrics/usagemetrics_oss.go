// +build !consulent

package usagemetrics

import "github.com/hashicorp/consul/agent/consul/state"

func (u *UsageMetricsReporter) emitEnterpriseUsage(state.ServiceUsage) {}
