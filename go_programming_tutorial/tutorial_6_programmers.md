# Hekate - Complete Code Walkthrough with Mini Demos

## Table of Contents
1. [The Big Picture - Architecture Overview](#the-big-picture)
2. [Configuration Structures](#configuration-structures)
3. [The Lua VM and Integration](#the-lua-vm-and-integration)
4. [Lua Value Conversion - The Magic Bridge](#lua-value-conversion-the-magic-bridge)
5. [Population Loading](#population-loading)
6. [Model Execution](#model-execution)
7. [The Main Loop](#the-main-loop)
8. [Saving Results](#saving-results)
9. [Helper Functions](#helper-functions)
10. [Design Patterns and Philosophy](#design-patterns-and-philosophy)

---

## The Big Picture - Architecture Overview

Before we dive into code, let's understand what Hekate does at a high level:

```
┌─────────────────────────────────────────────────────────────────┐
│                         Hekate Engine                          │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐     │
│  │  Load CSV    │ -> │  Execute     │ -> │  Save CSV    │     │
│  │  Population  │    │  Models      │    │  Results     │     │
│  └──────────────┘    └──────────────┘    └──────────────┘     │
│         │                   │                   │              │
│         ▼                   ▼                   ▼              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐     │
│  │  CSV Parser  │    │  Lua VM      │    │  CSV Writer  │     │
│  │  (encoding)  │    │  (gopher-lua)│    │  (encoding)  │     │
│  └──────────────┘    └──────────────┘    └──────────────┘     │
│                                                                 │
│  Data flows as: CSV → Go structs → Lua tables → Go structs → CSV│
└─────────────────────────────────────────────────────────────────┘
```

**Key Insight:** Hekate is essentially a **pipeline** that transforms CSV data through Lua scripts. The Go code provides the infrastructure (loading, saving, orchestration), and Lua provides the model logic (how people age, die, move, etc.).

---

## Mini Demo 1: Understanding map[string]interface{}

Before we dive into the main code, let's understand one of the most important concepts in Hekate: `map[string]interface{}`. This is how we handle dynamic data without knowing the structure in advance.

```go
// demo1_map_yaml.go
package main

import (
	"fmt"
	"log"

	"gopkg.in/yaml.v3"
)

func main() {
	// This is a YAML configuration as a string (no file needed!)
	yamlConfig := `
simulation:
  iterations: 10
  population_file: "population.csv"
  random_seed: 42

models:
  - name: "age_increment"
    priority: 1
    enabled: true
    parameters:
      rate: 1.0
      query: "UPDATE population SET age = age + 1"
  
  - name: "mortality"
    priority: 2
    enabled: false
    parameters:
      mortality_rates:
        infant: 0.005
        adult: 0.001
      script: |
        function transition()
          -- Lua code here
        end
`

	// Parse the YAML into a generic map
	// This is the key: map[string]interface{} can hold ANYTHING!
	var config map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	if err != nil {
		log.Fatal("Failed to parse YAML:", err)
	}

	// Let's explore what we got
	fmt.Println("=== Parsed YAML ===")
	fmt.Printf("Top-level keys: %v\n\n", getKeys(config))

	// Access the simulation section
	simulation := config["simulation"].(map[string]interface{})
	fmt.Println("Simulation parameters:")
	for key, value := range simulation {
		fmt.Printf("  %s: %v (type: %T)\n", key, value, value)
	}

	fmt.Println("\nModels:")
	models := config["models"].([]interface{})
	for i, model := range models {
		modelMap := model.(map[string]interface{})
		fmt.Printf("  Model %d: %s (priority: %v)\n",
			i+1,
			modelMap["name"],
			modelMap["priority"],
		)

		// Get parameters (which is also a map[string]interface{})
		if params, ok := modelMap["parameters"].(map[string]interface{}); ok {
			fmt.Println("    Parameters:")
			for key, value := range params {
				fmt.Printf("      %s: %v (type: %T)\n", key, value, value)
			}
		}
	}

	// The magic of map[string]interface{}
	fmt.Println("\n=== The Power of map[string]interface{} ===")
	fmt.Println("We can access ANY key without knowing it in advance!")
	
	// Dynamically check for a key
	keyToCheck := "random_seed"
	if sim, ok := config["simulation"].(map[string]interface{}); ok {
		if value, exists := sim[keyToCheck]; exists {
			fmt.Printf("  Found '%s': %v\n", keyToCheck, value)
		} else {
			fmt.Printf("  '%s' not found\n", keyToCheck)
		}
	}
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
```

**Run it:**
```bash
go run demo1_map_yaml.go
```

**Key Takeaways:**
- `map[string]interface{}` can hold ANY type
- We don't need to define structs for every possible configuration
- Users can add new parameters without code changes

---

## Configuration Structures

Now let's look at how Hekate structures its configuration.

```go
// ModelConfig represents a single model definition in YAML
type ModelConfig struct {
    Name        string                 `yaml:"name"`
    Type        string                 `yaml:"type"`
    Priority    int                    `yaml:"priority"`
    Enabled     bool                   `yaml:"enabled"`
    Description string                 `yaml:"description"`
    Parameters  map[string]interface{} `yaml:"parameters"`
}
```

**Explanation:**

This struct represents one model defined in your `config.yaml` file. Let's look at a real example:

```yaml
- name: "mortality"
  type: "lua_model"
  priority: 2
  enabled: true
  description: "Age-specific mortality model"
  parameters:
    mortality_rates:
      infant: 0.005
      child: 0.0005
    script: |
      function transition(population, params)
        -- Lua code here
      end
```

**Key fields:**

- **Name**: A human-readable identifier. This is used for logging and debugging.
- **Type**: Currently only `"lua_model"` is supported. This is a design choice - we could add other types later (e.g., `"python_model"`).
- **Priority**: Controls execution order. Lower numbers run first. This is crucial because models have dependencies (age before mortality, mortality before fertility).
- **Enabled**: Allows you to turn models on/off without removing them from the config.
- **Description**: Documentation that appears in logs.
- **Parameters**: A flexible map that can contain anything. This is where we put the Lua script and any model-specific data (rates, probabilities, etc.).

**Why `map[string]interface{}`?**

This is Go's way of saying "I don't know what the keys will be." It's a map where the keys are strings and the values can be anything. This is how we support arbitrary model parameters without changing the code.

```go
// SimulationConfig holds the full simulation configuration
type SimulationConfig struct {
    Simulation SimulationParameters `yaml:"simulation"`
    Models     []ModelConfig        `yaml:"models"`
}
```

**Explanation:**

This is the top-level configuration. It has two sections:
1. **Simulation**: Global settings (iterations, file paths, etc.)
2. **Models**: A list of individual model configurations.

```go
// SimulationParameters holds simulation-level settings
type SimulationParameters struct {
    Iterations     int    `yaml:"iterations"`
    PopulationFile string `yaml:"population_file"`
    OutputFile     string `yaml:"output_file"`
    RandomSeed     int64  `yaml:"random_seed"`
    Verbose        bool   `yaml:"verbose"`
    IDColumn       string `yaml:"id_column"` // REQUIRED: Primary key and ordering
}
```

**Explanation:**

These are the global simulation settings:

- **Iterations**: How many years to simulate.
- **PopulationFile**: Where to read the starting population from.
- **OutputFile**: Where to save the final population.
- **RandomSeed**: For reproducibility - same seed = same results.
- **Verbose**: Whether to print detailed execution logs.
- **IDColumn**: Which column is the unique identifier for each person.

**The ID Column is Critical!**

```go
IDColumn string `yaml:"id_column"` // REQUIRED: Primary key and ordering
```

Why is this required? Imagine you have a population with 1,000 people. Without unique IDs, you couldn't:
- Track specific individuals over time
- Link children to parents (mother_id, father_id)
- Identify partners (partner_id)
- Ensure consistent output ordering

Think of it like a database primary key - it must be unique for each person.

```go
// ColumnInfo stores metadata about a column
type ColumnInfo struct {
    Name string
    Type string // "int", "string", "bool"
}
```

**Explanation:**

When Hekate loads your CSV, it needs to know the structure. This struct stores:
- **Name**: The column name from the CSV header (e.g., "age", "sex")
- **Type**: The data type detected from the data ("int", "string", "bool")

This is used to properly convert CSV text values into Go types.

```go
// Population is a slice of maps - fully dynamic!
type Population []map[string]interface{}
```

**Explanation:**

This is the core data structure! `Population` is just a slice of maps. Each map represents one person, with keys as column names and values as the data.

**Why use `[]map[string]interface{}` instead of a struct?**

| Approach | Pros | Cons |
|----------|------|------|
| **Struct** | Type-safe, fast, memory efficient | Must define all columns upfront - can't handle new columns without code changes |
| **Map** | Dynamic - handles any CSV structure, no code changes for new columns | Slower, more memory, type assertions needed |

Hekate chooses **maps** because we want to handle any CSV structure without recompiling. A user can add a `"smoking_status"` column to their CSV and immediately use it in Lua scripts.

---

## Mini Demo 2: Type Assertions and Safe Access

When working with `map[string]interface{}`, we need to safely extract values. This demo shows how.

```go
// demo2_type_assertions.go
package main

import (
	"fmt"
	"log"

	"gopkg.in/yaml.v3"
)

func main() {
	// A YAML configuration with various types
	yamlConfig := `
settings:
  name: "Hekate Demo"
  version: 2.0
  enabled: true
  count: 42
  rates:
    low: 0.01
    medium: 0.05
    high: 0.10
  tags: ["demo", "test", "simple"]
`

	var config map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	if err != nil {
		log.Fatal("Failed to parse YAML:", err)
	}

	settings := config["settings"].(map[string]interface{})

	// Safe way to extract values with type checking
	fmt.Println("=== Safe Type Assertions ===")

	// String
	if name, ok := settings["name"].(string); ok {
		fmt.Printf("  name: %s (string)\n", name)
	} else {
		fmt.Println("  name: not a string")
	}

	// Float64 (YAML numbers become float64 by default)
	if version, ok := settings["version"].(float64); ok {
		fmt.Printf("  version: %.1f (float64)\n", version)
	} else {
		fmt.Println("  version: not a float64")
	}

	// Bool
	if enabled, ok := settings["enabled"].(bool); ok {
		fmt.Printf("  enabled: %v (bool)\n", enabled)
	} else {
		fmt.Println("  enabled: not a bool")
	}

	// Int (needs conversion from float64)
	if countFloat, ok := settings["count"].(float64); ok {
		count := int(countFloat)
		fmt.Printf("  count: %d (int, converted from float64)\n", count)
	} else {
		fmt.Println("  count: not a number")
	}

	// Nested map
	if rates, ok := settings["rates"].(map[string]interface{}); ok {
		fmt.Println("  rates:")
		for k, v := range rates {
			fmt.Printf("    %s: %v\n", k, v)
		}
	} else {
		fmt.Println("  rates: not a map")
	}

	// Slice
	if tags, ok := settings["tags"].([]interface{}); ok {
		fmt.Println("  tags:")
		for _, tag := range tags {
			fmt.Printf("    %v\n", tag)
		}
	} else {
		fmt.Println("  tags: not a slice")
	}

	// What happens when we try to get a key that doesn't exist?
	fmt.Println("\n=== Accessing Missing Keys ===")
	missing := settings["nonexistent"]
	if missing == nil {
		fmt.Println("  Missing key returns nil (no panic!)")
	} else {
		fmt.Printf("  Got: %v\n", missing)
	}
}
```

**Key Takeaways:**
- Always use `value, ok := map[key].(type)` for safe access
- Missing keys return `nil`, not an error
- YAML numbers become `float64` by default

---

## The Lua VM and Integration

This is the heart of Hekate - how we embed Lua and make it work with Go.

```go
// LuaVM wraps the Lua interpreter with methods for Hekate
type LuaVM struct {
    L *lua.LState
}
```

**Explanation:**

`LuaVM` is a wrapper around `lua.LState` (the Lua interpreter state). We wrap it to add our own methods and functions.

Think of `lua.LState` as a Lua virtual machine. Each instance has its own global environment, variables, and functions. If you create multiple instances, they don't share state.

```go
// NewLuaVM creates a new Lua VM with Hekate-specific functions
func NewLuaVM(randomSeed int64) *LuaVM {
    L := lua.NewState()

    // Seed Lua's random number generator for reproducibility
    if randomSeed > 0 {
        err := L.DoString(fmt.Sprintf("math.randomseed(%d)", randomSeed))
        if err != nil {
            log.Printf("Warning: Failed to seed Lua random: %v", err)
        }
    } else {
        err := L.DoString(fmt.Sprintf("math.randomseed(%d)", time.Now().UnixNano()))
        if err != nil {
            log.Printf("Warning: Failed to seed Lua random: %v", err)
        }
    }
    // ... rest of initialization
}
```

**Explanation:**

This creates a new Lua interpreter. The `randomSeed` parameter is crucial for reproducibility.

**Why seed Lua's random generator?**

Lua has its own random number generator (`math.random()`). If we don't seed it, it uses a default seed, which means results won't be reproducible. By seeding it with the same value as Go's `rand.Seed()`, we ensure that both Go and Lua produce the same random sequence each run.

```go
// Register Hekate-specific functions
L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
    msg := L.ToString(1)
    log.Printf("[Lua] %s", msg)
    return 0
}))
```

**Explanation:**

This registers a Go function called `log` that can be called from Lua. The `L.NewFunction` creates a function that Lua can call, and the closure defines what Go code runs.

**Understanding `L.NewFunction`:**

```go
func(L *lua.LState) int {
    // L is the Lua state (the interpreter)
    // The function returns an int (number of return values pushed to Lua stack)
}
```

When Lua calls `log("Hello world")`:
1. Lua pushes the string "Hello world" onto the Lua stack
2. Our Go function is called with `L` as the parameter
3. `L.ToString(1)` gets the first argument from the stack
4. We print it with Go's `log.Printf`
5. We return 0 (no values to push back to Lua)

---

## Mini Demo 3: Registering Lua Functions

This demo shows how to register Go functions for Lua to call.

```go
// demo5_register_functions.go
package main

import (
	"fmt"
	"log"
	"math/rand"

	lua "github.com/yuin/gopher-lua"
)

func main() {
	L := lua.NewState()
	defer L.Close()

	// Register a Go function that Lua can call
	L.SetGlobal("greet", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		fmt.Printf("Hello, %s! (from Go)\n", name)
		return 0
	}))

	// Register a Go function that returns a value
	L.SetGlobal("double", L.NewFunction(func(L *lua.LState) int {
		num := L.ToInt(1)
		L.Push(lua.LNumber(num * 2))
		return 1
	}))

	// Register a Go function that uses random
	L.SetGlobal("random_number", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(rand.Float64()))
		return 1
	}))

	// Lua script that calls our Go functions
	script := `
		print("Calling Go functions from Lua:")
		greet("Lua User")
		
		local result = double(21)
		print("21 doubled is", result)
		
		local r = random_number()
		print("Random number:", r)
		
		print("Done!")
	`

	err := L.DoString(script)
	if err != nil {
		log.Fatal("Lua error:", err)
	}
}
```

**Key Takeaways:**
- `L.NewFunction` wraps a Go function for Lua
- Lua can call Go functions with arguments
- Go functions can return values to Lua
- The return value of the wrapper function is the number of return values pushed to Lua

---

## Lua Value Conversion - The Magic Bridge

This is where Go and Lua data structures meet. Since Go and Lua have different type systems, we need to convert between them.

```go
// toLuaValue converts Go values to Lua values
func toLuaValue(L *lua.LState, val interface{}) lua.LValue {
    switch v := val.(type) {
    case nil:
        return lua.LNil
    case bool:
        return lua.LBool(v)
    case int:
        return lua.LNumber(v)
    case int64:
        return lua.LNumber(v)
    case float64:
        return lua.LNumber(v)
    case string:
        return lua.LString(v)
    case []interface{}:
        tbl := L.NewTable()
        for _, item := range v {
            tbl.Append(toLuaValue(L, item))
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

**Explanation:**

This function converts any Go value to the corresponding Lua value. It's a type switch that handles all the cases we support.

**Visual Mapping:**

| Go Type | Lua Type | Notes |
|---------|----------|-------|
| `nil` | `nil` | `lua.LNil` |
| `bool` | `boolean` | `lua.LBool` |
| `int`, `int64`, `float64` | `number` | `lua.LNumber` |
| `string` | `string` | `lua.LString` |
| `[]interface{}` | `table` (array-style) | Items become sequential indices |
| `map[string]interface{}` | `table` (object-style) | Keys become string properties |

**Why this matters:**

When we pass a Go population to Lua, we need to convert this:
```go
[]map[string]interface{}{
    {"person_id": 1, "age": 25, "sex": "F"},
    {"person_id": 2, "age": 30, "sex": "M"},
}
```

Into this Lua table:
```lua
{
    { person_id = 1, age = 25, sex = "F" },
    { person_id = 2, age = 30, sex = "M" }
}
```

```go
// luaTableToSlice converts a Lua table to a Go slice of maps
func luaTableToSlice(tbl *lua.LTable) ([]map[string]interface{}, error) {
    var result []map[string]interface{}

    tbl.ForEach(func(key lua.LValue, value lua.LValue) {
        if tblVal, ok := value.(*lua.LTable); ok {
            row := make(map[string]interface{})
            tblVal.ForEach(func(k lua.LValue, v lua.LValue) {
                if k.Type() == lua.LTString {
                    row[k.String()] = luaValueToGo(v)
                }
            })
            result = append(result, row)
        }
    })

    return result, nil
}
```

**Explanation:**

This is the reverse of `toLuaValue`. It takes a Lua table and converts it back to a Go slice of maps. This is what we use when Lua returns the modified population.

The structure we expect from Lua is:
```
population = {
    { person_id = 1, age = 26, sex = "F" },  -- Modified by Lua
    { person_id = 2, age = 31, sex = "M" },
}
```

We iterate through the outer table, then through each row's properties, converting Lua values back to Go values.

```go
// luaValueToGo converts Lua values to Go values
func luaValueToGo(val lua.LValue) interface{} {
    if val == lua.LNil {
        return nil
    }

    switch v := val.(type) {
    case lua.LBool:
        return bool(v)
    case lua.LNumber:
        return float64(v)
    case lua.LString:
        return string(v)
    case *lua.LTable:
        // Check if it's a list or map
        isList := true
        var listLen int
        v.ForEach(func(key lua.LValue, value lua.LValue) {
            if key.Type() != lua.LTNumber {
                isList = false
            }
            listLen++
        })

        if isList && listLen > 0 {
            result := []interface{}{}
            for i := 1; i <= listLen; i++ {
                val := v.RawGetInt(i)
                result = append(result, luaValueToGo(val))
            }
            return result
        }
        // It's a map
        result := map[string]interface{}{}
        v.ForEach(func(key lua.LValue, value lua.LValue) {
            if key.Type() == lua.LTString {
                result[key.String()] = luaValueToGo(value)
            }
        })
        return result
    default:
        return nil
    }
}
```

**Explanation:**

This converts Lua values back to Go values. The tricky part is handling Lua tables which can be either lists or maps.

**How we detect a list vs a map:**

```go
isList := true
var listLen int
v.ForEach(func(key lua.LValue, value lua.LValue) {
    if key.Type() != lua.LTNumber {
        isList = false
    }
    listLen++
})
```

- If all keys are numbers (indices), it's a list
- If any key is a string, it's a map

**Visual Examples:**

Lua list:
```lua
{ "a", "b", "c" }
```
- Keys: 1, 2, 3 (all numbers) → isList = true

Lua map:
```lua
{ name = "Alice", age = 25 }
```
- Keys: "name", "age" (strings) → isList = false

---

## Mini Demo 4: Go to Lua Conversion

This demonstrates the full conversion cycle between Go and Lua.

```go
// demo3_go_lua_conversion.go
package main

import (
	"fmt"
	"log"

	lua "github.com/yuin/gopher-lua"
)

func main() {
	// Create a Lua state
	L := lua.NewState()
	defer L.Close()

	// Go data: a map with various types
	goData := map[string]interface{}{
		"person_id": 1,
		"age":       25,
		"sex":       "F",
		"alive":     true,
		"children":  []interface{}{"Alice", "Bob"},
		"address": map[string]interface{}{
			"city":  "London",
			"postcode": "SW1A 1AA",
		},
	}

	fmt.Println("=== Converting Go to Lua ===")

	// Convert Go map to Lua table
	luaTable := goToLuaTable(L, goData)

	// Set it as a global variable in Lua
	L.SetGlobal("person", luaTable)

	// Run a Lua script that reads the data
	script := `
		print("Lua sees:")
		print("  person_id:", person.person_id)
		print("  age:", person.age)
		print("  sex:", person.sex)
		print("  alive:", person.alive)
		print("  first child:", person.children[1])
		print("  city:", person.address.city)
		print("  postcode:", person.address.postcode)
		
		-- Modify the data
		person.age = person.age + 1
		person.alive = false
	`

	err := L.DoString(script)
	if err != nil {
		log.Fatal("Failed to run Lua script:", err)
	}

	fmt.Println("\n=== Converting Lua back to Go ===")

	// Get the modified data back from Lua
	luaPerson := L.GetGlobal("person")
	if luaPerson.Type() != lua.LTTable {
		log.Fatal("Expected a table")
	}

	// Convert Lua table back to Go map
	goResult := luaToGoMap(luaPerson.(*lua.LTable))

	fmt.Printf("  person_id: %v\n", goResult["person_id"])
	fmt.Printf("  age: %v (modified by Lua!)\n", goResult["age"])
	fmt.Printf("  sex: %v\n", goResult["sex"])
	fmt.Printf("  alive: %v (modified by Lua!)\n", goResult["alive"])
	fmt.Printf("  first child: %v\n", goResult["children"].([]interface{})[0])
	fmt.Printf("  city: %v\n", goResult["address"].(map[string]interface{})["city"])
}

func goToLuaTable(L *lua.LState, val interface{}) *lua.LTable {
	tbl := L.NewTable()

	switch v := val.(type) {
	case map[string]interface{}:
		for k, v2 := range v {
			tbl.RawSetString(k, goToLuaValue(L, v2))
		}
	case []interface{}:
		for i, v2 := range v {
			tbl.RawSetInt(i+1, goToLuaValue(L, v2))
		}
	default:
		return nil
	}
	return tbl
}

func goToLuaValue(L *lua.LState, val interface{}) lua.LValue {
	switch v := val.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case int:
		return lua.LNumber(v)
	case int64:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		tbl := L.NewTable()
		for i, item := range v {
			tbl.RawSetInt(i+1, goToLuaValue(L, item))
		}
		return tbl
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, item := range v {
			tbl.RawSetString(k, goToLuaValue(L, item))
		}
		return tbl
	default:
		return lua.LNil
	}
}

func luaToGoMap(tbl *lua.LTable) map[string]interface{} {
	result := make(map[string]interface{})

	tbl.ForEach(func(key lua.LValue, value lua.LValue) {
		if key.Type() == lua.LTString {
			result[key.String()] = luaToGoValue(value)
		}
	})

	return result
}

func luaToGoValue(val lua.LValue) interface{} {
	switch v := val.(type) {
	case lua.LNil:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		isList := true
		var listLen int
		v.ForEach(func(key lua.LValue, value lua.LValue) {
			if key.Type() != lua.LTNumber {
				isList = false
			}
			listLen++
		})

		if isList && listLen > 0 {
			result := []interface{}{}
			for i := 1; i <= listLen; i++ {
				val := v.RawGetInt(i)
				result = append(result, luaToGoValue(val))
			}
			return result
		}

		result := make(map[string]interface{})
		v.ForEach(func(key lua.LValue, value lua.LValue) {
			if key.Type() == lua.LTString {
				result[key.String()] = luaToGoValue(value)
			}
		})
		return result
	default:
		return nil
	}
}
```

**Key Takeaways:**
- Go maps become Lua tables
- Lua tables can be lists (numeric indices) or maps (string keys)
- Values can be modified in Lua and read back in Go

---

## Population Loading

Now let's look at how Hekate loads CSV data into the `Population` structure.

```go
func loadPopulationDynamic(csvFile string, idColumn string) (Population, []ColumnInfo, error) {
    file, err := os.Open(csvFile)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to open CSV: %w", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, nil, fmt.Errorf("failed to read CSV: %w", err)
    }

    if len(records) == 0 {
        return nil, nil, fmt.Errorf("CSV file is empty")
    }
```

**Explanation:**

- `os.Open` opens the CSV file
- `defer file.Close()` ensures the file is closed when the function returns
- `csv.NewReader` creates a CSV reader
- `reader.ReadAll()` reads all records at once
- We check if the file is empty

**Note:** Reading all records at once is fine for moderate-sized files. For very large files (millions of rows), you'd want to stream the data.

```go
    header := records[0]
    columns := make([]ColumnInfo, len(header))
    foundID := false
    for i, col := range header {
        col = strings.TrimSpace(col)
        isKey := (col == idColumn)
        if isKey {
            foundID = true
        }
        colType := "string"
        for j := 1; j < len(records) && j < 5; j++ {
            if len(records[j]) > i {
                val := strings.TrimSpace(records[j][i])
                if val != "" {
                    if _, err := strconv.Atoi(val); err == nil {
                        colType = "int"
                    } else if val == "true" || val == "false" || val == "True" || val == "False" {
                        colType = "bool"
                    }
                    break
                }
            }
        }
        columns[i] = ColumnInfo{
            Name: col,
            Type: colType,
        }
    }
```

**Explanation:**

This is where we detect the column structure:

1. `records[0]` is the header row (column names)
2. We create a `ColumnInfo` slice with the same length as the header
3. We check if each column matches the `idColumn` (foundID tracks if we found it)
4. We try to detect the type by looking at the first few rows (1-4):
   - If it's a number → `"int"`
   - If it's true/false → `"bool"`
   - Otherwise → `"string"`

**Why look at multiple rows?**

The first data row might be empty or missing. Looking at up to 4 rows gives us a better chance of detecting the actual type.

```go
    if !foundID {
        return nil, nil, fmt.Errorf("ID column '%s' not found in CSV header. Available columns: %s",
            idColumn, strings.Join(header, ", "))
    }
```

**Explanation:**

If we didn't find the ID column in the header, return an error listing all available columns.

```go
    var population Population

    for i := 1; i < len(records); i++ {
        record := records[i]
        if len(record) < len(columns) {
            log.Printf("Warning: Row %d has insufficient fields, skipping", i)
            continue
        }

        row := make(map[string]interface{})
        for j, col := range columns {
            val := strings.TrimSpace(record[j])
            if val == "" {
                row[col.Name] = nil
                continue
            }

            switch col.Type {
            case "int":
                if intVal, err := strconv.Atoi(val); err == nil {
                    row[col.Name] = intVal
                } else {
                    row[col.Name] = val
                }
            case "bool":
                if val == "true" || val == "True" || val == "1" {
                    row[col.Name] = true
                } else if val == "false" || val == "False" || val == "0" {
                    row[col.Name] = false
                } else {
                    row[col.Name] = val
                }
            default:
                row[col.Name] = val
            }
        }
        population = append(population, row)
    }

    return population, columns, nil
}
```

**Explanation:**

This is where we build the actual population data:

1. For each data row (starting at index 1):
   - Check if the row has enough fields
   - Create an empty map for this person
   - For each column:
     - Trim whitespace
     - If empty, set to nil
     - Otherwise, convert based on the detected type:
       - `"int"`: try `strconv.Atoi`, fallback to string
       - `"bool"`: check for true/false/1/0, fallback to string
       - `"string"`: keep as string
   - Append the row to the population

**Why `nil` for empty values?**

In Lua, we want to distinguish between `nil` (missing) and `""` (empty string). This allows Lua scripts to check `if person.field == nil then` for missing data.

---

## Mini Demo 5: CSV Parsing with Type Detection

This demonstrates how Hekate reads CSV data and detects column types.

```go
// demo6_csv_reading.go
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"strings"
)

func main() {
	// CSV data as a string (instead of a file)
	csvData := `person_id,age,sex,alive,income
1,25,F,true,45000
2,30,M,true,35000
3,45,F,false,0
`

	reader := csv.NewReader(strings.NewReader(csvData))
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal("Failed to read CSV:", err)
	}

	if len(records) == 0 {
		log.Fatal("CSV is empty")
	}

	// Detect columns from header
	header := records[0]
	columns := make([]ColumnInfo, len(header))
	for i, col := range header {
		col = strings.TrimSpace(col)
		// Try to detect type from first data row
		colType := "string"
		if len(records) > 1 && len(records[1]) > i {
			val := strings.TrimSpace(records[1][i])
			if _, err := strconv.Atoi(val); err == nil {
				colType = "int"
			} else if val == "true" || val == "false" {
				colType = "bool"
			}
		}
		columns[i] = ColumnInfo{
			Name: col,
			Type: colType,
		}
	}

	// Parse the data
	fmt.Println("=== CSV Data Loaded ===")
	for _, info := range columns {
		fmt.Printf("  Column: %s (type: %s)\n", info.Name, info.Type)
	}

	fmt.Println("\nData rows:")
	for i := 1; i < len(records); i++ {
		row := records[i]
		fmt.Printf("  Row %d: ", i)
		for j, col := range columns {
			val := strings.TrimSpace(row[j])
			switch col.Type {
			case "int":
				if intVal, err := strconv.Atoi(val); err == nil {
					fmt.Printf("%s=%d ", col.Name, intVal)
				} else {
					fmt.Printf("%s=%s ", col.Name, val)
				}
			case "bool":
				if val == "true" || val == "True" {
					fmt.Printf("%s=true ", col.Name)
				} else if val == "false" || val == "False" {
					fmt.Printf("%s=false ", col.Name)
				} else {
					fmt.Printf("%s=%s ", col.Name, val)
				}
			default:
				fmt.Printf("%s=%s ", col.Name, val)
			}
		}
		fmt.Println()
	}
}

type ColumnInfo struct {
	Name string
	Type string
}
```

**Key Takeaways:**
- CSV headers become column names
- Types are detected from data
- Empty values become `nil` (handled gracefully)

---

## Model Execution

Now let's look at how Hekate executes a Lua model.

```go
func executeLuaModel(vm *LuaVM, model ModelConfig, population Population, verbose bool) (Population, error) {
    scriptInterface, ok := model.Parameters["script"]
    if !ok {
        return nil, fmt.Errorf("model '%s' missing 'script' parameter", model.Name)
    }

    script, ok := scriptInterface.(string)
    if !ok {
        return nil, fmt.Errorf("model '%s' script is not a string", model.Name)
    }
```

**Explanation:**

1. First, we get the `script` parameter from the model's parameters map.
2. We check if it exists with `ok`.
3. We type-assert it to a string. If it's not a string, we error.

**Why this is important:**

The user's YAML config might be missing the `script` field, or it might be a number or something else. We check and provide clear error messages.

```go
    if verbose {
        log.Printf("  ▶ Executing: %s (priority: %d)", model.Name, model.Priority)
    } else {
        log.Printf("  ▶ %s", model.Name)
    }
```

**Explanation:**

Log what model we're executing. If verbose, include the priority.

```go
    popSlice := []map[string]interface{}(population)

    result, err := vm.ExecuteLuaScript(script, popSlice, model.Parameters)
    if err != nil {
        return nil, err
    }

    return Population(result), nil
}
```

**Explanation:**

1. Convert the population to a `[]map[string]interface{}` (which is the same type, just a type conversion to match the function signature).
2. Call `ExecuteLuaScript` which runs the Lua code.
3. Convert the result back to `Population`.
4. Return the updated population.

```go
// ExecuteLuaScript executes a Lua script and returns the result
func (vm *LuaVM) ExecuteLuaScript(script string, population []map[string]interface{}, params map[string]interface{}) ([]map[string]interface{}, error) {
    // Convert population to Lua table
    luaPop := vm.L.NewTable()
    for _, person := range population {
        luaPerson := vm.L.NewTable()
        for k, v := range person {
            luaPerson.RawSetString(k, toLuaValue(vm.L, v))
        }
        luaPop.Append(luaPerson)
    }
```

**Explanation:**

This converts the Go population to a Lua table:

1. `vm.L.NewTable()` creates a new Lua table (the outer table that will hold all the people)
2. For each person in the Go slice:
   - Create a new Lua table for that person
   - For each key-value pair in the person's map:
     - Convert the Go value to a Lua value with `toLuaValue`
     - Set it on the Lua person table with `RawSetString`
   - Append the person table to the outer table

**Visual:**

Go data:
```go
[]map[string]interface{}{
    {"person_id": 1, "age": 25},
    {"person_id": 2, "age": 30},
}
```

Becomes Lua:
```lua
{
    { person_id = 1, age = 25 },
    { person_id = 2, age = 30 }
}
```

```go
    // Convert params to Lua table
    luaParams := vm.L.NewTable()
    for k, v := range params {
        luaParams.RawSetString(k, toLuaValue(vm.L, v))
    }

    // Register globals
    vm.L.SetGlobal("params", luaParams)
    vm.L.SetGlobal("population", luaPop)
```

**Explanation:**

1. Convert the Go parameters to a Lua table.
2. Register `params` and `population` as global variables in Lua.
3. Now the Lua script can access `params.fertility_rate` and `population[1].age`.

```go
    // Execute the script
    if err := vm.L.DoString(script); err != nil {
        return nil, fmt.Errorf("failed to execute Lua script: %w", err)
    }
```

**Explanation:**

`DoString` compiles and executes the Lua script. This registers the `transition` function in the Lua environment but doesn't call it yet.

```go
    // Get the transition function
    fn := vm.L.GetGlobal("transition")
    if fn.Type() != lua.LTFunction {
        return nil, fmt.Errorf("script must define a 'transition' function")
    }
```

**Explanation:**

After running the script, we look for the `transition` function. This is the function that the user must define in their Lua script.

**Why a specific name?**

Hekate requires the model function to be called `transition`. This is a design choice that makes it easy for Hekate to find the function. It's like a contract: "You must define a function called `transition`."

```go
    // Call the transition function
    if err := vm.L.CallByParam(lua.P{
        Fn:      fn,
        NRet:    1,
        Protect: true,
    }, luaPop, luaParams); err != nil {
        return nil, fmt.Errorf("failed to call transition: %w", err)
    }
```

**Explanation:**

This is where we actually call the Lua `transition` function.

- `Fn: fn` - The Lua function to call
- `NRet: 1` - Expect one return value (the modified population)
- `Protect: true` - Catch any Lua errors and return them as Go errors
- `luaPop` - The population table (first argument)
- `luaParams` - The parameters table (second argument)

**In Lua terms, we're doing:**
```lua
local result = transition(population, params)
```

```go
    // Get the result
    result := vm.L.Get(-1)
    vm.L.Pop(1)
```

**Explanation:**

After the function call, the return value is on the top of the Lua stack. We get it with `Get(-1)` and then pop it off with `Pop(1)`.

```go
    // Convert Lua table back to Go slice
    resultPop, err := luaTableToSlice(result.(*lua.LTable))
    if err != nil {
        return nil, fmt.Errorf("failed to convert result: %w", err)
    }

    return resultPop, nil
}
```

**Explanation:**

1. The result should be a Lua table (the modified population). We type-assert it to `*lua.LTable`.
2. `luaTableToSlice` converts it back to a Go slice of maps.
3. Return the result.

---

## Mini Demo 6: Simple Population Simulation

This demonstrates the core simulation loop without the Lua complexity.

```go
// demo4_simple_simulation.go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Person is a simple struct for this demo
type Person struct {
	ID    int
	Age   int
	Sex   string
	Alive bool
}

// Population is a slice of people
type Population []Person

func main() {
	// Seed random
	rand.Seed(time.Now().UnixNano())

	// 1. Create initial population
	population := createPopulation()
	fmt.Println("Initial population:")
	printStats(population)

	// 2. Run simulation for 5 years
	fmt.Println("\n=== Running Simulation ===")
	for year := 1; year <= 5; year++ {
		fmt.Printf("\nYear %d:\n", year)

		// Age everyone
		population = agePopulation(population)
		fmt.Printf("  After aging: %d alive\n", countAlive(population))

		// Apply mortality
		population = applyMortality(population)
		fmt.Printf("  After mortality: %d alive\n", countAlive(population))

		// Print age distribution every 2 years
		if year%2 == 0 || year == 5 {
			printAgeDistribution(population)
		}
	}

	// 3. Show final results
	fmt.Println("\n=== Final Population ===")
	printStats(population)
	printDetails(population)
}

func createPopulation() Population {
	// Create 10 people with various ages
	return Population{
		{ID: 1, Age: 25, Sex: "F", Alive: true},
		{ID: 2, Age: 30, Sex: "M", Alive: true},
		{ID: 3, Age: 45, Sex: "F", Alive: true},
		{ID: 4, Age: 68, Sex: "M", Alive: true},
		{ID: 5, Age: 82, Sex: "F", Alive: true},
		{ID: 6, Age: 2, Sex: "M", Alive: true},
		{ID: 7, Age: 15, Sex: "F", Alive: true},
		{ID: 8, Age: 35, Sex: "M", Alive: true},
		{ID: 9, Age: 55, Sex: "F", Alive: true},
		{ID: 10, Age: 70, Sex: "M", Alive: true},
	}
}

func agePopulation(pop Population) Population {
	for i := range pop {
		if pop[i].Alive {
			pop[i].Age++
		}
	}
	return pop
}

func applyMortality(pop Population) Population {
	for i := range pop {
		if !pop[i].Alive {
			continue
		}
		// Age-based mortality
		var prob float64
		if pop[i].Age < 30 {
			prob = 0.001 // 0.1%
		} else if pop[i].Age < 65 {
			prob = 0.01 // 1%
		} else if pop[i].Age < 85 {
			prob = 0.05 // 5%
		} else {
			prob = 0.10 // 10%
		}
		if rand.Float64() < prob {
			pop[i].Alive = false
		}
	}
	return pop
}

func countAlive(pop Population) int {
	count := 0
	for _, p := range pop {
		if p.Alive {
			count++
		}
	}
	return count
}

func printStats(pop Population) {
	alive := countAlive(pop)
	fmt.Printf("Total: %d, Alive: %d, Dead: %d\n", len(pop), alive, len(pop)-alive)
	printAgeDistribution(pop)
}

func printAgeDistribution(pop Population) {
	children := 0
	adults := 0
	elderly := 0

	for _, p := range pop {
		if !p.Alive {
			continue
		}
		if p.Age < 18 {
			children++
		} else if p.Age < 65 {
			adults++
		} else {
			elderly++
		}
	}
	fmt.Printf("  Age distribution: Children: %d, Adults: %d, Elderly: %d\n",
		children, adults, elderly)
}

func printDetails(pop Population) {
	fmt.Println("\nIndividual details:")
	fmt.Println("ID\tAge\tSex\tAlive")
	fmt.Println(strings.Repeat("-", 30))
	for _, p := range pop {
		fmt.Printf("%d\t%d\t%s\t%v\n", p.ID, p.Age, p.Sex, p.Alive)
	}
}
```

**Key Takeaways:**
- The simulation loop: age → mortality → statistics
- Models execute in priority order
- Population state is maintained between iterations

---

## The Main Function

Now let's look at the heart of Hekate - the main function that orchestrates everything.

```go
func main() {
    if len(os.Args) < 2 {
        log.Fatal("Usage: go run main.go <config.yaml>")
    }
    configFile := os.Args[1]
```

**Explanation:**

Hekate expects one command-line argument: the path to the YAML configuration file.

**Example:**
```bash
go run main.go config.yaml
```
Here, `os.Args[0]` is "main.go" and `os.Args[1]` is "config.yaml".

```go
    // 1. Read and parse YAML config
    configBytes, err := os.ReadFile(configFile)
    if err != nil {
        log.Fatalf("Failed to read config: %v", err)
    }

    var simConfig SimulationConfig
    if err := yaml.Unmarshal(configBytes, &simConfig); err != nil {
        log.Fatalf("Failed to parse YAML: %v", err)
    }
```

**Explanation:**

- `os.ReadFile` reads the entire YAML file into memory
- `yaml.Unmarshal` parses the YAML into our `SimulationConfig` struct
- The `yaml` tags on our struct fields tell the parser how to map YAML keys to Go fields

**The YAML tags in action:**

```go
type SimulationParameters struct {
    Iterations     int    `yaml:"iterations"`
    PopulationFile string `yaml:"population_file"`
    // ...
}
```

The `yaml:"iterations"` tag says: "When parsing YAML, look for a key called 'iterations' and put its value into this field."

```go
    // Validate ID column
    if simConfig.Simulation.IDColumn == "" {
        log.Fatal("ERROR: id_column is required in simulation section of config.yaml")
    }
```

**Explanation:**

This is a validation check. The `id_column` is required because Hekate needs to know which column to use as the unique identifier. If it's missing, we exit with a clear error message.

```go
    // Set random seed
    if simConfig.Simulation.RandomSeed > 0 {
        rand.Seed(simConfig.Simulation.RandomSeed)
    } else {
        rand.Seed(time.Now().UnixNano())
    }
```

**Explanation:**

- If a seed is provided, use it (reproducible results)
- If not, use the current time (different results each run)
- `rand.Seed` initializes Go's random number generator

```go
    log.Printf("═══ Hekate: Microsimulation Engine ═══")
    log.Printf("Iterations: %d", simConfig.Simulation.Iterations)
    log.Printf("Population file: %s", simConfig.Simulation.PopulationFile)
    log.Printf("ID column: %s", simConfig.Simulation.IDColumn)
    log.Printf("Random seed: %d", simConfig.Simulation.RandomSeed)
    log.Printf("Models loaded: %d", len(simConfig.Models))
```

**Explanation:**

Log the configuration to show the user what's happening. This is especially useful for debugging and verifying the config was read correctly.

```go
    // 2. Load population
    population, columns, err := loadPopulationDynamic(simConfig.Simulation.PopulationFile, simConfig.Simulation.IDColumn)
    if err != nil {
        log.Fatalf("Failed to load population: %v", err)
    }

    log.Printf("Loaded %d individuals with %d columns", len(population), len(columns))
```

**Explanation:**

- `loadPopulationDynamic` reads the CSV and returns a `Population` (slice of maps) and column metadata
- We log the number of people and columns loaded

```go
    // 3. Filter and sort models
    enabledModels := filterEnabledModels(simConfig.Models)
    sortModelsByPriority(enabledModels)

    log.Printf("Enabled models: %d", len(enabledModels))
    for _, model := range enabledModels {
        log.Printf("  - %s (priority: %d)", model.Name, model.Priority)
    }
```

**Explanation:**

- `filterEnabledModels` keeps only models with `Enabled: true`
- `sortModelsByPriority` sorts them by priority (lower number = higher priority)
- We log which models will actually run

---

## Mini Demo 7: Priority-Based Sorting

This demonstrates how models are sorted by priority.

```go
// demo7_sorting.go
package main

import (
	"fmt"
	"sort"
)

type Model struct {
	Name     string
	Priority int
}

func main() {
	models := []Model{
		{Name: "fertility", Priority: 3},
		{Name: "mortality", Priority: 2},
		{Name: "age_increment", Priority: 1},
		{Name: "migration", Priority: 4},
	}

	fmt.Println("=== Before Sorting ===")
	for _, m := range models {
		fmt.Printf("  %s (priority: %d)\n", m.Name, m.Priority)
	}

	// Sort by priority (lower number = higher priority)
	sort.Slice(models, func(i, j int) bool {
		return models[i].Priority < models[j].Priority
	})

	fmt.Println("\n=== After Sorting ===")
	for _, m := range models {
		fmt.Printf("  %s (priority: %d)\n", m.Name, m.Priority)
	}
}
```

**Key Takeaways:**
- Lower priority number = runs first
- Sorting ensures models execute in the correct order

---

## Population Saving

Now let's look at how Hekate saves the final population to CSV.

```go
func savePopulationDynamic(population Population, columns []ColumnInfo, outputFile string, idColumn string) error {
    file, err := os.Create(outputFile)
    if err != nil {
        return fmt.Errorf("failed to create output file: %w", err)
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    colNames := getColumnNames(columns)
    if err := writer.Write(colNames); err != nil {
        return fmt.Errorf("failed to write header: %w", err)
    }
```

**Explanation:**

1. Create the output file.
2. Create a CSV writer.
3. Get the column names from the `ColumnInfo` struct.
4. Write the header row.

```go
    // Sort population by ID column
    sort.Slice(population, func(i, j int) bool {
        id1 := fmt.Sprintf("%v", population[i][idColumn])
        id2 := fmt.Sprintf("%v", population[j][idColumn])
        return id1 < id2
    })
```

**Explanation:**

This sorts the population by the ID column. This ensures consistent output ordering, making it easier to compare results across runs.

**Why string comparison?**

`fmt.Sprintf("%v", value)` converts the value to a string. This works for ints, strings, and any other type. The sort is lexicographic (alphabetical), which works fine for IDs.

```go
    // Write data
    for _, row := range population {
        record := make([]string, len(columns))
        for i, col := range columns {
            val := row[col.Name]
            if val == nil {
                record[i] = ""
            } else {
                switch v := val.(type) {
                case bool:
                    if v {
                        record[i] = "true"
                    } else {
                        record[i] = "false"
                    }
                case int:
                    record[i] = strconv.Itoa(v)
                case int64:
                    record[i] = strconv.FormatInt(v, 10)
                case float64:
                    record[i] = strconv.FormatFloat(v, 'f', -1, 64)
                case string:
                    record[i] = v
                default:
                    record[i] = fmt.Sprintf("%v", v)
                }
            }
        }
        if err := writer.Write(record); err != nil {
            return fmt.Errorf("failed to write record: %w", err)
        }
    }

    return nil
}
```

**Explanation:**

This writes each row to the CSV:

1. For each person in the population:
   - Create a string slice with the same length as the columns
   - For each column:
     - If the value is nil → empty string
     - Otherwise, convert based on type:
       - `bool` → "true" or "false"
       - `int` → `strconv.Itoa`
       - `int64` → `strconv.FormatInt`
       - `float64` → `strconv.FormatFloat`
       - `string` → keep as is
       - default → `fmt.Sprintf("%v", v)`
   - Write the record to the CSV

---

## Helper Functions

```go
func getColumnNames(columns []ColumnInfo) []string {
    names := make([]string, len(columns))
    for i, col := range columns {
        names[i] = col.Name
    }
    return names
}
```

**Explanation:**

Just extracts the column names from the `ColumnInfo` slice.

```go
func filterEnabledModels(models []ModelConfig) []ModelConfig {
    var enabled []ModelConfig
    for _, model := range models {
        if model.Enabled {
            enabled = append(enabled, model)
        }
    }
    return enabled
}
```

**Explanation:**

Filters the models to only those with `Enabled: true`.

```go
func sortModelsByPriority(models []ModelConfig) {
    sort.Slice(models, func(i, j int) bool {
        return models[i].Priority < models[j].Priority
    })
}
```

**Explanation:**

Sorts models by priority (lower number = higher priority). This ensures models run in the correct order (age before mortality before fertility, etc.).

---

## Design Patterns and Philosophy

### 1. **Model as Data**

The `ModelConfig` struct and the `Parameters` map allow models to be defined in YAML rather than code. This is a key design pattern:

```
Code (Go) → Infrastructure (loading, saving, orchestration)
Data (YAML) → Models (what the simulation does)
```

### 2. **Interpreter Pattern**

Hekate is essentially an interpreter for the Lua DSL. The Go code provides the interpreter (the Lua VM), and the Lua scripts are the programs being interpreted.

### 3. **Plugin Architecture**

By embedding Lua, Hekate can support new models without recompilation. Users just add new YAML files with Lua scripts.

### 4. **Pipeline Pattern**

Data flows through a pipeline:
```
CSV → Parse → Validate → Execute Models → Save → CSV
```

### 5. **Strategy Pattern**

Different models implement different strategies (aging, mortality, fertility) through the same interface (the `transition` function).

### 6. **Dynamic Typing**

By using `interface{}` and maps, Hekate can work with any CSV structure. This is a trade-off:
- ✅ **Pros**: Flexibility, no need to predefine schemas
- ❌ **Cons**: No compile-time type safety, slower performance

---

## Summary of Mini Demos

| Demo | Concept | Hekate Location |
|------|---------|-----------------|
| Demo 1 | `map[string]interface{}` with YAML | `ModelConfig.Parameters` |
| Demo 2 | Safe type assertions | `executeLuaModel` |
| Demo 3 | Go ↔ Lua conversion | `toLuaValue`, `luaValueToGo` |
| Demo 4 | Simple simulation loop | `main` function |
| Demo 5 | Lua function registration | `NewLuaVM` |
| Demo 6 | CSV parsing with type detection | `loadPopulationDynamic` |
| Demo 7 | Priority-based sorting | `sortModelsByPriority` |

---

## Summary

**What you've learned:**

1. **Configuration**: How Hekate reads and parses YAML configurations into Go structs.
2. **Lua Integration**: How Hekate embeds a Lua interpreter, registers functions, and executes Lua scripts.
3. **Data Conversion**: How Hekate converts between Go and Lua data structures.
4. **Population Loading**: How Hekate reads CSV data into a dynamic Go structure.
5. **Model Execution**: How Hekate calls Lua models and processes the results.
6. **Population Saving**: How Hekate writes the final population to CSV.

**The Key Insight:**

Hekate is essentially a **bridge** between Go and Lua:
- Go provides the infrastructure (file I/O, data structures, orchestration)
- Lua provides the model logic (demographic transitions, probabilities, rules)

**Why this architecture works:**

1. **Flexibility**: Models can be changed without recompiling
2. **Accessibility**: Lua is easier for non-programmers than Go
3. **Performance**: Go handles the heavy lifting (I/O, data structures)
4. **Simplicity**: The Go code is straightforward and maintainable

**The Mental Model:**

Think of Hekate as a **factory**:
- The CSV is the raw materials
- The models (Lua scripts) are the machines that process the materials
- The output CSV is the finished product
- The Go code is the factory floor that organizes everything

---

