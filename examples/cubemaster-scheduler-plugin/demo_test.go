// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"testing"

	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/config"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/selector/filter"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/selector/score"
)

// Compile-time assertions that both plugins satisfy the standard interfaces.
var (
	_ filter.Selector = (*AgentCapacityFilter)(nil)
	_ score.Selector  = (*AgentBalanceScore)(nil)
)

func TestPluginsRegistered(t *testing.T) {
	if _, ok := filter.Factories()["agent_capacity"]; !ok {
		t.Fatal("agent_capacity not registered in filter registry")
	}
	if _, ok := score.Factories()["agent_balance"]; !ok {
		t.Fatal("agent_balance not registered in score registry")
	}
}

// TestPipelineSeesPlugins shows the plugin names flowing into the strategy
// pipeline the way the scheduler assembles them at startup.
func TestPipelineSeesPlugins(t *testing.T) {
	w := 0.7
	sc := &config.SchedulerConf{
		ActiveProfile: "agents",
		Profiles: map[string]*config.SchedulerProfileConf{
			"agents": {
				Filters: []string{"cpu", "agent_capacity"},
				Scorers: map[string]*config.ScorerOverride{
					"image_score":   {},
					"agent_balance": {Weight: &w},
				},
			},
		},
	}
	p := sc.ResolvePipeline()
	if len(p.Filters) != 2 || p.Filters[1] != "agent_capacity" {
		t.Errorf("filters = %v, want [cpu agent_capacity]", p.Filters)
	}
	found := false
	for _, b := range p.Scorers {
		if b.Name == "agent_balance" && b.Override.Weight != nil && *b.Override.Weight == 0.7 {
			found = true
		}
	}
	if !found {
		t.Errorf("agent_balance not bound with weight 0.7: %+v", p.Scorers)
	}
}
