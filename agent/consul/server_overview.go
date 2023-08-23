package consul

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/usagemetrics"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

type OverviewManager struct {
	stateProvider usagemetrics.StateProvider
	logger        hclog.Logger
	interval      time.Duration

	currentSummary *structs.CatalogSummary
	sync.RWMutex
}

func NewOverviewManager(logger hclog.Logger, sp usagemetrics.StateProvider, interval time.Duration) *OverviewManager {
	return &OverviewManager{
		stateProvider:  sp,
		logger:         logger.Named("catalog-overview"),
		interval:       interval,
		currentSummary: &structs.CatalogSummary{},
	}
}

func (m *OverviewManager) GetCurrentSummary() *structs.CatalogSummary {
	m.RLock()
	defer m.RUnlock()
	return m.currentSummary
}

func (m *OverviewManager) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state := m.stateProvider.State()
			catalog, err := state.CatalogDump()
			if err != nil {
				m.logger.Error("failed to update overview", "error", err)
				continue
			}

			summary := getCatalogOverview(catalog)
			m.Lock()
			m.currentSummary = summary
			m.Unlock()
		}
	}
}

// getCatalogOverview returns a breakdown of the number of nodes, services, and checks
// in the passing/warning/critical states. In Enterprise, it will also return this
// breakdown for each partition and namespace.
func getCatalogOverview(catalog *structs.CatalogContents) *structs.CatalogSummary {
	nodeChecks := make(map[string][]*structs.HealthCheck)
	serviceInstanceChecks := make(map[string][]*structs.HealthCheck)
	checkSummaries := make(map[string]structs.HealthSummary)

	entMetaIDString := func(id string, entMeta acl.EnterpriseMeta) string {
		return fmt.Sprintf("%s/%s/%s", id, entMeta.PartitionOrEmpty(), entMeta.NamespaceOrEmpty())
	}

	// Compute the health check summaries by taking the pass/warn/fail counts
	// of each unique part/ns/checkname combo and storing them. Also store the
	// per-node and per-service instance checks for their respective summaries below.
	for _, check := range catalog.Checks {
		checkID := entMetaIDString(check.Name, check.EnterpriseMeta)
		summary, ok := checkSummaries[checkID]
		if !ok {
			summary = structs.HealthSummary{
				Name:           check.Name,
				EnterpriseMeta: check.EnterpriseMeta,
			}
		}

		summary.Add(check.Status)
		checkSummaries[checkID] = summary

		if check.ServiceID != "" {
			serviceInstanceID := entMetaIDString(fmt.Sprintf("%s/%s", check.Node, check.ServiceID), check.EnterpriseMeta)
			serviceInstanceChecks[serviceInstanceID] = append(serviceInstanceChecks[serviceInstanceID], check)
		} else {
			nodeID := structs.NodeNameString(check.Node, &check.EnterpriseMeta)
			nodeChecks[nodeID] = append(nodeChecks[nodeID], check)
		}
	}

	// Compute the service instance summaries by taking the unhealthiest check for
	// a given service instance as its health status and totaling the counts for each
	// partition/ns/service combination.
	serviceSummaries := make(map[string]structs.HealthSummary)
	for _, svc := range catalog.Services {
		sid := structs.NewServiceID(svc.ServiceName, &svc.EnterpriseMeta)
		summary, ok := serviceSummaries[sid.String()]
		if !ok {
			summary = structs.HealthSummary{
				Name:           svc.ServiceName,
				EnterpriseMeta: svc.EnterpriseMeta,
			}
		}

		// Compute whether this service instance is healthy based on its associated checks.
		serviceInstanceID := entMetaIDString(fmt.Sprintf("%s/%s", svc.Node, svc.ServiceID), svc.EnterpriseMeta)
		status := api.HealthPassing
		for _, checks := range serviceInstanceChecks[serviceInstanceID] {
			if checks.Status == api.HealthWarning && status == api.HealthPassing {
				status = api.HealthWarning
			}
			if checks.Status == api.HealthCritical {
				status = api.HealthCritical
			}
		}

		summary.Add(status)
		serviceSummaries[sid.String()] = summary
	}

	// Compute the node summaries by taking the unhealthiest check for each node
	// as its health status and totaling the passing/warning/critical counts for
	// each partition.
	nodeSummaries := make(map[string]structs.HealthSummary)
	for _, node := range catalog.Nodes {
		summary, ok := nodeSummaries[node.Partition]
		if !ok {
			summary = structs.HealthSummary{
				EnterpriseMeta: *structs.NodeEnterpriseMetaInPartition(node.Partition),
			}
		}

		// Compute whether this node is healthy based on its associated checks.
		status := api.HealthPassing
		nodeID := structs.NodeNameString(node.Node, structs.NodeEnterpriseMetaInPartition(node.Partition))
		for _, checks := range nodeChecks[nodeID] {
			if checks.Status == api.HealthWarning && status == api.HealthPassing {
				status = api.HealthWarning
			}
			if checks.Status == api.HealthCritical {
				status = api.HealthCritical
			}
		}

		summary.Add(status)
		nodeSummaries[node.Partition] = summary
	}

	// Construct the summary.
	summary := &structs.CatalogSummary{}
	for _, healthSummary := range nodeSummaries {
		summary.Nodes = append(summary.Nodes, healthSummary)
	}
	for _, healthSummary := range serviceSummaries {
		summary.Services = append(summary.Services, healthSummary)
	}
	for _, healthSummary := range checkSummaries {
		summary.Checks = append(summary.Checks, healthSummary)
	}

	summarySort := func(slice []structs.HealthSummary) func(int, int) bool {
		return func(i, j int) bool {
			if slice[i].PartitionOrEmpty() < slice[j].PartitionOrEmpty() {
				return true
			}
			if slice[i].NamespaceOrEmpty() < slice[j].NamespaceOrEmpty() {
				return true
			}
			return slice[i].Name < slice[j].Name
		}
	}
	sort.Slice(summary.Nodes, summarySort(summary.Nodes))
	sort.Slice(summary.Services, summarySort(summary.Services))
	sort.Slice(summary.Checks, summarySort(summary.Checks))

	return summary
}
