// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package main

// metrics.go aggregates one strategy's run into the metric families CubeMaster
// exposes for scheduling, plus the small statistics helpers behind them.

import (
	"math"
	"sort"
)

// Stats aggregates one strategy's run.
type Stats struct {
	Success   int
	Failed    int
	Hit       int // local template hits
	Miss      int // remote template pulls
	Latencies []float64
	cpuPack   []float64 // per-tick aggregate CPU packing rate
	finalUtil []float64 // per-node CPU util at the end
}

func (s *Stats) successRate() float64 {
	if total := s.Success + s.Failed; total > 0 {
		return float64(s.Success) / float64(total) * 100
	}
	return 0
}

func (s *Stats) hitRate() float64 {
	if total := s.Hit + s.Miss; total > 0 {
		return float64(s.Hit) / float64(total) * 100
	}
	return 0
}

func (s *Stats) avgPack() float64 { return mean(s.cpuPack) }
func (s *Stats) stddev() float64  { return stddev(s.finalUtil) }
func (s *Stats) p50() float64     { return quantile(s.Latencies, 0.50) }
func (s *Stats) p95() float64     { return quantile(s.Latencies, 0.95) }

// quantile returns the q-quantile of values in ms; 0 when empty.
func quantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	i := int(q*float64(len(sorted))) - 1
	if i < 0 {
		i = 0
	}
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	var sq float64
	for _, v := range values {
		d := v - m
		sq += d * d
	}
	return math.Sqrt(sq / float64(len(values)-1))
}
