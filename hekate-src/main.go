// ============================================================================
// main.go - Hekate Entry Point
// ============================================================================

package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: hekate <config.yaml>")
	}
	configFile := os.Args[1]

	// ---- Load config ----
	config, err := LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}
	config.LogSummary()

	// ---- Filter and sort models ----
	enabledModels := FilterEnabledModels(config.Models)
	SortModelsByPriority(enabledModels)

	log.Printf("Enabled models: %d", len(enabledModels))
	for _, model := range enabledModels {
		log.Printf("  - %s (priority: %d)", model.Name, model.Priority)
	}

	verbose := config.Simulation.Verbose

	// ---- Stage 1: Lua syntax validation ----
	if !config.Simulation.SkipValidation {
		validationConfig := &SimulationConfig{
			Simulation: config.Simulation,
			Models:     enabledModels,
		}

		// NEW: pass verbose to control output verbosity
		report := ValidateAllScripts(validationConfig, config.Simulation.StrictMode, verbose)

		if report.FailedModels > 0 {
			logFile := config.Simulation.ValidationLogFile
			if err := WriteValidationLog(report, logFile); err != nil {
				log.Printf("Warning: could not write validation log: %v", err)
			} else {
				log.Printf("📄 Detailed report written to: %s", logFile)
			}
			log.Fatalf("Simulation aborted: %d model(s) failed validation", report.FailedModels)
		}
	} else {
		log.Printf("⚠️  Lua validation DISABLED (skip_validation: true)")
	}

	// ---- Stage 2: Runtime column validation ----
	if !config.Simulation.SkipValidation {
		// NEW: pass verbose
		runtimeReport := ValidateAllColumnsAgainstCSV(config, enabledModels, verbose)

		if runtimeReport.FailedModels > 0 {
			if err := WriteRuntimeValidationLog(runtimeReport, config.Simulation.ValidationLogFile); err != nil {
				log.Printf("Warning: could not write runtime validation log: %v", err)
			} else {
				log.Printf("📄 Detailed report written to: %s", config.Simulation.ValidationLogFile)
			}
			log.Fatalf("Simulation aborted: %d model(s) reference missing columns",
				runtimeReport.FailedModels)
		}
	}

	// ---- Create the Lua VM ----
	luaVM := NewLuaVM(config.Simulation.RandomSeed)
	defer luaVM.Close()

	// ---- Dispatch ----
	if config.Simulation.StreamingMode {
		log.Printf("\n--- Using Streaming Area-by-Area Processing ---")
		RunStreamingSimulation(config, enabledModels, luaVM)
	} else {
		log.Printf("\n--- Using Bulk Processing (load all into memory) ---")
		RunBulkSimulation(config, enabledModels, luaVM)
	}
}