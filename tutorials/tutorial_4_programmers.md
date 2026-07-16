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

### Full Demo Code

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

### Code Explanation

```go
package main

import (
	"fmt"
	"log"

	"gopkg.in/yaml.v3"  // The YAML library we're using
)
```

**What's happening here?**

We're importing three packages:
- `fmt`: For printing output to the console
- `log`: For logging errors if something goes wrong
- `gopkg.in/yaml.v3`: A popular Go library that parses YAML files into Go data structures

The YAML library is the key player here. It knows how to read YAML text and convert it into Go types (strings, numbers, booleans, maps, and slices).

```go
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
```

**What is YAML?**

YAML (YAML Ain't Markup Language) is a human-readable data serialization format. Think of it like JSON but designed to be easier for humans to read and write.

**YAML syntax explained:**

- `simulation:` - This is a key (like a dictionary key). Everything indented under it belongs to it.
- `iterations: 10` - A key-value pair. Here, `iterations` is the key, `10` is the value.
- `models:` - Another key. The `-` indicates this is a list (array) of items.
- `- name: "age_increment"` - The first item in the models list. `name` is a key with value `"age_increment"`.
- `parameters:` - A nested map (dictionary) inside the model.
- `script: |` - The `|` indicates a multi-line string. Everything indented after it is part of the script.

**The YAML structure visualized:**

```
config (map)
├── simulation (map)
│   ├── iterations: 10
│   ├── population_file: "population.csv"
│   └── random_seed: 42
└── models (list)
    ├── [0] (map)
    │   ├── name: "age_increment"
    │   ├── priority: 1
    │   ├── enabled: true
    │   └── parameters (map)
    │       ├── rate: 1.0
    │       └── query: "UPDATE population SET age = age + 1"
    └── [1] (map)
        ├── name: "mortality"
        ├── priority: 2
        ├── enabled: false
        └── parameters (map)
            ├── mortality_rates (map)
            │   ├── infant: 0.005
            │   └── adult: 0.001
            └── script: "function transition()\n  -- Lua code here\nend"
```

```go
	// Parse the YAML into a generic map
	// This is the key: map[string]interface{} can hold ANYTHING!
	var config map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	if err != nil {
		log.Fatal("Failed to parse YAML:", err)
	}
```

**What's happening here?**

1. `var config map[string]interface{}` - We're declaring a variable called `config` that is a map where:
   - Keys are strings (`string`)
   - Values can be anything (`interface{}`)
   
   `interface{}` in Go means "any type at all." It's like a wildcard. This is how we can store numbers, strings, booleans, lists, and nested maps all in the same structure.

2. `yaml.Unmarshal([]byte(yamlConfig), &config)` - This is the magic line:
   - `[]byte(yamlConfig)` converts our YAML string into bytes (the format the library expects)
   - `&config` is a pointer to our config variable (the library fills it with the parsed data)
   - The library reads the YAML and automatically creates maps, slices, and primitive values

**What does the data look like after parsing?**

```go
// After parsing, config looks like this in memory:
config = map[string]interface{}{
    "simulation": map[string]interface{}{
        "iterations": 10,        // int
        "population_file": "population.csv", // string
        "random_seed": 42,       // int
    },
    "models": []interface{}{
        map[string]interface{}{
            "name": "age_increment",  // string
            "priority": 1,            // int
            "enabled": true,          // bool
            "parameters": map[string]interface{}{
                "rate": 1.0,          // float64
                "query": "UPDATE population SET age = age + 1", // string
            },
        },
        map[string]interface{}{
            "name": "mortality",      // string
            "priority": 2,            // int
            "enabled": false,         // bool
            "parameters": map[string]interface{}{
                "mortality_rates": map[string]interface{}{
                    "infant": 0.005,   // float64
                    "adult": 0.001,    // float64
                },
                "script": "function transition()\n  -- Lua code here\nend", // string
            },
        },
    },
}
```

**Notice how YAML types map to Go types:**
| YAML Type | Go Type |
|-----------|---------|
| `10` (integer) | `int` |
| `1.0` (decimal) | `float64` |
| `"population.csv"` (quoted text) | `string` |
| `true` / `false` | `bool` |
| `simulation:` (section) | `map[string]interface{}` |
| `- item` (list item) | `[]interface{}` |

```go
	// Let's explore what we got
	fmt.Println("=== Parsed YAML ===")
	fmt.Printf("Top-level keys: %v\n\n", getKeys(config))
```

**`getKeys` function explained:**

```go
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))  // Create a slice with capacity equal to map size
	for k := range m {                 // Loop through all keys in the map
		keys = append(keys, k)         // Add each key to our slice
	}
	return keys                        // Return the slice of keys
}
```

This function extracts all the keys from a map. It's useful for exploring what's in a map without knowing the structure in advance.

```go
	// Access the simulation section
	simulation := config["simulation"].(map[string]interface{})
```

**Type Assertion Explained:**

`.(map[string]interface{})` is called a **type assertion**. It tells Go: "I know this value is actually a map with string keys and any values, so convert it to that type."

**Why do we need this?**

`config["simulation"]` returns an `interface{}` (any type). To use it as a map, we need to tell Go what type it really is. If we're wrong, the program will panic (crash).

**Safe vs Unsafe Type Assertions:**

```go
// UNSAFE - will panic if the type is wrong
simulation := config["simulation"].(map[string]interface{})

// SAFE - handles the case where the type is wrong
simulation, ok := config["simulation"].(map[string]interface{})
if !ok {
    log.Fatal("simulation section is not a map!")
}
```

In the demo code, we use the unsafe version because we know the YAML structure. In production code, Hekate uses the safe version with `ok` checking.

```go
	fmt.Println("Simulation parameters:")
	for key, value := range simulation {
		fmt.Printf("  %s: %v (type: %T)\n", key, value, value)
	}
```

**What does this loop do?**

- `for key, value := range simulation` - Iterates through each key-value pair in the simulation map
- `%T` is a format specifier that prints the **type** of the value
- This shows us not just the data, but what type Go thinks it is

**Example output:**
```
  iterations: 10 (type: int)
  population_file: population.csv (type: string)
  random_seed: 42 (type: int)
```

```go
	fmt.Println("\nModels:")
	models := config["models"].([]interface{})
	for i, model := range models {
		modelMap := model.(map[string]interface{})
		fmt.Printf("  Model %d: %s (priority: %v)\n",
			i+1,
			modelMap["name"],
			modelMap["priority"],
		)
```

**What's happening here?**

1. `config["models"]` gets the models list
2. `.([]interface{})` asserts it's a slice of any type
3. The `for` loop iterates through each model
4. Each model is type-asserted to a map
5. We access the "name" and "priority" fields from each model

**Why `[]interface{}` and not `[]map[string]interface{}`?**

Because YAML lists can contain different types of items. While in our case they're all maps, the library doesn't know that in advance, so it uses the most general type: `[]interface{}`.

```go
		// Get parameters (which is also a map[string]interface{})
		if params, ok := modelMap["parameters"].(map[string]interface{}); ok {
			fmt.Println("    Parameters:")
			for key, value := range params {
				fmt.Printf("      %s: %v (type: %T)\n", key, value, value)
			}
		}
	}
```

**The safe type assertion pattern:**

```go
if params, ok := modelMap["parameters"].(map[string]interface{}); ok {
    // We only get here if 'ok' is true (the type assertion succeeded)
    // 'params' is now a map[string]interface{}
    fmt.Println("    Parameters:")
    // ... use params safely
}
// If 'ok' is false, we skip this block (no panic!)
```

This is the **preferred pattern** in production code. It handles the case where:
- The key doesn't exist
- The key exists but has a different type (e.g., a string instead of a map)

```go
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
```

**The Power of Dynamic Access:**

This demonstrates the key advantage of `map[string]interface{}`:

1. We check if the "simulation" section exists and is a map
2. If it is, we check if it contains the "random_seed" key
3. We don't need to know the structure in advance

**Without `map[string]interface{}`:**

If we used a struct, we'd need to define it first:
```go
type SimulationConfig struct {
    Iterations     int
    PopulationFile string
    RandomSeed     int
}
```

If a user adds a new field to their YAML, the struct wouldn't know about it. With `map[string]interface{}`, any new fields are automatically available.

```go
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
```

**This function demonstrates:**
- Creating a slice with a specific capacity (`make([]string, 0, len(m))`)
- Iterating through a map's keys
- Returning a slice of strings

**Run it:**
```bash
go run demo1_map_yaml.go
```

**Expected Output:**
```
=== Parsed YAML ===
Top-level keys: [simulation models]

Simulation parameters:
  iterations: 10 (type: int)
  population_file: population.csv (type: string)
  random_seed: 42 (type: int)

Models:
  Model 1: age_increment (priority: 1)
    Parameters:
      rate: 1 (type: float64)
      query: UPDATE population SET age = age + 1 (type: string)
  Model 2: mortality (priority: 2)
    Parameters:
      mortality_rates: map[adult:0.001 infant:0.005] (type: map[string]interface {})
      script: function transition()
          -- Lua code here
        end (type: string)

=== The Power of map[string]interface{} ===
We can access ANY key without knowing it in advance!
  Found 'random_seed': 42
```

**Key Takeaways:**

1. **`map[string]interface{}` can hold ANY type** - This is why it's so powerful. The same map can contain strings, numbers, booleans, nested maps, and slices.

2. **We don't need to define structs for every possible configuration** - In a traditional approach, you'd define a struct like `type Config struct { Simulation SimulationConfig; Models []ModelConfig }`. With maps, you don't need to pre-define anything.

3. **Users can add new parameters without code changes** - If a user adds a `"start_date"` field to their YAML, it's automatically available in the map. No code changes needed!

4. **Type assertions are how we access the data** - Since the values are `interface{}`, we need to tell Go what type we expect with `.(type)` assertions.

5. **Use the `ok` pattern for safety** - `value, ok := map[key].(type)` is safer than `value := map[key].(type)` because it won't panic if the type is wrong.

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

### Full Demo Code

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

### Code Explanation

```go
package main

import (
	"fmt"
	"log"

	"gopkg.in/yaml.v3"
)
```

**What's happening here?**

We're importing the same packages as before, plus `fmt` for output and `log` for error handling. The YAML library will parse our configuration string.

```go
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
```

**What's in this YAML?**

This YAML demonstrates different data types:
- `name: "Hekate Demo"` - A string (quotes optional in YAML)
- `version: 2.0` - A floating-point number
- `enabled: true` - A boolean
- `count: 42` - An integer
- `rates:` - A nested map (dictionary) with string keys and number values
- `tags: ["demo", "test", "simple"]` - An array (list) of strings

**Why these types?**

Hekate configurations use all these types:
- Strings for names and descriptions
- Numbers for rates, priorities, and counts
- Booleans for toggling features on/off
- Nested maps for complex parameters
- Arrays for lists of items

```go
	var config map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlConfig), &config)
	if err != nil {
		log.Fatal("Failed to parse YAML:", err)
	}

	settings := config["settings"].(map[string]interface{})
```

**Parsing the YAML:**

1. `var config map[string]interface{}` - Create an empty map to hold the parsed data
2. `yaml.Unmarshal([]byte(yamlConfig), &config)` - Parse the YAML string into the map
3. `settings := config["settings"].(map[string]interface{})` - Extract the "settings" section and assert it's a map

**What if "settings" doesn't exist?**

If the YAML didn't have a "settings" section, `config["settings"]` would return `nil`, and the type assertion would panic. In production code, we'd use the safe pattern with `ok`.

```go
	// Safe way to extract values with type checking
	fmt.Println("=== Safe Type Assertions ===")

	// String
	if name, ok := settings["name"].(string); ok {
		fmt.Printf("  name: %s (string)\n", name)
	} else {
		fmt.Println("  name: not a string")
	}
```

**String Type Assertion Explained:**

```go
if name, ok := settings["name"].(string); ok {
    // If we get here, ok is true, and name is a string
    fmt.Printf("  name: %s (string)\n", name)
} else {
    // If we get here, either the key doesn't exist or it's not a string
    fmt.Println("  name: not a string")
}
```

**Why this is safe:**

The `ok` variable tells us if the type assertion succeeded. If it returns `false`, we don't try to use the value, avoiding a panic.

**What could go wrong without `ok`:**

```go
// DANGEROUS! This will panic if "name" is not a string
name := settings["name"].(string)
```

If someone changes the YAML to `name: 123` (a number), this would crash the program.

```go
	// Float64 (YAML numbers become float64 by default)
	if version, ok := settings["version"].(float64); ok {
		fmt.Printf("  version: %.1f (float64)\n", version)
	} else {
		fmt.Println("  version: not a float64")
	}
```

**Float64 Type Assertion:**

In YAML, all numbers are parsed as `float64` by default. This includes both integers and decimals. This is why we check for `float64` even for what looks like an integer.

**Why `float64` and not `float32`?**

`float64` provides higher precision and is the default in Go's YAML library. It can represent larger numbers and more decimal places than `float32`.

```go
	// Bool
	if enabled, ok := settings["enabled"].(bool); ok {
		fmt.Printf("  enabled: %v (bool)\n", enabled)
	} else {
		fmt.Println("  enabled: not a bool")
	}
```

**Bool Type Assertion:**

Booleans in YAML can be written as `true`, `false`, `True`, `False`, `TRUE`, `FALSE`, `yes`, `no`, `on`, `off`, etc. The YAML library normalizes all of these to Go's `true` and `false`.

```go
	// Int (needs conversion from float64)
	if countFloat, ok := settings["count"].(float64); ok {
		count := int(countFloat)
		fmt.Printf("  count: %d (int, converted from float64)\n", count)
	} else {
		fmt.Println("  count: not a number")
	}
```

**Int Type Assertion (with conversion):**

This is a common pattern in Hekate:
1. First, assert the value is a `float64` (since YAML parses numbers as floats)
2. Then, convert it to an `int` using `int(countFloat)`

**Why not assert directly to `int`?**

YAML doesn't distinguish between integers and floats - everything is a number. The library chooses `float64` as the default representation. If you try `settings["count"].(int)`, it will fail because the underlying type is `float64`.

**Alternative approach for numbers:**

```go
// You could also use a type switch to handle both int and float
switch v := settings["count"].(type) {
case int:
    fmt.Printf("count: %d (int)\n", v)
case float64:
    fmt.Printf("count: %d (float64 converted to int)\n", int(v))
default:
    fmt.Println("count: not a number")
}
```

```go
	// Nested map
	if rates, ok := settings["rates"].(map[string]interface{}); ok {
		fmt.Println("  rates:")
		for k, v := range rates {
			fmt.Printf("    %s: %v\n", k, v)
		}
	} else {
		fmt.Println("  rates: not a map")
	}
```

**Nested Map Type Assertion:**

This demonstrates accessing a nested structure. The "rates" key contains another map. We:
1. Assert it's a `map[string]interface{}`
2. If successful, iterate through it
3. Print each key-value pair

**Why is this important in Hekate?**

Models often have nested parameters like:
```yaml
mortality_rates:
  infant: 0.005
  child: 0.0005
  adult: 0.001
  elderly: 0.05
```

The Lua script can then access these as `params.mortality_rates.infant`.

```go
	// Slice
	if tags, ok := settings["tags"].([]interface{}); ok {
		fmt.Println("  tags:")
		for _, tag := range tags {
			fmt.Printf("    %v\n", tag)
		}
	} else {
		fmt.Println("  tags: not a slice")
	}
```

**Slice Type Assertion:**

This handles arrays in YAML. Notice we assert to `[]interface{}` (slice of any type) because the YAML library doesn't know what type the array items will be.

**Why `[]interface{}` and not `[]string`?**

The YAML library parses arrays as `[]interface{}` because it doesn't know the element type in advance. While our tags are all strings, they could theoretically be numbers or booleans.

**Accessing array elements:**

If we know the elements are strings, we can convert them:
```go
for _, tag := range tags {
    if str, ok := tag.(string); ok {
        fmt.Printf("    %s\n", str)
    }
}
```

```go
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

**Missing Keys Behavior:**

This is a key feature of maps in Go:
- Accessing a key that doesn't exist returns the **zero value** for the map's value type
- For `map[string]interface{}`, the zero value is `nil`
- This means `settings["nonexistent"]` returns `nil`, **not an error**

**Why this is important:**

This allows us to safely check for missing keys:
```go
if settings["optional_field"] == nil {
    // The key doesn't exist - use a default value
} else {
    // The key exists - use its value
}
```

However, be careful! A key can exist with a value of `nil` (if someone set it that way). In practice, this rarely happens with YAML parsing.

**Run it:**
```bash
go run demo2_type_assertions.go
```

**Expected Output:**
```
=== Safe Type Assertions ===
  name: Hekate Demo (string)
  version: 2.0 (float64)
  enabled: true (bool)
  count: 42 (int, converted from float64)
  rates:
    low: 0.01
    medium: 0.05
    high: 0.1
  tags:
    demo
    test
    simple

=== Accessing Missing Keys ===
  Missing key returns nil (no panic!)
```

**Key Takeaways:**
1. **Always use `value, ok := map[key].(type)` for safe access** - This prevents panics from type mismatches
2. **Missing keys return `nil`, not an error** - You can check for existence with `if value == nil`
3. **YAML numbers become `float64` by default** - Convert to `int` if you need an integer
4. **Nested structures require nested type assertions** - Each level needs to be asserted
5. **Arrays are `[]interface{}`** - You need to assert each element's type

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

**Why wrap it?**

Wrapping allows us to:
1. Add convenience methods (like `ExecuteLuaScript`)
2. Keep track of additional state
3. Provide a cleaner API for the rest of the code

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

**The seeding logic:**

```go
if randomSeed > 0 {
    // User provided a seed - use it for reproducibility
    L.DoString(fmt.Sprintf("math.randomseed(%d)", randomSeed))
} else {
    // No seed provided - use current time for randomness
    L.DoString(fmt.Sprintf("math.randomseed(%d)", time.Now().UnixNano()))
}
```

**Why `DoString`?**

`DoString` compiles and executes a Lua string. It's how we run Lua code from Go. Here, we're running a simple Lua statement: `math.randomseed(42)`.

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

### Full Demo Code

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

### Code Explanation

```go
package main

import (
	"fmt"
	"log"
	"math/rand"

	lua "github.com/yuin/gopher-lua"
)
```

**What's happening here?**

We're importing:
- `fmt` - for formatted output
- `log` - for error logging
- `math/rand` - Go's random number generator
- `github.com/yuin/gopher-lua` - The Lua library we're using

**What is gopher-lua?**

`gopher-lua` is a pure Go implementation of the Lua 5.1 virtual machine. It allows us to embed Lua in Go programs without requiring a separate Lua installation.

**Key features of gopher-lua:**
- Pure Go - no C dependencies
- Supports Lua 5.1 syntax
- Can call Go functions from Lua
- Can call Lua functions from Go
- Efficient and thread-safe

```go
func main() {
	L := lua.NewState()
	defer L.Close()
```

**Creating a Lua State:**

1. `lua.NewState()` - Creates a new Lua interpreter instance
2. `defer L.Close()` - Ensures the Lua state is cleaned up when the function returns

**Why `defer`?**

`defer` schedules a function call to run after the surrounding function completes. It's commonly used for cleanup operations like closing files or, in this case, cleaning up the Lua state.

**What happens if we don't close the Lua state?**

The Lua state holds memory for all its variables, functions, and tables. Not closing it could lead to memory leaks if the program runs for a long time.

```go
	// Register a Go function that Lua can call
	L.SetGlobal("greet", L.NewFunction(func(L *lua.LState) int {
		name := L.ToString(1)
		fmt.Printf("Hello, %s! (from Go)\n", name)
		return 0
	}))
```

**Registering a Function - Step by Step:**

1. `L.NewFunction(...)` - Creates a Go function that can be called from Lua
2. The function takes `*lua.LState` as its only parameter
3. `L.ToString(1)` - Gets the first argument from the Lua stack (index 1 is the first argument)
4. `fmt.Printf` - Prints the greeting using Go's formatting
5. `return 0` - Returns 0 to indicate we're not pushing any return values to Lua
6. `L.SetGlobal("greet", ...)` - Makes the function available in Lua as `greet`

**Lua Stack Indexing:**

```
Lua stack when calling greet("Alice"):
┌─────────────────────────────────────────┐
│ Index: -1 (top)    │ "Alice" (string)  │
│ Index:  1 (bottom) │ "Alice" (string)  │
└─────────────────────────────────────────┘
```

`L.ToString(1)` gets the value at index 1 (the first argument).

```go
	// Register a Go function that returns a value
	L.SetGlobal("double", L.NewFunction(func(L *lua.LState) int {
		num := L.ToInt(1)
		L.Push(lua.LNumber(num * 2))
		return 1
	}))
```

**Registering a Function with Return Values:**

1. `L.ToInt(1)` - Gets the first argument as an int
2. `lua.LNumber(num * 2)` - Converts the Go int to a Lua number
3. `L.Push(...)` - Pushes the value onto the Lua stack (this is what Lua will get as the return value)
4. `return 1` - Returns 1 to indicate we pushed one return value

**In Lua terms:**

When Lua calls `double(21)`, it's equivalent to:
```lua
local result = 21 * 2
return result
```

**Pushing Multiple Return Values:**

Lua supports multiple return values, so we could push more:
```go
L.Push(lua.LNumber(num * 2))
L.Push(lua.LNumber(num * 3))
return 2  // Two return values
```

```go
	// Register a Go function that uses random
	L.SetGlobal("random_number", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(rand.Float64()))
		return 1
	}))
