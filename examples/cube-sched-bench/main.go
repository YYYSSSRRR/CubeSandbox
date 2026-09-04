// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		workloadName = flag.String("workload", "burst", "workload to simulate: burst | template-heavy | mixed")
		seed         = flag.Int64("seed", 42, "deterministic seed")
		ticks        = flag.Int("ticks", 200, "simulation ticks")
		nodes        = flag.Int("nodes", 8, "number of nodes in the pool")
		strategies   = flag.String("strategies", "default,agent-burst,agent-template-heavy,agent-mixed", "comma-separated strategies")
		driftEvery   = flag.Int("drift-every", 8, "template cache drift interval in ticks (0 disables)")
		driftProb    = flag.Float64("drift-prob", 0.5, "per-node probability of losing cached images on a drift tick")
	)
	flag.Parse()

	workload, ok := workloads[*workloadName]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown workload %q (have burst, template-heavy, mixed)\n", *workloadName)
		os.Exit(2)
	}

	all := policies()
	sel := make([]Policy, 0, 4)
	for _, name := range parseStrategies(*strategies) {
		p, ok := all[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown strategy %q\n", name)
			os.Exit(2)
		}
		sel = append(sel, p)
	}

	params := Params{
		Nodes:              *nodes,
		Ticks:              *ticks,
		NodeRes:            Res{Cpu: 8, Mem: 8192},
		TemplateDriftEvery: *driftEvery,
		DriftProb:          *driftProb,
	}

	fmt.Printf("workload=%s seed=%d ticks=%d nodes=%d\n", workload.Name, *seed, *ticks, *nodes)
	printReport(workload.Name, params, workload, sel, *seed)
}

// parseStrategies splits a comma-separated strategy list.
func parseStrategies(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if t := trimSpace(s[start:i]); t != "" {
				out = append(out, t)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
