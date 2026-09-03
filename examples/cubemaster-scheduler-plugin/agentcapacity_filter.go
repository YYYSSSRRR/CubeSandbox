// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

// Package plugin demonstrates how to write a custom CubeMaster scheduling
// plugin without touching the CubeMaster source itself.
//
// It implements one filter (AgentCapacityFilter) and one scorer
// (AgentBalanceScore), then registers them through RegisterPlugins. The service
// then picks them up by name from the scheduler config (filters/scorers lists
// or a strategy Profile) and applies weights/disable exactly like built-ins.
package plugin

import (
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/constants"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/node"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/scheduler/selctx"
)

// AgentCapacityFilter keeps only nodes that still report schedulable CPU
// capacity. It is an intentionally simple filter showing the three mandatory
// pieces of a filter plugin: a constructor, an ID and a Select.
type AgentCapacityFilter struct{}

// NewAgentCapacityFilter builds the filter. Filter factories take no arguments;
// anything the plugin needs at runtime comes from selCtx or from package-level
// config on each Select call.
func NewAgentCapacityFilter() *AgentCapacityFilter {
	return &AgentCapacityFilter{}
}

// ID must be unique among registered filters. The short name used for Register
// is what appears in the YAML; the ID is what appears in logs.
func (f *AgentCapacityFilter) ID() string {
	return constants.SelectorFilterID + "/agent_capacity"
}

// String makes the filter readable in logs.
func (f *AgentCapacityFilter) String() string {
	return f.ID()
}

// Select returns the subset of selCtx.Nodes() that passed the filter. Nodes
// dropped here never reach the scoring phase.
func (f *AgentCapacityFilter) Select(selCtx *selctx.SelectorCtx) (node.NodeList, error) {
	if selCtx == nil {
		return node.NodeList{}, nil
	}
	in := selCtx.Nodes()
	out := make(node.NodeList, 0, in.Len())
	for i := range in {
		if in[i] != nil && in[i].QuotaCpu > 0 {
			out.Append(in[i])
		}
	}
	return out, nil
}
