// ============================================================================
// simulation_bulk.go - Bulk Simulation Mode
// ============================================================================
//
// Loads the entire population into memory, runs all iterations, then saves.
// Fast for small-to-medium populations but memory scales with size.
//
// Best for: populations up to ~1M people
// Memory:   scales linearly with population size
//
// ============================================================================

package main

import (
	"fmt"
	"log"
)

// RunBulkSimulation runs the simulation with the entire population in memory.
//
// Algorithm:
//  1. Load all people from the CSV into a Population slice
//  2. For each iteration (year):
//     a. For each enabled model (in priority order):
//        i.  Compute a unique, deterministic seed for (year, model)
//        ii. Execute the model on the full population
//  3. Save the final population to the output CSV
func RunBulkSimulation(config *SimulationConfig, enabledModels []ModelConfig, luaVM *LuaVM) {
	// ---- Load entire population ----
	population, columns, err := LoadPopulationDynamic(
		config.Simulation.PopulationFile,
		config.Simulation.IDColumn,
	)
	if err != nil {
		log.Fatalf("Failed to load population: %v", err)
	}
	log.Printf("Loaded %d individuals with %d columns", len(population), len(columns))

	// ---- Main iteration loop ----
	for i := 0; i < config.Simulation.Iterations; i++ {
		log.Printf("\n═══ Iteration %d/%d ═══", i+1, config.Simulation.Iterations)

		for modelIndex, model := range enabledModels {
			// Compute a unique, reproducible seed for this (year, model) pair.
			//
			// The formula spreads seeds out so different models don't share
			// the same random sequence:
			//   base_seed + year*1000 + model_index
			modelSeed := config.Simulation.RandomSeed + int64(i)*1000 + int64(modelIndex)

			switch model.Type {
			case "lua_model":
				population, err = ExecuteLuaModel(luaVM, model, population, config.Simulation.Verbose, modelSeed)
			default:
				log.Fatalf("Unknown model type: %s", model.Type)
			}
			if err != nil {
				log.Fatalf("Model '%s' execution failed: %v", model.Name, err)
			}
		}
	}

	// ---- Save final population ----
	if err := SavePopulationDynamic(population, columns, config.Simulation.OutputFile, config.Simulation.IDColumn); err != nil {
		log.Fatalf("Failed to save population: %v", err)
	}
	log.Printf("\n═══ Simulation Complete ═══")
	log.Printf("Results saved to %s", config.Simulation.OutputFile)
}

// ExecuteLuaModel runs a single Lua model on a population.
//
// CRITICAL: This is where the reproducibility guarantee is enforced. The
// vm.Seed(seed) call resets the VM's dedicated random source to a known
// state before the model runs, ensuring identical results across runs.
//
// NEVER remove or reorder the vm.Seed() call - doing so will break
// reproducibility.
func ExecuteLuaModel(vm *LuaVM, model ModelConfig, population Population, verbose bool, seed int64) (Population, error) {
	// ---- Extract the Lua script from the model parameters ----
	scriptInterface, ok := model.Parameters["script"]
	if !ok {
		return nil, fmt.Errorf("model '%s' missing 'script' parameter", model.Name)
	}
	script, ok := scriptInterface.(string)
	if !ok {
		return nil, fmt.Errorf("model '%s' script is not a string", model.Name)
	}

	if verbose {
		log.Printf("  ▶ Executing: %s (priority: %d, seed: %d)", model.Name, model.Priority, seed)
	} else {
		log.Printf("  ▶ %s", model.Name)
	}

	// ========================================================================
	// THE REPRODUCIBILITY FIX
	// ========================================================================
	// Reset the VM's dedicated random source to the derived seed. This is
	// what guarantees that, given the same config, the same random sequence
	// is used on every run.
	//
	// DO NOT REMOVE. See lua_vm.go for the full explanation.
	// ========================================================================
	vm.Seed(seed)

	// ---- Run the script ----
	popSlice := []map[string]interface{}(population)
	result, err := vm.ExecuteLuaScript(script, popSlice, model.Parameters)
	if err != nil {
		return nil, err
	}
	return Population(result), nil
}