# Tutorial: Using Lua with Go

*A hands-on guide to embedding Lua in a Go application, using Hekate as the worked example.*

---

## Table of Contents

1. [Who this is for](#1-who-this-is-for)
2. [Setting up the environment (Linux)](#2-setting-up-the-environment-linux)
3. [Part 1 — Running Lua from Go](#3-part-1--running-lua-from-go)
4. [Part 2 — Calling Go functions from Lua](#4-part-2--calling-go-functions-from-lua)
5. [Part 3 — Passing data between Go and Lua](#5-part-3--passing-data-between-go-and-lua)
6. [Part 4 — Getting results back from Lua](#6-part-4--getting-results-back-from-lua)
7. [Part 5 — Randomness and reproducibility](#7-part-5--randomness-and-reproducibility)
8. [Part 6 — Putting it together: how Hekate uses Lua](#8-part-6--putting-it-together-how-hekate-uses-lua)
9. [Part 7 — Loading Lua from YAML (the Hekate model file)](#9-part-7--loading-lua-from-yaml-the-hekate-model-file)
10. [Common pitfalls](#10-common-pitfalls)
11. [Further reading](#11-further-reading)

---

## 1. Who this is for

You know a bit of Go and you've heard of Lua. You want to embed Lua in a Go program so that users can write small scripts that drive your application's behaviour — without recompiling your Go binary every time they want to change something.

Everything here is worked through in the context of Hekate, a demographic microsimulation engine. But the techniques are general: any Go program that wants a scripting layer can use them.

**Assumed knowledge:** basic Go (structs, functions, packages, `go run`). No prior Lua knowledge is required — we introduce Lua as we go.

---

## 2. Setting up the environment (Linux)

### 2.1 Install Go

Hekate targets Go 1.21 or later. If you don't have Go installed:

```bash
# Check what you have
go version
```

If you get "command not found" or a version older than 1.21, install from the official tarball:

```bash
# Download (check https://go.dev/dl/ for the current version)
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz

# Remove any old install and extract
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz

# Put it on your PATH (add to ~/.bashrc or ~/.zshrc to make it permanent)
export PATH=$PATH:/usr/local/go/bin
```

Verify:

```bash
go version
# go version go1.21.5 linux/amd64
```

### 2.2 Create a working directory

```bash
mkdir -p ~/lua-go-tutorial
cd ~/lua-go-tutorial
go mod init lua-go-tutorial
```

`go mod init` creates a `go.mod` file. It records your module name and its dependencies, the same way a `package.json` or `requirements.txt` would in other ecosystems.

### 2.3 Add the Lua dependency

We use **gopher-lua**, a pure-Go implementation of Lua 5.1. Because it's pure Go, there's no C library to install, no CGO, and no compiler toolchain beyond Go itself — which is exactly why Hekate chose it.

```bash
go get github.com/yuin/gopher-lua
go mod tidy
```

Your `go.mod` should now list `github.com/yuin/gopher-lua` as a dependency. That's the entire setup. No `apt install lua5.1`, no `liblua-dev`.

> **Why pure-Go matters:** Hekate ships as a single self-contained binary for Linux, macOS and Windows. If gopher-lua needed a native Lua shared library, the build matrix would become far more complicated and users would have to install Lua separately. Pure-Go dependencies keep cross-compilation to a single `GOOS=... GOARCH=... go build` command.

### 2.4 Verify the toolchain works

Create `hello.go`:

```go
package main

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

func main() {
	L := lua.NewState()
	defer L.Close()

	if err := L.DoString(`print("Hello from Lua!")`); err != nil {
		panic(err)
	}
	fmt.Println("Hello from Go!")
}
```

Run it:

```bash
go run hello.go
```

Expected output:

```
Hello from Lua!
Hello from Go!
```

If that works, your environment is ready. Everything else in this tutorial builds on it.

---

## 3. Part 1 — Running Lua from Go

### 3.1 The Lua state

The central concept in gopher-lua is the **Lua state**, represented by `*lua.LState`. Think of it as an isolated Lua interpreter: its own global variables, its own functions, its own stack. Two states don't share anything.

```go
L := lua.NewState()
defer L.Close()
```

`NewLuaVM` in Hekate's `lua_vm.go` does exactly this — it just wraps the result in a `LuaVM` struct so that a dedicated random source can travel alongside it (we'll get to that in Part 5):

```go
type LuaVM struct {
	L      *lua.LState
	random *rand.Rand
	seed   int64
}
```

### 3.2 Running a chunk of Lua

`L.DoString(source)` compiles and runs a Lua chunk:

```go
err := L.DoString(`
    local x = 10
    local y = 20
    print("x + y =", x + y)
`)
```

(`local` means "scoped to this chunk". Without it, the variable would go into the global namespace.)

### 3.3 Running Lua from a file

`L.DoFile(path)` does the same thing but reads the source from disk:

```go
if err := L.DoFile("model.lua"); err != nil {
    log.Fatal(err)
}
```

Hekate doesn't use `DoFile` because it deliberately stores Lua *inline in YAML* — see Part 7. But for a general-purpose scripting layer, `DoFile` is often the right choice.

### 3.4 What "running" actually means

A subtle but important point: `DoString` runs the chunk **immediately**, top to bottom. If the chunk defines functions, those functions are registered in the Lua state but not *called*. That's how Hekate works:

- The chunk is a `function transition(population, params) ... end` declaration.
- `DoString` runs the chunk, which defines the `transition` function as a global.
- Hekate then looks up `transition` and calls it separately, once per iteration.

This separation is what makes the "model as data" pattern work: the chunk *registers* the model's behaviour, and the engine *invokes* it.

---

## 4. Part 2 — Calling Go functions from Lua

This is where things get interesting. A scripting layer is only useful if the script can call back into Go to do real work — logging, arithmetic, file access, whatever.

### 4.1 Registering a Go function

`L.NewFunction` takes a Go function of the form `func(*lua.LState) int` and returns a `*lua.LFunction` that Lua can call. `L.SetGlobal` attaches it to a global name:

```go
L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
	msg := L.ToString(1)         // read the 1st argument
	log.Printf("[Lua] %s", msg)  // do something with it in Go
	return 0                     // we push 0 return values back to Lua
}))
```

Now in Lua:

```lua
log("Hello from Lua")
```

**The convention:** arguments arrive on the Lua stack; `L.ToString(n)`, `L.ToInt(n)`, `L.CheckNumber(n)` etc. read the nth argument. The return value of the wrapper is *how many values you pushed back onto the stack* for Lua to receive.

### 4.2 Returning a value

Here's a function that doubles its argument:

```go
L.SetGlobal("double", L.NewFunction(func(L *lua.LState) int {
	n := L.ToInt(1)
	L.Push(lua.LNumber(n * 2))  // push 1 result onto the stack
	return 1                    // tell Lua we returned 1 value
}))
```

```lua
local r = double(21)
print(r)  -- 42
```

### 4.3 Arguments and the stack

When Lua calls your Go function, its arguments are already on the stack, in order. You can also do `L.GetTop()` to find how many arguments were supplied — useful for variadic functions.

Hekate's `LinearPredict` uses exactly this pattern: it takes an intercept followed by an even number of (coefficient, variable) pairs. It iterates in steps of 2 until it hits the top of the stack:

```go
func (h *HekateStats) LinearPredict(L *lua.LState) int {
	intercept := L.CheckNumber(1)
	prediction := float64(intercept)

	for i := 2; i <= L.GetTop(); i += 2 {
		if i+1 > L.GetTop() {
			break  // odd number of trailing args — stop safely
		}
		coef := L.CheckNumber(i)
		varVal := L.CheckNumber(i + 1)
		prediction += float64(coef) * float64(varVal)
	}

	L.Push(lua.LNumber(prediction))
	return 1
}
```

So from Lua:

```lua
local income = hekate_stats.linear_predict(
    25000,           -- intercept
    500,   age,      -- age effect
    3000,  edu,      -- education effect
    -2000, gender    -- gender effect
)
```

*The number of (coef, var) pairs doesn't have to be known to the Go function in advance — it's discovered at runtime from the stack.*

### 4.4 A note on `Check` vs `To`

- `L.ToString(n)` returns `""` if the argument is missing or not a string.
- `L.CheckString(n)` **raises a Lua error** if the argument is missing or not a string.

Use `To*` for optional arguments and `Check*` for required ones. Hekate uses `Check*` in the statistics functions because passing the wrong number of arguments is a user error that should fail loudly and early.

---

## 5. Part 3 — Passing data between Go and Lua

Numbers and strings are easy. Real applications pass **structured** data: tables, maps, slices, nested objects. Lua and Go have different type systems, so a translation layer is needed.

### 5.1 The mismatch

| Go | Lua | Notes |
|----|-----|-------|
| `bool` | `boolean` | 1:1 |
| `int`, `int64` | `number` | Lua only has one numeric type, a 64-bit float |
| `float64` | `number` | 1:1 |
| `string` | `string` | 1:1 |
| `[]interface{}` | `table` (array) | Go slice becomes a 1-indexed Lua table |
| `map[string]interface{}` | `table` (object) | Go map keys become string fields |
| `nil` | `nil` | 1:1 |

Lua has *only one* compound type: the table. A table can act as an array (numeric indices) or as a record (string keys), or both at once. That's why the conversion code has to inspect keys to decide whether a Lua table should come back to Go as a slice or as a map.

### 5.2 Go → Lua

Hekate's `toLuaValue` in `lua_conversion.go` is a textbook recursive converter:

```go
func toLuaValue(L *lua.LState, val interface{}) lua.LValue {
	switch v := val.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case int:
		return lua.LNumber(v)      // integers become numbers
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		tbl := L.NewTable()
		for _, item := range v {
			tbl.Append(toLuaValue(L, item))  // Append uses 1-based index
		}
		return tbl
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, item := range v {
			tbl.RawSetString(k, toLuaValue(L, item))
		}
		return tbl
	default:
		return lua.LNil
	}
}
```

Two things worth noting:

- `tbl.Append(x)` in gopher-lua automatically uses **1-based indexing** — so Lua sees `{ x, y, z }` as `{ [1]=x, [2]=y, [3]=z }`, matching Lua convention.
- `RawSetString` sets a table field **without** triggering Lua's `__newindex` metatable hooks. This is the safe choice when building tables from Go, because you don't want user Lua code to interfere with the tables the engine is preparing.

### 5.3 Lua → Go

The reverse is `luaValueToGo` in `lua_conversion.go`. The interesting part is deciding whether a Lua table is a list or an object:

```go
case *lua.LTable:
	// A table is treated as a list if all its keys are numeric.
	isList := true
	maxIndex := 0
	v.ForEach(func(key lua.LValue, value lua.LValue) {
		if key.Type() != lua.LTNumber {
			isList = false
		}
		if n, ok := key.(lua.LNumber); ok {
			if int(n) > maxIndex {
				maxIndex = int(n)
			}
		}
	})

	if isList && maxIndex > 0 {
		// Array-style table → []interface{}
		result := make([]interface{}, 0, maxIndex)
		for i := 1; i <= maxIndex; i++ {
			result = append(result, luaValueToGo(v.RawGetInt(i)))
		}
		return result
	}

	// Object-style table → map[string]interface{}
	result := map[string]interface{}{}
	v.ForEach(func(key lua.LValue, value lua.LValue) {
		if key.Type() == lua.LTString {
			result[key.String()] = luaValueToGo(value)
		}
	})
	return result
```

**The heuristic is not perfect.** A Lua table with all-numeric keys is *almost always* meant as an array, but a table like `{ [5]="x", [1]="y" }` would be a sparse array. Hekate sidesteps the ambiguity because its population data is always well-formed: either an array of person-tables, or a person-table with string fields. If your use case involves sparse arrays, you'd want a stricter representation.

### 5.4 Passing the whole population

The Hekate population is a `[]map[string]interface{}` — one map per person, keyed by column name. Passing it to Lua is just a loop:

```go
luaPop := vm.L.NewTable()
for _, person := range population {
	luaPerson := vm.L.NewTable()
	for k, v := range person {
		luaPerson.RawSetString(k, toLuaValue(vm.L, v))
	}
	luaPop.Append(luaPerson)
}
```

Lua then sees:

```lua
{
  { person_id = 1, age = 25, sex = "F", alive = true },
  { person_id = 2, age = 30, sex = "M", alive = true },
  ...
}
```

And can iterate naturally:

```lua
for _, person in ipairs(population) do
    if person.alive then
        person.age = person.age + 1
    end
end
```

Notice that `ipairs` iterates in order and stops at the first `nil`; `pairs` iterates in unspecified order over all keys. For array-like tables, `ipairs` is the correct choice.

---

## 6. Part 4 — Getting results back from Lua

### 6.1 The `transition` contract

Hekate expects every Lua model to define a global function called `transition` that takes the population and a parameters table, and returns the new population:

```lua
function transition(population, params)
    -- mutate population in place, or build a new one
    return population
end
```

This is a **contract**, not a Lua feature. `ExecuteLuaScript` in `lua_vm.go` is what enforces it.

### 6.2 Calling a Lua function from Go

The skeleton, stripped down:

```go
// 1. Push the function, then its arguments, then invoke it.
if err := vm.L.CallByParam(lua.P{
	Fn:      fn,
	NRet:    1,       // we expect 1 return value
	Protect: true,    // catch Lua errors and turn them into Go errors
}, luaPop, luaParams); err != nil {
	return nil, err
}

// 2. The return value is now on top of the stack.
result := vm.L.Get(-1)
vm.L.Pop(1)

// 3. Convert it back to Go.
return luaTableToSlice(result.(*lua.LTable))
```

`CallByParam` is the low-level call API: `lua.P` says "call `fn`, take this many arguments from the stack, expect `NRet` results, and bubble up errors". `Protect: true` is essential — without it, an error inside Lua becomes a Go panic instead of a returned `error`.

### 6.3 The full extraction pattern

Here's the whole of `ExecuteLuaScript`, annotated:

```go
func (vm *LuaVM) ExecuteLuaScript(script string, population []map[string]interface{}, params map[string]interface{}) ([]map[string]interface{}, error) {
	// --- Convert the population to a Lua table ---
	luaPop := vm.L.NewTable()
	for _, person := range population {
		luaPerson := vm.L.NewTable()
		for k, v := range person {
			luaPerson.RawSetString(k, toLuaValue(vm.L, v))
		}
		luaPop.Append(luaPerson)
	}

	// --- Convert the parameters to a Lua table ---
	luaParams := vm.L.NewTable()
	for k, v := range params {
		luaParams.RawSetString(k, toLuaValue(vm.L, v))
	}

	// --- Clear any previous transition function, then expose the data ---
	vm.L.SetGlobal("transition", lua.LNil)
	vm.L.SetGlobal("population", luaPop)
	vm.L.SetGlobal("params", luaParams)

	// --- Run the script (this defines the transition function) ---
	if err := vm.L.DoString(script); err != nil {
		return nil, fmt.Errorf("failed to execute Lua script: %w", err)
	}

	// --- Check the contract ---
	fn := vm.L.GetGlobal("transition")
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("script must define a 'transition' function")
	}

	// --- Call transition(population, params) ---
	if err := vm.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, luaPop, luaParams); err != nil {
		return nil, fmt.Errorf("failed to call transition: %w", err)
	}

	// --- Collect the result ---
	result := vm.L.Get(-1)
	vm.L.Pop(1)

	if result.Type() != lua.LTTable {
		return nil, fmt.Errorf("transition function must return a table, got %s", result.Type())
	}

	return luaTableToSlice(result.(*lua.LTable))
}
```

### 6.4 Two subtleties

**a) `SetGlobal("transition", lua.LNil)` before running the chunk.** If a user supplies a chunk that doesn't define `transition`, we don't want a stale one from a previous model lingering. Clearing first makes the contract check meaningful. This is a subtle but important bug-prevention step.

**b) The same `L` is reused across models and iterations.** Hekate creates *one* `LuaVM` for the whole run (`main.go` → `NewLuaVM(...)` → passed to `RunBulkSimulation`). Each model execution does `DoString(script)`, which re-runs the chunk. Globals like `population`, `params` and `transition` are overwritten each time. Any *other* global a script leaves behind persists into the next model — which is why Hekate scripts are encouraged to use `local` for helpers.

---

## 7. Part 5 — Randomness and reproducibility

This is the part most people get wrong. It's worth taking the time to do it properly, because it's the difference between "the model works" and "the model produces results a reviewer can trust".

### 7.1 The three-layer RNG problem

There are *three* different random-number generators in play, and every single one of them can silently break reproducibility:

1. **Go's global `math/rand`.** Before Go 1.20, `rand.Seed(42)` made the global generator deterministic. **Since Go 1.20, `rand.Seed` is a no-op.** The global generator is auto-seeded from the OS and cannot be made deterministic. Any code that calls `rand.Float64()` directly is irreproducible.
2. **Lua's `math.random`.** In gopher-lua, `math.randomseed(n)` does **not** seed the underlying generator. Calling it has no effect. So you cannot rely on Lua's own random functions for reproducibility either.
3. **The draw functions in `hekate_stats`.** These are written by Hekate and *could* be correct — but only if they use a random source that Hekate controls.

The fix is the same in all three cases: **don't use any generator that you can't control.** Give every Lua VM its own `*rand.Rand` instance, use it exclusively, and reseed it deterministically before every model execution.

### 7.2 The dedicated RNG

The core of the fix is three lines in `lua_vm.go`:

```go
type LuaVM struct {
	L      *lua.LState
	random *rand.Rand   // ← this is the whole trick
	seed   int64
}

func (vm *LuaVM) Seed(seed int64) {
	vm.random = rand.New(rand.NewSource(seed))
	vm.seed = seed
}
```

A `*rand.Rand` created with `rand.New(rand.NewSource(seed))` is completely independent of the global generator. Two `rand.Rand` instances created with the same seed will produce the same sequence, forever. Two created with different seeds will diverge.

### 7.3 Overriding `math.random`

Hekate replaces Lua's `math.random` with a wrapper around `vm.random`:

```go
mathTable := L.GetGlobal("math").(*lua.LTable)

L.SetField(mathTable, "random", L.NewFunction(func(L *lua.LState) int {
	if L.GetTop() >= 1 {
		n := L.ToInt(1)
		if n > 0 {
			L.Push(lua.LNumber(vm.random.Intn(n) + 1))
			return 1
		}
	}
	L.Push(lua.LNumber(vm.random.Float64()))
	return 1
}))
```

Now when a Lua script calls `math.random()`, it doesn't reach Lua's broken generator — it reaches *our* `vm.random`. And because Hekate reseeds `vm.random` before every model, the sequence is fully deterministic.

The same trick reseeds via `math.randomseed` (for users who insist on calling it) and registers a couple of extra globals (`random`, `random_int`) for convenience.

### 7.4 The `_draw` functions

Hekate's statistical toolkit has two categories of function:

- `*_predict` — pure functions that compute a probability or expected value. **No randomness.**
- `*_draw` — sample from a distribution. **Use `vm.random`.**

Compare `LogisticPredict` and `LogisticDraw` in `hekate_stats.go`:

```go
// Deterministic: computes p = 1/(1+e^(-linear))
func (h *HekateStats) LogisticPredict(L *lua.LState) int {
	// ... compute `linear` from args ...
	prob := 1.0 / (1.0 + math.Exp(-linear))
	L.Push(lua.LNumber(prob))
	return 1
}

// Stochastic: computes p, then samples with vm.random
func (h *HekateStats) LogisticDraw(L *lua.LState) int {
	// ... compute `linear` from args ...
	prob := 1.0 / (1.0 + math.Exp(-linear))

	drawn := h.vm.random.Float64() < prob   // ← the ONLY random call
	L.Push(lua.LBool(drawn))
	return 1
}
```

Notice `h.vm.random` — the `HekateStats` struct holds a pointer back to the VM so that draw functions can reach the RNG. That's why `RegisterHekateStats` passes `vm` in when constructing it:

```go
func RegisterHekateStats(L *lua.LState, vm *LuaVM) {
	stats := &HekateStats{vm: vm}
	// ...
}
```

### 7.5 The seed formula

If you reseed before every model execution with the *same* seed, all models would draw identical random sequences — which is not what you want, because independent models should have independent randomness.

Hekate uses a spread-out formula:

```
seed = base_seed + (iteration × 1000) + model_index
```

In `simulation_bulk.go`:

```go
modelSeed := config.Simulation.RandomSeed + int64(i)*1000 + int64(modelIndex)
vm.Seed(modelSeed)
```

The `× 1000` gives up to 1000 models per iteration without overlap. The `+ modelIndex` gives each model in a given year a different seed. The base seed (from `config.yaml`) anchors the whole run.

**Why this matters:** the same config, run twice, produces byte-for-byte identical output — regardless of machine, time of day, or order in which the OS schedules goroutines. This is the reproducibility guarantee. It only works because *every* random call routes through `vm.random`.

### 7.6 What breaks reproducibility

The three things Hekate's documentation warns about are all direct consequences of §7.1:

- **Global `math/rand`**: auto-seeded, uncontrollable.
- **`math.randomseed()` in user scripts**: either a no-op (Lua's) or a reseeding of `vm.random` to an arbitrary user value, breaking the engine's carefully-derived sequence.
- **`os.time()` or any other external state**: the output can never be reproduced by someone running the same config tomorrow.

Hekate's *pre-flight validator* (`lua_validate.go`) catches the `math.randomseed` mistake automatically by grepping the script for that string:

```go
if strings.Contains(script, "math.randomseed") {
	warnings = append(warnings,
		"script calls math.randomseed() — this is unnecessary and may break reproducibility. Remove the call.")
}
```

A user still gets a warning, but the run isn't aborted — a pragmatic compromise (it's a warning, not an error).

---

## 8. Part 6 — Putting it together: how Hekate uses Lua

At this point you have all the pieces. Here they are assembled, in the order the engine actually uses them:

### 8.1 Startup — once per run

```go
// main.go
config := LoadConfig("config.yaml")                 // parse YAML
if err := config.Validate(); err != nil { ... }     // check required fields

enabledModels := FilterEnabledModels(config.Models) // drop disabled models
SortModelsByPriority(enabledModels)                 // lowest priority first

ValidateAllScripts(...)                             // compile each chunk
ValidateAllColumnsAgainstCSV(...)                   // scan for person.foo refs

luaVM := NewLuaVM(config.Simulation.RandomSeed)     // one VM for the run
defer luaVM.Close()
```

### 8.2 Per iteration, per model — the inner loop

```go
// simulation_bulk.go
for i := 0; i < iterations; i++ {                    // for each year
	for modelIndex, model := range enabledModels {   // for each model
		modelSeed := baseSeed + int64(i)*1000 + int64(modelIndex)
		vm.Seed(modelSeed)                           // reset RNG deterministically

		population, err = ExecuteLuaModel(
			vm, model, population, verbose, modelSeed,
		)
	}
}
```

### 8.3 Inside `ExecuteLuaModel` — one model, one iteration

```go
// simulation_bulk.go
script := model.Parameters["script"].(string)        // the Lua source
vm.Seed(seed)                                        // RNG ready

result, err := vm.ExecuteLuaScript(
	script, []map[string]interface{}(population), model.Parameters,
)
return Population(result), nil
```

### 8.4 Inside `ExecuteLuaScript` — the Lua boundary

Already shown in full in §6.3. Every piece you've seen so far is load-bearing:

- The `map[string]interface{}` representation of a person is what allows arbitrary CSV schemas.
- `toLuaValue` is what makes that map visible to Lua.
- `NewFunction` and `RegisterHekateStats` are what give Lua access to Go's statistical toolbox.
- `CallByParam` with `Protect: true` is what makes user errors Go errors.
- `vm.Seed` is what makes the whole thing reproducible.

Remove any one of these and the engine stops working properly.

---

## 9. Part 7 — Loading Lua from YAML (the Hekate model file)

We deliberately left this until last, because it's *not* really about Lua — it's about the delivery mechanism for Lua code. The tutorial in the earlier draft started here and it made the conversation confusing: YAML first, Lua later. This section is the correct place for it: once you understand how Lua and Go talk to each other, the question "where does the Lua source come from?" has an easy answer.

### 9.1 The literal block scalar

Hekate stores Lua as a YAML literal block scalar:

```yaml
models:
  - name: "aging"
    type: "lua_model"
    priority: 1
    enabled: true
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive == true or person.alive == "true" then
              person.age = (tonumber(person.age) or 0) + 1
            end
          end
          return population
        end
```

The `|` after `script:` starts a literal block: everything indented *more* than the `|` is treated as a single string, preserving newlines. YAML hands that string to Go as `model.Parameters["script"]`. From that point on, it is indistinguishable from any other Lua source you might have loaded.

### 9.2 The indentation gotcha

This is the one thing everyone trips over. **The indentation of the first line after `|` defines the block.** A later line that dedents below that indentation ends the block.

```yaml
script: |
  function transition(pop, p)
    return pop
  end        # ← still inside
end          # ← still inside, at the block's baseline
```

```yaml
script: |
  function transition(pop, p)
    return pop
  end
end_of_file: # ← this dedent ends the block
```

The first line's indentation is *context*, not content — YAML strips it from every line. Lua itself does not care about indentation, so any consistent indentation works. What matters is that nothing in the block dedents below the first content line.

**If you get a strange "could not find expected ':'" error on a `transition` line**, this is almost certainly why.

### 9.3 `params` — getting YAML values into Lua

Anything under `parameters:` other than `script` becomes a key in the `params` table that Lua receives:

```yaml
parameters:
  birth_rate: 0.04
  min_age: 15
  max_age: 50
  script: |
    function transition(population, params)
      local rate   = params.birth_rate   -- 0.04
      local minage = params.min_age      -- 15
      -- ...
    end
```

That's the mechanism: `ExecuteLuaScript` builds a Lua table from `model.Parameters`, sets it as `params`, and passes it as the second argument to `transition`. It's the clean pattern. (An earlier version of the README described a `{birth_rate}` substitution syntax; that has been superseded — the `params` table is what the engine implements.)

### 9.4 Helper functions in the same chunk

You don't have to fit everything into `transition`. You can define helpers in the same block:

```yaml
script: |
  -- module-level lookup table, immutable, shared across iterations
  local AGE_BANDS = { [0]="infant", [1]="child", [2]="adult", [3]="elder" }

  local function band_for(age)
    if age < 1   then return 0
    elseif age < 18 then return 1
    elseif age < 65 then return 2
    else              return 3
    end
  end

  function transition(population, params)
    for _, person in ipairs(population) do
      local band = band_for(tonumber(person.age) or 0)
      person.band = AGE_BANDS[band]
    end
    return population
  end
```

Three rules apply:

- **`local function` must be defined *before* use.** Lua does not hoist local functions.
- **`function foo()` (global) is hoisted** at the top of the chunk — usable anywhere in the chunk, at the cost of polluting the global namespace shared with other models.
- **Module-level locals persist between iterations.** They are evaluated once, when the chunk is first run, and then kept alive. This is useful for immutable lookup tables. It is a reproducibility trap if you mutate them — because on the *second* run of the chunk, they already have the values left over from the first. **Rule: module-level locals stay immutable; mutable state goes inside `transition`.**

### 9.5 When the script gets long

Past a few hundred lines, a YAML literal block starts to feel heavy: the config file becomes mostly Lua, and it's harder to navigate. Hekate's suggestion is to split the model into several models in the `models:` list rather than one large block. Each model runs on its own, has its own seed, and is easier to test in isolation.

You cannot, however, put Lua in separate `.lua` files — the Hekate engine deliberately doesn't load them. No `require`, no `dofile`, no file I/O. Everything goes in the config. This is a deliberate design decision: the config file *is* the model, and it's the only thing you need to hand a colleague to reproduce a run.

---

## 10. Common pitfalls

A short list of things that bit people in the earlier tutorials, or that are easy to miss:

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| `person.agge` (typo) | Run fails at year 47 | Pre-flight validator catches this; read the suggestion in the error (`did you mean 'age'?`) |
| Not wrapping numeric columns in `tonumber()` | Arithmetic silently yields `0` or errors | CSV values are loaded as strings; always `tonumber(person.X)` |
| YAML dedent below the block baseline | "could not find expected ':'" | Keep every line of a literal block indented at least as far as the first |
| `math.randomseed()` in user code | Intermittent run-to-run differences | Remove it — Hekate seeds for you |
| Mutating a module-level local | Results differ between runs | Mutable state goes inside `transition` |
| `local function foo()` used before its definition | `attempt to call nil` | Move the definition above the call, or forward-declare with `local foo` |
| Calling `math.random` before any model | Small deviations in output | Never call `math.random` outside `transition`; the engine seeds right before each model, not before the chunk |
| Passing too few args to a `hekate_stats.*` function | "Lua error: argument #5 …" | Count the pairs — each is `(coef, var)` |

---

## 11. Further reading

- **[gopher-lua documentation](https://github.com/yuin/gopher-lua)** — the library's own README, examples and reference.
- **[Lua 5.1 manual](https://www.lua.org/manual/5.1/)** — gopher-lua implements Lua 5.1; this is the reference for the language semantics.
- **Hekate's `lua_vm.go`** — the full, commented implementation of the wrapper around `lua.LState`. Worth reading end to end.
- **Hekate's `hekate_stats.go`** — worked examples of Go functions exposed to Lua, including both pure-predict and stochastic-draw variants.
- **Hekate's `runtime_validate.go`** — an example of scanning Lua source code with regular expressions to catch configuration mistakes. A nice pattern for any project that accepts user-supplied scripts.
- **Hekate's `llm_prompt_template.md`** — a prompt template for asking an AI assistant to write Hekate models. Useful as a realistic example of what a good context block for script generation looks like.

---

*This tutorial accompanies the Hekate codebase. Where behaviour described here differs from the code, the code is authoritative — open an issue on the repository and we'll reconcile.*
