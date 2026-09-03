// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package score

import (
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/config"
)

// configuredSelector decorates a Selector with config-driven weight/disable
// overrides. A nil override field delegates to the wrapped selector, so the
// original plugin_conf behaviour is preserved when no override is set.
type configuredSelector struct {
	Selector
	weight  *float64
	disable *bool
}

func (c *configuredSelector) Weight() float64 {
	if c.weight != nil {
		return *c.weight
	}
	return c.Selector.Weight()
}

func (c *configuredSelector) Disable() bool {
	if c.disable != nil {
		return *c.disable
	}
	return c.Selector.Disable()
}

// applyOverride wraps sel when override actually changes something; otherwise
// it returns sel untouched.
func applyOverride(sel Selector, override *config.ScorerOverride) Selector {
	if override == nil || (override.Weight == nil && override.Disable == nil) {
		return sel
	}
	return &configuredSelector{Selector: sel, weight: override.Weight, disable: override.Disable}
}
