// ============================================================================
// population_io.go - CSV Loading and Saving
// ============================================================================
//
// Functions for reading populations from CSV and writing them back. The
// format is deliberately simple (CSV) so users can inspect and edit it with
// any tool.
//
// ============================================================================

package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

// LoadPopulationDynamic loads a CSV file into a Population slice.
//
// This is the bulk-mode loader - it reads the entire file into memory.
// For streaming mode, use the streaming helpers instead.
//
// Column types are detected by sampling the first 20 rows. If the ID column
// is not found in the header, an error is returned listing the available
// columns (helpful for debugging config typos).
func LoadPopulationDynamic(csvFile string, idColumn string) (Population, []ColumnInfo, error) {
	records, columns, err := readCSVWithTypes(csvFile)
	if err != nil {
		return nil, nil, err
	}

	// Validate that the ID column exists
	foundID := false
	for _, col := range columns {
		if col.Name == idColumn {
			foundID = true
			break
		}
	}
	if !foundID {
		header := make([]string, len(columns))
		for i, c := range columns {
			header[i] = c.Name
		}
		return nil, nil, fmt.Errorf("ID column '%s' not found in CSV header. Available columns: %s",
			idColumn, strings.Join(header, ", "))
	}

	population := buildPopulationFromRecords(records, columns)
	return population, columns, nil
}

// LoadDataFileRows loads a CSV as rows without requiring a specific ID column.
//
// Used for model datafiles (lookup tables, rule tables, etc.) that don't
// have a unique ID column. Same type detection and parsing as
// LoadPopulationDynamic, just without the ID check.
func LoadDataFileRows(csvFile string) (Population, []ColumnInfo, error) {
	records, columns, err := readCSVWithTypes(csvFile)
	if err != nil {
		return nil, nil, err
	}
	return buildPopulationFromRecords(records, columns), columns, nil
}

// readCSVWithTypes reads a CSV file and returns its raw records plus the
// detected column info (name + type). Shared implementation used by both
// LoadPopulationDynamic and LoadDataFileRows.
func readCSVWithTypes(csvFile string) ([][]string, []ColumnInfo, error) {
	file, err := os.Open(csvFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("CSV file is empty")
	}

	// ---- Detect column names and types ----
	header := records[0]
	columns := make([]ColumnInfo, len(header))
	for i, col := range header {
		col = strings.TrimSpace(col)

		// Sample up to 20 data rows for type detection
		colType := "string"
		for j := 1; j < len(records) && j < 20; j++ {
			if len(records[j]) > i {
				val := strings.TrimSpace(records[j][i])
				if val != "" {
					if _, err := strconv.Atoi(val); err == nil {
						colType = "int"
					} else if _, err := strconv.ParseFloat(val, 64); err == nil {
						colType = "float"
					} else if val == "true" || val == "false" || val == "True" || val == "False" {
						colType = "bool"
					}
					break
				}
			}
		}
		columns[i] = ColumnInfo{Name: col, Type: colType}
	}

	return records, columns, nil
}

// buildPopulationFromRecords converts raw CSV records (including the header
// row at index 0) into a Population slice using the detected column types.
func buildPopulationFromRecords(records [][]string, columns []ColumnInfo) Population {
	var population Population
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < len(columns) {
			log.Printf("Warning: Row %d has insufficient fields, skipping", i)
			continue
		}
		row := make(map[string]interface{})
		for j, col := range columns {
			val := strings.TrimSpace(record[j])
			if val == "" {
				row[col.Name] = nil
				continue
			}
			switch col.Type {
			case "int":
				if intVal, err := strconv.Atoi(val); err == nil {
					row[col.Name] = intVal
				} else {
					row[col.Name] = val
				}
			case "float":
				if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
					row[col.Name] = floatVal
				} else {
					row[col.Name] = val
				}
			case "bool":
				if val == "true" || val == "True" || val == "1" {
					row[col.Name] = true
				} else if val == "false" || val == "False" || val == "0" {
					row[col.Name] = false
				} else {
					row[col.Name] = val
				}
			default:
				row[col.Name] = val
			}
		}
		population = append(population, row)
	}
	return population
}

// SavePopulationDynamic writes a Population to a CSV file.
//
// Column discovery: the output header starts with the load-time column order
// (from []ColumnInfo), then appends any new string-key columns that models
// have added to the population. New columns are sorted alphabetically for
// deterministic output across runs.
//
// The population is sorted by the ID column before writing to guarantee
// deterministic output ordering - this is important for reproducibility
// because map iteration order in Go is random.
func SavePopulationDynamic(population Population, columns []ColumnInfo, outputFile string, idColumn string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// ---- Discover output columns ----
	// Start with the load-time column order so existing columns keep their
	// position in the CSV. Then collect any new string-key columns that
	// models have added, sorted alphabetically for determinism.
	seen := make(map[string]bool, len(columns))
	colNames := make([]string, 0, len(columns))
	for _, c := range columns {
		seen[c.Name] = true
		colNames = append(colNames, c.Name)
	}

	newColsSet := make(map[string]bool)
	for _, row := range population {
		for k := range row {
			if !seen[k] {
				newColsSet[k] = true
			}
		}
	}
	if len(newColsSet) > 0 {
		newCols := make([]string, 0, len(newColsSet))
		for k := range newColsSet {
			newCols = append(newCols, k)
		}
		sort.Strings(newCols)
		colNames = append(colNames, newCols...)
	}

	// ---- Write header ----
	if err := writer.Write(colNames); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// ---- Sort by ID for deterministic output ordering ----
	// Map iteration is randomized in Go, so without this sort, output rows
	// would appear in a different (but valid) order each run, breaking
	// byte-for-byte reproducibility.
	sort.Slice(population, func(i, j int) bool {
		id1 := fmt.Sprintf("%v", population[i][idColumn])
		id2 := fmt.Sprintf("%v", population[j][idColumn])
		return id1 < id2
	})

	// ---- Write data rows ----
	for _, row := range population {
		record := make([]string, len(colNames))
		for i, col := range colNames {
			val := row[col]
			if val == nil {
				record[i] = ""
			} else {
				switch v := val.(type) {
				case bool:
					if v {
						record[i] = "true"
					} else {
						record[i] = "false"
					}
				case int:
					record[i] = strconv.Itoa(v)
				case int64:
					record[i] = strconv.FormatInt(v, 10)
				case float64:
					record[i] = strconv.FormatFloat(v, 'f', -1, 64)
				case string:
					record[i] = v
				default:
					record[i] = fmt.Sprintf("%v", v)
				}
			}
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}
	return nil
}

// copyFile copies a file from src to dst, ensuring the data is flushed to
// disk before returning. Used to produce the final output in streaming mode.
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return err
	}

	if err := dest.Sync(); err != nil {
		return err
	}
	return nil
}