// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package main

// model.go holds the simulated entities: resources, nodes, requests and the
// parameters shared by every run. It contains no scheduling logic.

// Res is a quota pair (CPU in cores, memory in MB).
type Res struct {
	Cpu int64
	Mem int64
}

// simNode is one schedulable machine in the pool.
type simNode struct {
	id        int
	res       Res
	cpuUsed   int64
	memUsed   int64
	templates map[string]bool
}

// fits reports whether the node has spare capacity for req.
func (n *simNode) fits(req Req) bool {
	return n.res.Cpu-n.cpuUsed >= req.Cpu && n.res.Mem-n.memUsed >= req.Mem
}

// has reports whether the node already caches the template image (always true
// for requests without a template).
func (n *simNode) has(template string) bool {
	if template == "" {
		return true
	}
	return n.templates[template]
}

// cpuUtil returns the node CPU usage ratio clamped to [0,1].
func (n *simNode) cpuUtil() float64 {
	if n.res.Cpu <= 0 {
		return 1
	}
	u := float64(n.cpuUsed) / float64(n.res.Cpu)
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	return u
}

// Req is one sandbox create request.
type Req struct {
	Cpu      int64
	Mem      int64
	Template string
}

// runReq is a scheduled request with its arrival tick and lifetime.
type runReq struct {
	req    Req
	arrive int
	life   int
	placed bool // allocated to a node
	finish int  // tick at which it releases its node
	node   int
	cold   bool // template was not cached on the chosen node
}

// Params controls one simulation run.
type Params struct {
	Nodes   int
	Ticks   int
	NodeRes Res

	// TemplateDriftEvery, when > 0, makes some nodes lose all cached template
	// images every N ticks (simulated node restart / image GC). This stops the
	// hit rate from saturating so that template-locality strategies stay
	// distinguishable. Drift decisions are deterministic and identical for
	// every strategy, so comparisons stay fair.
	TemplateDriftEvery int
	// DriftProb is the probability that one node loses its caches on a drift
	// tick.
	DriftProb float64
}
