// ============================================================================
// runtime_validate.go - CSV Column Name Validation
// ============================================================================
//
// This file implements the second stage of validation: checking that the
// columns referenced in Lua scripts actually exist in the CSV.
//
// See the header of lua_validate.go for the design philosophy. The same
// verbose/non-verbose rules apply here.
//
// ============================================================================

package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

// ============================================================================
// Result Types
// ============================================================================

type RuntimeValidationResult struct {
	ModelName      string
	Success        bool
	MissingColumns []string
	UnusedColumns  []string
	AllowedColumns []string
	Error          string
}

type RuntimeValidationReport struct {
	TotalModels  int
	ValidModels  int
	FailedModels int
	TotalMissing int
	CSVColumns   []string
	Results      []RuntimeValidationResult
}

// ============================================================================
// Main Entry Point
// ============================================================================

// ValidateAllColumnsAgainstCSV performs runtime validation for all models.
//
// If verbose is false, only a compact summary is printed. Errors are always
// shown because they cause the run to abort.
func ValidateAllColumnsAgainstCSV(config *SimulationConfig, models []ModelConfig, verbose bool) *RuntimeValidationReport {
	report := &RuntimeValidationReport{
		TotalModels: len(models),
		Results:     make([]RuntimeValidationResult, 0, len(models)),
	}

	if verbose {
		log.Printf("")
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("  Runtime Validation: Column Names vs CSV")
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("")
	}

	// ---- Read CSV header ----
	csvColumns, err := readCSVHeader(config.Simulation.PopulationFile)
	if err != nil {
		log.Printf("  ❌ Failed to read CSV header: %v", err)
		report.FailedModels = len(models)
		return report
	}
	report.CSVColumns = csvColumns

	if verbose {
		log.Printf("  CSV columns: %s", strings.Join(csvColumns, ", "))
		log.Printf("")
	}

	// Build fast lookup map
	columnSet := make(map[string]bool, len(csvColumns))
	for _, col := range csvColumns {
		columnSet[col] = true
	}

	// ---- Validate each model ----
	for _, model := range models {
		result := validateModelColumns(model, columnSet, csvColumns)
		report.Results = append(report.Results, result)

		if result.Success {
			report.ValidModels++
			if verbose {
				log.Printf("  ✅ %-30s (priority: %d)", model.Name, model.Priority)
			}
		} else {
			report.FailedModels++
			report.TotalMissing += len(result.MissingColumns)
			// Errors are ALWAYS shown, even in non-verbose mode
			log.Printf("  ❌ %-30s (priority: %d)", model.Name, model.Priority)
			for _, col := range result.MissingColumns {
				suggestion := findClosestMatch(col, csvColumns)
				if suggestion != "" {
					log.Printf("       └─ column '%s' not found (did you mean '%s'?)", col, suggestion)
				} else {
					log.Printf("       └─ column '%s' not found in CSV", col)
				}
			}
		}
	}

	// ---- Summary ----
	if verbose {
		log.Printf("")
		log.Printf("════════════════════════════════════════════════════════")
		if report.FailedModels == 0 {
			log.Printf("  ✅ All %d models reference valid columns", report.ValidModels)
		} else {
			log.Printf("  ❌ %d of %d models reference missing columns (%d total)",
				report.FailedModels, report.TotalModels, report.TotalMissing)
		}
		log.Printf("════════════════════════════════════════════════════════")
		log.Printf("")
	} else {
		if report.FailedModels > 0 {
			log.Printf("❌ Column validation: %d of %d models reference missing columns",
				report.FailedModels, report.TotalModels)
		}
	}

	return report
}

// ============================================================================
// Model Validation
// ============================================================================

func validateModelColumns(model ModelConfig, columnSet map[string]bool, csvColumns []string) RuntimeValidationResult {
	result := RuntimeValidationResult{
		ModelName: model.Name,
		Success:   false,
	}

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

	allowedColumns := extractAllowedColumns(script)
	result.AllowedColumns = allowedColumns
	allowedSet := make(map[string]bool, len(allowedColumns))
	for _, c := range allowedColumns {
		allowedSet[c] = true
	}

	referenced := scanForColumnReferences(script)

	var missing []string
	for col := range referenced {
		if columnSet[col] {
			continue
		}
		if allowedSet[col] {
			continue
		}
		if isSpecialColumn(col) {
			continue
		}
		missing = append(missing, col)
	}

	sort.Strings(missing)
	result.MissingColumns = missing
	result.Success = len(missing) == 0
	return result
}

// ============================================================================
// CSV Header Reading
// ============================================================================

func readCSVHeader(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("cannot read header: %w", err)
	}

	for i := range header {
		header[i] = strings.TrimSpace(header[i])
	}

	return header, nil
}

// ============================================================================
// Lua Script Scanning
// ============================================================================

