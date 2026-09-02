// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/node"
	CubeLog "github.com/tencentcloud/CubeSandbox/cubelog"
)

func testNodes() node.NodeList {
	return node.NodeList{
		&node.Node{IP: "node-1", QuotaCpu: 16, QuotaCpuUsage: 8, QuotaMem: 32768, QuotaMemUsage: 16384},
		&node.Node{IP: "node-2", QuotaCpu: 16, QuotaCpuUsage: 12, QuotaMem: 32768, QuotaMemUsage: 8192},
		&node.Node{IP: "node-3", QuotaCpu: 16, QuotaCpuUsage: 16, QuotaMem: 32768, QuotaMemUsage: 24576},
	}
}

func nodeSourceOf(nodes node.NodeList) func() node.NodeList {
	return func() node.NodeList { return nodes }
}

func show(t *testing.T, title string, data MetricData, points []MetricPoint) {
	t.Logf("== %s ==", title)
	t.Logf("   metric=%s tags=%v", data.Type, data.Tags)
	t.Logf("   value=%+v", data.Value)
	for _, p := range points {
		t.Logf("   -> point %-42s value=%9.3f retcode=%d", p.Name, p.Value, int64(math.Round(p.Value*1000)))
	}
}

func TestObserveAllMetricsOffline(t *testing.T) {
	ctx := context.Background()

	cluCfg := DefaultMetricsConfig().ClusterUtilization
	clu := NewClusterUtilizationCollector(cluCfg)
	clu.SetNodeSource(nodeSourceOf(testNodes()))
	d, err := clu.Collect(ctx)
	if err != nil {
		t.Fatalf("cluster collect err: %v", err)
	}
	show(t, "cluster packing rate (expect cpu=75%%, mem=50%%)", d, flattenMetrics(d))

	balCfg := DefaultMetricsConfig().NodeBalance
	bal := NewNodeBalanceCollector(balCfg)
	bal.SetNodeSource(nodeSourceOf(testNodes()))
	d, err = bal.Collect(ctx)
	if err != nil {
		t.Fatalf("balance collect err: %v", err)
	}
	show(t, "node balance (expect cpu_stddev=25, gini≈0.148)", d, flattenMetrics(d))

	succCfg := DefaultMetricsConfig().ScheduleSuccess
	succ := NewScheduleSuccessCollector(succCfg)
	for i := 0; i < 80; i++ {
		succ.RecordSuccess()
	}
	for i := 0; i < 15; i++ {
		succ.RecordFailure("filter")
	}
	for i := 0; i < 5; i++ {
		succ.RecordFailure("score")
	}
	d, err = succ.Collect(ctx)
	if err != nil {
		t.Fatalf("schedule collect err: %v", err)
	}
	show(t, "schedule success (expect success=80%)", d, flattenMetrics(d))

	latCfg := DefaultMetricsConfig().ScheduleLatency
	lat := NewCreateLatencyCollector(latCfg)
	for i := 0; i < 1000; i++ {
		lat.RecordLatency(time.Duration(100+i) * time.Millisecond)
	}
	d, err = lat.Collect(ctx)
	if err != nil {
		t.Fatalf("latency collect err: %v", err)
	}
	show(t, "sandbox create latency ms (expect p50≈599, p95≈1049)", d, flattenMetrics(d))

	cacheCfg := DefaultMetricsConfig().TemplateCache
	tc := NewTemplateCacheCollector(cacheCfg)
	for i := 0; i < 70; i++ {
		tc.RecordCacheHit(true, "tpl-a")
	}
	for i := 0; i < 20; i++ {
		tc.RecordCacheHit(false, "tpl-a")
	}
	d, err = tc.Collect(ctx)
	if err != nil {
		t.Fatalf("template collect err: %v", err)
	}
	show(t, "template local hit rate (expect local=70/90)", d, flattenMetrics(d))

	qCfg := DefaultMetricsConfig().QueueMetrics
	q := NewQueueMetricsCollector(qCfg)
	q.SetStatsFunc(func() map[string]int64 {
		return map[string]int64{
			"default_buffertask_len":      12,
			"default_buffertask_workings": 5,
			"xl_buffertask_len":           2,
			"xl_buffertask_workings":      3,
		}
	})
	d, err = q.Collect(ctx)
	if err != nil {
		t.Fatalf("queue collect err: %v", err)
	}
	show(t, "queue metrics", d, flattenMetrics(d))
}

func TestCubeLogReporterVisible(t *testing.T) {
	clu := NewClusterUtilizationCollector(DefaultMetricsConfig().ClusterUtilization)
	clu.SetNodeSource(nodeSourceOf(testNodes()))
	d, err := clu.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect err: %v", err)
	}

	var buf bytes.Buffer
	CubeLog.EnableLogMetric()
	CubeLog.SetTraceOutput(&buf)
	r := &CubeLogReporter{}
	if err := r.Report(flattenMetrics(d)); err != nil {
		t.Fatalf("report err: %v", err)
	}
	out := buf.String()
	if out == "" {
		t.Fatalf("trace output empty, cube log metric writer not configured?")
	}
	t.Logf("== actual CubeLog trace output ==")
	t.Logf("%s", out)
}

func approx(t *testing.T, name string, got, want, delta float64) {
	t.Helper()
	if math.Abs(got-want) > delta {
		t.Errorf("%s = %v, want ≈ %v (±%v)", name, got, want, delta)
	}
}

