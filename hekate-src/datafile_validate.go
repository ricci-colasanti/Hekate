// ============================================================================
// datafile_validate.go - Pre-Flight Datafile Validation
// ============================================================================
//
// Checks that every enabled model's datafile exists, is a regular file, and
// resolves inside data_dir. Runs before the simulation starts so that typos
// are caught immediately rather than mid-run.
//
// ERROR MESSAGES
// --------------
// When a datafile is missing, the error prints:
//   - the user-written path (as it appears in the YAML)
//   - the resolved absolute path (what the engine actually looked for)
//   - a "did you mean" suggestion, if a similar filename exists in data_dir
//
// Printing the resolved path matters: path conventions differ between
// population_file (CWD-relative) and datafile (data_dir-relative), and a
// failure with only the user-written path is ambiguous. "data/data/x.csv"
// is the classic symptom, and it's invisible unless you see the resolved
// path.
//
// ============================================================================

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// ValidateDataFiles checks all model datafiles and returns an error
// describing the first problem found, or nil if all are valid.
//
// If verbose is true, prints a banner and per-model status. Errors are
// always printed, regardless of verbose.
func ValidateDataFiles(config *SimulationConfig, models []ModelConfig, verbose bool) error {
	dataDir := config.Simulation.DataDir

	// Count models with datafiles
	withDatafile := 0
	for _, m := range models {
		if m.DataFile != "" {
			withDatafile++
		}
	}
	if withDatafile == 0 {
		return nil // nothing to check
	}

	if verbose {
		log.Printf("")
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("  Datafile Validation")
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("")
		if dataDir != "" {
			log.Printf("  data_dir: %s", dataDir)
		} else {
			log.Printf("  data_dir: (not set)")
		}
		log.Printf("")
	}

	// Read the directory once for "did you mean" suggestions.
	var candidates []string
	if dataDir != "" {
		if entries, err := os.ReadDir(dataDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					candidates = append(candidates, e.Name())
				}
			}
		}
	}

	var firstErr error
	for _, model := range models {
		if model.DataFile == "" {
			continue
		}

		absPath, err := ResolveDataPath(dataDir, model.DataFile)
		if err != nil {
			// Path resolution itself failed (escaped data_dir, absolute path
			// when data_dir is set, etc.). This is a different class of error
			// from "file not found" and shouldn't suggest filenames.
			firstErr = fmt.Errorf("model '%s': datafile: %w", model.Name, err)
			log.Printf("  ❌ %-30s datafile '%s': %v", model.Name, model.DataFile, err)
			continue
		}

		if err := StatDataPath(absPath); err != nil {
			firstErr = fmt.Errorf("model '%s': datafile '%s': %w", model.Name, model.DataFile, err)

			// "Did you mean" suggestion
			suggestion := ""
			if len(candidates) > 0 {
				base := filepath.Base(model.DataFile)
				suggestion = findClosestMatch(base, candidates)
			}

			if suggestion != "" {
				log.Printf("  ❌ %-30s datafile '%s' not found (did you mean '%s'?)",
					model.Name, model.DataFile, suggestion)
			} else {
				log.Printf("  ❌ %-30s datafile '%s' not found", model.Name, model.DataFile)
			}
			// The resolved path is the diagnostic. Without it, the user sees
			// "not found" and has to reason about which directory conventions
			// apply to which config key.
			log.Printf("       └─ resolved as: %s", absPath)
			continue
		}

		if verbose {
			log.Printf("  ✅ %-30s %s", model.Name, model.DataFile)
		}
	}

	if verbose {
		log.Printf("")
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("")
	}

	return firstErr
}