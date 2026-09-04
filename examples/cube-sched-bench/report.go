// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package main

// report.go prints one workload's results as a table, plus each strategy's
// delta relative to the default pipeline.

import (
	"fmt"
	"sort"
)

// printReport runs each strategy over the same seeded schedule and prints a
// quantifiable comparison.
func printReport(workload string, params Params, spec WorkloadSpec, strategies []Policy, seed int64) {
	fmt.Println("strategy                       succ%   pack%   balance stddev   hit%    p50(ms)  p95(ms)")
	rows := make(map[string]Stats)
	for _, pol := range strategies {
		r := makeRand(seed)
		schedule := spec.Schedule(r, params.Ticks, params.Nodes)
		st := simulate(params, schedule, pol)
		rows[pol.Name] = st
		fmt.Printf("%-28s  %5.1f   %5.1f      %5.2f     %5.1f   %7.1f  %7.1f\n",
			pol.Name, st.successRate(), st.avgPack()*100, st.stddev(), st.hitRate(), st.p50(), st.p95())
	}

	base, ok := rows[policyDefault]
	if !ok {
		return
	}
	fmt.Println("\nvs default:")
	names := make([]string, 0, len(rows))
	for name := range rows {
		if name != policyDefault {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		cur := rows[name]
		fmt.Printf("%-26s succ %+5.1fpp   pack %+5.1fpp   hit %+6.1fpp   p95 %+7.1fms\n",
			name,
			cur.successRate()-base.successRate(),
			(cur.avgPack()-base.avgPack())*100,
			cur.hitRate()-base.hitRate(),
			cur.p95()-base.p95())
	}
}
