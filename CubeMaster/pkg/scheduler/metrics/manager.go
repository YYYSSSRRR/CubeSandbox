// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/constants"
	"github.com/tencentcloud/CubeSandbox/cubelog"
)

// CubeLogReporter reports each MetricPoint as one CubeLog.RequestTrace.
//
// RequestTrace.RetCode is an int64 and cannot hold fractional values, so
// values are encoded at thousandth precision: RetCode = round(value * 1000).
// E.g. utilization 65.3% -> 65300, latency 342.7ms -> 342700 (~us precision).
type CubeLogReporter struct{}

// Report sinks a batch of MetricPoints, one log trace per point.
func (r *CubeLogReporter) Report(points []MetricPoint) error {
	now := time.Now()
	for _, p := range points {
		trace := &CubeLog.RequestTrace{
			Caller: constants.CubeMasterServiceID,
			Callee: "enhanced_metric",
			Action: p.Name,
			// RetCode is an int64; encode the value at thousandth precision.
			RetCode:   int64(math.Round(p.Value * 1000)),
			Timestamp: now,
		}
		if p.Tags != nil {
			if cluster := p.Tags["cluster"]; cluster != "" {
				trace.CalleeCluster = cluster
			}
		}
		CubeLog.Trace(trace)
	}
	return nil
}

// defaultCollectInterval is the fallback interval when GetInterval() <= 0.
const defaultCollectInterval = 10 * time.Second

// MetricsManager orchestrates metric collectors.
//
// Collection model: each registered collector runs its own goroutine, sampling
// at its own GetInterval() and reporting immediately after each sample (no
// storage layer). A collector failure never affects the others.
type MetricsManager struct {
	collectors map[MetricType]MetricCollector
	reporter   MetricReporter
	config     *MetricsConfig

	mu       sync.RWMutex // guards concurrent access to collectors
	wg       sync.WaitGroup
	stopChan chan struct{}
	stopOnce sync.Once
}

// NewMetricsManager creates a metrics manager; a nil config falls back to the
// default configuration.
func NewMetricsManager(config *MetricsConfig) *MetricsManager {
	if config == nil {
		config = DefaultMetricsConfig()
	}
	return &MetricsManager{
		collectors: make(map[MetricType]MetricCollector),
		reporter:   &CubeLogReporter{},
		config:     config,
		stopChan:   make(chan struct{}),
	}
}

// RegisterCollector registers a collector. It must be called before Start;
// registering the same type twice returns an error.
func (m *MetricsManager) RegisterCollector(metricType MetricType, collector MetricCollector) error {
	if collector == nil {
		return fmt.Errorf("metrics: nil collector for %s", metricType)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, dup := m.collectors[metricType]; dup {
		return fmt.Errorf("metrics: collector %s already registered", metricType)
	}
	m.collectors[metricType] = collector
	return nil
}

// Start registers the built-in collectors and starts one sampling goroutine
// per registered collector.
func (m *MetricsManager) Start() {
	m.registerBuiltinCollectors()

	m.mu.RLock()
	collectors := make([]MetricCollector, 0, len(m.collectors))
	for _, c := range m.collectors {
		collectors = append(collectors, c)
	}
	m.mu.RUnlock()

	for _, c := range collectors {
		m.startCollector(c)
	}
}

// Stop stops all collectors. It is safe to call multiple times.
func (m *MetricsManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
		m.wg.Wait()
	})
}

// SetQueueStatsFunc injects the queue stats function into the queue collector
// (must be called after Start so the built-in collectors are registered).
func (m *MetricsManager) SetQueueStatsFunc(fn QueueStatsFunc) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if c, ok := m.collectors[MetricQueueMetrics].(*QueueMetricsCollector); ok {
		c.SetStatsFunc(fn)
	}
}

// startCollector starts the periodic sampling goroutine for one collector.
func (m *MetricsManager) startCollector(c MetricCollector) {
	interval := c.GetInterval()
	if interval <= 0 {
		interval = m.config.CollectInterval
	}
	if interval <= 0 {
		interval = defaultCollectInterval
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collectOnce(c)
			case <-m.stopChan:
				return
			}
		}
	}()
}

// collectOnce samples one collector and reports the result.
func (m *MetricsManager) collectOnce(c MetricCollector) {
	data, err := c.Collect(context.Background())
	if err != nil {
		CubeLog.WithContext(context.Background()).Errorf("metrics collect %s error: %v", c.GetName(), err)
		return
	}
	if data.Type == "" {
		return // nothing to collect (disabled or no samples), skip
	}

	points := flattenMetrics(data)
	if len(points) == 0 {
		return
	}
	if err := m.reporter.Report(points); err != nil {
		CubeLog.WithContext(context.Background()).Errorf("metrics report %s error: %v", data.Type, err)
	}
}

// registerBuiltinCollectors registers the built-in collectors whose config
// Enabled field is true.
func (m *MetricsManager) registerBuiltinCollectors() {
	if m.config.ClusterUtilization.Enabled {
		_ = m.RegisterCollector(MetricClusterUtilization,
			NewClusterUtilizationCollector(m.config.ClusterUtilization))
	}
	if m.config.NodeBalance.Enabled {
		_ = m.RegisterCollector(MetricNodeBalance,
			NewNodeBalanceCollector(m.config.NodeBalance))
	}
	if m.config.ScheduleSuccess.Enabled {
		_ = m.RegisterCollector(MetricScheduleSuccess,
			NewScheduleSuccessCollector(m.config.ScheduleSuccess))
	}
	if m.config.ScheduleLatency.Enabled {
		_ = m.RegisterCollector(MetricScheduleLatency,
			NewCreateLatencyCollector(m.config.ScheduleLatency))
	}
	if m.config.TemplateCache.Enabled {
		_ = m.RegisterCollector(MetricTemplateCacheHit,
			NewTemplateCacheCollector(m.config.TemplateCache))
	}
	if m.config.QueueMetrics.Enabled {
		_ = m.RegisterCollector(MetricQueueMetrics,
			NewQueueMetricsCollector(m.config.QueueMetrics))
	}
}

// ---- Convenience hooks that drive the event-based collectors ----

// collector returns the registered collector of a type, or nil when disabled.
func (m *MetricsManager) collector(t MetricType) MetricCollector {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.collectors[t]
}

// RecordScheduleSuccess records one successful scheduling attempt.
func (m *MetricsManager) RecordScheduleSuccess() {
	if c, ok := m.collector(MetricScheduleSuccess).(*ScheduleSuccessCollector); ok {
		c.RecordSuccess()
	}
}

// RecordScheduleFailure records one failed scheduling attempt
// (failureType: "filter" | "score" | other).
func (m *MetricsManager) RecordScheduleFailure(failureType string) {
	if c, ok := m.collector(MetricScheduleSuccess).(*ScheduleSuccessCollector); ok {
		c.RecordFailure(failureType)
	}
}

// RecordCreateLatency records one sandbox create latency.
func (m *MetricsManager) RecordCreateLatency(latency time.Duration) {
	if c, ok := m.collector(MetricScheduleLatency).(*CreateLatencyCollector); ok {
		c.RecordLatency(latency)
	}
}

// RecordCacheHit records one template cache hit.
func (m *MetricsManager) RecordCacheHit(local bool, templateID string) {
	if c, ok := m.collector(MetricTemplateCacheHit).(*TemplateCacheCollector); ok {
		c.RecordCacheHit(local, templateID)
	}
}
