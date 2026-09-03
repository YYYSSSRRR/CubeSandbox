// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/constants"
)

// Built-in names of the three agent-oriented scheduling strategies. Operators
// enable one by setting scheduler.active_profile; the profile definitions
// themselves come from ensureDefaultSchedulerProfiles so a single line in YAML
// is enough to switch strategy.
const (
	ProfileAgentBurst         = "agent-burst"
	ProfileAgentTemplateHeavy = "agent-template-heavy"
	ProfileAgentMixed         = "agent-mixed"
)

func f64p(v float64) *float64 { return &v }
func boolp(v bool) *bool      { return &v }

// defaultAgentProfiles returns the three built-in strategy profiles. They reuse
// the registered filter/scorer plugins, so no plugin code is required to use
// them; only scheduler.active_profile needs to be set in the configuration.
func defaultAgentProfiles() map[string]*SchedulerProfileConf {
	return map[string]*SchedulerProfileConf{
		ProfileAgentBurst: {
			// Short-lived, high-concurrency agent sandboxes: spread requests
			// across nodes and off hot spots to keep create latency low.
			Filters: []string{"cpu", "mem", "disk", "template_locality", "realtime_create_num"},
			Scorers: map[string]*ScorerOverride{
				"real_time_weighted_average": {},
				"image_score":                {Weight: f64p(0.5)},
			},
		},
		ProfileAgentTemplateHeavy: {
			// Many copies of the same template: maximise local image hits so
			// new sandboxes skip pulling the image on cold start.
			Filters: []string{"cpu", "mem", "template_locality"},
			Scorers: map[string]*ScorerOverride{
				"image_score":                {Weight: f64p(2.0)},
				"real_time_weighted_average": {Weight: f64p(0.3)},
			},
		},
		ProfileAgentMixed: {
			// Mixed specs and long-lived sessions: balance packing with
			// affinity so larger/longer sandboxes stay stable.
			Filters: []string{"cpu", "mem", "disk", "template_locality"},
			Scorers: map[string]*ScorerOverride{
				"real_time_weighted_average": {},
				"affinity_score":             {Weight: f64p(0.6)},
				"image_score":                {Weight: f64p(0.3)},
			},
		},
	}
}

// ensureDefaultSchedulerProfiles makes the built-in agent profiles and minimal
// scorer plugin_conf available. It never overwrites operator-provided values:
// profiles and plugin_conf entries are only filled when missing, and
// active_profile is left untouched (default "" keeps legacy behaviour).
func ensureDefaultSchedulerProfiles(sc *SchedulerConf) {
	if sc == nil {
		return
	}
	if sc.Score == nil {
		sc.Score = &SchedulerScoreConf{}
	}

	// Minimal plugin_conf defaults so that enabling any built-in scorer through
	// a profile cannot panic at startup when the operator did not configure it.
	pc := &sc.Score.ScorePluginConf
	if pc.RealTimeWeightedAverage == nil {
		pc.RealTimeWeightedAverage = &RealTimeWeightedAverage{
			Weight: 1.0,
			EnableWeightFactors: []string{
				constants.WeightFactorMvmNum,
				constants.WeightFactorLocalCreateNum,
				constants.WeightFactorQuotaCpu,
				constants.WeightFactorQuotaMem,
			},
		}
	}
	if pc.ImageScore == nil {
		pc.ImageScore = &ImageScore{
			Weight: 1.0,
			EnableWeightFactors: []string{
				constants.WeightFactorImageID,
				constants.WeightFactorTemplateID,
			},
		}
	}
	if pc.AffinityScore == nil {
		pc.AffinityScore = &AffinityScore{Weight: 1.0}
	}

	if sc.Profiles == nil {
		sc.Profiles = make(map[string]*SchedulerProfileConf, len(defaultAgentProfiles()))
	}
	for name, profile := range defaultAgentProfiles() {
		if _, exists := sc.Profiles[name]; !exists {
			sc.Profiles[name] = profile
		}
	}
}

// ProfileEnabled reports whether the scheduler is configured to run the named
// built-in profile.
func (s *SchedulerConf) ProfileEnabled(name string) bool {
	return s != nil && s.ActiveProfile == name
}
