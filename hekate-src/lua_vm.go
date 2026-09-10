// ============================================================================
// lua_vm.go - Lua VM with Dedicated Random Source
// ============================================================================
//
// CRITICAL SECTION FOR REPRODUCIBILITY
//
// This file is the heart of Hekate's reproducibility guarantee. Read
// carefully before modifying.
//
// BACKGROUND
// ----------
// In Go 1.20+, math/rand.Seed() became a no-op. The global random generator
// is now auto-seeded with time and CANNOT be made deterministic. Any code
// using the global `rand` package produces different results each run.
//
// Additionally, gopher-lua's `math.randomseed()` is a no-op - it does not
// actually seed Lua's internal RNG. So we can't rely on Lua's native
// randomness either.
//
// SOLUTION
// --------
// Each LuaVM owns its own *rand.Rand instance (vm.random). We override Lua's
// `math.random` to use this instance, and we override `math.randomseed` to
// reseed it. Before each model execution, we call vm.Seed() to reset the RNG
// to a known state derived from the config's random_seed, the current
// iteration, and the model index.
//
// This gives us:
//   - Full reproducibility across runs
//   - Independence between models (each gets a different seed)
//   - No dependency on global state or system time
//
// ============================================================================

package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// LuaVM wraps a Lua interpreter state together with its dedicated random
// number generator. The `random` field is THE critical piece for
// reproducibility.
type LuaVM struct {
	L      *lua.LState // The Lua interpreter
	random *rand.Rand  // Dedicated RNG - NOT the global one!
	seed   int64       // The current seed (for reference/debugging)
}

// Seed resets the VM's random source to a new state determined by `seed`.
//
// Called before each model execution with a derived seed, this ensures the
// model starts with a known, reproducible random state. Two calls with the
// same seed will produce identical subsequent random sequences.
func (vm *LuaVM) Seed(seed int64) {
	vm.random = rand.New(rand.NewSource(seed))
	vm.seed = seed
}

// NewLuaVM creates a new Lua VM with Hekate-specific functions.
//
// The critical work here is:
//  1. Creating a DEDICATED random source (not using the global rand)
//  2. Overriding Lua's math.random to use that source
//  3. Overriding Lua's math.randomseed to reseed that source
//  4. Registering all the helper functions that Lua scripts can call
func NewLuaVM(randomSeed int64) *LuaVM {
	L := lua.NewState()

	// Determine the initial seed
	var seed int64
	if randomSeed > 0 {
		seed = randomSeed
	} else {
		seed = time.Now().UnixNano()
	}

	// Create the VM with a DEDICATED random source.
	vm := &LuaVM{
		L:      L,
		random: rand.New(rand.NewSource(seed)),
		seed:   seed,
	}

	// ========================================================================
	// Override math.random to use OUR dedicated RNG
	// ========================================================================
	mathTable := L.GetGlobal("math").(*lua.LTable)

	L.SetField(mathTable, "random", L.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		if top >= 1 {
			n := L.ToInt(1)
			if n > 0 {
				// math.random(n) -> integer in [1, n]
				L.Push(lua.LNumber(vm.random.Intn(n) + 1))
				return 1
			} else if n == 0 {
				// math.random(0) is a Lua edge case - return 0
				L.Push(lua.LNumber(0.0))
				return 1
			}
		}
		// math.random() -> float in [0, 1)
		L.Push(lua.LNumber(vm.random.Float64()))
		return 1
	}))

	// Override math.randomseed to reseed OUR random source.
	L.SetField(mathTable, "randomseed", L.NewFunction(func(L *lua.LState) int {
		newSeed := int64(L.ToInt(1))
		vm.Seed(newSeed)
		return 0
	}))

	// ========================================================================
	// Register Go random functions as alternatives
	// ========================================================================
	L.SetGlobal("random", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(vm.random.Float64()))
		return 1
	}))

	L.SetGlobal("random_int", L.NewFunction(func(L *lua.LState) int {
		min := int(L.ToInt(1))
		max := int(L.ToInt(2))
		if max <= min {
			max = min + 1
		}
		L.Push(lua.LNumber(vm.random.Intn(max-min) + min))
		return 1
	}))

	// ========================================================================
	// Register utility functions for Lua scripts
	// ========================================================================

	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		msg := L.ToString(1)
		log.Printf("[Lua] %s", msg)
		return 0
	}))

	// current_year() - WARNING: depends on system time, breaks reproducibility
	L.SetGlobal("current_year", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().Year()))
		return 1
	}))

	L.SetGlobal("table_contains", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		val := L.CheckAny(2)
		found := false
		tbl.ForEach(func(key lua.LValue, value lua.LValue) {
			if value == val {
				found = true
			}
		})
		L.Push(lua.LBool(found))
		return 1
	}))

	// ========================================================================
	// Register hekate_stats functions
	// ========================================================================
	// We pass vm so that the draw functions can access vm.random for
	// reproducibility.
	RegisterHekateStats(L, vm)

	// Clear any stale global state
	L.SetGlobal("transition", lua.LNil)

	return vm
}

