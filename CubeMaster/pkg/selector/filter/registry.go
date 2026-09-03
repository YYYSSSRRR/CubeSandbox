// Copyright (c) 2024 Tencent Inc.
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"fmt"
	"sync"
)

// Factory builds a filter plugin. Factories are registered at assembly time
// (package init) and must be safe to call concurrently afterwards. There is no
// dynamic hot-loading: a newly registered plugin takes effect on restart.
type Factory func() Selector

var (
	regMu    sync.RWMutex
	registry = map[string]Factory{}
)

// Register registers a filter plugin under a unique name. Registering the same
// name twice is an error so a plugin can never silently replace a built-in one.
func Register(name string, factory Factory) error {
	if name == "" {
		return fmt.Errorf("filter: register with empty name")
	}
	if factory == nil {
		return fmt.Errorf("filter: register %q with nil factory", name)
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[name]; dup {
		return fmt.Errorf("filter: plugin %q already registered", name)
	}
	registry[name] = factory
	return nil
}

// Factories returns a snapshot of all registered factories keyed by name.
func Factories() map[string]Factory {
	regMu.RLock()
	defer regMu.RUnlock()
	snapshot := make(map[string]Factory, len(registry))
	for name, f := range registry {
		snapshot[name] = f
	}
	return snapshot
}

func lookupFactory(name string) (Factory, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	f, ok := registry[name]
	return f, ok
}
