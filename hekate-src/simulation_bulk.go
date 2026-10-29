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
	"path/filepath"
	"strings"
)

// RunBulkSimulation runs the simulation with the entire population in memory.
//
// Algorithm:
//  1. Load all people from the CSV into a Population slice
//  2. For each iteration (year):
//     a. For each enabled model (in priority order):
//        i.  Compute a unique, deterministic seed for (year, model)
//        ii. Execute the model on the full population
//     b. If output_every_iteration is true, write output_N.csv
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
				population, err = ExecuteLuaModel(
					luaVM, model, population, config.Simulation.Verbose,
					modelSeed, config.Simulation.DataDir,
				)
			default:
				log.Fatalf("Unknown model type: %s", model.Type)
			}
			if err != nil {
				log.Fatalf("Model '%s' execution failed: %v", model.Name, err)
			}
		}

		// ---- Per-iteration snapshot (optional) ----
		if config.Simulation.OutputEveryIteration {
			snapshotFile := iterationOutputPath(config.Simulation.OutputFile, i+1)
			// Shallow-copy the population before saving. SavePopulationDynamic
			// sorts the slice in place by ID; since the slice is small and the
			// sort is stable-by-ID, reusing it would be fine, but a copy makes
			// the intent explicit and costs only the slice header.
			snapshot := make(Population, len(population))
			copy(snapshot, population)
			if err := SavePopulationDynamic(snapshot, columns, snapshotFile, config.Simulation.IDColumn); err != nil {
				log.Fatalf("Failed to write iteration snapshot %s: %v", snapshotFile, err)
			}
			log.Printf("  📄 Snapshot: %s", snapshotFile)
		}
	}

	// ---- Save final population ----
	if err := SavePopulationDynamic(population, columns, config.Simulation.OutputFile, config.Simulation.IDColumn); err != nil {
		log.Fatalf("Failed to save population: %v", err)
	}
	log.Printf("\n═══ Simulation Complete ═══")
	log.Printf("Results saved to %s", config.Simulation.OutputFile)
}

// iterationOutputPath derives the per-iteration snapshot path from the final
// output path. For output_file "output.csv" and iteration 3, returns
// "output_3.csv". For "results/final.csv", returns "results/final_3.csv".
func iterationOutputPath(finalPath string, iteration int) string {
	dir := filepath.Dir(finalPath)
	base := filepath.Base(finalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, iteration, ext))
}

// ExecuteLuaModel runs a single Lua model on a population.
//
// If the model declares a `datafile`, it is resolved against dataDir,
// loaded (cached per VM), and injected into `params.datafile` before the
// script runs. Scripts without a datafile see exactly the params they
// saw before — the injection is purely additive.
//
// CRITICAL: This is where the reproducibility guarantee is enforced. The
// vm.Seed(seed) call resets the VM's dedicated random source to a known
// state before the model runs, ensuring identical results across runs.
//
// NEVER remove or reorder the vm.Seed() call - doing so will break
// reproducibility.
func ExecuteLuaModel(vm *LuaVM, model ModelConfig, population Population, verbose bool, seed int64, dataDir string) (Population, error) {
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

	// ---- Build the params map ----
	// Copy the original parameters, then add the datafile (if any) under
	// the key "datafile". This is additive: scripts that don't declare a
	// datafile see exactly the same params as before.
	params := make(map[string]interface{}, len(model.Parameters)+1)
	for k, v := range model.Parameters {
		params[k] = v
	}

	if model.DataFile != "" {
		absPath, err := ResolveDataPath(dataDir, model.DataFile)
		if err != nil {
			return nil, fmt.Errorf("model '%s': datafile: %w", model.Name, err)
		}
		rows, err := vm.LoadDataFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("model '%s': datafile '%s': %w", model.Name, model.DataFile, err)
		}
		params["datafile"] = []map[string]interface{}(rows)
		if verbose {
			log.Printf("    datafile: %s (%d rows)", model.DataFile, len(rows))
		}
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
	result, err := vm.ExecuteLuaScript(script, popSlice, params)
	if err != nil {
		return nil, err
	}
	return Population(result), nil
}