// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"slices"
	"sync"
	"time"
)

// ScheduleSuccessCollector measures the schedule success rate. It is driven by
// the Manager's RecordScheduleSuccess / RecordScheduleFailure and reports the
// counters of the current window on Collect.
type ScheduleSuccessCollector struct {
	config    ScheduleSuccessConfig
	mu        sync.Mutex
	stats     ScheduleStats
	lastReset time.Time
}

// ScheduleStats holds the schedule counters for one window.
type ScheduleStats struct {
	TotalAttempts   int64
	SuccessCount    int64
	FilterFailCount int64
	ScoreFailCount  int64
}

// NewScheduleSuccessCollector creates a schedule success collector.
func NewScheduleSuccessCollector(config ScheduleSuccessConfig) *ScheduleSuccessCollector {
	return &ScheduleSuccessCollector{
		config:    config,
		lastReset: time.Now(),
	}
}

// RecordSuccess records one successful scheduling attempt.
func (c *ScheduleSuccessCollector) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.SuccessCount++
	c.stats.TotalAttempts++
}

// RecordFailure records one failed attempt (failureType: "filter" | "score" | other).
func (c *ScheduleSuccessCollector) RecordFailure(failureType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.TotalAttempts++
	switch failureType {
	case "filter":
		c.stats.FilterFailCount++
	case "score":
		c.stats.ScoreFailCount++
	}
}

// Collect outputs the schedule success metrics.
func (c *ScheduleSuccessCollector) Collect(_ context.Context) (MetricData, error) {
	if !c.config.Enabled {
		return MetricData{}, nil
	}

	// A plain mutex keeps reset and read on the same lock, avoiding any
	// read-lock-then-write race.
	c.mu.Lock()
	defer c.mu.Unlock()

	if resetPeriod := c.config.ResetPeriod; resetPeriod > 0 && time.Since(c.lastReset) > resetPeriod {
		c.stats = ScheduleStats{}
		c.lastReset = time.Now()
	}

	successRate := percent(c.stats.SuccessCount, c.stats.TotalAttempts)
	filterFailRate := percent(c.stats.FilterFailCount, c.stats.TotalAttempts)
	scoreFailRate := percent(c.stats.ScoreFailCount, c.stats.TotalAttempts)

	return MetricData{
		Type: MetricScheduleSuccess,
		Value: ScheduleSuccess{
			TotalAttempts:  c.stats.TotalAttempts,
			SuccessCount:   c.stats.SuccessCount,
			FailCount:      c.stats.TotalAttempts - c.stats.SuccessCount,
			SuccessRate:    successRate,
			FilterFailRate: filterFailRate,
			ScoreFailRate:  scoreFailRate,
		},
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetInterval returns the collection interval.
func (c *ScheduleSuccessCollector) GetInterval() time.Duration {
	return 30 * time.Second
}

// GetName returns the metric name.
func (c *ScheduleSuccessCollector) GetName() string {
	return "schedule_success"
}

// IsEnabled reports whether the collector is enabled.
func (c *ScheduleSuccessCollector) IsEnabled() bool {
	return c.config.Enabled
}

// CreateLatencyCollector measures sandbox create latency (P50/P95/P99, in ms).
type CreateLatencyCollector struct {
	config ScheduleLatencyConfig
	buffer *LatencyRingBuffer
}

// NewCreateLatencyCollector creates a create latency collector.
func NewCreateLatencyCollector(config ScheduleLatencyConfig) *CreateLatencyCollector {
	return &CreateLatencyCollector{
		config: config,
		buffer: NewLatencyRingBuffer(config.SampleSize),
	}
}

// RecordLatency records one create latency.
func (c *CreateLatencyCollector) RecordLatency(latency time.Duration) {
	c.buffer.Add(latency)
}

// Collect outputs the latency percentile and stats metrics (milliseconds).
func (c *CreateLatencyCollector) Collect(_ context.Context) (MetricData, error) {
	if !c.config.Enabled {
		return MetricData{}, nil
	}

	summary := c.buffer.Snapshot()
	if summary.Count == 0 {
		return MetricData{}, nil // no samples yet, skip this report
	}

	return MetricData{
		Type: MetricScheduleLatency,
		Value: CreateLatency{
			P50:         summary.P50Ms,
			P95:         summary.P95Ms,
			P99:         summary.P99Ms,
			Avg:         summary.AvgMs,
			Max:         summary.MaxMs,
			Min:         summary.MinMs,
			SampleCount: summary.Count,
		},
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetInterval returns the collection interval.
func (c *CreateLatencyCollector) GetInterval() time.Duration {
	return 30 * time.Second
}

// GetName returns the metric name.
func (c *CreateLatencyCollector) GetName() string {
	return "schedule_latency"
}

// IsEnabled reports whether the collector is enabled.
func (c *CreateLatencyCollector) IsEnabled() bool {
	return c.config.Enabled
}

// LatencySummary is a latency snapshot in milliseconds.
type LatencySummary struct {
	Count int64
	AvgMs float64
	MaxMs float64
	MinMs float64
	P50Ms float64
	P95Ms float64
	P99Ms float64
}

// LatencyRingBuffer is a thread-safe ring buffer of latency samples.
type LatencyRingBuffer struct {
	mu      sync.Mutex
	size    int
	samples []time.Duration
	index   int
	count   int
}

// NewLatencyRingBuffer creates a latency ring buffer.
func NewLatencyRingBuffer(size int) *LatencyRingBuffer {
	if size <= 0 {
		size = 10000
	}
	return &LatencyRingBuffer{
		size:    size,
		samples: make([]time.Duration, size),
	}
}

// Add appends one latency sample.
func (r *LatencyRingBuffer) Add(latency time.Duration) {
	if latency <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.samples[r.index] = latency
	r.index = (r.index + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

// Snapshot copies the current samples and computes stats in milliseconds.
// Count is 0 when the buffer is empty.
func (r *LatencyRingBuffer) Snapshot() LatencySummary {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count == 0 {
		return LatencySummary{}
	}

	copied := make([]time.Duration, r.count)
	copy(copied, r.samples[:r.count])
	slices.Sort(copied)

	var total time.Duration
	minDur, maxDur := copied[0], copied[0]
	for _, d := range copied {
		total += d
		if d < minDur {
			minDur = d
		}
		if d > maxDur {
			maxDur = d
		}
	}

	return LatencySummary{
		Count: int64(r.count),
		AvgMs: float64(total) / float64(r.count) / float64(time.Millisecond),
		MaxMs: float64(maxDur) / float64(time.Millisecond),
		MinMs: float64(minDur) / float64(time.Millisecond),
		P50Ms: quantileMs(copied, 0.50),
		P95Ms: quantileMs(copied, 0.95),
		P99Ms: quantileMs(copied, 0.99),
	}
}

// quantileMs returns the q-quantile of a sorted sample set, in milliseconds.
func quantileMs(sorted []time.Duration, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(q*float64(len(sorted))) - 1 // lower-side quantile
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return float64(sorted[idx]) / float64(time.Millisecond)
}

// percent returns part/total*100, or 0 when total <= 0.
func percent(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