```

**Using Go's Random Generator:**

This function uses Go's `rand.Float64()` to generate a random number between 0 and 1. This is useful if we want to use the same random seed for both Go and Lua code.

**Why use Go's random instead of Lua's?**

By using Go's random generator, we ensure:
1. The random sequence is the same in Go and Lua (if we seed them the same way)
2. The random numbers are reproducible (if we use a fixed seed)
3. We have more control over the random generation

```go
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
```

**The Lua Script Explained:**

1. `print("Calling Go functions from Lua:")` - Lua's built-in print
2. `greet("Lua User")` - Calls our Go function with a string argument
3. `local result = double(21)` - Calls our Go function and stores the result
4. `print("21 doubled is", result)` - Prints the result
5. `local r = random_number()` - Gets a random number from Go
6. `print("Random number:", r)` - Prints the random number
7. `print("Done!")` - Final message

```go
	err := L.DoString(script)
	if err != nil {
		log.Fatal("Lua error:", err)
	}
}
```

**Executing the Script:**

`L.DoString(script)` compiles and executes the Lua script. If there's an error (syntax error, runtime error, etc.), it returns an error that we check.

**Common Lua Errors:**
- Syntax errors: `"print('unclosed string)"` (missing quote)
- Runtime errors: Calling a function that doesn't exist
- Type errors: Passing the wrong type to a function

**Run it:**
```bash
go run demo5_register_functions.go
```

**Expected Output:**
```
Calling Go functions from Lua:
Hello, Lua User! (from Go)
21 doubled is	42
Random number:	0.123456789 (or similar)
Done!
```

**Key Takeaways:**
1. **`L.NewFunction` wraps a Go function for Lua** - This is how we expose Go functionality to Lua
2. **Lua can call Go functions with arguments** - Use `L.ToString()`, `L.ToInt()`, etc. to get arguments
3. **Go functions can return values to Lua** - Push values with `L.Push()` and return the count
4. **The return value of the wrapper function is the number of return values pushed** - This tells Lua how many values to expect
5. **`DoString` executes Lua code** - It compiles and runs the script in one step

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

---

## Mini Demo 4: Go to Lua Conversion

This demonstrates the full conversion cycle between Go and Lua.

### Full Demo Code

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
			"city":     "London",
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

### Code Explanation

```go
package main

