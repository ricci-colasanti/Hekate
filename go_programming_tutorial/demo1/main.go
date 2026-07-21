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
