// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"
)

func TestResolvePipelineLegacy(t *testing.T) {
	w := 1.5
	sc := &SchedulerConf{
		Filter: &SchedulerFilterConf{EnableFilters: []string{"cpu", "mem"}},
		Score: &SchedulerScoreConf{
			EnableScorers:   []string{"image_score"},
			ResourceWeights: map[string]float64{"cpu": 1},
			Scorers:         map[string]*ScorerOverride{"image_score": {Weight: &w}},
		},
	}
	p := sc.ResolvePipeline()
	if p.Profile != "" {
		t.Fatalf("legacy pipeline should have empty profile, got %q", p.Profile)
	}
	if len(p.Filters) != 2 || p.Filters[0] != "cpu" || p.Filters[1] != "mem" {
		t.Errorf("legacy filters = %v", p.Filters)
	}
	if len(p.Scorers) != 1 || p.Scorers[0].Name != "image_score" || p.Scorers[0].Override.Weight == nil || *p.Scorers[0].Override.Weight != 1.5 {
		t.Errorf("legacy scorers = %+v", p.Scorers)
	}
}

func TestResolvePipelineProfile(t *testing.T) {
	w := 2.0
	dis := false
	sc := &SchedulerConf{
		ActiveProfile: "burst",
		Profiles: map[string]*SchedulerProfileConf{
			"burst": {
				Filters: []string{"cpu", "template_locality"},
				Scorers: map[string]*ScorerOverride{
					"image_score":    {Weight: &w},
					"affinity_score": {Disable: &dis},
				},
			},
		},
	}
	p := sc.ResolvePipeline()
	if p.Profile != "burst" {
		t.Fatalf("profile = %q, want burst", p.Profile)
	}
	if len(p.Filters) != 2 || p.Filters[1] != "template_locality" {
		t.Errorf("profile filters = %v", p.Filters)
	}
	if len(p.Scorers) != 2 {
		t.Fatalf("profile scorers count = %d, want 2", len(p.Scorers))
	}
	got := map[string]*ScorerOverride{}
	for _, b := range p.Scorers {
		got[b.Name] = b.Override
	}
	if ov, ok := got["image_score"]; !ok || ov == nil || ov.Weight == nil || *ov.Weight != 2.0 {
		t.Errorf("image_score override = %+v", got["image_score"])
	}
	if ov, ok := got["affinity_score"]; !ok || ov == nil || ov.Disable == nil || *ov.Disable {
		t.Errorf("affinity_score override = %+v", got["affinity_score"])
	}
}

func TestResolvePipelineUnknownProfileFallsBack(t *testing.T) {
	sc := &SchedulerConf{
		ActiveProfile: "ghost",
		Profiles: map[string]*SchedulerProfileConf{
			"other": {Filters: []string{"disk"}},
		},
		Filter: &SchedulerFilterConf{EnableFilters: []string{"mem"}},
	}
	p := sc.ResolvePipeline()
	if p.Profile != "" {
		t.Fatalf("unknown profile should fall back to legacy, got %q", p.Profile)
	}
	if len(p.Filters) != 1 || p.Filters[0] != "mem" {
		t.Errorf("fallback filters = %v", p.Filters)
	}
}

func TestResolvePipelineNil(t *testing.T) {
	var sc *SchedulerConf
	if p := sc.ResolvePipeline(); len(p.Filters) != 0 || len(p.Scorers) != 0 {
		t.Fatalf("nil scheduler conf should yield empty pipeline, got %+v", p)
	}
}