// Close releases the resources held by the Lua VM.
func (vm *LuaVM) Close() {
	vm.L.Close()
}

// ExecuteLuaScript runs a Lua script and returns the transformed population.
//
// The script MUST define a function called `transition(population, params)`.
func (vm *LuaVM) ExecuteLuaScript(script string, population []map[string]interface{}, params map[string]interface{}) ([]map[string]interface{}, error) {
	// Convert Go population to Lua table
	luaPop := vm.L.NewTable()
	for _, person := range population {
		luaPerson := vm.L.NewTable()
		for k, v := range person {
			luaPerson.RawSetString(k, toLuaValue(vm.L, v))
		}
		luaPop.Append(luaPerson)
	}

	// Convert params to Lua table
	luaParams := vm.L.NewTable()
	for k, v := range params {
		luaParams.RawSetString(k, toLuaValue(vm.L, v))
	}

	// Clear previous transition function and set globals
	vm.L.SetGlobal("transition", lua.LNil)
	vm.L.SetGlobal("population", luaPop)
	vm.L.SetGlobal("params", luaParams)

	// Execute the script (defines the transition function)
	if err := vm.L.DoString(script); err != nil {
		return nil, fmt.Errorf("failed to execute Lua script: %w", err)
	}

	// Verify the script defined a transition function
	fn := vm.L.GetGlobal("transition")
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("script must define a 'transition' function")
	}

	// Call transition(population, params)
	if err := vm.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, luaPop, luaParams); err != nil {
		return nil, fmt.Errorf("failed to call transition: %w", err)
	}

	// Get the return value
	result := vm.L.Get(-1)
	vm.L.Pop(1)

	// Validate the return value type
	if result.Type() != lua.LTTable {
		return nil, fmt.Errorf("transition function must return a table, got %s", result.Type())
	}

	// Convert Lua table back to Go slice
	resultPop, err := luaTableToSlice(result.(*lua.LTable))
	if err != nil {
		return nil, fmt.Errorf("failed to convert result: %w", err)
	}
	return resultPop, nil
}

// ============================================================================
// Hekate Statistics Functions Registration
// ============================================================================
// These are registered as methods under the `hekate_stats` table in Lua.
//
// The vm parameter is stored inside HekateStats so that draw functions can
// access the dedicated random source (vm.random) for reproducibility.

// RegisterHekateStats registers all statistical functions under hekate_stats.
func RegisterHekateStats(L *lua.LState, vm *LuaVM) {
	stats := &HekateStats{vm: vm}
	statsTable := L.NewTable()

	// Prediction functions (no randomness)
	L.SetField(statsTable, "linear_predict", L.NewFunction(stats.LinearPredict))
	L.SetField(statsTable, "linear_predict_default", L.NewFunction(stats.LinearPredictDefault))
	L.SetField(statsTable, "logistic_predict", L.NewFunction(stats.LogisticPredict))
	L.SetField(statsTable, "poisson_predict", L.NewFunction(stats.PoissonPredict))
	L.SetField(statsTable, "softmax_predict", L.NewFunction(stats.SoftmaxPredict))

	// Draw functions (use vm.random for reproducibility)
	L.SetField(statsTable, "logistic_draw", L.NewFunction(stats.LogisticDraw))
	L.SetField(statsTable, "categorical_draw", L.NewFunction(stats.CategoricalDraw))
	L.SetField(statsTable, "poisson_draw", L.NewFunction(stats.PoissonDraw))

	L.SetGlobal("hekate_stats", statsTable)
}