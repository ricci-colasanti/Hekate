// ============================================================================
// lua_validate.go - Lua Script Pre-Flight Validation
// ============================================================================
//
// This file implements the syntax validation step that runs BEFORE the
// simulation begins. It compiles every Lua script in the config and reports
// any errors found, along with helpful suggestions for common mistakes.
//
// WHY THIS MATTERS
// ----------------
// Without validation, a Lua syntax error in year 47 of a 50-year simulation
// would only surface after hours of wasted computation. By validating up
// front, users get immediate feedback and can fix errors before starting
// a long run.
//
// VERBOSE MODE
// ------------
// When `verbose: true` in config:
//   - Full banner is printed
//   - Every model's status is shown (pass/warn/fail)
//   - Every warning is displayed line-by-line
//
// When `verbose: false`:
//   - Compact summary only
//   - Warnings are counted but not displayed
//   - Errors are ALWAYS displayed (critical)
//
// ============================================================================

package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// ValidationResult holds the outcome of validating a single script.
type ValidationResult struct {
	ModelName string
	Success   bool
	Error     string
	Warnings  []string
}

// ValidationReport holds the aggregate result across all models.
type ValidationReport struct {
	TotalModels    int
	ValidModels    int
	FailedModels   int
	TotalWarnings  int
	Results        []ValidationResult
	ValidationTime time.Duration
}

// ValidateAllScripts compiles every Lua script in the config and returns
// a detailed report. Errors do NOT halt the caller - it's the caller's
// responsibility to decide whether to proceed.
//
// If strictMode is true, warnings are also treated as errors (used in CI).
// If verbose is false, warnings are counted but not printed (compact mode).
func ValidateAllScripts(config *SimulationConfig, strictMode bool, verbose bool) *ValidationReport {
	startTime := time.Now()

	report := &ValidationReport{
		TotalModels: len(config.Models),
		Results:     make([]ValidationResult, 0, len(config.Models)),
	}

	if verbose {
		log.Printf("")
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("  Lua Script Pre-Flight Validation")
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("")
	}

	// Load the population CSV's column types once. Used to decide which
	// columns need tonumber() in scripts. Best-effort: if the CSV can't be
	// read, we skip the tonumber warning rather than fail here — the CSV
	// itself is validated separately by ValidateAllColumnsAgainstCSV.
	numericColumns := loadNumericColumns(config.Simulation.PopulationFile)

	for _, model := range config.Models {
		result := validateSingleScript(model, strictMode, numericColumns)
		report.Results = append(report.Results, result)

		if result.Success && len(result.Warnings) == 0 {
			report.ValidModels++
			if verbose {
				log.Printf("  ✅ %-30s (priority: %d)", model.Name, model.Priority)
			}
		} else if result.Success && len(result.Warnings) > 0 {
			report.ValidModels++
			report.TotalWarnings += len(result.Warnings)
			if verbose {
				log.Printf("  ⚠️  %-30s (priority: %d) — %d warning(s)",
					model.Name, model.Priority, len(result.Warnings))
				for _, w := range result.Warnings {
					log.Printf("       └─ %s", w)
				}
			}
		} else {
			report.FailedModels++
			// Errors are ALWAYS shown, even in non-verbose mode
			log.Printf("  ❌ %-30s (priority: %d)", model.Name, model.Priority)
			log.Printf("       └─ %s", result.Error)
		}
	}

	report.ValidationTime = time.Since(startTime)

	if verbose {
		log.Printf("")
		log.Printf("════════════════════════════════════════════════════════")
		if report.FailedModels == 0 {
			log.Printf("  ✅ All %d models validated successfully", report.ValidModels)
			if report.TotalWarnings > 0 {
				log.Printf("     (%d warning(s) — see above)", report.TotalWarnings)
			}
		} else {
			log.Printf("  ❌ Validation FAILED: %d of %d models have errors",
				report.FailedModels, report.TotalModels)
		}
		log.Printf("  ⏱  Validation took %v", report.ValidationTime.Round(time.Millisecond))
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("")
	} else {
		// Compact summary in non-verbose mode (only if there are failures
		// or warnings to report)
		if report.FailedModels > 0 {
			log.Printf("❌ Lua validation: %d of %d models failed (see log)",
				report.FailedModels, report.TotalModels)
		} else if report.TotalWarnings > 0 {
			log.Printf("⚠️  Lua validation: passed with %d warning(s) (set verbose: true to see)",
				report.TotalWarnings)
		}
	}

	return report
}

