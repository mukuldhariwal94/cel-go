// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file contains code that demonstrates common CEL features.
// This code is intended for use with the CEL Codelab: go/cel-codelab-go
package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"

	"github.com/golang/glog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	rpcpb "google.golang.org/genproto/googleapis/rpc/context/attribute_context"
	structpb "google.golang.org/protobuf/types/known/structpb"
	tpb "google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	exercise1()
	exercise2()
	exercise3()
	exercise4()
	exercise5()
	exercise6()
	exercise7()
	exercise8()

	// Memory benchmarking exercises
	memoryBenchmark()
}

// exercise1 evaluates a simple literal expression: "Hello, World!"
//
// Compile, eval, profit!
func exercise1() {
	fmt.Println("=== Exercise 1: Hello World ===\n")
	// 1. Create CEL environment
	env, err := cel.NewEnv()
	if err != nil {
		glog.Exit(err)
	}

	// 2. Compile expression
	ast := compile(env, `"Hello, World!"`, cel.StringType)

	// 3. Create program
	prg, err := env.Program(ast)
	if err != nil {
		glog.Exit(err)
	}

	// 4. Evaluate
	eval(prg, map[string]any{})
	fmt.Println()
}

// exercise2 shows how to declare and use variables in expressions.
//
// Given a `request` of type `google.rpc.context.AttributeContext.Request`
// determine whether a specific auth claim is set.
func exercise2() {
	fmt.Println("=== Exercise 2: Variables ===\n")

	// ✅ Register proto type properly
	env, err := cel.NewEnv(
		cel.Types(&rpcpb.AttributeContext_Request{}),
		cel.Variable("request", cel.ObjectType("google.rpc.context.AttributeContext.Request")),
	)
	if err != nil {
		glog.Exit(err)
	}

	// Expression
	expr := `request.auth.claims.group == "admin"`

	// Compile
	ast := compile(env, expr, cel.BoolType)

	// Program
	prg, err := env.Program(ast)
	if err != nil {
		glog.Exit(err)
	}

	// Case 1: no claims
	req1 := request(auth("user:me@acme.co", emptyClaims), time.Now())

	fmt.Println("---- Case 1 ----")
	eval(prg, req1)

	// Case 2: with claims
	claims := map[string]string{
		"group": "admin",
	}
	req2 := request(auth("user:me@acme.co", claims), time.Now())

	fmt.Println("---- Case 2 ----")
	eval(prg, req2)
}

// exercise3 demonstrates how CEL's commutative logical operators work.
//
// Construct an expression which checks whether the `request.auth.claims.group`
// value is equal to `admin` or the `request.auth.principal` is
// `user:me@acme.co` and the `request.time` is during work hours (9:00 - 17:00)
//
// Evaluate the expression once with a request containing no claims but which
// sets the appropriate principal and occurs at 12:00 hours. Then evaluate the
// request a second time at midnight. Observe the difference in output.
func exercise3() {
	fmt.Println("=== Exercise 3: Logical AND/OR ===\n")

	fmt.Println()
}

// exercise4 demonstrates how to extend CEL with custom functions.
//
// Declare a `contains` member function on map types that returns a boolean
// indicating whether the map contains the key-value pair.
func exercise4() {
	fmt.Println("=== Exercise 4: Customization ===\n")

	fmt.Println()
}

// exercise5 covers how to build complex objects as CEL literals.
//
// Given the input `now`, construct a JWT with an expiry of 5 minutes.
func exercise5() {
	fmt.Println("=== Exercise 5: Building JSON ===\n")

	fmt.Println()
}

// exercise6 describes how to build proto message types within CEL.
//
// Given an input `jwt` and time `now` construct a
// `google.rpc.context.AttributeContext.Request` with the `time` and `auth`
// fields populated according to the go/api-attributes specification.
func exercise6() {
	fmt.Println("=== Exercise 6: Building Protos ===\n")

	fmt.Println()
}

