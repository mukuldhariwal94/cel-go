// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cel

import (
	"fmt"
	"sync"

	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/functions"
)

// functionDispatcher is an immutable index of lazily initialized function bindings.
// Child registries contain only changed declarations; nil entries hide bindings
// removed by a replacement declaration. Program-specific bindings live in a
// separate interpreter.ExtendDispatcher layer.
type functionDispatcher struct {
	parent  *functionDispatcher
	entries map[string]*functionBindings
}

type functionBindings struct {
	decl     *decls.FunctionDecl
	once     sync.Once
	bindings []*functions.Overload
}

func newFunctionDispatcher(e *Env, parent *functionDispatcher) (*functionDispatcher, error) {
	d := &functionDispatcher{parent: parent, entries: make(map[string]*functionBindings)}
	if parent != nil {
		// Hide all replaced declarations first, so collision checks below do not
		// depend on the iteration order of the function map.
		for name, old := range e.parent.functions {
			if e.functions[name] == old {
				continue
			}
			names, err := old.BindingNames()
			if err != nil {
				return nil, err
			}
			for _, id := range names {
				d.entries[id] = nil
			}
		}
	}
	for name, fn := range e.functions {
		if parent != nil && e.parent.functions[name] == fn {
			continue
		}
		names, err := fn.BindingNames()
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			continue
		}
		entry := &functionBindings{decl: fn}
		for _, id := range names {
			if d.lookup(id) != nil {
				return nil, fmt.Errorf("overload already exists '%s'", id)
			}
			d.entries[id] = entry
		}
	}
	return d, nil
}

func (d *functionDispatcher) lookup(id string) *functionBindings {
	if entry, found := d.entries[id]; found {
		return entry
	}
	if d.parent != nil {
		return d.parent.lookup(id)
	}
	return nil
}

func (d *functionDispatcher) Add(...*functions.Overload) error {
	return fmt.Errorf("cannot add bindings to a shared function dispatcher")
}

func (d *functionDispatcher) FindOverload(id string) (*functions.Overload, bool) {
	entry := d.lookup(id)
	if entry == nil {
		return nil, false
	}
	entry.once.Do(func() {
		// BindingNames validated this immutable declaration during construction.
		entry.bindings, _ = entry.decl.Bindings()
	})
	for _, binding := range entry.bindings {
		if binding.Operator == id {
			return binding, true
		}
	}
	return nil, false
}

func (d *functionDispatcher) OverloadIds() []string {
	ids := make([]string, 0, len(d.entries))
	for id, entry := range d.entries {
		if entry != nil {
			ids = append(ids, id)
		}
	}
	if d.parent != nil {
		for _, id := range d.parent.OverloadIds() {
			if _, shadowed := d.entries[id]; !shadowed {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
