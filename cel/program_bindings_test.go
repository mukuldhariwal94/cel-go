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
	"slices"
	"strings"
	"sync"
	"testing"

	"cel.dev/cel-go/common/functions"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

func TestProgramSelectedFunctionBindings(t *testing.T) {
	identity := func(v ref.Val) ref.Val { return v }
	env, err := NewEnv(
		Variable("x", DynType),
		Function("identity",
			Overload("identity_int", []*Type{IntType}, IntType, UnaryBinding(identity)),
			Overload("identity_string", []*Type{StringType}, StringType, UnaryBinding(identity))),
		Function("singleton", Overload("singleton_int", []*Type{IntType}, IntType), SingletonUnaryBinding(identity)),
		Function("unused", Overload("unused_int", []*Type{IntType}, IntType, UnaryBinding(identity))),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, expr string
		unchecked  bool
		want       ref.Val
	}{
		{name: "static", expr: "identity(1)", want: types.Int(1)},
		{name: "dynamic", expr: "identity(x)", want: types.String("value")},
		{name: "singleton", expr: "singleton(1)", want: types.Int(1)},
		{name: "unchecked", expr: "identity(1)", unchecked: true, want: types.Int(1)},
		{name: "constant", expr: "true", want: types.True},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var a *Ast
			var iss *Issues
			if tc.unchecked {
				a, iss = env.Parse(tc.expr)
			} else {
				a, iss = env.Compile(tc.expr)
			}
			if iss.Err() != nil {
				t.Fatal(iss.Err())
			}
			p, err := env.Program(a)
			if err != nil {
				t.Fatal(err)
			}
			entry := env.sharedDispatcher.lookup("unused_int")
			if entry == nil || entry.bindings != nil {
				t.Fatal("unused function bindings should remain uninitialized")
			}
			got, _, err := p.Eval(map[string]any{"x": "value"})
			if err != nil || got != tc.want {
				t.Fatalf("Eval() = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestProgramSelectedFunctionOverrideIsolation(t *testing.T) {
	env, err := NewEnv(Function("identity", Overload("identity_int", []*Type{IntType}, IntType,
		UnaryBinding(func(v ref.Val) ref.Val { return v }))))
	if err != nil {
		t.Fatal(err)
	}
	for _, unchecked := range []bool{false, true} {
		var a *Ast
		var iss *Issues
		if unchecked {
			a, iss = env.Parse("identity(1)")
		} else {
			a, iss = env.Compile("identity(1)")
		}
		if iss.Err() != nil {
			t.Fatal(iss.Err())
		}
		operator := "identity_int"
		if unchecked {
			operator = "identity"
		}
		overridden, err := env.Program(a, Functions(&functions.Overload{
			Operator: operator, Unary: func(ref.Val) ref.Val { return types.Int(2) },
		}))
		if err != nil {
			t.Fatal(err)
		}
		normal, err := env.Program(a)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			p    Program
			want ref.Val
		}{{overridden, types.Int(2)}, {normal, types.Int(1)}} {
			got, _, err := tc.p.Eval(NoVars())
			if err != nil || got != tc.want {
				t.Fatalf("unchecked=%v: Eval() = %v, %v; want %v", unchecked, got, err, tc.want)
			}
		}
	}
}

const programBindingsExpr = `name.startsWith('app-1') || (name.endsWith('-svc') && name.contains('-') && name.matches('^[a-z][a-z0-9-]{0,62}$') && name.size() <= 63)`

func programBindingsSetup(t testing.TB) (*Env, *Ast) {
	t.Helper()
	e, err := NewEnv(Variable("name", StringType))
	if err != nil {
		t.Fatal(err)
	}
	a, iss := e.Compile(programBindingsExpr)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	p, err := e.Program(a)
	if err != nil {
		t.Fatal(err)
	}
	v, _, err := p.Eval(map[string]any{"name": "app-1"})
	if err != nil || v != types.True {
		t.Fatalf("eval: %v %v", v, err)
	}
	return e, a
}

func programBindingsEnv(t testing.TB, e *Env, mode string) *Env {
	t.Helper()
	var err error
	switch mode {
	case "Extended":
		e, err = e.Extend(Variable("extra", StringType))
	case "ChangedFunctions":
		e, err = e.Extend(Function("unused", Overload("unused_string", []*Type{StringType}, StringType, UnaryBinding(func(v ref.Val) ref.Val { return v }))))
	}
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func BenchmarkProgramBindings(b *testing.B) {
	for _, mode := range []string{"Shared", "Extended", "ChangedFunctions"} {
		b.Run(mode, func(b *testing.B) {
			e, a := programBindingsSetup(b)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				env := programBindingsEnv(b, e, mode)
				if _, err := env.Program(a); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestLazyBindingsSharing(t *testing.T) {
	identity := func(v ref.Val) ref.Val { return v }
	base, err := NewEnv(Variable("x", IntType), Function("identity",
		Overload("identity_int", []*Type{IntType}, IntType, UnaryBinding(identity))))
	if err != nil {
		t.Fatal(err)
	}
	variableChild, err := base.Extend(Variable("extra", StringType))
	if err != nil {
		t.Fatal(err)
	}
	functionChild, err := base.Extend(Function("unused",
		Overload("unused_int", []*Type{IntType}, IntType, UnaryBinding(identity))))
	if err != nil {
		t.Fatal(err)
	}
	// Parse deliberately avoids checker initialization before concurrent planning.
	a, iss := base.Parse("identity(x)")
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	var wg sync.WaitGroup
	for _, env := range []*Env{base, variableChild, functionChild} {
		for range 20 {
			wg.Go(func() {
				p, err := env.Program(a)
				if err != nil {
					t.Error(err)
					return
				}
				got, _, err := p.Eval(map[string]any{"x": int64(1)})
				if err != nil || got != types.Int(1) {
					t.Errorf("Eval() = %v, %v", got, err)
				}
			})
		}
	}
	wg.Wait()
	if base.sharedDispatcher != variableChild.sharedDispatcher {
		t.Error("variable-only extension did not reuse the dispatcher")
	}
	shared := base.sharedDispatcher.lookup("identity_int")
	if shared != functionChild.sharedDispatcher.lookup("identity_int") {
		t.Error("function extension did not reuse unchanged bindings")
	}
	unused := functionChild.sharedDispatcher.lookup("unused_int")
	if unused == nil || unused.bindings != nil {
		t.Fatal("unused bindings were constructed")
	}
	ids := functionChild.sharedDispatcher.OverloadIds()
	if !slices.Contains(ids, "unused_int") {
		t.Error("uninitialized binding missing from OverloadIds")
	}
	if unused.bindings != nil {
		t.Error("listing overload IDs constructed unused bindings")
	}
}

func TestLazyBindingsReplacement(t *testing.T) {
	base, err := NewEnv(Function("identity", Overload("identity_int", []*Type{IntType}, IntType,
		UnaryBinding(func(v ref.Val) ref.Val { return v }))))
	if err != nil {
		t.Fatal(err)
	}
	child, err := base.Extend(Function("identity", Overload("identity_int", []*Type{IntType}, IntType,
		UnaryBinding(func(ref.Val) ref.Val { return types.Int(2) }))))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		env  *Env
		want ref.Val
	}{{child, types.Int(2)}, {base, types.Int(1)}} {
		a, iss := tc.env.Compile("identity(1)")
		if iss.Err() != nil {
			t.Fatal(iss.Err())
		}
		p, err := tc.env.Program(a)
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := p.Eval(NoVars())
		if err != nil || got != tc.want {
			t.Fatalf("Eval() = %v, %v; want %v", got, err, tc.want)
		}
	}
	if base.sharedDispatcher.lookup("identity_int") == child.sharedDispatcher.lookup("identity_int") {
		t.Error("replacement incorrectly reused the old binding")
	}
}

func TestLazyBindingsValidateUnusedFunctions(t *testing.T) {
	identity := func(v ref.Val) ref.Val { return v }
	for _, tc := range []struct {
		name string
		opts []EnvOption
		want string
	}{
		{"singleton", []EnvOption{Function("unused", Overload("unused_int", []*Type{IntType}, IntType,
			UnaryBinding(identity)), SingletonUnaryBinding(identity))}, "singleton function incompatible"},
		{"collision", []EnvOption{
			Function("first", Overload("duplicate", []*Type{IntType}, IntType, UnaryBinding(identity))),
			Function("second", Overload("duplicate", []*Type{IntType}, IntType, UnaryBinding(identity))),
		}, "overload already exists 'duplicate'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := NewEnv(tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			a, iss := e.Parse("true")
			if iss.Err() != nil {
				t.Fatal(iss.Err())
			}
			// The cached error must also be returned by subsequent Program calls.
			for range 2 {
				if _, err := e.Program(a); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("Program() error = %v, want %q", err, tc.want)
				}
			}
		})
	}
}

func TestLazyBindingNamesMatchBindings(t *testing.T) {
	e, err := NewEnv(OptionalTypes())
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range e.functions {
		names, err := fn.BindingNames()
		if err != nil {
			t.Fatal(err)
		}
		bindings, err := fn.Bindings()
		if err != nil {
			t.Fatal(err)
		}
		var actual []string
		for _, binding := range bindings {
			actual = append(actual, binding.Operator)
		}
		if !slices.Equal(names, actual) {
			t.Errorf("%s: BindingNames() = %v, want %v", fn.Name(), names, actual)
		}
	}
}

func TestLazyBindingsHideRemovedOverloads(t *testing.T) {
	identity := func(v ref.Val) ref.Val { return v }
	base, err := NewEnv(Function("identity",
		Overload("identity_int", []*Type{IntType}, IntType, UnaryBinding(identity)),
		Overload("identity_string", []*Type{StringType}, StringType, UnaryBinding(identity))))
	if err != nil {
		t.Fatal(err)
	}
	child, err := base.Extend(func(e *Env) (*Env, error) {
		e.ensureMutableFunctions()
		e.functions["identity"] = e.functions["identity"].Subset(IncludeOverloads("identity_int"))
		return e, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := child.initDispatcher(); err != nil {
		t.Fatal(err)
	}
	if _, found := child.sharedDispatcher.FindOverload("identity_string"); found {
		t.Error("removed overload inherited from parent")
	}
	if slices.Contains(child.sharedDispatcher.OverloadIds(), "identity_string") {
		t.Error("removed overload listed by child")
	}
	if _, found := base.sharedDispatcher.FindOverload("identity_string"); !found {
		t.Error("removing child overload changed parent")
	}
}

func TestLazyBindingsChildRepairsInvalidParent(t *testing.T) {
	identity := func(v ref.Val) ref.Val { return v }
	base, err := NewEnv(Function("identity",
		Overload("identity_int", []*Type{IntType}, IntType, UnaryBinding(identity)),
		SingletonUnaryBinding(identity)))
	if err != nil {
		t.Fatal(err)
	}
	child, err := base.Extend(func(e *Env) (*Env, error) {
		// Replace the invalid declaration instead of merging its singleton binding.
		e.ensureMutableFunctions()
		delete(e.functions, "identity")
		return Function("identity", Overload("identity_int", []*Type{IntType}, IntType, UnaryBinding(identity)))(e)
	})
	if err != nil {
		t.Fatal(err)
	}
	a, iss := child.Compile("identity(1)")
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	p, err := child.Program(a)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := p.Eval(NoVars())
	if err != nil || got != types.Int(1) {
		t.Fatalf("Eval() = %v, %v", got, err)
	}
	if _, _, err := base.initDispatcher(); err == nil {
		t.Error("invalid parent should still fail")
	}
}
