// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/constants"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/node"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/scheduler/selctx"
)

// AgentBalanceScore scores each node by its remaining quota ratio, preferring
// nodes that are less loaded. It shows the shape of a scorer plugin: every
// candidate node that participates is returned with a Score in [0,1]; the
// scheduler weights these scores by the plugin's Weight() and normalizes.
type AgentBalanceScore struct{}

// NewAgentBalanceScore builds the scorer. The plugin ships a default weight of
// 1; operators can override it from config without changing this code.
func NewAgentBalanceScore() *AgentBalanceScore {
	return &AgentBalanceScore{}
}

// ID must be unique among registered scorers.
func (s *AgentBalanceScore) ID() string {
	return constants.SelectorScoreID + "/agent_balance"
}

// String makes the scorer readable in logs.
func (s *AgentBalanceScore) String() string {
	return s.ID()
}

// Weight is the plugin's default contribution weight (overridable in config).
func (s *AgentBalanceScore) Weight() float64 {
	return 1
}

// Disable reports whether the plugin should be skipped this round. Leave it
// driven by nothing here; operators control it via the config override.
func (s *AgentBalanceScore) Disable() bool {
	return false
}

// Select returns one NodeScore per candidate node, scaled by how much quota is
// left on that node (0 = fully busy, 1 = idle).
func (s *AgentBalanceScore) Select(selCtx *selctx.SelectorCtx) (node.NodeScoreList, error) {
	if selCtx == nil {
		return nil, nil
	}
	in := selCtx.Nodes()
	out := make(node.NodeScoreList, 0, in.Len())
	for i := range in {
		n := in[i]
		if n == nil {
			continue
		}
		out.Append(&node.NodeScore{
			InsID:    n.ID(),
			Score:    freeRatio(n),
			MvmNum:   n.MvmNum,
			OrigNode: n,
		})
	}
	return out, nil
}

// freeRatio returns the fraction of CPU quota still unused, clamped to [0,1].
func freeRatio(n *node.Node) float64 {
	if n == nil || n.QuotaCpu <= 0 {
		return 0
	}
	used := float64(n.QuotaCpuUsage) / float64(n.QuotaCpu)
	if used < 0 {
		used = 0
	}
	if used > 1 {
		used = 1
	}
	return 1 - used
}
