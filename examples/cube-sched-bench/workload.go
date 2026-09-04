// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package main

// workload.go defines the reproducible request generators. A workload turns a
// seeded RNG into the full list of requests for a run (each with its arrival
// tick and lifetime), so every strategy replays the exact same schedule.

import (
	"math/rand/v2"
)

// WorkloadSpec describes one reproducible workload scenario.
type WorkloadSpec struct {
	Name string
	// Schedule returns the requests for the whole run given a seeded RNG.
	Schedule func(r *rand.Rand, ticks, nodes int) []runReq
}

// Request specs. Each request is deliberately small so the interesting signal
// is how strategies place many of them.
const (
	smallCpu = 1
	smallMem = 1024
)

// nodeCPUCores is the per-node CPU quota the generators assume when sizing
// arrival rates to keep the cluster roughly 60-90% loaded.
const nodeCPUCores = 8

func makeReq(cpu, mem int64, template string) Req {
	return Req{Cpu: cpu, Mem: mem, Template: template}
}

func toRun(req Req, life, arrive int) runReq {
	return runReq{req: req, arrive: arrive, life: life}
}

// burstLoad: high-concurrency, short-lived sandboxes. Arrivals come in spikes
// around a busy baseline.
func burstLoad(r *rand.Rand, ticks, nodes int) []runReq {
	out := make([]runReq, 0, ticks*nodes*3)
	base := maxInt(int(float64(nodes*nodeCPUCores)*0.6/3), 1) // ~3-tick avg life
	for t := 0; t < ticks; t++ {
		n := base + r.IntN(3)
		if t%15 == 0 {
			n += base / 2 // periodic burst
		}
		for k := 0; k < n; k++ {
			life := 1 + r.IntN(5)
			tpl := ""
			if r.IntN(2) == 0 {
				tpl = "t1"
			}
			out = append(out, toRun(makeReq(smallCpu, smallMem, tpl), life, t))
		}
	}
	return out
}

// templateHeavyLoad: repeated creation of the same few templates at a sustained
// rate, so local image hits matter most.
func templateHeavyLoad(r *rand.Rand, ticks, nodes int) []runReq {
	templates := []string{"codex", "interpreter", "codex", "codex"} // skewed to codex
	out := make([]runReq, 0, ticks*nodes*3)
	base := maxInt(int(float64(nodes*nodeCPUCores)*0.7/5), 1) // ~5-tick avg life
	for t := 0; t < ticks; t++ {
		n := base + r.IntN(2)
		for k := 0; k < n; k++ {
			life := 2 + r.IntN(6)
			tpl := templates[r.IntN(len(templates))]
			out = append(out, toRun(makeReq(smallCpu, smallMem, tpl), life, t))
		}
	}
	return out
}

// mixedLoad: mixed specs (some long-lived) across several templates.
func mixedLoad(r *rand.Rand, ticks, nodes int) []runReq {
	out := make([]runReq, 0, ticks*nodes*2)
	base := maxInt(int(float64(nodes*nodeCPUCores)*0.7/6), 1)
	for t := 0; t < ticks; t++ {
		n := base + r.IntN(2)
		for k := 0; k < n; k++ {
			life := 2 + r.IntN(4)
			if r.IntN(10) < 3 {
				life += 15 // a few long-lived sessions
			}
			cpu := int64(smallCpu + r.IntN(2))
			mem := int64(smallMem) * int64(1+r.IntN(2))
			tpl := ""
			switch r.IntN(3) {
			case 0:
				tpl = "agent-a"
			case 1:
				tpl = "agent-b"
			}
			out = append(out, toRun(makeReq(cpu, mem, tpl), life, t))
		}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var workloads = map[string]WorkloadSpec{
	"burst":          {Name: "burst", Schedule: burstLoad},
	"template-heavy": {Name: "template-heavy", Schedule: templateHeavyLoad},
	"mixed":          {Name: "mixed", Schedule: mixedLoad},
}
