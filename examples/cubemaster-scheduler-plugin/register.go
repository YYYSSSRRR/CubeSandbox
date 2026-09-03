// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"

	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/selector/filter"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/selector/score"
)

// RegisterPlugins registers this package's plugins with the scheduler plugin
// registries. Call it once during service startup (or rely on the package
// init below). Registration is not hot: changes take effect on restart.
func RegisterPlugins() error {
	if err := filter.Register("agent_capacity", func() filter.Selector { return NewAgentCapacityFilter() }); err != nil {
		return fmt.Errorf("register filter: %w", err)
	}
	if err := score.Register("agent_balance", func() score.Selector { return NewAgentBalanceScore() }); err != nil {
		return fmt.Errorf("register scorer: %w", err)
	}
	return nil
}

// init auto-registers the plugins as soon as this package is imported by the
// CubeMaster binary. If you prefer explicit wiring, drop the import and call
// RegisterPlugins() from the assembly point instead.
func init() {
	_ = RegisterPlugins()
}
