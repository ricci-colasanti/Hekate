// ============================================================================
// config.go - Configuration Structures and Loading
// ============================================================================
//
// This file defines the config structures that map to the YAML file, and
// the functions for loading and validating that config.
//
// The YAML structure is:
//
//	simulation:
//	  iterations: 10
//	  population_file: "population.csv"
//	  ...
//	models:
//	  - name: "model_name"
//	    ...
//
// ============================================================================

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// Configuration Structures
// ============================================================================

// ModelConfig represents a single model definition from the YAML config.
//
// A "model" is a Lua script that transforms the population each iteration.
// Models run in priority order (lower numbers first), which matters because
// many models have dependencies.
type ModelConfig struct {
	Name        string                 `yaml:"name"`        // Human-readable name for logging
	Type        string                 `yaml:"type"`        // Currently only "lua_model"
	Priority    int                    `yaml:"priority"`    // Execution order (lower = earlier)
	Enabled     bool                   `yaml:"enabled"`     // Toggle without removing from config
	Description string                 `yaml:"description"` // Documentation for humans
	Parameters  map[string]interface{} `yaml:"parameters"`  // Arbitrary parameters (including "script")
}

// SimulationConfig is the top-level config structure.
type SimulationConfig struct {
	Simulation SimulationParameters `yaml:"simulation"`
	Models     []ModelConfig        `yaml:"models"`
}

// SimulationParameters holds global simulation settings.
type SimulationParameters struct {
	Iterations     int    `yaml:"iterations"`      // Number of years to simulate
	PopulationFile string `yaml:"population_file"` // Input CSV path
	OutputFile     string `yaml:"output_file"`     // Output CSV path
	RandomSeed     int64  `yaml:"random_seed"`     // Base seed for reproducibility
	Verbose        bool   `yaml:"verbose"`         // Detailed logging

	// IDColumn is the unique identifier column in the CSV.
	// REQUIRED. Used as a primary key and for deterministic output ordering.
	IDColumn string `yaml:"id_column"`

	// AreaColumn is the column used to group people by geographic area.
	// REQUIRED when StreamingMode is true. The CSV must be SORTED by this
	// column so that people in the same area appear consecutively.
	AreaColumn string `yaml:"area_column"`

	// StreamingMode enables area-by-area processing for large populations.
	StreamingMode bool `yaml:"streaming_mode"`

	// StrictMode treats validation warnings as errors.
	// Recommended for CI/CD pipelines to ensure high-quality configs.
	// Default: false (warnings are displayed but do not halt the run).
	StrictMode bool `yaml:"strict_mode"`

	// SkipValidation disables pre-flight Lua validation entirely.
	// NOT recommended — only use when debugging Hekate itself or when
	// a false-positive warning is blocking a legitimate config.
	// Default: false (validation always runs).
	SkipValidation bool `yaml:"skip_validation"`

	// ValidationLogFile is the path where the validation report is written
	// when validation fails. Defaults to "hekate_validation.log".
	ValidationLogFile string `yaml:"validation_log_file"`
}

// ColumnInfo describes a single column detected from the CSV.
type ColumnInfo struct {
	Name string
	Type string // "int", "float", "string", "bool"
}

// Population is the core data structure - a slice of maps.
//
// Each map represents one person. Keys are CSV column names, values are
// the parsed data. Maps (not structs) are used so Hekate can handle any
// CSV structure without recompilation.
type Population []map[string]interface{}

// ============================================================================
// Config Loading and Validation
// ============================================================================

// LoadConfig reads a YAML config file and returns the parsed configuration.
//
// Defaults applied:
//   - RandomSeed: generated from current time if not set
//   - ValidationLogFile: "hekate_validation.log" if not set
func LoadConfig(filename string) (*SimulationConfig, error) {
	configBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config SimulationConfig
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply defaults
	if config.Simulation.RandomSeed == 0 {
		config.Simulation.RandomSeed = time.Now().UnixNano()
	}
	if config.Simulation.ValidationLogFile == "" {
		config.Simulation.ValidationLogFile = "hekate_validation.log"
	}

	return &config, nil
}

// Validate checks that all required fields are present and consistent.
//
// This is a STRUCTURAL validation - it checks that the config itself is
// usable. It does NOT validate the Lua scripts (that's done separately by
// ValidateAllScripts).
func (c *SimulationConfig) Validate() error {
	// ---- Required fields ----
	if c.Simulation.IDColumn == "" {
		return fmt.Errorf("id_column is required in simulation section")
	}
	if c.Simulation.PopulationFile == "" {
		return fmt.Errorf("population_file is required")
	}
	if c.Simulation.OutputFile == "" {
		return fmt.Errorf("output_file is required")
	}

	// ---- Streaming-mode consistency ----
	if c.Simulation.StreamingMode && c.Simulation.AreaColumn == "" {
		return fmt.Errorf("area_column is required when streaming_mode is true")
	}

	// ---- Iterations must be positive ----
	if c.Simulation.Iterations < 0 {
		return fmt.Errorf("iterations must be >= 0 (got %d)", c.Simulation.Iterations)
	}

	// ---- At least one model must be present ----
	if len(c.Models) == 0 {
		return fmt.Errorf("no models defined in config")
	}

	// ---- Every model must have required fields ----
	for i, model := range c.Models {
		if model.Name == "" {
			return fmt.Errorf("model #%d is missing 'name' field", i+1)
		}
		if model.Type == "" {
			return fmt.Errorf("model '%s' is missing 'type' field", model.Name)
		}
		if model.Type != "lua_model" {
			return fmt.Errorf("model '%s' has unsupported type '%s' (only 'lua_model' is supported)",
				model.Name, model.Type)
		}
		if model.Parameters == nil {
			return fmt.Errorf("model '%s' has no parameters", model.Name)
		}
	}

	return nil
}

// LogSummary prints a human-readable summary of the configuration.
func (c *SimulationConfig) LogSummary() {
	log.Printf("═══ Hekate: Microsimulation Engine ═══")
	log.Printf("Iterations: %d", c.Simulation.Iterations)
	log.Printf("Population file: %s", c.Simulation.PopulationFile)
	log.Printf("ID column: %s", c.Simulation.IDColumn)
	if c.Simulation.StreamingMode {
		log.Printf("Mode: STREAMING (area-by-area)")
		log.Printf("Area column: %s", c.Simulation.AreaColumn)
	} else {
		log.Printf("Mode: BULK (load all into memory)")
	}
	log.Printf("Random seed: %d", c.Simulation.RandomSeed)
	if c.Simulation.StrictMode {
		log.Printf("Strict mode: ENABLED (warnings treated as errors)")
	}
	if c.Simulation.SkipValidation {
		log.Printf("Lua validation: DISABLED")
	}
	log.Printf("Models loaded: %d", len(c.Models))
}