// exercise7 introduces macros for dealing with repeated fields and maps.
//
// Determine whether the `jwt.extra_claims` has at least one key that starts
// with the `group` prefix, and ensure that all group-like keys have list
// values containing only strings that end with '@acme.co`.
func exercise7() {
	fmt.Println("=== Exercise 7: Macros ===\n")

	fmt.Println()
}

// exercise8 covers some useful features of CEL-Go which can be used to
// improve performance and better understand evaluation behavior.
//
// Turn on the optimization, exhaustive eval, and state tracking
// `cel.ProgramOption` flags to see the impact on evaluation behavior.
//
// Also, turn on the homogeneous aggregate literals flag to disable
// heterogeneous list and map literals.
func exercise8() {
	fmt.Println("=== Exercise 8: Tuning ===\n")

	fmt.Println()
}

// Memory benchmarking functionality

// MemoryStats represents memory usage at a specific point
type MemoryStats struct {
	Alloc      uint64 // bytes allocated and in use
	TotalAlloc uint64 // bytes allocated (even if freed)
	Sys        uint64 // bytes obtained from system
	NumGC      uint32 // number of garbage collections
}

// MemoryBenchmark represents memory usage across CEL phases
type MemoryBenchmark struct {
	Expression   string
	Iterations   int
	BaselineKB   float64
	EnvKB        float64
	CompileKB    float64
	ProgramKB    float64
	EvaluationKB float64
	TotalKB      float64
}

// getMemStats returns current memory statistics
func getMemStats() MemoryStats {
	runtime.GC() // Force garbage collection for cleaner measurements
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		NumGC:      m.NumGC,
	}
}

// bytesToKB converts bytes to kilobytes
func bytesToKB(bytes uint64) float64 {
	return float64(bytes) / 1024.0
}

// safeBytesToKB safely calculates the difference between after and before memory stats
func safeBytesToKB(after, before uint64) float64 {
	if after >= before {
		return bytesToKB(after - before)
	}
	// Memory decreased - likely due to garbage collection during measurement
	// Return the actual difference as negative to show memory was freed
	return bytesToKB(before - after) * -1.0
}

// printMemoryBenchmark prints formatted memory benchmark results
func printMemoryBenchmark(bench MemoryBenchmark) {
	fmt.Printf("Expression: %s\n", bench.Expression)
	if bench.Iterations > 1 {
		fmt.Printf("Iterations: %d (averages below)\n", bench.Iterations)
	}
	fmt.Printf("\n")
	fmt.Printf("Baseline:          %8.2f KB\n", bench.BaselineKB)
	fmt.Printf("Env Creation:      %8.2f KB\n", bench.EnvKB)
	fmt.Printf("Compile (AST):     %8.2f KB\n", bench.CompileKB)
	fmt.Printf("Program Creation:  %8.2f KB\n", bench.ProgramKB)
	fmt.Printf("Evaluation:        %8.2f KB\n", bench.EvaluationKB)
	fmt.Printf("Total:             %8.2f KB\n", bench.TotalKB)
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n\n")
}

