package hoststats

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// Collector collects host resource usage stats
type Collector struct {
	numCores      int
	cpuCalculator map[string]*cpuStatsCalculator
	hostStats     *HostStats
	hostStatsLock sync.RWMutex
	dataDir       string

	metrics    Metrics
	baseLabels []metrics.Label

	logger hclog.Logger
}

// NewCollector returns a Collector. The dataDir is passed in
// so that we can present the disk related statistics for the mountpoint where the dataDir exists
func NewCollector(ctx context.Context, logger hclog.Logger, dataDir string, opts ...CollectorOption) *Collector {
	logger = logger.Named("host_stats")
	collector := initCollector(logger, dataDir)
	go collector.loop(ctx)
	return collector
}

// initCollector initializes the Collector but does not start the collection loop
func initCollector(logger hclog.Logger, dataDir string, opts ...CollectorOption) *Collector {
	numCores := runtime.NumCPU()
	statsCalculator := make(map[string]*cpuStatsCalculator)
	collector := &Collector{
		cpuCalculator: statsCalculator,
		numCores:      numCores,
		logger:        logger,
		dataDir:       dataDir,
	}

	for _, opt := range opts {
		opt(collector)
	}

	if collector.metrics == nil {
		collector.metrics = metrics.Default()
	}
	return collector
}

func (h *Collector) loop(ctx context.Context) {
	// Start collecting host stats right away and then keep collecting every
	// collection interval
	next := time.NewTimer(0)
	defer next.Stop()
	for {
		select {
		case <-next.C:
			h.collect()
			next.Reset(hostStatsCollectionInterval)
			h.Stats().Emit(h.metrics, h.baseLabels)

		case <-ctx.Done():
			return
		}
	}
}

// collect will collect stats related to resource usage of the host
func (h *Collector) collect() {
	h.hostStatsLock.Lock()
	defer h.hostStatsLock.Unlock()
	hs := &HostStats{Timestamp: time.Now().UTC().UnixNano()}

	// Determine up-time
	uptime, err := host.Uptime()
	if err != nil {
		h.logger.Error("failed to collect uptime stats", "error", err)
		uptime = 0
	}
	hs.Uptime = uptime

	// Collect memory stats
	mstats, err := h.collectMemoryStats()
	if err != nil {
		h.logger.Error("failed to collect memory stats", "error", err)
		mstats = &MemoryStats{}
	}
	hs.Memory = mstats

	// Collect cpu stats
	cpus, err := h.collectCPUStats()
	if err != nil {
		h.logger.Error("failed to collect cpu stats", "error", err)
		cpus = []*CPUStats{}
	}
	hs.CPU = cpus

	// Collect disk stats
	diskStats, err := h.collectDiskStats(h.dataDir)
	if err != nil {
		h.logger.Error("failed to collect dataDir disk stats", "error", err)
	}
	hs.DataDirStats = diskStats

	// Update the collected status object.
	h.hostStats = hs
}

func (h *Collector) collectDiskStats(dir string) (*DiskStats, error) {
	usage, err := disk.Usage(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to collect disk usage stats: %w", err)
	}
	return h.toDiskStats(usage), nil
}

func (h *Collector) collectMemoryStats() (*MemoryStats, error) {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	mem := &MemoryStats{
		Total:       memStats.Total,
		Available:   memStats.Available,
		Used:        memStats.Used,
		UsedPercent: memStats.UsedPercent,
		Free:        memStats.Free,
	}

	return mem, nil
}

// Stats returns the host stats that has been collected
func (h *Collector) Stats() *HostStats {
	h.hostStatsLock.RLock()
	defer h.hostStatsLock.RUnlock()

	if h.hostStats == nil {
		return &HostStats{}
	}

	return h.hostStats.Clone()
}

// toDiskStats merges UsageStat and PartitionStat to create a DiskStat
func (h *Collector) toDiskStats(usage *disk.UsageStat) *DiskStats {
	ds := DiskStats{
		Size:              usage.Total,
		Used:              usage.Used,
		Available:         usage.Free,
		UsedPercent:       usage.UsedPercent,
		InodesUsedPercent: usage.InodesUsedPercent,
		Path:              usage.Path,
	}
	if math.IsNaN(ds.UsedPercent) {
		ds.UsedPercent = 0.0
	}
	if math.IsNaN(ds.InodesUsedPercent) {
		ds.InodesUsedPercent = 0.0
	}

	return &ds
}

type CollectorOption func(c *Collector)

func WithMetrics(m *metrics.Metrics) CollectorOption {
	return func(c *Collector) {
		c.metrics = m
	}
}

func WithBaseLabels(labels []metrics.Label) CollectorOption {
	return func(c *Collector) {
		c.baseLabels = labels
	}
}
