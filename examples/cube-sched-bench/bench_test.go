// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"
)

func TestDeterministicReproduction(t *testing.T) {
	params := Params{Nodes: 6, Ticks: 60, NodeRes: Res{Cpu: 8, Mem: 8192}}
	spec := workloads["template-heavy"]
	pol := policies()[policyDefault]

	r1 := makeRand(42)
	s1 := simulate(params, spec.Schedule(r1, params.Ticks, params.Nodes), pol)
	r2 := makeRand(42)
	s2 := simulate(params, spec.Schedule(r2, params.Ticks, params.Nodes), pol)

	if s1.Success != s2.Success || s1.Failed != s2.Failed || len(s1.Latencies) != len(s2.Latencies) {
		t.Fatalf("not reproducible: run1 succ=%d fail=%d lat=%d, run2 succ=%d fail=%d lat=%d",
			s1.Success, s1.Failed, len(s1.Latencies), s2.Success, s2.Failed, len(s2.Latencies))
	}
}

// TestTemplateHeavyRewardsImageLocality asserts the built-in template-heavy
// strategy keeps more local hits than the load-only default on the same
// template workload, under cache drift (so hits do not saturate).
func TestTemplateHeavyRewardsImageLocality(t *testing.T) {
	params := Params{Nodes: 6, Ticks: 120, NodeRes: Res{Cpu: 8, Mem: 8192}, TemplateDriftEvery: 8, DriftProb: 0.5}
	spec := workloads["template-heavy"]
	all := policies()

	r := makeRand(7)
	base := simulate(params, spec.Schedule(r, params.Ticks, params.Nodes), all[policyDefault])
	r = makeRand(7)
	heavy := simulate(params, spec.Schedule(r, params.Ticks, params.Nodes), all["agent-template-heavy"])

	if heavy.Hit <= base.Hit {
		t.Errorf("agent-template-heavy hits (%d) not above default (%d) under drift", heavy.Hit, base.Hit)
	}
}