// benchmarkExpression measures memory usage for a CEL expression across all phases
func benchmarkExpression(expr string, envOptions []cel.EnvOption, variables map[string]any, iterations int) MemoryBenchmark {
	var totalBaseline, totalEnv, totalCompile, totalProgram, totalEval, totalTotal float64

	for i := 0; i < iterations; i++ {
		// Clear memory before each iteration
		runtime.GC()
		runtime.GC() // Call twice for more thorough cleanup

		// Baseline measurement
		baseline := getMemStats()

		// 1. Environment creation
		var env *cel.Env
		var err error
		envBefore := getMemStats()
		env, err = cel.NewEnv(envOptions...)
		if err != nil {
			glog.Exit(err)
		}
		envAfter := getMemStats()
		envMem := safeBytesToKB(envAfter.Alloc, envBefore.Alloc)

		// 2. Compilation (AST creation)
		var ast *cel.Ast
		compileBefore := getMemStats()
		var iss *cel.Issues
		ast, iss = env.Compile(expr)
		if iss.Err() != nil {
			glog.Exit(iss.Err())
		}
		compileAfter := getMemStats()
		compileMem := safeBytesToKB(compileAfter.Alloc, compileBefore.Alloc)

		// 3. Program creation
		var prg cel.Program
		programBefore := getMemStats()
		prg, err = env.Program(ast)
		if err != nil {
			glog.Exit(err)
		}
		programAfter := getMemStats()
		programMem := safeBytesToKB(programAfter.Alloc, programBefore.Alloc)

		// 4. Evaluation - measure multiple evaluations to capture memory usage
		evalBefore := getMemStats()
		
		// Run evaluation multiple times to accumulate measurable memory usage
		for evalRun := 0; evalRun < 10; evalRun++ {
			_, _, err = prg.Eval(variables)
			if err != nil {
				glog.Exit(err)
			}
		}
		
		evalAfter := getMemStats()
		// Divide by 10 since we ran evaluation 10 times
		evalMem := safeBytesToKB(evalAfter.Alloc, evalBefore.Alloc) / 10.0
		
		// Debug: Print raw memory values for complex expressions (uncomment to debug)
		// if strings.Contains(expr, "map") && i == 0 {
		//     fmt.Printf("DEBUG %s: evalBefore=%d, evalAfter=%d, diff=%d, evalMem=%.2f\n", 
		//         expr, evalBefore.Alloc, evalAfter.Alloc, int64(evalAfter.Alloc)-int64(evalBefore.Alloc), evalMem)
		// }

		// Calculate total memory usage from baseline
		finalStats := getMemStats()
		totalMem := safeBytesToKB(finalStats.Alloc, baseline.Alloc)
		baselineMem := bytesToKB(baseline.Alloc)

		// Accumulate totals for averaging
		totalBaseline += baselineMem
		totalEnv += envMem
		totalCompile += compileMem
		totalProgram += programMem
		totalEval += evalMem
		totalTotal += totalMem
	}

	// Calculate averages
	iterationsFloat := float64(iterations)
	return MemoryBenchmark{
		Expression:   expr,
		Iterations:   iterations,
		BaselineKB:   totalBaseline / iterationsFloat,
		EnvKB:        totalEnv / iterationsFloat,
		CompileKB:    totalCompile / iterationsFloat,
		ProgramKB:    totalProgram / iterationsFloat,
		EvaluationKB: totalEval / iterationsFloat,
		TotalKB:      totalTotal / iterationsFloat,
	}
}

