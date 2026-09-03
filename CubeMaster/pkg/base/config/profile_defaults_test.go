// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestEnsureDefaultsFillsProfilesAndPluginConf(t *testing.T) {
	sc := &SchedulerConf{}
	ensureDefaultSchedulerProfiles(sc)

	if sc.Score == nil {
		t.Fatal("Score should be initialized")
	}
	pc := &sc.Score.ScorePluginConf
	if pc.RealTimeWeightedAverage == nil || pc.ImageScore == nil || pc.AffinityScore == nil {
		t.Fatalf("plugin_conf defaults missing: %+v", pc)
	}

	if sc.Profiles == nil {
		t.Fatal("Profiles should be initialized")
	}
	for _, name := range []string{ProfileAgentBurst, ProfileAgentTemplateHeavy, ProfileAgentMixed} {
		if _, ok := sc.Profiles[name]; !ok {
			t.Errorf("built-in profile %q missing", name)
		}
	}
	if sc.ActiveProfile != "" {
		t.Errorf("ActiveProfile should stay empty by default, got %q", sc.ActiveProfile)
	}
}

func TestEnsureDefaultsKeepsOperatorOverrides(t *testing.T) {
	opWeight := 9.0
	sc := &SchedulerConf{
		ActiveProfile: ProfileAgentBurst,
		Score: &SchedulerScoreConf{
			ScorePluginConf: ScorePluginConf{
				ImageScore: &ImageScore{Weight: 4.0}, // operator-provided
			},
		},
		Profiles: map[string]*SchedulerProfileConf{
			ProfileAgentBurst: {
				Filters: []string{"cpu"},
				Scorers: map[string]*ScorerOverride{
					"image_score": {Weight: &opWeight},
				},
			},
		},
	}
	ensureDefaultSchedulerProfiles(sc)

	// Operator's image_score weight survives.
	if sc.Score.ScorePluginConf.ImageScore.Weight != 4.0 {
		t.Errorf("operator image plugin weight overwritten: %+v", sc.Score.ScorePluginConf.ImageScore)
	}
	// Missing profiles are still added alongside the operator-defined one.
	if _, ok := sc.Profiles[ProfileAgentMixed]; !ok {
		t.Error("default profile not added next to operator profile")
	}
	// ActiveProfile is untouched.
	if sc.ActiveProfile != ProfileAgentBurst {
		t.Errorf("ActiveProfile changed to %q", sc.ActiveProfile)
	}
}

func TestBuiltinProfileResolves(t *testing.T) {
	sc := &SchedulerConf{ActiveProfile: ProfileAgentTemplateHeavy}
	ensureDefaultSchedulerProfiles(sc)

	p := sc.ResolvePipeline()
	if p.Profile != ProfileAgentTemplateHeavy {
		t.Fatalf("resolved profile = %q, want %q", p.Profile, ProfileAgentTemplateHeavy)
	}
	if len(p.Filters) != 3 {
		t.Errorf("template-heavy filters = %v", p.Filters)
	}
	// image_score should be bound with weight 2.0.
	found := false
	for _, b := range p.Scorers {
		if b.Name == "image_score" && b.Override != nil && b.Override.Weight != nil && *b.Override.Weight == 2.0 {
			found = true
		}
	}
	if !found {
		t.Errorf("image_score override missing in %+v", p.Scorers)
	}
	if !sc.ProfileEnabled(ProfileAgentTemplateHeavy) {
		t.Error("ProfileEnabled should report true for the active built-in profile")
	}
}
