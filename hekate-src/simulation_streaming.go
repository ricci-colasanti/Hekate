// ============================================================================
// simulation_streaming.go - Streaming Simulation Mode
// ============================================================================
//
// Processes the population one area at a time, keeping only one area in
// memory. Memory usage is constant (~20MB) regardless of population size.
//
// REQUIREMENT: The CSV must be SORTED by the area_column. This allows us
// to detect area boundaries by simply checking when the area ID changes.
//
// Best for: populations of 1M+ people
// Memory:   constant (~20-50MB) regardless of size
//
// ============================================================================

package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

// RunStreamingSimulation runs the simulation in streaming (area-by-area) mode.
func RunStreamingSimulation(config *SimulationConfig, enabledModels []ModelConfig, luaVM *LuaVM) {
	// Detect column types once from the original file. We reuse these
	// types for every iteration so that column types are stable across
	// the whole run.
	originalColumns, err := DetectColumns(config.Simulation.PopulationFile)
	if err != nil {
		log.Fatalf("Failed to detect columns from original file: %v", err)
	}
	log.Printf("Detected %d columns (fixed for all iterations)", len(originalColumns))

	// ---- Iteration loop ----
	for iter := 0; iter < config.Simulation.Iterations; iter++ {
		log.Printf("\n═══ Iteration %d/%d ═══", iter+1, config.Simulation.Iterations)

		// Determine input and output filenames for this iteration
		var inputFile string
		if iter == 0 {
			inputFile = config.Simulation.PopulationFile
		} else {
			inputFile = fmt.Sprintf("year_%d.csv", iter)
		}
		outputFile := fmt.Sprintf("year_%d.csv", iter+1)

		err = ProcessYearStreamingFixed(
			inputFile,
			outputFile,
			enabledModels,
			originalColumns,
			config.Simulation.AreaColumn,
			config.Simulation.Verbose,
			luaVM,
			config.Simulation.RandomSeed,
			iter,
			config.Simulation.DataDir,
		)
		if err != nil {
			log.Fatalf("Iteration %d failed: %v", iter+1, err)
		}
	}

	// ---- Copy last intermediate file to final output ----
	lastFile := fmt.Sprintf("year_%d.csv", config.Simulation.Iterations)
	if err := copyFile(lastFile, config.Simulation.OutputFile); err != nil {
		log.Fatalf("Failed to copy final output: %v", err)
	}

	// ---- Clean up intermediate files ----
	for i := 1; i <= config.Simulation.Iterations; i++ {
		os.Remove(fmt.Sprintf("year_%d.csv", i))
	}

	log.Printf("\n═══ Simulation Complete ═══")
	log.Printf("Results saved to %s", config.Simulation.OutputFile)
}

// DetectColumns samples the CSV to determine column names and types.
//
// Type detection samples up to 20 data rows per column and picks the first
// non-empty value's inferred type.
func DetectColumns(filename string) ([]ColumnInfo, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// Sample up to 20 rows for type inference
	rows := make([][]string, 0)
	for i := 0; i < 20; i++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	// Infer type for each column
	columns := make([]ColumnInfo, len(header))
	for i, col := range header {
		col = strings.TrimSpace(col)
		colType := "string" // default
		for _, row := range rows {
			if i < len(row) {
				val := strings.TrimSpace(row[i])
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
	return columns, nil
}

// ProcessYearStreamingFixed processes one iteration (year) of the simulation
// in streaming mode. Reads from `inputFile`, writes to `outputFile`.
func ProcessYearStreamingFixed(inputFile, outputFile string, models []ModelConfig, columns []ColumnInfo, areaColumn string, verbose bool, luaVM *LuaVM, baseSeed int64, year int, dataDir string) error {
	// ---- Open input and output files ----
	inFile, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inFile.Close()

	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	reader := csv.NewReader(inFile)
	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	// ---- Skip input header, write our fixed-column header ----
	_, err = reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	colNames := GetColumnNames(columns)
	if err := writer.Write(colNames); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// ---- Locate the area column index ----
	areaIdx := -1
	for i, col := range columns {
		if col.Name == areaColumn {
			areaIdx = i
			break
		}
	}
	if areaIdx == -1 {
		return fmt.Errorf("area column '%s' not found", areaColumn)
	}

	// ---- Stream through the file, accumulating records for the current area ----
	currentArea := ""
	areaRecords := make([][]string, 0)
	areaCount := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read record: %w", err)
		}

		// Safety check: ensure the record has the area column
		if areaIdx >= len(record) {
			return fmt.Errorf("record missing area column '%s' at index %d", areaColumn, areaIdx)
		}
		areaID := record[areaIdx]

		// Validate sort order
		if currentArea != "" && areaID < currentArea {
			return fmt.Errorf("CSV is not sorted by area column '%s': found '%s' after '%s'. Sort the file first.",
				areaColumn, areaID, currentArea)
		}

		// Detect area boundary
		if currentArea != "" && areaID != currentArea {
			if len(areaRecords) > 0 {
				areaSeed := baseSeed + int64(year)*100000 + int64(areaCount)
				if err := processAreaFixed(areaRecords, columns, models, verbose, luaVM, writer, areaSeed, dataDir); err != nil {
					return fmt.Errorf("area %s error: %w", currentArea, err)
				}
				areaCount++
				areaRecords = make([][]string, 0)
			}
		}

		currentArea = areaID
		areaRecords = append(areaRecords, record)
	}

	// ---- Process the final area ----
	if len(areaRecords) > 0 {
		areaSeed := baseSeed + int64(year)*100000 + int64(areaCount)
		if err := processAreaFixed(areaRecords, columns, models, verbose, luaVM, writer, areaSeed, dataDir); err != nil {
			return fmt.Errorf("area %s error: %w", currentArea, err)
		}
		areaCount++
	}

	writer.Flush()
	if verbose {
		log.Printf("  Processed %d areas", areaCount)
	}
	return nil
}

// processAreaFixed processes a single area's records through all models.
func processAreaFixed(records [][]string, columns []ColumnInfo, models []ModelConfig, verbose bool, luaVM *LuaVM, writer *csv.Writer, baseSeed int64, dataDir string) error {
	// ---- Convert CSV records to Population ----
	pop := make(Population, len(records))
	for i, record := range records {
		row := make(map[string]interface{})
		for j, col := range columns {
			if j >= len(record) {
				row[col.Name] = nil
				continue
			}
			val := strings.TrimSpace(record[j])
			if val == "" {
				row[col.Name] = nil
				continue
			}
			// Convert based on detected column type
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
		pop[i] = row
	}

	// ---- Execute each model in order ----
	var err error
	for modelIndex, model := range models {
		// Derive a unique seed for this specific model
		modelSeed := baseSeed + int64(modelIndex)

		switch model.Type {
		case "lua_model":
			pop, err = ExecuteLuaModel(luaVM, model, pop, verbose, modelSeed, dataDir)
		default:
			return fmt.Errorf("unknown model type: %s", model.Type)
		}
		if err != nil {
			return err
		}
	}

	// ---- Convert Population back to CSV records ----
	for _, row := range pop {
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