import (
	"fmt"
	"log"

	lua "github.com/yuin/gopher-lua"
)
```

**What's happening here?**

We're importing our packages, including the gopher-lua library. This demo will show the complete round-trip conversion from Go to Lua and back.

```go
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
			"city":     "London",
			"postcode": "SW1A 1AA",
		},
	}
```

**The Go Data Structure:**

This is a realistic example of what a person record might look like in Hekate:
- `person_id`: 1 (int) - Unique identifier
- `age`: 25 (int) - Age in years
- `sex`: "F" (string) - Female
- `alive`: true (bool) - Person is alive
- `children`: ["Alice", "Bob"] (slice) - List of children's names
- `address`: {city: "London", postcode: "SW1A 1AA"} (nested map) - Address information

**Why use nested structures?**

Real demographic data often has nested structures:
- Addresses (street, city, postcode)
- Family relationships (parents, children, spouse)
- Employment history (job, employer, dates)

While Hekate primarily uses flat CSV structures, the conversion system needs to handle nested data for flexibility.

```go
	fmt.Println("=== Converting Go to Lua ===")

	// Convert Go map to Lua table
	luaTable := goToLuaTable(L, goData)

	// Set it as a global variable in Lua
	L.SetGlobal("person", luaTable)
```

**Converting Go to Lua:**

1. `goToLuaTable(L, goData)` - Convert the Go map to a Lua table
2. `L.SetGlobal("person", luaTable)` - Make it available in Lua as `person`

Now the Lua script can access `person.age`, `person.children[1]`, etc.

```go
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
```

**The Lua Script Explained:**

1. **Reading the data:**
   - `person.person_id` - Access a simple property
   - `person.children[1]` - Access the first element of an array (Lua is 1-indexed!)
   - `person.address.city` - Access nested property

2. **Modifying the data:**
   - `person.age = person.age + 1` - Increment age by 1
   - `person.alive = false` - Mark as dead

**Important: Lua is 1-indexed!**

In Lua, arrays start at index 1, not 0. This is a common source of confusion:
- Lua: `person.children[1]` is the first child
- Go: `person.children[0]` is the first child

```go
	err := L.DoString(script)
	if err != nil {
		log.Fatal("Failed to run Lua script:", err)
	}
