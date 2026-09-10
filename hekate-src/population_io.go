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
	foundID := false
	for i, col := range header {
		col = strings.TrimSpace(col)
		if col == idColumn {
			foundID = true
		}

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

	// Validate that the ID column exists
	if !foundID {
		return nil, nil, fmt.Errorf("ID column '%s' not found in CSV header. Available columns: %s",
			idColumn, strings.Join(header, ", "))
	}

	// ---- Parse data rows ----
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
	return population, columns, nil
}

// SavePopulationDynamic writes a Population to a CSV file.
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

	// ---- Write header ----
	colNames := GetColumnNames(columns)
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
		record := make([]string, len(columns))
		for i, col := range columns {
			val := row[col.Name]
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