// loadNumericColumns returns the set of column names in the population CSV
// that are typed as int or float. Returns an empty set if the CSV can't be
// read — the warning is best-effort, not a hard error.
//
// This is what removes the hard-coded column list: the validator learns
// which columns are numeric from the CSV itself, not from a list in Go.
func loadNumericColumns(csvPath string) map[string]bool {
	cols, err := DetectColumns(csvPath)
	if err != nil {
		return map[string]bool{}
	}
	out := make(map[string]bool, len(cols))
	for _, c := range cols {
		if c.Type == "int" || c.Type == "float" {
			out[c.Name] = true
		}
	}
	return out
}

// validateSingleScript validates one model's Lua script.
//
// Strategy:
//  1. Extract the script from model.Parameters
//  2. Check for common mistakes (via source scanning)
//  3. Try to compile it in a fresh Lua VM
//  4. Check that a "transition" function was defined
//  5. Return a structured result
//
// numericColumns is the set of CSV columns typed as int/float; the scanner
// uses it to decide which person.<col> reads should be wrapped in tonumber().
func validateSingleScript(model ModelConfig, strictMode bool, numericColumns map[string]bool) ValidationResult {
	result := ValidationResult{
		ModelName: model.Name,
		Success:   false,
	}

	// ---- Extract the script ----
	scriptInterface, ok := model.Parameters["script"]
	if !ok {
		result.Error = "missing 'script' parameter"
		return result
	}
	script, ok := scriptInterface.(string)
	if !ok {
		result.Error = "'script' parameter is not a string"
		return result
	}

	// ---- Scan for common mistakes (warnings) ----
	result.Warnings = scanForCommonMistakes(script, model, numericColumns)

	// ---- Try to compile the script ----
	compileErr := tryCompileScript(script)
	if compileErr != "" {
		result.Error = compileErr
		return result
	}

	// ---- Strict mode: treat warnings as errors ----
	if strictMode && len(result.Warnings) > 0 {
		result.Error = fmt.Sprintf("strict mode: %d warning(s) treated as errors", len(result.Warnings))
		return result
	}

	result.Success = true
	return result
}

// tryCompileScript compiles the script in a fresh Lua VM and verifies that
// a "transition" function is defined. Returns an error string (empty on success).
func tryCompileScript(script string) (errMsg string) {
	defer func() {
		if r := recover(); r != nil {
			errMsg = fmt.Sprintf("Lua interpreter panic: %v", r)
		}
	}()

	L := lua.NewState()
	defer L.Close()

	fn, err := L.LoadString(script)
	if err != nil {
		return formatLuaError(err)
	}

	L.Push(fn)
	if err := L.PCall(0, lua.MultRet, nil); err != nil {
		return formatLuaError(err)
	}

	transitionFn := L.GetGlobal("transition")
	if transitionFn.Type() != lua.LTFunction {
		return "script does not define a 'transition(population, params)' function"
	}

	return ""
}

// formatLuaError converts a gopher-lua error into a human-friendly message.
func formatLuaError(err error) string {
	msg := err.Error()
	suggestions := suggestFixForError(msg)
	if suggestions != "" {
		return fmt.Sprintf("%s\n       💡 %s", msg, suggestions)
	}
	return msg
}

// suggestFixForError returns a helpful suggestion for common Lua errors.
func suggestFixForError(errMsg string) string {
	lower := strings.ToLower(errMsg)

	switch {
	case strings.Contains(lower, "expected 'end'"):
		return "Check that every 'function', 'if', 'for', and 'while' has a matching 'end'."
	case strings.Contains(lower, "unexpected symbol near"):
		return "Look for a typo in the line above the error. Common causes: missing comma, extra bracket."
	case strings.Contains(lower, "unfinished string"):
		return "Check for unclosed quotes. Remember: Lua strings must use matching quotes (\"...\" or '...')."
	case strings.Contains(lower, "malformed number"):
		return "Check for typos in numeric literals (e.g. '1.2.3' or '0xZZ')."
	case strings.Contains(lower, "'=' expected"):
		return "You may have used '==' in an assignment or '=' in a comparison."
	case strings.Contains(lower, "unexpected symbol"):
		return "The parser found something it didn't expect. Check the line for typos or missing punctuation."
	}
	return ""
}

