// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0
//

// Package filter provides filter functions for node.Node.
package filter

import (
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/config"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/base/node"
	"github.com/tencentcloud/CubeSandbox/CubeMaster/pkg/scheduler/selctx"
)

type Selector interface {
	Select(selCtx *selctx.SelectorCtx) (node.NodeList, error)

	ID() string
}

// Built-in filters self-register here so custom plugins registered from other
// packages participate in the exact same registry.
func init() {
	must := func(name string, factory Factory) {
		if err := Register(name, factory); err != nil {
			panic(err)
		}
	}
	must("cpu", func() Selector { return NewCpuFilter() })
	must("mem", func() Selector { return NewMemFilter() })
	must("template_locality", func() Selector { return NewTemplateLocalityFilter() })
	must("realtime_create_num", func() Selector { return NewRealtimecreatelimit() })
	must("disk", func() Selector { return NewDiskFilter() })
	must("thirtparty", func() Selector { return NewThirtpartyFilter() })
}

func NewSelector() []Selector {
	conf := config.GetConfig().Scheduler
	if conf == nil {
		return []Selector{}
	}
	return buildFilters(conf.ResolvePipeline().Filters)
}

// buildFilters constructs the filter plugins enabled by names, in the given
// order. Unknown plugin names are skipped so a historical config never fails
// to start.
func buildFilters(names []string) []Selector {
	ss := make([]Selector, 0, len(names))
	for _, name := range names {
		factory, ok := lookupFactory(name)
		if !ok {
			continue
		}
		ss = append(ss, factory())
	}
	return ss
}