```

**Executing the Script:**

The script runs and modifies the data. The changes are made to the Lua table that was created from the Go data.

```go
	fmt.Println("\n=== Converting Lua back to Go ===")

	// Get the modified data back from Lua
	luaPerson := L.GetGlobal("person")
	if luaPerson.Type() != lua.LTTable {
		log.Fatal("Expected a table")
	}

	// Convert Lua table back to Go map
	goResult := luaToGoMap(luaPerson.(*lua.LTable))
```

**Converting Lua back to Go:**

1. `L.GetGlobal("person")` - Get the Lua table
2. `luaPerson.Type() != lua.LTTable` - Verify it's a table
3. `luaToGoMap(luaPerson.(*lua.LTable))` - Convert it back to a Go map

**The Modified Data:**

The Lua script changed two fields:
- `person.age = person.age + 1` (25 → 26)
- `person.alive = false` (true → false)

```go
	fmt.Printf("  person_id: %v\n", goResult["person_id"])
	fmt.Printf("  age: %v (modified by Lua!)\n", goResult["age"])
	fmt.Printf("  sex: %v\n", goResult["sex"])
	fmt.Printf("  alive: %v (modified by Lua!)\n", goResult["alive"])
	fmt.Printf("  first child: %v\n", goResult["children"].([]interface{})[0])
	fmt.Printf("  city: %v\n", goResult["address"].(map[string]interface{})["city"])
}
```

**Printing the Result:**

Notice the type assertions needed to access nested structures:
- `goResult["children"].([]interface{})[0]` - Assert to slice, then index
- `goResult["address"].(map[string]interface{})["city"]` - Assert to map, then key

**The conversion functions:**

```go
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
```

**`goToLuaTable` Explained:**

1. Create a new Lua table
2. Check the type of the input:
   - If it's a map: iterate through keys and values, recursively convert each
   - If it's a slice: iterate through items, recursively convert each
3. Return the table

**Why two separate functions?**

`goToLuaTable` is the entry point for converting complex types (maps and slices). `goToLuaValue` handles primitive types (strings, numbers, booleans) and calls `goToLuaTable` recursively for nested structures.

```go
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
```

**`goToLuaValue` Explained:**

This function handles all Go types:
1. **Primitive types** - Direct conversion to Lua types
2. **Slices** - Create a Lua table and recursively convert each item
3. **Maps** - Create a Lua table and recursively convert each value

**The recursion is important!**

When we have nested data like `address: {city: "London", postcode: "SW1A 1AA"}`, `goToLuaValue` calls itself recursively to convert the inner map.

```go
func luaToGoMap(tbl *lua.LTable) map[string]interface{} {
	result := make(map[string]interface{})

	tbl.ForEach(func(key lua.LValue, value lua.LValue) {
		if key.Type() == lua.LTString {
			result[key.String()] = luaToGoValue(value)
		}
	})

	return result
}
```

**`luaToGoMap` Explained:**

1. Create a Go map
2. Iterate through the Lua table
3. Only process string keys (ignore numeric indices)
4. Recursively convert each value with `luaToGoValue`

**Why ignore numeric indices?**

In Lua, tables can have both string keys and numeric indices. When we're converting a map (object), we only care about the string keys. Numeric indices would be converted when handling lists.

```go
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

