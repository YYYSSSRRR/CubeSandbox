// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0
//

// Package score provides the score of a node.
package score

import (
	"context"

	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/config"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/node"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/recov"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/scheduler/selctx"
)

type Selector interface {
	Select(selCtx *selctx.SelectorCtx) (node.NodeScoreList, error)

	ID() string

	Weight() float64

	Disable() bool
}

// Built-in scorers self-register here so custom plugins registered from other
// packages participate in the exact same registry.
func init() {
	must := func(name string, factory Factory) {
		if err := Register(name, factory); err != nil {
			panic(err)
		}
	}
	must("real_time_weighted_average", func() Selector { return NewRealTimeWeightedAverageScore() })
	must("multi_factor_weighted_average", func() Selector { return NewMultiFactorWeightedAverageScore() })
	must("affinity_score", func() Selector { return NewAffinityScore() })
	must("image_score", func() Selector { return NewImageScore() })
}

func NewSelector(ctx context.Context) []Selector {
	conf := config.GetConfig().Scheduler
	if conf == nil {
		return []Selector{}
	}
	return buildScorers(conf.Score, conf.ResolvePipeline(), ctx)
}

// buildScorers constructs the scoring plugins bound by spec, applying each
// scorer's override. Legacy mode keeps the guard that a nil ResourceWeights or
// empty enable list yields an empty pipeline; profile mode is governed by the
// profile's own scorer bindings.
func buildScorers(scoreConf *config.SchedulerScoreConf, spec config.ResolvedPipeline, ctx context.Context) []Selector {
	if spec.Profile == "" && (scoreConf == nil || scoreConf.ResourceWeights == nil || len(scoreConf.EnableScorers) == 0) {
		return []Selector{}
	}
	if len(spec.Scorers) == 0 {
		return []Selector{}
	}

	ss := make([]Selector, 0, len(spec.Scorers))
	for _, binding := range spec.Scorers {
		factory, ok := lookupFactory(binding.Name)
		if !ok {
			continue
		}
		ss = append(ss, applyOverride(factory(), binding.Override))
	}

	if scoreConf != nil && scoreConf.ScorePluginConf.MultiFactorWeightedAverage != nil {
		recov.GoWithRecover(func() {
			loopAsyncScore(ctx)
		})
	}
	return ss
}
