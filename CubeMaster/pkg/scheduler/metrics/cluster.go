// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/node"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/localcache"
)

// nodeSource returns a snapshot of healthy nodes. Production uses localcache;
// tests and the benchmark simulator can override it.
type nodeSource func() node.NodeList

// ClusterUtilizationCollector collects the cluster packing rate, i.e. the
// aggregated CPU/memory quota utilization across all healthy nodes.
type ClusterUtilizationCollector struct {
	config ClusterUtilizationConfig
	src    nodeSource
}

// NewClusterUtilizationCollector creates a cluster utilization collector.
func NewClusterUtilizationCollector(config ClusterUtilizationConfig) *ClusterUtilizationCollector {
	return &ClusterUtilizationCollector{config: config}
}

// SetNodeSource overrides the node source. When nil, localcache.GetHealthyNodes(-1) is used.
func (c *ClusterUtilizationCollector) SetNodeSource(src func() node.NodeList) {
	c.src = src
}

func (c *ClusterUtilizationCollector) healthyNodes() node.NodeList {
	if c.src != nil {
		return c.src()
	}
	return localcache.GetHealthyNodes(-1)
}

// Collect gathers the current cluster packing-rate metrics.
func (c *ClusterUtilizationCollector) Collect(_ context.Context) (MetricData, error) {
	if !c.config.Enabled {
		return MetricData{}, nil
	}

	nodes := c.healthyNodes()

	var totalCpuQuota, totalCpuUsage, totalMemQuota, totalMemUsage int64
	for _, node := range nodes {
		totalCpuQuota += node.QuotaCpu
		totalCpuUsage += node.QuotaCpuUsage
		totalMemQuota += node.QuotaMem
		totalMemUsage += node.QuotaMemUsage
	}

	cpuUtil := percentRatio(totalCpuUsage, totalCpuQuota)
	memUtil := percentRatio(totalMemUsage, totalMemQuota)

	return MetricData{
		Type: MetricClusterUtilization,
		Value: ClusterUtilization{
			CPUUtilization:    cpuUtil,
			MemoryUtilization: memUtil,
			TotalNodes:        int64(nodes.Len()),
			ActiveNodes:       int64(nodes.Len()),
		},
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetInterval returns the collection interval.
func (c *ClusterUtilizationCollector) GetInterval() time.Duration {
	return 10 * time.Second
}

// GetName returns the metric name.
func (c *ClusterUtilizationCollector) GetName() string {
	return "cluster_utilization"
}

// IsEnabled reports whether the collector is enabled.
func (c *ClusterUtilizationCollector) IsEnabled() bool {
	return c.config.Enabled
}

// NodeBalanceCollector collects how evenly load is spread across nodes, i.e.
// the dispersion of per-node CPU/memory usage rates.
type NodeBalanceCollector struct {
	config NodeBalanceConfig
	src    nodeSource
}

// NewNodeBalanceCollector creates a node balance collector.
func NewNodeBalanceCollector(config NodeBalanceConfig) *NodeBalanceCollector {
	return &NodeBalanceCollector{config: config}
}

// SetNodeSource overrides the node source. When nil, localcache.GetHealthyNodes(-1) is used.
func (c *NodeBalanceCollector) SetNodeSource(src func() node.NodeList) {
	c.src = src
}

func (c *NodeBalanceCollector) healthyNodes() node.NodeList {
	if c.src != nil {
		return c.src()
	}
	return localcache.GetHealthyNodes(-1)
}

// Collect gathers the current node-balance metrics.
func (c *NodeBalanceCollector) Collect(_ context.Context) (MetricData, error) {
	if !c.config.Enabled {
		return MetricData{}, nil
	}

	nodes := c.healthyNodes()
	num := nodes.Len()
	if num == 0 {
		return MetricData{
			Type:      MetricNodeBalance,
			Value:     NodeBalance{},
			Timestamp: time.Now().Unix(),
		}, nil
	}

	cpuRates := make([]float64, 0, num)
	memRates := make([]float64, 0, num)
	for _, node := range nodes {
		cpuRates = append(cpuRates, percentRatio(node.QuotaCpuUsage, node.QuotaCpu))
		memRates = append(memRates, percentRatio(node.QuotaMemUsage, node.QuotaMem))
	}

	return MetricData{
		Type: MetricNodeBalance,
		Value: NodeBalance{
			CPUStdDev: sampleStdDev(cpuRates),
			MemStdDev: sampleStdDev(memRates),
			NodeGini:  giniCoefficient(cpuRates),
			NumNodes:  int64(num),
		},
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetInterval returns the collection interval.
func (c *NodeBalanceCollector) GetInterval() time.Duration {
	return 10 * time.Second
}

// GetName returns the metric name.
func (c *NodeBalanceCollector) GetName() string {
	return "node_balance"
}

// IsEnabled reports whether the collector is enabled.
func (c *NodeBalanceCollector) IsEnabled() bool {
	return c.config.Enabled
}

// percentRatio returns usage/quota*100, or 0 when quota <= 0.
func percentRatio(usage, quota int64) float64 {
	if quota <= 0 {
		return 0
	}
	return float64(usage) / float64(quota) * 100
}

// sampleStdDev returns the sample standard deviation (n-1 denominator).
func sampleStdDev(values []float64) float64 {
	n := len(values)
	if n < 2 {
		return 0
	}
	var mean float64
	for _, v := range values {
		mean += v
	}
	mean /= float64(n)

	var variance float64
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(n-1))
}

// giniCoefficient returns the Gini coefficient in [0,1]; 0 means perfectly even.
func giniCoefficient(values []float64) float64 {
	n := len(values)
	if n <= 1 {
		return 0
	}

	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	var totalSum, weightedSum float64
	for i, v := range sorted {
		totalSum += v
		weightedSum += float64(i+1) * v
	}
	if totalSum == 0 {
		return 0
	}

	return (2*weightedSum)/(float64(n)*totalSum) - float64(n+1)/float64(n)
}