**`luaToGoValue` Explained:**

This is the reverse of `goToLuaValue`:

1. **Primitive types** - Direct conversion from Lua types
2. **Tables** - Determine if it's a list or map:
   - If all keys are numbers → it's a list
   - If any key is a string → it's a map
3. Convert accordingly

**Run it:**
```bash
go run demo3_go_lua_conversion.go
```

**Expected Output:**
```
=== Converting Go to Lua ===
Lua sees:
  person_id:	1
  age:	25
  sex:	F
  alive:	true
  first child:	Alice
  city:	London
  postcode:	SW1A 1AA

=== Converting Lua back to Go ===
  person_id: 1
  age: 26 (modified by Lua!)
  sex: F
  alive: false (modified by Lua!)
  first child: Alice
  city: London
```

**Key Takeaways:**

1. **Go maps become Lua tables** - This is the foundation of Hekate's data model
2. **Lua tables can be lists (numeric indices) or maps (string keys)** - The conversion system handles both
3. **Values can be modified in Lua and read back in Go** - This is how models transform the population
4. **Nested structures are handled recursively** - Maps within maps become tables within tables
5. **Lua is 1-indexed** - Remember this when accessing arrays from Lua
6. **Type assertions are needed when accessing nested data** - Go's type system requires explicit conversion

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

