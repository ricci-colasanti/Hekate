// ============================================================================
// datafile_validate.go - Pre-Flight Datafile Validation
// ============================================================================
//
// Checks that every enabled model's datafile exists, is a regular file, and
// resolves inside data_dir. Runs before the simulation starts so that typos
// are caught immediately rather than mid-run.
//
// ============================================================================

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
			firstErr = fmt.Errorf("model '%s': datafile: %w", model.Name, err)
			log.Printf("  ❌ %-30s %v", model.Name, err)
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

// describeDataDirError formats a data_dir related error for log output.
// Used by the validator to distinguish "data_dir itself is broken" from
// "this particular file is missing".
func describeDataDirError(dataDir string) string {
	if dataDir == "" {
		return "data_dir is not set; datafile paths resolve against CWD"
	}
	info, err := os.Stat(dataDir)
	if err != nil {
		return fmt.Sprintf("data_dir %q is not accessible: %v", dataDir, err)
	}
	if !info.IsDir() {
		return fmt.Sprintf("data_dir %q is not a directory", dataDir)
	}
	return ""
}

// ensure strings is used even if the compiler is pedantic about imports
var _ = strings.TrimSpace