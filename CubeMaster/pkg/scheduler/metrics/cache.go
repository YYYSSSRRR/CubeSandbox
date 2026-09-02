// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"sync"
	"time"
)

// TemplateCacheCollector counts template cache hits. It is driven by the
// Manager's RecordCacheHit.
type TemplateCacheCollector struct {
	config    TemplateCacheConfig
	mu        sync.Mutex
	stats     TemplateCacheStats
	lastReset time.Time
}

// TemplateCacheStats holds the template cache counters.
type TemplateCacheStats struct {
	TotalRequests  int64
	LocalHitCount  int64
	RemoteHitCount int64
}

// NewTemplateCacheCollector creates a template cache collector.
func NewTemplateCacheCollector(config TemplateCacheConfig) *TemplateCacheCollector {
	return &TemplateCacheCollector{
		config:    config,
		lastReset: time.Now(),
	}
}

// RecordCacheHit records one cache hit. local=true means the template image is
// already local to the scheduled node; false means it was fetched remotely.
func (c *TemplateCacheCollector) RecordCacheHit(local bool, templateID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats.TotalRequests++
	if local {
		c.stats.LocalHitCount++
	} else {
		c.stats.RemoteHitCount++
	}
}

// Collect outputs the template cache hit metrics.
func (c *TemplateCacheCollector) Collect(_ context.Context) (MetricData, error) {
	if !c.config.Enabled {
		return MetricData{}, nil
	}

	// Reset and read share the same lock.
	c.mu.Lock()
	defer c.mu.Unlock()

	if resetPeriod := c.config.ResetPeriod; resetPeriod > 0 && time.Since(c.lastReset) > resetPeriod {
		c.stats = TemplateCacheStats{}
		c.lastReset = time.Now()
	}

	if c.stats.TotalRequests == 0 {
		return MetricData{}, nil // no requests yet, skip this report
	}

	totalHits := c.stats.LocalHitCount + c.stats.RemoteHitCount

	return MetricData{
		Type: MetricTemplateCacheHit,
		Value: TemplateCache{
			TotalRequests:  c.stats.TotalRequests,
			LocalHitCount:  c.stats.LocalHitCount,
			RemoteHitCount: c.stats.RemoteHitCount,
			CacheHitRate:   percent(totalHits, c.stats.TotalRequests),
			LocalHitRate:   percent(c.stats.LocalHitCount, c.stats.TotalRequests),
		},
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetInterval returns the collection interval.
func (c *TemplateCacheCollector) GetInterval() time.Duration {
	return 60 * time.Second
}

// GetName returns the metric name.
func (c *TemplateCacheCollector) GetName() string {
	return "template_cache"
}

// IsEnabled reports whether the collector is enabled.
func (c *TemplateCacheCollector) IsEnabled() bool {
	return c.config.Enabled
}

// QueueMetricsCollector collects queue metrics. The real queue state
// (bufferqueue) lives in the scheduler, so it is fetched through an injected
// statsFn; without it Collect returns no data instead of fabricating values.
type QueueMetricsCollector struct {
	config  QueueMetricsConfig
	statsFn QueueStatsFunc
}

// QueueStatsFunc returns a per-instance-type queue metrics snapshot. Keys look
// like "default_buffertask_len" / "default_buffertask_workings", values are
// int64 counters.
type QueueStatsFunc func() map[string]int64

// NewQueueMetricsCollector creates a queue metrics collector.
func NewQueueMetricsCollector(config QueueMetricsConfig) *QueueMetricsCollector {
	return &QueueMetricsCollector{config: config}
}

// SetStatsFunc injects the queue metrics fetch function.
func (c *QueueMetricsCollector) SetStatsFunc(fn QueueStatsFunc) {
	c.statsFn = fn
}

// Collect gathers the queue metrics.
func (c *QueueMetricsCollector) Collect(_ context.Context) (MetricData, error) {
	if !c.config.Enabled || c.statsFn == nil {
		return MetricData{}, nil
	}

	stats := c.statsFn()
	if len(stats) == 0 {
		return MetricData{}, nil
	}

	value := make(map[string]interface{}, len(stats))
	for k, v := range stats {
		value[k] = v
	}

	return MetricData{
		Type:      MetricQueueMetrics,
		Value:     value,
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetInterval returns the collection interval.
func (c *QueueMetricsCollector) GetInterval() time.Duration {
	return 10 * time.Second
}

// GetName returns the metric name.
func (c *QueueMetricsCollector) GetName() string {
	return "queue_metrics"
}

// IsEnabled reports whether the collector is enabled.
func (c *QueueMetricsCollector) IsEnabled() bool {
	return c.config.Enabled
}