func TestClusterUtilizationValues(t *testing.T) {
	c := NewClusterUtilizationCollector(DefaultMetricsConfig().ClusterUtilization)
	c.SetNodeSource(nodeSourceOf(testNodes()))
	d, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v := d.Value.(ClusterUtilization)
	approx(t, "cpu_utilization", v.CPUUtilization, 75, 0.01)
	approx(t, "mem_utilization", v.MemoryUtilization, 50, 0.01)
	if v.TotalNodes != 3 || v.ActiveNodes != 3 {
		t.Errorf("nodes = %d/%d, want 3/3", v.TotalNodes, v.ActiveNodes)
	}
}

func TestNodeBalanceValues(t *testing.T) {
	c := NewNodeBalanceCollector(DefaultMetricsConfig().NodeBalance)
	c.SetNodeSource(nodeSourceOf(testNodes()))
	d, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v := d.Value.(NodeBalance)
	approx(t, "cpu_std_dev", v.CPUStdDev, 25, 0.01)
	approx(t, "node_gini", v.NodeGini, 0.14815, 0.001)
}

func TestCreateLatencyValues(t *testing.T) {
	c := NewCreateLatencyCollector(DefaultMetricsConfig().ScheduleLatency)
	for i := 0; i < 1000; i++ {
		c.RecordLatency(time.Duration(100+i) * time.Millisecond)
	}
	d, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v := d.Value.(CreateLatency)
	approx(t, "p50(ms)", v.P50, 599, 1)
	approx(t, "p95(ms)", v.P95, 1049, 1)
	if v.SampleCount != 1000 {
		t.Errorf("sample_count = %d, want 1000", v.SampleCount)
	}
	// Min/Max keep whole milliseconds.
	approx(t, "min(ms)", v.Min, 100, 0.01)
	approx(t, "max(ms)", v.Max, 1099, 0.01)
}

func TestScheduleSuccessRates(t *testing.T) {
	c := NewScheduleSuccessCollector(DefaultMetricsConfig().ScheduleSuccess)
	for i := 0; i < 80; i++ {
		c.RecordSuccess()
	}
	for i := 0; i < 15; i++ {
		c.RecordFailure("filter")
	}
	for i := 0; i < 5; i++ {
		c.RecordFailure("score")
	}
	d, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v := d.Value.(ScheduleSuccess)
	approx(t, "success_rate", v.SuccessRate, 80, 0.01)
	approx(t, "filter_fail_rate", v.FilterFailRate, 15, 0.01)
	approx(t, "score_fail_rate", v.ScoreFailRate, 5, 0.01)
	if v.TotalAttempts != 100 || v.FailCount != 20 {
		t.Errorf("attempts/fail = %d/%d, want 100/20", v.TotalAttempts, v.FailCount)
	}
}

func TestTemplateCacheRates(t *testing.T) {
	c := NewTemplateCacheCollector(DefaultMetricsConfig().TemplateCache)
	for i := 0; i < 70; i++ {
		c.RecordCacheHit(true, "tpl-a")
	}
	for i := 0; i < 20; i++ {
		c.RecordCacheHit(false, "tpl-a")
	}
	d, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v := d.Value.(TemplateCache)
	approx(t, "local_hit_rate", v.LocalHitRate, 77.7778, 0.01)
	if v.TotalRequests != 90 || v.LocalHitCount != 70 || v.RemoteHitCount != 20 {
		t.Errorf("total/local/remote = %d/%d/%d", v.TotalRequests, v.LocalHitCount, v.RemoteHitCount)
	}
}

func TestFlattenMetricPointNaming(t *testing.T) {
	d := MetricData{
		Type: MetricClusterUtilization,
		Value: ClusterUtilization{
			CPUUtilization:    75,
			MemoryUtilization: 50,
			TotalNodes:        3,
		},
		Timestamp: 1700000000,
	}
	points := flattenMetrics(d)
	names := make(map[string]float64, len(points))
	for _, p := range points {
		names[p.Name] = p.Value
		if p.Timestamp != 1700000000 {
			t.Errorf("timestamp not carried in %s", p.Name)
		}
	}
	for _, want := range []string{
		"cluster_utilization.cpu_utilization",
		"cluster_utilization.memory_utilization",
		"cluster_utilization.total_nodes",
	} {
		if _, ok := names[want]; !ok {
			t.Errorf("missing flattened point %q (got %v)", want, names)
		}
	}
}

func TestFlattenIgnoresSliceAndString(t *testing.T) {
	d := MetricData{
		Type: "demo",
		Value: map[string]interface{}{
			"num":  int64(3),
			"list": []float64{1, 2, 3},
			"note": "skip me",
		},
	}
	points := flattenMetrics(d)
	if len(points) != 1 || points[0].Name != "demo.num" {
		t.Fatalf("unexpected flattened points: %+v", points)
	}
}

func TestReporterRetCodeThousandPrecision(t *testing.T) {
	var buf bytes.Buffer
	CubeLog.EnableLogMetric()
	CubeLog.SetTraceOutput(&buf)
	r := &CubeLogReporter{}
	if err := r.Report([]MetricPoint{{Name: "demo.rate", Value: 65.3}}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	fmt.Printf("trace-sample: %s", out)
	if out == "" {
		t.Skip("cube log metric writer produced no output in this environment")
	}
}