1. `os.Open(csvFile)` - Opens the CSV file
2. `defer file.Close()` - Ensures the file is closed when the function returns
3. `csv.NewReader(file)` - Creates a CSV reader
4. `reader.ReadAll()` - Reads all records at once
5. We check if the file is empty

**Why use `defer` for file closing?**

`defer` ensures the file is closed even if an error occurs later in the function. This prevents file descriptor leaks.

**Why read all records at once?**

Reading all records at once is fine for moderate-sized files (up to a few hundred thousand rows). For very large files (millions of rows), you'd want to stream the data to reduce memory usage.

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

### Full Demo Code

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

### Code Explanation

```go
package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"strconv"
	"strings"
)
```

**What's happening here?**

We're importing:
- `encoding/csv` - Go's built-in CSV parsing library
- `fmt` - For formatted output
- `log` - For error handling
- `strconv` - For string conversions (to int, bool, etc.)
- `strings` - For string manipulation (trimming whitespace)

**The `encoding/csv` library:**

Go's standard library has excellent CSV support. It handles:
- Quoted fields (fields containing commas or newlines)
- Different delimiters (configurable)
- Comments (configurable)
- Lazy quoting (handles poorly formatted CSV)

```go
func main() {
	// CSV data as a string (instead of a file)
	csvData := `person_id,age,sex,alive,income
1,25,F,true,45000
2,30,M,true,35000
3,45,F,false,0
`
```

**The CSV Data:**

This CSV has:
- **Header**: `person_id,age,sex,alive,income`
- **Data rows**: 3 people with different attributes

| person_id | age | sex | alive | income |
|-----------|-----|-----|-------|--------|
| 1 | 25 | F | true | 45000 |
| 2 | 30 | M | true | 35000 |
| 3 | 45 | F | false | 0 |

```go
	reader := csv.NewReader(strings.NewReader(csvData))
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal("Failed to read CSV:", err)
	}

	if len(records) == 0 {
		log.Fatal("CSV is empty")
	}
```

**Creating the CSV Reader:**

1. `strings.NewReader(csvData)` - Creates a reader from the string (like a virtual file)
2. `csv.NewReader(...)` - Creates a CSV reader
3. `reader.ReadAll()` - Reads all records at once

**Why use `strings.NewReader`?**

This allows us to read CSV from a string instead of a file. It's useful for testing and demos.

```go
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
```

**Type Detection Explained:**

1. Get the header row
2. For each column, try to detect the type from the first data row
3. `strconv.Atoi(val)` - Try to convert to int
4. Check for "true" or "false" for booleans
5. Default to "string"

**Why use the first data row for type detection?**

The first data row is usually representative of the data in that column. However, as we saw in the full Hekate implementation, it's better to check multiple rows.

```go
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
```

**Parsing and Displaying Data:**

1. Print the column information (name and type)
2. For each data row:
   - For each column:
     - Convert the value based on its detected type
     - Print the column name and value

**The conversion logic:**

- **int**: `strconv.Atoi` - Convert string to int
- **bool**: Check for "true"/"false" or "True"/"False"
- **string**: Keep as is

```go
type ColumnInfo struct {
	Name string
	Type string
}
```

**The ColumnInfo Struct:**

This stores the column name and its detected type. It's used throughout Hekate to track the structure of the CSV data.

**Run it:**
```bash
go run demo6_csv_reading.go
```

**Expected Output:**
```
=== CSV Data Loaded ===
  Column: person_id (type: int)
  Column: age (type: int)
  Column: sex (type: string)
  Column: alive (type: bool)
  Column: income (type: int)

Data rows:
  Row 1: person_id=1 age=25 sex=F alive=true income=45000 
  Row 2: person_id=2 age=30 sex=M alive=true income=35000 
  Row 3: person_id=3 age=45 sex=F alive=false income=0 
```

**Key Takeaways:**

1. **CSV headers become column names** - The first row of the CSV defines the structure
2. **Types are detected from data** - We inspect the values to guess the type
3. **Empty values become `nil`** - In the full Hekate implementation, empty values are stored as `nil`
4. **Type detection is fallible** - It's a best-guess approach that works in most cases

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

---

## Mini Demo 6: Simple Population Simulation

This demonstrates the core simulation loop without the Lua complexity.

### Full Demo Code

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

### Code Explanation

```go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"
)
```

**What's happening here?**

We're importing packages for:
- `fmt` - Output formatting
- `log` - Error logging
- `math/rand` - Random number generation
- `strconv` - String conversion (though not used in this version)
- `strings` - String manipulation
- `time` - Time functions (for seeding the random generator)