// scanForCommonMistakes examines the script source for patterns that are
// legal Lua but usually indicate a mistake. Returns a list of warnings.
//
// numericColumns is the set of columns the CSV types as int or float. The
// tonumber() warning fires only for those columns — the CSV is the source
// of truth, so a string column like 'gender' is never warned about.
//
// The model is passed in so that warnings can be suppressed when the model
// already declares a datafile (e.g. the io.lines warning).
func scanForCommonMistakes(script string, model ModelConfig, numericColumns map[string]bool) []string {
	var warnings []string
	lines := strings.Split(script, "\n")

	// Precompile one LHS-assignment regex per numeric column, so we can
	// skip lines like `person.age = age + 1` (writing, not reading).
	lhsPatterns := make(map[string]*regexp.Regexp, len(numericColumns))
	for col := range numericColumns {
		lhsPatterns[col] = regexp.MustCompile(`^\s*person\.` + regexp.QuoteMeta(col) + `\s*=`)
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// ---- Check 1: # used as a comment (should be -- in Lua) ----
		if strings.HasPrefix(trimmed, "#") {
			warnings = append(warnings,
				fmt.Sprintf("line %d: '#' is not a comment in Lua — use '--' instead", i+1))
		}

		// ---- Check 2: missing tonumber on columns typed as numeric ----
		for col := range numericColumns {
			pattern := "person." + col
			if !strings.Contains(line, pattern) {
				continue
			}
			// Skip lines that write to the column (LHS assignment):
			// `person.age = ...` doesn't need tonumber().
			if lhsPatterns[col].MatchString(line) {
				continue
			}
			// Skip lines that already convert, or compare as strings.
			if strings.Contains(line, "tonumber(") || strings.Contains(line, "tostring(") {
				continue
			}
			if strings.Contains(line, "==") && strings.Contains(line, "\"") {
				continue
			}
			warnings = append(warnings,
				fmt.Sprintf("line %d: consider using tonumber(person.%s) — CSV types this column as numeric",
					i+1, col))
		}
	}

	// ---- Check 3: math.randomseed used (overrides per-model seed) ----
	if strings.Contains(script, "math.randomseed") {
		warnings = append(warnings,
			"script calls math.randomseed() — Hekate seeds the RNG per-model automatically; manual seeding will override that seed.")
	}

	// ---- Check 4: io.lines / io.open on a CSV when no datafile is declared ----
	// Soft deprecation: reading CSVs with io.lines works, but is slower and
	// loses type detection. The `datafile` key on the model is the blessed path.
	if model.DataFile == "" {
		for i, line := range lines {
			usesIO := strings.Contains(line, "io.lines(") || strings.Contains(line, "io.open(")
			if !usesIO {
				continue
			}
			// Only warn if the argument looks like a CSV file.
			if !strings.Contains(strings.ToLower(line), ".csv") {
				continue
			}
			warnings = append(warnings,
				fmt.Sprintf("line %d: script reads a CSV with io.lines()/io.open() — consider declaring it under 'datafile:' on the model so it's read once per run and type-detected", i+1))
		}
	}

	return warnings
}

// WriteValidationLog writes the validation report to a log file.
func WriteValidationLog(report *ValidationReport, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create validation log: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, "Hekate — Lua Script Validation Report\n")
	fmt.Fprintf(file, "======================================\n\n")
	fmt.Fprintf(file, "Generated: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Summary:\n")
	fmt.Fprintf(file, "  Total models:    %d\n", report.TotalModels)
	fmt.Fprintf(file, "  Valid models:    %d\n", report.ValidModels)
	fmt.Fprintf(file, "  Failed models:   %d\n", report.FailedModels)
	fmt.Fprintf(file, "  Total warnings:  %d\n", report.TotalWarnings)
	fmt.Fprintf(file, "  Validation time: %v\n\n", report.ValidationTime.Round(time.Millisecond))

	fmt.Fprintf(file, "Details:\n")
	fmt.Fprintf(file, "--------\n\n")
	for _, r := range report.Results {
		status := "✅ PASS"
		if !r.Success {
			status = "❌ FAIL"
		} else if len(r.Warnings) > 0 {
			status = "⚠️  WARN"
		}

		fmt.Fprintf(file, "[%s] %s\n", status, r.ModelName)
		if r.Error != "" {
			fmt.Fprintf(file, "  Error: %s\n", r.Error)
		}
		for _, w := range r.Warnings {
			fmt.Fprintf(file, "  Warning: %s\n", w)
		}
		fmt.Fprintf(file, "\n")
	}

	if report.FailedModels > 0 {
		fmt.Fprintf(file, "Next steps:\n")
		fmt.Fprintf(file, "  1. Fix the errors listed above in your config.yaml\n")
		fmt.Fprintf(file, "  2. Re-run Hekate to verify\n")
		fmt.Fprintf(file, "  3. If the error message is unclear, see:\n")
		fmt.Fprintf(file, "     - README.md section 'Troubleshooting'\n")
		fmt.Fprintf(file, "     - LLM_PROMPT_TEMPLATE.md for AI-assisted help\n")
	}

	return nil
}