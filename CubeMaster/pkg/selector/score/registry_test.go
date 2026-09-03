// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package score

import (
	"context"
	"testing"

	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/config"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/node"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/scheduler/selctx"
)

// stubSelector is a minimal Selector used to test registration/override logic
// without touching global config.
type stubSelector struct {
	id  string
	w   float64
	dis bool
}

func (s *stubSelector) Select(*selctx.SelectorCtx) (node.NodeScoreList, error) { return nil, nil }
func (s *stubSelector) ID() string                                             { return s.id }
func (s *stubSelector) Weight() float64                                        { return s.w }
func (s *stubSelector) Disable() bool                                          { return s.dis }

func TestRegisterScoreDedupes(t *testing.T) {
	if err := Register("_t_s_dup", func() Selector { return &stubSelector{id: "s"} }); err != nil {
		t.Fatal(err)
	}
	if err := Register("_t_s_dup", func() Selector { return &stubSelector{id: "s"} }); err == nil {
		t.Fatal("duplicate registration should error")
	}
	if err := Register("", func() Selector { return &stubSelector{} }); err == nil {
		t.Fatal("empty name should error")
	}
	if err := Register("_t_s_nil", nil); err == nil {
		t.Fatal("nil factory should error")
	}
}

func TestBuiltinScoreRegistered(t *testing.T) {
	for _, name := range []string{
		"real_time_weighted_average", "multi_factor_weighted_average", "affinity_score", "image_score",
	} {
		if _, ok := Factories()[name]; !ok {
			t.Errorf("built-in scorer %q not registered", name)
		}
	}
}

func TestApplyOverride(t *testing.T) {
	w := 9.0
	dis := true

	overridden := applyOverride(&stubSelector{id: "x", w: 1}, &config.ScorerOverride{Weight: &w, Disable: &dis})
	if overridden.Weight() != 9 {
		t.Errorf("Weight() = %v, want 9", overridden.Weight())
	}
	if !overridden.Disable() {
		t.Error("Disable() = false, want true")
	}

	// Empty override passes the original through unchanged.
	orig := &stubSelector{id: "x", w: 2}
	if got := applyOverride(orig, nil); got != orig {
		t.Error("nil override should return the original selector")
	}
	empty := &config.ScorerOverride{}
	if got := applyOverride(orig, empty); got != orig {
		t.Error("empty override should return the original selector")
	}
}

func TestBuildScorersOrderSkipAndOverride(t *testing.T) {
	if err := Register("_t_s_stub", func() Selector { return &stubSelector{id: "stub", w: 1} }); err != nil {
		t.Fatal(err)
	}
	w := 5.0
	dis := true
	scoreConf := &config.SchedulerScoreConf{
		EnableScorers:   []string{"_t_s_stub", "no_such_scorer"},
		ResourceWeights: map[string]float64{"cpu": 1},
	}
	spec := config.ResolvedPipeline{
		Scorers: []config.ScoreBinding{
			{Name: "_t_s_stub", Override: &config.ScorerOverride{Weight: &w, Disable: &dis}},
			{Name: "no_such_scorer"},
		},
	}
	selectors := buildScorers(scoreConf, spec, context.Background())
	if len(selectors) != 1 {
		t.Fatalf("got %d selectors, want 1 (unknown skipped)", len(selectors))
	}
	if selectors[0].Weight() != 5 {
		t.Errorf("Weight() = %v, want overridden 5", selectors[0].Weight())
	}
	if !selectors[0].Disable() {
		t.Error("Disable() = false, want true")
	}
}

// TestBuildScorersProfileMode verifies profile mode builds scorers from the
// pipeline spec alone: even with a nil Score section (no legacy plugin_conf),
// a registered scorer with an override is instantiated and wrapped.
func TestBuildScorersProfileMode(t *testing.T) {
	if err := Register("_t_s_prof", func() Selector { return &stubSelector{id: "prof", w: 1} }); err != nil {
		t.Fatal(err)
	}
	w := 7.0
	spec := config.ResolvedPipeline{
		Profile: "burst",
		Scorers: []config.ScoreBinding{{Name: "_t_s_prof", Override: &config.ScorerOverride{Weight: &w}}},
	}
	selectors := buildScorers(nil, spec, context.Background())
	if len(selectors) != 1 {
		t.Fatalf("got %d selectors, want 1", len(selectors))
	}
	if selectors[0].Weight() != 7 {
		t.Errorf("Weight() = %v, want profile override 7", selectors[0].Weight())
	}
}

// TestBuildScorersLegacyEmptyGuard preserves the legacy behaviour: with no
// active profile, an empty/absent Score section yields an empty pipeline.
func TestBuildScorersLegacyEmptyGuard(t *testing.T) {
	spec := config.ResolvedPipeline{} // legacy mode, Profile == ""
	selectors := buildScorers(nil, spec, context.Background())
	if len(selectors) != 0 {
		t.Fatalf("legacy empty guard should yield empty, got %d", len(selectors))
	}
}