// memoryBenchmark runs comprehensive memory benchmarks on various CEL expressions
// to understand memory usage across different phases of CEL evaluation
func memoryBenchmark() {
	fmt.Println("=== Memory Benchmarking ===\n")

	// Test expressions of varying complexity
	expressions := []struct {
		name       string
		expr       string
		envOpts    []cel.EnvOption
		variables  map[string]any
		iterations int
	}{
		{
			name:       "Simple",
			expr:       "1 + 1",
			envOpts:    []cel.EnvOption{},
			variables:  map[string]any{},
			iterations: 100,
		},
		{
			name: "Medium",
			expr: "object.spec.replicas <= 5",
			envOpts: []cel.EnvOption{
				cel.Variable("object", cel.DynType),
			},
			variables: map[string]any{
				"object": map[string]any{
					"spec": map[string]any{
						"replicas": 3,
					},
				},
			},
			iterations: 100,
		},
		{
			name: "Complex",
			expr: `size(object.spec.template.spec.containers) > 2 && request.userInfo.username == "admin"`,
			envOpts: []cel.EnvOption{
				cel.Variable("object", cel.DynType),
				cel.Variable("request", cel.DynType),
			},
			variables: map[string]any{
				"object": map[string]any{
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{"name": "app1"},
									map[string]any{"name": "app2"},
									map[string]any{"name": "app3"},
								},
							},
						},
					},
				},
				"request": map[string]any{
					"userInfo": map[string]any{
						"username": "admin",
					},
				},
			},
			iterations: 100,
		},
		{
			name: "Proto-based",
			expr: `request.auth.claims.group == "admin"`,
			envOpts: []cel.EnvOption{
				cel.Types(&rpcpb.AttributeContext_Request{}),
				cel.Variable("request", cel.ObjectType("google.rpc.context.AttributeContext.Request")),
			},
			variables: func() map[string]any {
				claims := map[string]string{"group": "admin"}
				return request(auth("user:me@acme.co", claims), time.Now())
			}(),
			iterations: 100,
		},
		// 🚀 ADD YOUR CUSTOM EXPRESSIONS HERE! 🚀
		{
			name:       "String Operations",
			expr:       `"hello" + " " + "world" + "!" + " " + string(42)`,
			envOpts:    []cel.EnvOption{},
			variables:  map[string]any{},
			iterations: 100,
		},
		{
			name: "List Operations",
			expr: `items.filter(x, x.active).map(x, x.name)`,
			envOpts: []cel.EnvOption{
				cel.Variable("items", cel.ListType(cel.DynType)),
			},
			variables: map[string]any{
				"items": []any{
					map[string]any{"name": "item1", "active": true},
					map[string]any{"name": "item2", "active": false},
					map[string]any{"name": "item3", "active": true},
				},
			},
			iterations: 100,
		},
		{
			name: "Mathematical",
			expr: `x * x + y * y > threshold * threshold`,
			envOpts: []cel.EnvOption{
				cel.Variable("x", cel.DoubleType),
				cel.Variable("y", cel.DoubleType),
				cel.Variable("threshold", cel.DoubleType),
			},
			variables: map[string]any{
				"x":         3.0,
				"y":         4.0,
				"threshold": 5.0,
			},
			iterations: 100,
		},
		{
			name: "Heavy Evaluation",
			expr: `items.map(i, i * 2).filter(i, i > 50).map(i, string(i)).size()`,
			envOpts: []cel.EnvOption{
				cel.Variable("items", cel.ListType(cel.IntType)),
			},
			variables: map[string]any{
				"items": func() []int {
					items := make([]int, 1000) // Bigger list
					for i := 0; i < 1000; i++ {
						items[i] = i
					}
					return items
				}(),
			},
			iterations: 50, // Fewer iterations since this is more expensive
		},
		{
			name: "Memory-Intensive Evaluation",
			expr: `items.map(i, "item-" + string(i)).filter(s, size(s) > 5)`,
			envOpts: []cel.EnvOption{
				cel.Variable("items", cel.ListType(cel.IntType)),
			},
			variables: map[string]any{
				"items": func() []int {
					items := make([]int, 500)
					for i := 0; i < 500; i++ {
						items[i] = i
					}
					return items
				}(),
			},
			iterations: 20,
		},
		{
			name: "md-complex-v2",
			// The complex CEL expression logic
			expr: `
				(request.userInfo.username == "admin") || (
					(!has(object.spec.replicas) || object.spec.replicas <= 2) &&
					size(object.spec.template.spec.containers) <= 11 &&
					object.spec.template.spec.containers.all(c, c.name.startsWith("app")) &&
					!object.spec.template.spec.containers.exists(c, c.name == "root-shell")
				)
			`,
			envOpts: []cel.EnvOption{
				cel.Variable("object", cel.DynType),
				cel.Variable("request", cel.DynType),
			},
			variables: map[string]any{
				"object": map[string]any{
					"spec": map[string]any{
						// replicas is omitted to test the !has() logic
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{"name": "app-frontend"},
									map[string]any{"name": "app-backend"},
									map[string]any{"name": "app-cache"},
								},
							},
						},
					},
				},
				"request": map[string]any{
					"userInfo": map[string]any{
						"username": "admin",
					},
				},
			},
			iterations: 100,
		},
	}

	for _, test := range expressions {
		fmt.Printf("=== %s Expression ===\n", test.name)
		benchmark := benchmarkExpression(test.expr, test.envOpts, test.variables, test.iterations)
		printMemoryBenchmark(benchmark)
	}

	// Demonstrate program reuse vs recompilation
	fmt.Println("=== Program Reuse vs Recompilation ===\n")
	demonstrateProgramReuse()
}