```go
// Person is a simple struct for this demo
type Person struct {
	ID    int
	Age   int
	Sex   string
	Alive bool
}

// Population is a slice of people
type Population []Person
```

**The Person Struct:**

This is a simplified version of Hekate's dynamic data model. Instead of using `map[string]interface{}`, we use a struct with fixed fields. This is easier to understand but less flexible.

**Why use a struct for the demo?**

- **Easier to understand** - No type assertions needed
- **Faster** - No map lookups
- **Type-safe** - Compiler checks the types

**But in Hekate, we use maps:**

In the real Hekate, we use maps to support any CSV structure. The struct approach is only for this demo.

```go
func main() {
	// Seed random
	rand.Seed(time.Now().UnixNano())

	// 1. Create initial population
	population := createPopulation()
	fmt.Println("Initial population:")
	printStats(population)
```

**Seeding the Random Generator:**

`rand.Seed(time.Now().UnixNano())` seeds Go's random generator with the current time. This ensures different results each run.

**If we used a fixed seed:**
```go
rand.Seed(42) // Always the same results
```

**Creating the Initial Population:**

`createPopulation()` creates 10 people with various ages and sexes.

```go
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
```

**The Simulation Loop:**

1. For each year (1 to 5):
   - Age everyone (increment age by 1)
   - Apply mortality (some people may die)
   - Every 2 years (or on the final year), print the age distribution

**The Order Matters:**

The order of operations in the simulation loop affects the results:
1. Age first (everyone gets older)
2. Then apply mortality (older people are more likely to die)

If we applied mortality before aging, a person who dies at age 99 would die before reaching 100, which is incorrect.

```go
	// 3. Show final results
	fmt.Println("\n=== Final Population ===")
	printStats(population)
	printDetails(population)
}
```

**Printing Final Results:**

After all years are simulated, print the final statistics and details.

```go
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
```

**Creating the Population:**

This creates a diverse population with:
- Different ages (2 to 82)
- Both sexes (F and M)
- All alive initially

```go
func agePopulation(pop Population) Population {
	for i := range pop {
		if pop[i].Alive {
			pop[i].Age++
		}
	}
	return pop
}
```

**Aging the Population:**