var columnRefPattern = regexp.MustCompile(`\bperson\.([a-zA-Z_][a-zA-Z0-9_]*)\b`)
var bracketRefPattern = regexp.MustCompile(`\bperson\s*\[\s*["']([a-zA-Z_][a-zA-Z0-9_]*)["']\s*\]`)
var allowColumnsPattern = regexp.MustCompile(`--\s*hekate:allow-columns\s+(.+)`)

func scanForColumnReferences(script string) map[string]bool {
	referenced := make(map[string]bool)

	for _, match := range columnRefPattern.FindAllStringSubmatch(script, -1) {
		if len(match) > 1 {
			referenced[match[1]] = true
		}
	}

	for _, match := range bracketRefPattern.FindAllStringSubmatch(script, -1) {
		if len(match) > 1 {
			referenced[match[1]] = true
		}
	}

	return referenced
}

func extractAllowedColumns(script string) []string {
	match := allowColumnsPattern.FindStringSubmatch(script)
	if len(match) < 2 {
		return nil
	}

	raw := strings.TrimSpace(match[1])
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ============================================================================
// Special Columns
// ============================================================================

func isSpecialColumn(name string) bool {
	switch name {
	default:
		return false
	}
}

// ============================================================================
// Suggestion Helper
// ============================================================================

func findClosestMatch(name string, candidates []string) string {
	best := ""
	bestDist := len(name)

	for _, c := range candidates {
		d := levenshtein(strings.ToLower(name), strings.ToLower(c))
		if d < bestDist {
			bestDist = d
			best = c
		}
	}

	threshold := len(name) * 2 / 5
	if bestDist <= threshold {
		return best
	}
	return ""
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)

	for j := 0; j <= len(rb); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(
				curr[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(rb)]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// ============================================================================
// Log Writing
// ============================================================================

func WriteRuntimeValidationLog(report *RuntimeValidationReport, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create runtime validation log: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, "Hekate — Runtime Validation Report (Column Names)\n")
	fmt.Fprintf(file, "=================================================\n\n")

	fmt.Fprintf(file, "Summary:\n")
	fmt.Fprintf(file, "  Total models:    %d\n", report.TotalModels)
	fmt.Fprintf(file, "  Valid models:    %d\n", report.ValidModels)
	fmt.Fprintf(file, "  Failed models:   %d\n", report.FailedModels)
	fmt.Fprintf(file, "  Missing columns: %d\n\n", report.TotalMissing)

	fmt.Fprintf(file, "CSV Columns Available:\n")
	fmt.Fprintf(file, "---------------------\n")
	for _, col := range report.CSVColumns {
		fmt.Fprintf(file, "  - %s\n", col)
	}
	fmt.Fprintf(file, "\n")

	fmt.Fprintf(file, "Model Details:\n")
	fmt.Fprintf(file, "--------------\n\n")

	for _, r := range report.Results {
		status := "✅ PASS"
		if !r.Success {
			status = "❌ FAIL"
		}

		fmt.Fprintf(file, "[%s] %s\n", status, r.ModelName)

		if r.Error != "" {
			fmt.Fprintf(file, "  Error: %s\n", r.Error)
		}

		if len(r.MissingColumns) > 0 {
			fmt.Fprintf(file, "  Missing columns:\n")
			for _, col := range r.MissingColumns {
				suggestion := findClosestMatch(col, report.CSVColumns)
				if suggestion != "" {
					fmt.Fprintf(file, "    - %s (did you mean '%s'?)\n", col, suggestion)
				} else {
					fmt.Fprintf(file, "    - %s\n", col)
				}
			}
		}

		if len(r.AllowedColumns) > 0 {
			fmt.Fprintf(file, "  Allowed columns (via hekate:allow-columns):\n")
			for _, col := range r.AllowedColumns {
				fmt.Fprintf(file, "    - %s\n", col)
			}
		}

		fmt.Fprintf(file, "\n")
	}

	if report.FailedModels > 0 {
		fmt.Fprintf(file, "How to Fix:\n")
		fmt.Fprintf(file, "-----------\n\n")
		fmt.Fprintf(file, "1. Check the column names in your Lua script against the CSV columns\n")
		fmt.Fprintf(file, "   listed above.\n\n")
		fmt.Fprintf(file, "2. If a column is added by an EARLIER model in the pipeline,\n")
		fmt.Fprintf(file, "   suppress the warning by adding a comment anywhere in the script:\n\n")
		fmt.Fprintf(file, "     -- hekate:allow-columns column1, column2\n\n")
		fmt.Fprintf(file, "3. Remember that column names are CASE-SENSITIVE.\n")
		fmt.Fprintf(file, "   The CSV has 'age' (lowercase), so use person.age, not person.Age.\n\n")
		fmt.Fprintf(file, "4. If a column name is misspelled, fix it and re-run.\n\n")
	}

	return nil
}