// demonstrateProgramReuse shows memory difference between reusing and recreating programs
func demonstrateProgramReuse() {
	expr := `object.spec.replicas <= 5`
	envOpts := []cel.EnvOption{
		cel.Variable("object", cel.DynType),
	}
	variables := map[string]any{
		"object": map[string]any{
			"spec": map[string]any{
				"replicas": 3,
			},
		},
	}
	iterations := 50

	// Test 1: Recompiling every time
	fmt.Println("--- Recompiling every time ---")
	totalMemRecompile := 0.0

	for i := 0; i < iterations; i++ {
		runtime.GC()
		runtime.GC()

		before := getMemStats()

		// Full cycle: env + compile + program + eval
		env, err := cel.NewEnv(envOpts...)
		if err != nil {
			glog.Exit(err)
		}

		ast, iss := env.Compile(expr)
		if iss.Err() != nil {
			glog.Exit(iss.Err())
		}

		prg, err := env.Program(ast)
		if err != nil {
			glog.Exit(err)
		}

		_, _, err = prg.Eval(variables)
		if err != nil {
			glog.Exit(err)
		}

		after := getMemStats()
		totalMemRecompile += safeBytesToKB(after.Alloc, before.Alloc)
	}
	avgRecompile := totalMemRecompile / float64(iterations)

	// Test 2: Reusing program (one-time setup)
	fmt.Println("--- Reusing compiled program ---")
	runtime.GC()
	runtime.GC()

	// One-time setup
	setupBefore := getMemStats()
	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		glog.Exit(err)
	}

	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		glog.Exit(iss.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		glog.Exit(err)
	}
	setupAfter := getMemStats()
	setupCost := safeBytesToKB(setupAfter.Alloc, setupBefore.Alloc)

	// Now measure just the evaluations
	runtime.GC()
	runtime.GC()
	evalBefore := getMemStats()

	for i := 0; i < iterations; i++ {
		_, _, err := prg.Eval(variables)
		if err != nil {
			glog.Exit(err)
		}
	}

	evalAfter := getMemStats()
	totalEvalMem := safeBytesToKB(evalAfter.Alloc, evalBefore.Alloc)
	avgReuse := totalEvalMem / float64(iterations)

	fmt.Printf("Expression: %s\n", expr)
	fmt.Printf("Iterations: %d\n\n", iterations)
	fmt.Printf("Recompile every time:  %8.2f KB (avg per iteration)\n", avgRecompile)
	fmt.Printf("Reuse program:         %8.2f KB (avg per iteration)\n", avgReuse)
	fmt.Printf("One-time setup cost:   %8.2f KB (total for env+compile+program)\n", setupCost)
	if avgRecompile > 0 && avgReuse >= 0 {
		savings := avgRecompile - avgReuse
		if savings > 0 {
			if avgReuse > 0 {
				improvement := avgRecompile / avgReuse
				fmt.Printf("Memory savings:        %8.2f KB (%.1fx improvement)\n", savings, improvement)
			} else {
				fmt.Printf("Memory savings:        %8.2f KB (reuse uses no measurable memory)\n", savings)
			}
		} else {
			fmt.Printf("Memory difference:     %8.2f KB\n", savings)
		}
	}
	fmt.Printf("\n" + strings.Repeat("=", 60) + "\n\n")
}

// Functions to assist with CEL execution.

// compile will parse and check an expression `expr` against a given
// environment `env` and determine whether the resulting type of the expression
// matches the `exprType` provided as input.
func compile(env *cel.Env, expr string, celType *cel.Type) *cel.Ast {
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		glog.Exit(iss.Err())
	}
	if !reflect.DeepEqual(ast.OutputType(), celType) {
		glog.Exitf(
			"Got %v, wanted %v result type", ast.OutputType(), celType)
	}
	fmt.Printf("%s\n\n", strings.ReplaceAll(expr, "\t", " "))
	return ast
}

