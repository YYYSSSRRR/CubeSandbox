// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"time"
)

// MetricsConfig configures the enhanced metrics collection.
// Each built-in collector is enabled through its own Enabled field; custom
// collectors register via RegisterCollector and control IsEnabled() themselves.
type MetricsConfig struct {
	// CollectInterval is the fallback interval when a collector's GetInterval() <= 0.
	CollectInterval time.Duration `yaml:"collect_interval"`

	// Built-in collector configs.
	ClusterUtilization ClusterUtilizationConfig `yaml:"cluster_utilization"`
	NodeBalance        NodeBalanceConfig        `yaml:"node_balance"`
	ScheduleSuccess    ScheduleSuccessConfig    `yaml:"schedule_success"`
	ScheduleLatency    ScheduleLatencyConfig    `yaml:"schedule_latency"`
	TemplateCache      TemplateCacheConfig      `yaml:"template_cache"`
	QueueMetrics       QueueMetricsConfig       `yaml:"queue_metrics"`
}

// ClusterUtilizationConfig configures the cluster utilization collector.
type ClusterUtilizationConfig struct {
	Enabled bool `yaml:"enabled"`
}

// NodeBalanceConfig configures the node balance collector.
type NodeBalanceConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ScheduleSuccessConfig configures the schedule success collector.
type ScheduleSuccessConfig struct {
	Enabled     bool          `yaml:"enabled"`
	ResetPeriod time.Duration `yaml:"reset_period"` // stats reset period
}

// ScheduleLatencyConfig configures the create latency collector.
type ScheduleLatencyConfig struct {
	Enabled    bool `yaml:"enabled"`
	SampleSize int  `yaml:"sample_size"` // ring-buffer size
}

// TemplateCacheConfig configures the template cache collector.
type TemplateCacheConfig struct {
	Enabled     bool          `yaml:"enabled"`
	ResetPeriod time.Duration `yaml:"reset_period"` // stats reset period
}

// QueueMetricsConfig configures the queue metrics collector.
type QueueMetricsConfig struct {
	Enabled bool `yaml:"enabled"`
}

// DefaultMetricsConfig returns the default configuration with every built-in
// collector enabled.
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		CollectInterval: 10 * time.Second,
		ClusterUtilization: ClusterUtilizationConfig{
			Enabled: true,
		},
		NodeBalance: NodeBalanceConfig{
			Enabled: true,
		},
		ScheduleSuccess: ScheduleSuccessConfig{
			Enabled:     true,
			ResetPeriod: 24 * time.Hour,
		},
		ScheduleLatency: ScheduleLatencyConfig{
			Enabled:    true,
			SampleSize: 10000,
		},
		TemplateCache: TemplateCacheConfig{
			Enabled:     true,
			ResetPeriod: 24 * time.Hour,
		},
		QueueMetrics: QueueMetricsConfig{
			Enabled: true,
		},
	}
}
