// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package main

// simulator.go runs a workload schedule over a node pool under a policy. It
// steps through ticks: each tick releases finished sandboxes, places the newly
// arrived requests using pick(), caches templates, and samples the cluster
// packing rate.

import (
	"math"
	"math/rand/v2"
)

// Latency model (ms). Every templated creation pays the base cost; a creation
// on a node without the image pays the extra pull cost on top. Only templated
// creations contribute to the latency percentiles so the image-locality effect
// is not diluted by template-less requests.
const (
	baseCreateMs = 120.0
	imagePullMs  = 500.0
)

// simulate runs one workload schedule against a policy and returns its stats.
func simulate(params Params, schedule []runReq, pol Policy) Stats {
	nodes := make([]*simNode, params.Nodes)
	for i := range nodes {
		nodes[i] = &simNode{
			id:        i,
			res:       params.NodeRes,
			templates: map[string]bool{},
		}
	}
	// Group requests by arrival tick.
	byTick := make([][]int, params.Ticks)
	for i := range schedule {
		t := schedule[i].arrive
		if t >= 0 && t < params.Ticks {
			byTick[t] = append(byTick[t], i)
		}
	}

	var st Stats
	for tick := 0; tick < params.Ticks; tick++ {
		// Some nodes lose their cached images on a drift tick (deterministic,
		// identical across strategies).
		driftNodeTemplates(nodes, tick, params.TemplateDriftEvery, params.DriftProb)
		// Release finished sandboxes first.
		for i := range schedule {
			r := &schedule[i]
			if r.placed && tick == r.finish {
				n := nodes[r.node]
				n.cpuUsed -= r.req.Cpu
				n.memUsed -= r.req.Mem
				r.placed = false
			}
		}
		// Place the requests that arrive now.
		for _, idx := range byTick[tick] {
			r := &schedule[idx]
			best := pick(nodes, r.req, pol)
			if best == nil {
				st.Failed++
				continue
			}
			r.placed = true
			r.node = best.id
			r.finish = tick + r.life
			best.cpuUsed += r.req.Cpu
			best.memUsed += r.req.Mem
			r.cold = r.req.Template != "" && !best.has(r.req.Template)
			if r.req.Template != "" {
				best.templates[r.req.Template] = true
			}
			st.Success++
			if r.cold {
				st.Miss++
			} else {
				st.Hit++
			}
			if r.req.Template != "" {
				lat := baseCreateMs
				if r.cold {
					lat += imagePullMs // remote image pull dominates the cold path
				}
				st.Latencies = append(st.Latencies, lat)
			}
		}
		// Snapshot aggregate packing.
		var usedC, totalC int64
		for _, n := range nodes {
			usedC += n.cpuUsed
			totalC += n.res.Cpu
		}
		if totalC > 0 {
			st.cpuPack = append(st.cpuPack, float64(usedC)/float64(totalC))
		}
	}
	for _, n := range nodes {
		st.finalUtil = append(st.finalUtil, n.cpuUtil())
	}
	return st
}

// pick selects the node the policy likes best among those with spare capacity.
func pick(nodes []*simNode, req Req, pol Policy) *simNode {
	var best *simNode
	bestScore := math.Inf(-1)
	bestUtil := 1.0
	for _, n := range nodes {
		if n == nil || !n.fits(req) {
			continue
		}
		score := pol.score(n, req)
		u := n.cpuUtil()
		// Prefer higher score; on ties the less loaded node, then the lowest id.
		if best == nil || score > bestScore+1e-12 ||
			(math.Abs(score-bestScore) <= 1e-12 && (u < bestUtil-1e-12 ||
				(math.Abs(u-bestUtil) <= 1e-12 && n.id < best.id))) {
			best = n
			bestScore = score
			bestUtil = u
		}
	}
	return best
}

// driftNodeTemplates invalidates cached template images on a subset of nodes
// every TemplateDriftEvery ticks, modelling node restarts / image GC. The
// decision is a pure function of (node id, drift slot) so it is deterministic
// and identical across strategies — only how a strategy's placements respond
// to the drift differs.
func driftNodeTemplates(nodes []*simNode, tick, every int, prob float64) {
	if every <= 0 || prob <= 0 || tick%every != 0 {
		return
	}
	slot := uint64(tick / every)
	for _, n := range nodes {
		if n == nil {
			continue
		}
		h := uint64(n.id)*73856093 ^ slot*19349663
		if float64(h%10000)/10000 < prob {
			n.templates = map[string]bool{}
		}
	}
}

// makeRand returns a deterministic generator for the given seed so the same
// schedule is replayed for every strategy in a comparison.
func makeRand(seed int64) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed)>>1|1))
}
