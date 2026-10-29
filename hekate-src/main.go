// ============================================================================
// main.go - Hekate Entry Point
// ============================================================================

package main

import (
	"fmt"
	"log"
	"os"
)

// Version is the current Hekate version.
// Bump this when cutting a release.
const Version = "0.9.1"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// ---- Handle flags ----
	switch os.Args[1] {
	case "--version", "-v":
		fmt.Printf("hekate %s\n", Version)
		return
	case "--help", "-h":
		printUsage()
		return
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

// printUsage prints the command-line usage message.
func printUsage() {
	fmt.Fprintf(os.Stderr, `hekate %s — Microsimulation Engine

Usage:
  hekate <config.yaml>     Run a simulation with the given config file
  hekate --version         Print the version and exit
  hekate --help            Print this help message and exit

Flags:
  -v, --version            Print the version and exit
  -h, --help               Print this help message and exit

Example:
  hekate config.yaml
`, Version)
}
