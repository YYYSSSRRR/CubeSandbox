// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"strings"
	"testing"
)

func TestRegisterDedupes(t *testing.T) {
	if err := Register("_t_dup_probe", func() Selector { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := Register("_t_dup_probe", func() Selector { return nil }); err == nil {
		t.Fatal("duplicate registration should error")
	}
	if err := Register("", func() Selector { return nil }); err == nil {
		t.Fatal("empty name should error")
	}
	if err := Register("_t_nil_probe", nil); err == nil {
		t.Fatal("nil factory should error")
	}
}

func TestBuiltinFilterRegistered(t *testing.T) {
	for _, name := range []string{"cpu", "mem", "template_locality", "realtime_create_num", "disk", "thirtparty"} {
		if _, ok := Factories()[name]; !ok {
			t.Errorf("built-in filter %q not registered", name)
		}
	}
}

func TestBuildFiltersOrderAndSkipUnknown(t *testing.T) {
	names := []string{"mem", "not_registered_plugin", "cpu"}
	selectors := buildFilters(names)
	if len(selectors) != 2 {
		t.Fatalf("got %d selectors, want 2 (unknown skipped)", len(selectors))
	}
	if !strings.Contains(selectors[0].ID(), "mem") {
		t.Errorf("first selector ID = %q, want mem first", selectors[0].ID())
	}
	if !strings.Contains(selectors[1].ID(), "cpu") {
		t.Errorf("second selector ID = %q, want cpu second", selectors[1].ID())
	}
}

func TestBuildFiltersNilInput(t *testing.T) {
	if got := buildFilters(nil); len(got) != 0 {
		t.Fatalf("nil names should yield empty slice, got %d", len(got))
	}
}
