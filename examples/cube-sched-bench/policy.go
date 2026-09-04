// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package main

// policy.go turns a scheduling strategy into a scoring rule used by pick().
// The built-in agent profiles are read from the CubeMaster configuration
// package so their weights stay in one place.

import (
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/config"
)

// policyDefault is the legacy pipeline modelled as a pure load-balancing
// scorer (no image preference).
const policyDefault = "default"

// Policy is a strategy in simulator terms: how strongly it prefers an idle
// node (wLoad) versus a node that already caches the requested template (wImg).
type Policy struct {
	Name  string
	wLoad float64
	wImg  float64
}

// score ranks a node for req. Lower load gives a higher score; a node that
// already has the template image gets an extra image bonus.
func (p Policy) score(n *simNode, req Req) float64 {
	s := -p.wLoad * n.cpuUtil()
	if req.Template != "" && n.has(req.Template) {
		s += p.wImg
	}
	return s
}

// agentProfiles returns the built-in agent strategy profiles, the single
// source of truth shared with the scheduler's configuration defaults.
func agentProfiles() map[string]*config.SchedulerProfileConf {
	return config.DefaultAgentProfiles()
}

// profileWeight extracts a scorer's effective weight from a profile entry,
// falling back to def when the scorer is absent or has no weight override.
func profileWeight(p *config.SchedulerProfileConf, scorer string, def float64) float64 {
	if p == nil || p.Scorers == nil {
		return def
	}
	if o, ok := p.Scorers[scorer]; ok && o != nil && o.Weight != nil {
		return *o.Weight
	}
	return def
}

// policies builds the strategies to compare: the load-only default plus one
// policy per built-in agent profile. real_time_weighted_average maps to the
// load term and image_score to the image-locality term.
func policies() map[string]Policy {
	out := map[string]Policy{}
	profiles := agentProfiles()
	out[policyDefault] = Policy{Name: policyDefault, wLoad: 1, wImg: 0}
	for name, p := range profiles {
		out[name] = Policy{
			Name:  name,
			wLoad: profileWeight(p, "real_time_weighted_average", 1),
			wImg:  profileWeight(p, "image_score", 0),
		}
	}
	return out
}