// eval will evaluate a given program `prg` against a set of variables `vars`
// and return the output, eval details (optional), or error that results from
// evaluation.
func eval(prg cel.Program,
	vars any) (out ref.Val, det *cel.EvalDetails, err error) {
	varMap, isMap := vars.(map[string]any)
	fmt.Println("------ input ------")
	if !isMap {
		fmt.Printf("(%T)\n", vars)
	} else {
		for k, v := range varMap {
			switch val := v.(type) {
			case proto.Message:
				bytes, err := prototext.Marshal(val)
				if err != nil {
					glog.Exitf("failed to marshal proto to text: %v", val)
				}
				fmt.Printf("%s = %s", k, string(bytes))
			case map[string]any:
				b, _ := json.MarshalIndent(v, "", "  ")
				fmt.Printf("%s = %v\n", k, string(b))
			case uint64:
				fmt.Printf("%s = %vu\n", k, v)
			default:
				fmt.Printf("%s = %v\n", k, v)
			}
		}
	}
	fmt.Println()
	out, det, err = prg.Eval(vars)
	report(out, det, err)
	fmt.Println()
	return
}

// report prints out the result of evaluation in human-friendly terms.
func report(result ref.Val, details *cel.EvalDetails, err error) {
	fmt.Println("------ result ------")
	if err != nil {
		fmt.Printf("error: %s\n", err)
	} else {
		fmt.Printf("value: %v (%T)\n", result, result)
	}
	if details != nil {
		fmt.Printf("\n------ eval states ------\n")
		state := details.State()
		stateIDs := state.IDs()
		ids := make([]int, len(stateIDs), len(stateIDs))
		for i, id := range stateIDs {
			ids[i] = int(id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			v, found := state.Value(int64(id))
			if !found {
				continue
			}
			fmt.Printf("%d: %v (%T)\n", id, v, v)
		}
	}
}

// mapContainsKeyValue implements the custom function:
//
//	`map.contains(key, value) bool`.
func mapContainsKeyValue(args ...ref.Val) ref.Val {
	// The declaration of the function ensures that only arguments which match
	// the mapContainsKey signature will be provided to the function.
	m := args[0].(traits.Mapper)

	// CEL has many interfaces for dealing with different type abstractions.
	// The traits.Mapper interface unifies field presence testing on proto
	// messages and maps.
	key := args[1]
	v, found := m.Find(key)

	// If not found and the value was non-nil, the value is an error per the
	// `Find` contract. Propagate it accordingly.
	if !found {
		if v != nil {
			return types.ValOrErr(v, "unsupported key type")
		}
		// Return CEL False if the key was not found.
		return types.False
	}
	// Otherwise whether the value at the key equals the value provided.
	return v.Equal(args[2])
}

// Functions for constructing CEL inputs.

// auth constructs a `google.rpc.context.AttributeContext.Auth` message.
func auth(user string, claims map[string]string) *rpcpb.AttributeContext_Auth {
	claimFields := make(map[string]*structpb.Value)
	for k, v := range claims {
		claimFields[k] = structpb.NewStringValue(v)
	}
	return &rpcpb.AttributeContext_Auth{
		Principal: user,
		Claims:    &structpb.Struct{Fields: claimFields},
	}
}

// request constructs a `google.rpc.context.AttributeContext.Request` message.
func request(auth *rpcpb.AttributeContext_Auth, t time.Time) map[string]any {
	req := &rpcpb.AttributeContext_Request{
		Auth: auth,
		Time: &tpb.Timestamp{Seconds: t.Unix()},
	}
	return map[string]any{"request": req}
}

// valueToJSON converts the CEL type to a protobuf JSON representation and
// marshals the result to a string.
func valueToJSON(val ref.Val) string {
	v, err := val.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
	if err != nil {
		glog.Exit(err)
	}
	marshaller := protojson.MarshalOptions{Indent: "    "}
	bytes, err := marshaller.Marshal(v.(proto.Message))
	if err != nil {
		glog.Exit(err)
	}
	return string(bytes)
}

var (
	emptyClaims = make(map[string]string)
)