1. Iterate through all people
2. If alive, increment age by 1
3. Dead people stay dead (and don't age)

```go
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
```

**Applying Mortality:**

1. Iterate through all people
2. Skip dead people
3. Calculate mortality probability based on age:
   - Under 30: 0.1% chance of death
   - 30-64: 1% chance
   - 65-84: 5% chance
   - 85+: 10% chance
4. Roll a random number (`rand.Float64()` returns 0.0 to 1.0)
5. If the random number is less than the probability, the person dies

```go
func countAlive(pop Population) int {
	count := 0
	for _, p := range pop {
		if p.Alive {
			count++
		}
	}
	return count
}
```

**Counting Alive People:**

Simple function that counts how many people are alive.

```go
func printStats(pop Population) {
	alive := countAlive(pop)
	fmt.Printf("Total: %d, Alive: %d, Dead: %d\n", len(pop), alive, len(pop)-alive)
	printAgeDistribution(pop)
}
```

**Printing Statistics:**

Prints:
- Total population
- Number alive
- Number dead
- Age distribution

```go
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
```

**Age Distribution:**

Categorizes the population into:
- Children: under 18
- Adults: 18-64
- Elderly: 65+

This is useful for understanding the population structure.

```go
func printDetails(pop Population) {
	fmt.Println("\nIndividual details:")
	fmt.Println("ID\tAge\tSex\tAlive")
	fmt.Println(strings.Repeat("-", 30))
	for _, p := range pop {
		fmt.Printf("%d\t%d\t%s\t%v\n", p.ID, p.Age, p.Sex, p.Alive)
	}
}
```

**Printing Details:**

Prints each person's details in a table format.

**Run it:**
```bash
go run demo4_simple_simulation.go
```

**Expected Output (will vary due to randomness):**
```
Initial population:
Total: 10, Alive: 10, Dead: 0
  Age distribution: Children: 2, Adults: 5, Elderly: 3

=== Running Simulation ===

Year 1:
  After aging: 10 alive
  After mortality: 10 alive

Year 2:
  After aging: 10 alive
  After mortality: 10 alive
  Age distribution: Children: 2, Adults: 5, Elderly: 3

Year 3:
  After aging: 10 alive
  After mortality: 10 alive

Year 4:
  After aging: 10 alive
  After mortality: 10 alive
  Age distribution: Children: 2, Adults: 5, Elderly: 3

Year 5:
  After aging: 10 alive
  After mortality: 9 alive
  Age distribution: Children: 2, Adults: 5, Elderly: 2

=== Final Population ===
Total: 10, Alive: 9, Dead: 1
  Age distribution: Children: 2, Adults: 5, Elderly: 2

Individual details:
ID	Age	Sex	Alive
------------------------------
1	30	F	true
2	35	M	true
3	50	F	true
4	73	M	true
5	87	F	true
6	7	M	true
7	20	F	true
8	40	M	true
9	60	F	true
10	75	M	false
```

**Key Takeaways:**

1. **The simulation loop: age → mortality → statistics** - This is the basic structure of demographic simulations
2. **Models execute in priority order** - Age before mortality (age affects mortality)
3. **Population state is maintained between iterations** - The population is updated each year
4. **Randomness drives the simulation** - Random numbers determine who dies
5. **Statistics help understand the population** - Age distribution, counts, etc.

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

1. `os.ReadFile(configFile)` - Reads the entire YAML file into memory
2. `yaml.Unmarshal(configBytes, &simConfig)` - Parses the YAML into our `SimulationConfig` struct

```go
    // Validate ID column
    if simConfig.Simulation.IDColumn == "" {
        log.Fatal("ERROR: id_column is required in simulation section of config.yaml")
    }
```

**Explanation:**

This is a validation check. The `id_column` is required because Hekate needs to know which column to use as the unique identifier.

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

```go
    log.Printf("═══ Hekate: Microsimulation Engine ═══")
    log.Printf("Iterations: %d", simConfig.Simulation.Iterations)
    log.Printf("Population file: %s", simConfig.Simulation.PopulationFile)
    log.Printf("ID column: %s", simConfig.Simulation.IDColumn)
    log.Printf("Random seed: %d", simConfig.Simulation.RandomSeed)
    log.Printf("Models loaded: %d", len(simConfig.Models))
```

**Explanation:**

Log the configuration to show the user what's happening.

```go
    // 2. Load population
    population, columns, err := loadPopulationDynamic(simConfig.Simulation.PopulationFile, simConfig.Simulation.IDColumn)
    if err != nil {
        log.Fatalf("Failed to load population: %v", err)
    }

    log.Printf("Loaded %d individuals with %d columns", len(population), len(columns))
```

**Explanation:**

1. `loadPopulationDynamic` reads the CSV and returns a `Population` and column metadata
2. We log the number of people and columns loaded

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

1. `filterEnabledModels` keeps only models with `Enabled: true`
2. `sortModelsByPriority` sorts them by priority (lower number = higher priority)
3. We log which models will actually run

---

## Mini Demo 7: Priority-Based Sorting

This demonstrates how models are sorted by priority.

### Full Demo Code

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

### Code Explanation

```go
package main

import (
	"fmt"
	"sort"
)
```

**What's happening here?**

We're importing `fmt` for output and `sort` for sorting. The `sort` package provides functions for sorting slices.

```go
type Model struct {
	Name     string
	Priority int
}
```

**The Model Struct:**

This is a simplified version of Hekate's `ModelConfig`. It has:
- `Name`: The model name
- `Priority`: The execution priority (lower = earlier)

```go
func main() {
	models := []Model{
		{Name: "fertility", Priority: 3},
		{Name: "mortality", Priority: 2},
		{Name: "age_increment", Priority: 1},
		{Name: "migration", Priority: 4},
	}
```

**Creating the Models:**

These models have different priorities:
- `age_increment`: Priority 1 (run first)
- `mortality`: Priority 2 (run second)
- `fertility`: Priority 3 (run third)
- `migration`: Priority 4 (run last)

**Why this order?**

In a demographic simulation, models have dependencies:
1. Age must be calculated first (age affects everything else)
2. Mortality depends on age (older people are more likely to die)
3. Fertility depends on age and mortality (who's alive to have children)
4. Migration is independent (can happen at any time)

```go
	fmt.Println("=== Before Sorting ===")
	for _, m := range models {
		fmt.Printf("  %s (priority: %d)\n", m.Name, m.Priority)
	}
```

**Printing Before Sorting:**

Shows the models in their original order.

```go
	// Sort by priority (lower number = higher priority)
	sort.Slice(models, func(i, j int) bool {
		return models[i].Priority < models[j].Priority
	})
```

**Sorting with `sort.Slice`:**

`sort.Slice` sorts a slice using a custom comparison function:
- `func(i, j int) bool` - Compares elements at indices i and j
- Returns `true` if element i should come before element j
- In this case, we sort by priority (lower priority = earlier)

```go
	fmt.Println("\n=== After Sorting ===")
	for _, m := range models {
		fmt.Printf("  %s (priority: %d)\n", m.Name, m.Priority)
	}
}
```

**Printing After Sorting:**

Shows the models sorted by priority.

**Run it:**
```bash
go run demo7_sorting.go
```

**Expected Output:**
```
=== Before Sorting ===
  fertility (priority: 3)
  mortality (priority: 2)
  age_increment (priority: 1)
  migration (priority: 4)

=== After Sorting ===
  age_increment (priority: 1)
  mortality (priority: 2)
  fertility (priority: 3)
  migration (priority: 4)
```

**Key Takeaways:**

1. **Lower priority number = runs first** - Priority 1 runs before priority 2
2. **Sorting ensures models execute in the correct order** - Age before mortality, etc.
3. **Models have dependencies** - The order matters for correct results
4. **`sort.Slice` is flexible** - You can sort by any criterion

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

1. `os.Create(outputFile)` - Create the output file (overwrites if exists)
2. `defer file.Close()` - Ensure the file is closed when done
3. `csv.NewWriter(file)` - Create a CSV writer
4. `defer writer.Flush()` - Ensure all data is written when done
5. `getColumnNames(columns)` - Get column names from metadata
6. `writer.Write(colNames)` - Write the header row

```go
    // Sort population by ID column
    sort.Slice(population, func(i, j int) bool {
        id1 := fmt.Sprintf("%v", population[i][idColumn])
        id2 := fmt.Sprintf("%v", population[j][idColumn])
        return id1 < id2
    })
```

**Explanation:**

This sorts the population by the ID column. This ensures consistent output ordering.

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

Sorts models by priority (lower number = higher priority).

---

## Design Patterns and Philosophy

### 1. **Model as Data**

The `ModelConfig` struct and the `Parameters` map allow models to be defined in YAML rather than code.

```
Code (Go) → Infrastructure (loading, saving, orchestration)
Data (YAML) → Models (what the simulation does)
```

### 2. **Interpreter Pattern**

Hekate is essentially an interpreter for the Lua DSL. The Go code provides the interpreter (the Lua VM), and the Lua scripts are the programs being interpreted.

### 3. **Plugin Architecture**

By embedding Lua, Hekate can support new models without recompilation.

### 4. **Pipeline Pattern**

Data flows through a pipeline:
```
CSV → Parse → Validate → Execute Models → Save → CSV
```

### 5. **Strategy Pattern**

Different models implement different strategies through the same interface (the `transition` function).

### 6. **Dynamic Typing**

By using `interface{}` and maps, Hekate can work with any CSV structure.

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

**The Mental Model:**

Think of Hekate as a **factory**:
- The CSV is the raw materials
- The models (Lua scripts) are the machines that process the materials
- The output CSV is the finished product
- The Go code is the factory floor that organizes everything