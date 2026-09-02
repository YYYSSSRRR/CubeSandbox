// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0
//
// Package metrics provides enhanced metrics collection for CubeSandbox scheduler.

package metrics

import (
	"context"
	"time"
)

// MetricType identifies a kind of metric.
type MetricType string

const (
	// Cluster metrics.
	MetricClusterUtilization MetricType = "cluster_utilization"

	// Node metrics.
	MetricNodeBalance MetricType = "node_balance"

	// Scheduling metrics.
	MetricScheduleSuccess MetricType = "schedule_success"
	MetricScheduleLatency MetricType = "schedule_latency"

	// Template metrics.
	MetricTemplateCacheHit MetricType = "template_cache_hit"

	// Queue metrics.
	MetricQueueMetrics MetricType = "queue_metrics"
)

// MetricData is one collection result produced by a collector.
type MetricData struct {
	Type      MetricType        `json:"type"`
	Value     interface{}       `json:"value"`
	Timestamp int64             `json:"timestamp"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// MetricCollector is the interface every collector implements. Implementing it
// and calling RegisterCollector is all that is needed to hook in a metric.
type MetricCollector interface {
	// Collect takes one sample. When there is nothing to collect (or the
	// collector is disabled) it should return an empty MetricData{} (empty
	// Type) with a nil error.
	Collect(ctx context.Context) (MetricData, error)

	// GetInterval returns this collector's collection interval; a value <= 0
	// falls back to MetricsConfig.CollectInterval.
	GetInterval() time.Duration

	// GetName returns the metric name.
	GetName() string

	// IsEnabled reports whether the collector is currently enabled.
	IsEnabled() bool
}

// MetricPoint is the smallest reportable unit: one scalar value. Value is a
// float64; its unit is implied by Name (see each collector's docs).
type MetricPoint struct {
	Name      string
	Value     float64
	Timestamp int64
	Tags      map[string]string
}

// MetricReporter sinks MetricPoints.
type MetricReporter interface {
	Report(points []MetricPoint) error
}

// ClusterUtilization is the cluster packing rate (CPU/memory quota utilization).
type ClusterUtilization struct {
	CPUUtilization    float64 `json:"cpu_utilization"`    // percent 0-100
	MemoryUtilization float64 `json:"memory_utilization"` // percent 0-100
	TotalNodes        int64   `json:"total_nodes"`
	ActiveNodes       int64   `json:"active_nodes"`
}

// NodeBalance is the node load balance.
type NodeBalance struct {
	CPUStdDev float64 `json:"cpu_std_dev"` // std dev of per-node CPU usage rate (%)
	MemStdDev float64 `json:"mem_std_dev"` // std dev of per-node memory usage rate (%)
	NodeGini  float64 `json:"node_gini"`   // Gini coefficient of CPU usage, 0-1
	NumNodes  int64   `json:"num_nodes"`
}

// ScheduleSuccess is the schedule success rate.
type ScheduleSuccess struct {
	TotalAttempts  int64   `json:"total_attempts"`
	SuccessCount   int64   `json:"success_count"`
	FailCount      int64   `json:"fail_count"`
	SuccessRate    float64 `json:"success_rate"`     // percent 0-100
	FilterFailRate float64 `json:"filter_fail_rate"` // percent 0-100
	ScoreFailRate  float64 `json:"score_fail_rate"`  // percent 0-100
}

// CreateLatency is the sandbox create latency, in milliseconds.
type CreateLatency struct {
	P50         float64 `json:"p50"` // milliseconds
	P95         float64 `json:"p95"` // milliseconds
	P99         float64 `json:"p99"` // milliseconds
	Avg         float64 `json:"avg"` // milliseconds
	Max         float64 `json:"max"` // milliseconds
	Min         float64 `json:"min"` // milliseconds
	SampleCount int64   `json:"sample_count"`
}

// TemplateCache is the template cache hit statistics.
type TemplateCache struct {
	TotalRequests  int64   `json:"total_requests"`
	LocalHitCount  int64   `json:"local_hit_count"`
	RemoteHitCount int64   `json:"remote_hit_count"`
	CacheHitRate   float64 `json:"cache_hit_rate"` // percent 0-100
	LocalHitRate   float64 `json:"local_hit_rate"` // percent 0-100
}
