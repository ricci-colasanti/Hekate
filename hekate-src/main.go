package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
	"gopkg.in/yaml.v3"
)

// ============================================
// Configuration Structures
// ============================================

type ModelConfig struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`
	Priority    int                    `yaml:"priority"`
	Enabled     bool                   `yaml:"enabled"`
	Description string                 `yaml:"description"`
	Parameters  map[string]interface{} `yaml:"parameters"`
}

type SimulationConfig struct {
	Simulation SimulationParameters `yaml:"simulation"`
	Models     []ModelConfig        `yaml:"models"`
}

type SimulationParameters struct {
	Iterations     int    `yaml:"iterations"`
	PopulationFile string `yaml:"population_file"`
	OutputFile     string `yaml:"output_file"`
	RandomSeed     int64  `yaml:"random_seed"`
	Verbose        bool   `yaml:"verbose"`
	IDColumn       string `yaml:"id_column"`
	AreaColumn     string `yaml:"area_column"`
	StreamingMode  bool   `yaml:"streaming_mode"`
}

type ColumnInfo struct {
	Name string
	Type string // "int", "float", "string", "bool"
}

type Population []map[string]interface{}

// ============================================
// Hekate Statistics Functions
// ============================================

type HekateStats struct{}

func (h *HekateStats) LinearPredict(L *lua.LState) int {
	intercept := L.CheckNumber(1)
	prediction := float64(intercept)
	for i := 2; i <= L.GetTop(); i += 2 {
		if i+1 > L.GetTop() {
			break
		}
		coef := L.CheckNumber(i)
		varVal := L.CheckNumber(i + 1)
		prediction += float64(coef) * float64(varVal)
	}
	L.Push(lua.LNumber(prediction))
	return 1
}

func (h *HekateStats) LinearPredictDefault(L *lua.LState) int {
	intercept := L.CheckNumber(1)
	globalDefault := float64(L.CheckNumber(2))
	prediction := float64(intercept)

	for i := 3; i <= L.GetTop(); i += 3 {
		if i+2 > L.GetTop() {
			break
		}
		coef := L.CheckNumber(i)
		varVal := L.CheckAny(i + 1)

		var numVal float64
		if varVal == lua.LNil || varVal.Type() != lua.LTNumber {
			// Use the per-variable default if provided, otherwise use global default
			if i+2 <= L.GetTop() {
				numVal = float64(L.CheckNumber(i + 2))
			} else {
				numVal = globalDefault
			}
		} else {
			numVal = float64(lua.LVAsNumber(varVal))
		}
		prediction += float64(coef) * numVal
	}
	L.Push(lua.LNumber(prediction))
	return 1
}

func (h *HekateStats) LogisticPredict(L *lua.LState) int {
	intercept := L.CheckNumber(1)
	linear := float64(intercept)
	for i := 2; i <= L.GetTop(); i += 2 {
		if i+1 > L.GetTop() {
			break
		}
		coef := L.CheckNumber(i)
		varVal := L.CheckNumber(i + 1)
		linear += float64(coef) * float64(varVal)
	}
	result := 1.0 / (1.0 + math.Exp(-linear))
	L.Push(lua.LNumber(result))
	return 1
}

// ============================================
// Lua VM
// ============================================

type LuaVM struct {
	L *lua.LState
}

func NewLuaVM(randomSeed int64) *LuaVM {
	L := lua.NewState()

	if randomSeed > 0 {
		_ = L.DoString(fmt.Sprintf("math.randomseed(%d)", randomSeed))
	} else {
		_ = L.DoString(fmt.Sprintf("math.randomseed(%d)", time.Now().UnixNano()))
	}

	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		msg := L.ToString(1)
		log.Printf("[Lua] %s", msg)
		return 0
	}))

	L.SetGlobal("random", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(rand.Float64()))
		return 1
	}))

	L.SetGlobal("random_int", L.NewFunction(func(L *lua.LState) int {
		min := int(L.ToInt(1))
		max := int(L.ToInt(2))
		if max <= min {
			max = min + 1
		}
		L.Push(lua.LNumber(rand.Intn(max-min) + min))
		return 1
	}))

	L.SetGlobal("current_year", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(time.Now().Year()))
		return 1
	}))

	L.SetGlobal("table_contains", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		val := L.CheckAny(2)
		found := false
		tbl.ForEach(func(key lua.LValue, value lua.LValue) {
			if value == val {
				found = true
			}
		})
		L.Push(lua.LBool(found))
		return 1
	}))

	stats := &HekateStats{}
	statsTable := L.NewTable()
	L.SetField(statsTable, "linear_predict", L.NewFunction(stats.LinearPredict))
	L.SetField(statsTable, "linear_predict_default", L.NewFunction(stats.LinearPredictDefault))
	L.SetField(statsTable, "logistic_predict", L.NewFunction(stats.LogisticPredict))
	L.SetGlobal("hekate_stats", statsTable)

	return &LuaVM{L: L}
}

func (vm *LuaVM) Close() {
	vm.L.Close()
}

func (vm *LuaVM) ExecuteLuaScript(script string, population []map[string]interface{}, params map[string]interface{}) ([]map[string]interface{}, error) {
	luaPop := vm.L.NewTable()
	for _, person := range population {
		luaPerson := vm.L.NewTable()
		for k, v := range person {
			luaPerson.RawSetString(k, toLuaValue(vm.L, v))
		}
		luaPop.Append(luaPerson)
	}

	luaParams := vm.L.NewTable()
	for k, v := range params {
		luaParams.RawSetString(k, toLuaValue(vm.L, v))
	}

	// Clear any old global state
	vm.L.SetGlobal("transition", lua.LNil)
	vm.L.SetGlobal("population", luaPop)
	vm.L.SetGlobal("params", luaParams)

	if err := vm.L.DoString(script); err != nil {
		return nil, fmt.Errorf("failed to execute Lua script: %w", err)
	}

	fn := vm.L.GetGlobal("transition")
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("script must define a 'transition' function")
	}

	if err := vm.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, luaPop, luaParams); err != nil {
		return nil, fmt.Errorf("failed to call transition: %w", err)
	}

	result := vm.L.Get(-1)
	vm.L.Pop(1)

	// Check that result is a table
	if result.Type() != lua.LTTable {
		return nil, fmt.Errorf("transition function must return a table, got %s", result.Type())
	}

	resultPop, err := luaTableToSlice(result.(*lua.LTable))
	if err != nil {
		return nil, fmt.Errorf("failed to convert result: %w", err)
	}
	return resultPop, nil
}

// ============================================
// Lua Value Conversion
// ============================================

func toLuaValue(L *lua.LState, val interface{}) lua.LValue {
	switch v := val.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case int:
		return lua.LNumber(v)
	case int64:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		tbl := L.NewTable()
		for _, item := range v {
			tbl.Append(toLuaValue(L, item))
		}
		return tbl
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, item := range v {
			tbl.RawSetString(k, toLuaValue(L, item))
		}
		return tbl
	default:
		return lua.LNil
	}
}

func luaTableToSlice(tbl *lua.LTable) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	tbl.ForEach(func(key lua.LValue, value lua.LValue) {
		if tblVal, ok := value.(*lua.LTable); ok {
			row := make(map[string]interface{})
			tblVal.ForEach(func(k lua.LValue, v lua.LValue) {
				if k.Type() == lua.LTString {
					row[k.String()] = luaValueToGo(v)
				}
			})
			result = append(result, row)
		}
	})
	return result, nil
}

func luaValueToGo(val lua.LValue) interface{} {
	if val == lua.LNil {
		return nil
	}
	switch v := val.(type) {
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		isList := true
		maxIndex := 0
		v.ForEach(func(key lua.LValue, value lua.LValue) {
			if key.Type() != lua.LTNumber {
				isList = false
			}
			if n, ok := key.(lua.LNumber); ok {
				if int(n) > maxIndex {
					maxIndex = int(n)
				}
			}
		})
		if isList && maxIndex > 0 {
			result := make([]interface{}, 0, maxIndex)
			for i := 1; i <= maxIndex; i++ {
				val := v.RawGetInt(i)
				result = append(result, luaValueToGo(val))
			}
			return result
		}
		result := map[string]interface{}{}
		v.ForEach(func(key lua.LValue, value lua.LValue) {
			if key.Type() == lua.LTString {
				result[key.String()] = luaValueToGo(value)
			}
		})
		return result
	default:
		return nil
	}
}

// ============================================
// Main
// ============================================

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <config.yaml>")
	}
	configFile := os.Args[1]

	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	var simConfig SimulationConfig
	if err := yaml.Unmarshal(configBytes, &simConfig); err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	if simConfig.Simulation.IDColumn == "" {
		log.Fatal("ERROR: id_column is required in simulation section of config.yaml")
	}
	if simConfig.Simulation.StreamingMode && simConfig.Simulation.AreaColumn == "" {
		log.Fatal("ERROR: area_column is required when streaming_mode is true")
	}

	// Set random seed for Go's global rand
	if simConfig.Simulation.RandomSeed > 0 {
		rand.Seed(simConfig.Simulation.RandomSeed)
	} else {
		simConfig.Simulation.RandomSeed = time.Now().UnixNano()
		rand.Seed(simConfig.Simulation.RandomSeed)
	}

	log.Printf("═══ Hekate: Microsimulation Engine ═══")
	log.Printf("Iterations: %d", simConfig.Simulation.Iterations)
	log.Printf("Population file: %s", simConfig.Simulation.PopulationFile)
	log.Printf("ID column: %s", simConfig.Simulation.IDColumn)
	if simConfig.Simulation.StreamingMode {
		log.Printf("Mode: STREAMING (area-by-area)")
		log.Printf("Area column: %s", simConfig.Simulation.AreaColumn)
	} else {
		log.Printf("Mode: BULK (load all into memory)")
	}
	log.Printf("Random seed: %d", simConfig.Simulation.RandomSeed)
	log.Printf("Models loaded: %d", len(simConfig.Models))

	enabledModels := filterEnabledModels(simConfig.Models)
	sortModelsByPriority(enabledModels)

	log.Printf("Enabled models: %d", len(enabledModels))
	for _, model := range enabledModels {
		log.Printf("  - %s (priority: %d)", model.Name, model.Priority)
	}

	luaVM := NewLuaVM(simConfig.Simulation.RandomSeed)
	defer luaVM.Close()

	if simConfig.Simulation.StreamingMode {
		log.Printf("\n--- Using Streaming Area-by-Area Processing ---")
		runStreamingSimulation(&simConfig, enabledModels, luaVM)
	} else {
		log.Printf("\n--- Using Bulk Processing (load all into memory) ---")
		runBulkSimulation(&simConfig, enabledModels, luaVM)
	}
}

// ============================================
// Bulk Simulation
// ============================================

func runBulkSimulation(simConfig *SimulationConfig, enabledModels []ModelConfig, luaVM *LuaVM) {
	population, columns, err := loadPopulationDynamic(simConfig.Simulation.PopulationFile, simConfig.Simulation.IDColumn)
	if err != nil {
		log.Fatalf("Failed to load population: %v", err)
	}
	log.Printf("Loaded %d individuals with %d columns", len(population), len(columns))

	for i := 0; i < simConfig.Simulation.Iterations; i++ {
		log.Printf("\n═══ Iteration %d/%d ═══", i+1, simConfig.Simulation.Iterations)
		for _, model := range enabledModels {
			switch model.Type {
			case "lua_model":
				population, err = executeLuaModel(luaVM, model, population, simConfig.Simulation.Verbose)
			default:
				log.Fatalf("Unknown model type: %s", model.Type)
			}
			if err != nil {
				log.Fatalf("Model '%s' execution failed: %v", model.Name, err)
			}
		}
	}

	if err := savePopulationDynamic(population, columns, simConfig.Simulation.OutputFile, simConfig.Simulation.IDColumn); err != nil {
		log.Fatalf("Failed to save population: %v", err)
	}
	log.Printf("\n═══ Simulation Complete ═══")
	log.Printf("Results saved to %s", simConfig.Simulation.OutputFile)
}

// ============================================
// Streaming Simulation (Fixed Column Set)
// ============================================

func runStreamingSimulation(simConfig *SimulationConfig, enabledModels []ModelConfig, luaVM *LuaVM) {
	// Detect columns from the original file ONCE
	originalColumns, err := detectColumns(simConfig.Simulation.PopulationFile)
	if err != nil {
		log.Fatalf("Failed to detect columns from original file: %v", err)
	}
	log.Printf("Detected %d columns (fixed for all iterations)", len(originalColumns))

	for iter := 0; iter < simConfig.Simulation.Iterations; iter++ {
		log.Printf("\n═══ Iteration %d/%d ═══", iter+1, simConfig.Simulation.Iterations)

		var inputFile string
		if iter == 0 {
			inputFile = simConfig.Simulation.PopulationFile
		} else {
			inputFile = fmt.Sprintf("year_%d.csv", iter)
		}
		outputFile := fmt.Sprintf("year_%d.csv", iter+1)

		err = processYearStreamingFixed(
			inputFile,
			outputFile,
			enabledModels,
			originalColumns,
			simConfig.Simulation.AreaColumn,
			simConfig.Simulation.Verbose,
			luaVM,
		)
		if err != nil {
			log.Fatalf("Iteration %d failed: %v", iter+1, err)
		}
	}

	// Final output: copy last year's file
	lastFile := fmt.Sprintf("year_%d.csv", simConfig.Simulation.Iterations)
	if err := copyFile(lastFile, simConfig.Simulation.OutputFile); err != nil {
		log.Fatalf("Failed to copy final output: %v", err)
	}

	// Clean up intermediate files
	for i := 1; i <= simConfig.Simulation.Iterations; i++ {
		os.Remove(fmt.Sprintf("year_%d.csv", i))
	}

	log.Printf("\n═══ Simulation Complete ═══")
	log.Printf("Results saved to %s", simConfig.Simulation.OutputFile)
}

// ============================================
// Streaming Helper Functions
// ============================================

func detectColumns(filename string) ([]ColumnInfo, error) {
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

	// Read more rows for better type detection (20 rows)
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

	columns := make([]ColumnInfo, len(header))
	for i, col := range header {
		col = strings.TrimSpace(col)
		colType := "string"
		for _, row := range rows {
			if i < len(row) {
				val := strings.TrimSpace(row[i])
				if val != "" {
					// Try int first
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

// processYearStreamingFixed processes one year using streaming,
// but only writes the fixed set of columns (no new columns added).
func processYearStreamingFixed(inputFile, outputFile string, models []ModelConfig, columns []ColumnInfo, areaColumn string, verbose bool, luaVM *LuaVM) error {
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

	// Skip the header from the input file (we use fixed columns instead)
	_, err = reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Write the fixed columns as header
	colNames := getColumnNames(columns)
	if err := writer.Write(colNames); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Find area column index
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

	// Process area by area
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

		// Check bounds for areaIdx
		if areaIdx >= len(record) {
			return fmt.Errorf("record missing area column '%s' at index %d", areaColumn, areaIdx)
		}
		areaID := record[areaIdx]

		// Validate sort order
		if currentArea != "" && areaID < currentArea {
			return fmt.Errorf("CSV is not sorted by area column '%s': found '%s' after '%s'. Sort the file first.",
				areaColumn, areaID, currentArea)
		}

		if currentArea != "" && areaID != currentArea {
			if len(areaRecords) > 0 {
				if err := processAreaFixed(areaRecords, columns, models, verbose, luaVM, writer); err != nil {
					return fmt.Errorf("area %s error: %w", currentArea, err)
				}
				areaCount++
				areaRecords = make([][]string, 0)
			}
		}

		currentArea = areaID
		areaRecords = append(areaRecords, record)
	}

	if len(areaRecords) > 0 {
		if err := processAreaFixed(areaRecords, columns, models, verbose, luaVM, writer); err != nil {
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

// processAreaFixed processes one area but only writes the fixed columns
func processAreaFixed(records [][]string, columns []ColumnInfo, models []ModelConfig, verbose bool, luaVM *LuaVM, writer *csv.Writer) error {
	// Convert records to Population
	pop := make(Population, len(records))
	for i, record := range records {
		row := make(map[string]interface{})
		for j, col := range columns {
			// Check bounds to prevent panic
			if j >= len(record) {
				row[col.Name] = nil
				continue
			}
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
		pop[i] = row
	}

	// Execute models
	var err error
	for _, model := range models {
		switch model.Type {
		case "lua_model":
			pop, err = executeLuaModel(luaVM, model, pop, verbose)
		default:
			return fmt.Errorf("unknown model type: %s", model.Type)
		}
		if err != nil {
			return err
		}
	}

	// Write records using ONLY the fixed columns
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

// ============================================
// Helper Functions
// ============================================

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

	// Sync to ensure data is written to disk
	if err := dest.Sync(); err != nil {
		return err
	}
	return nil
}

func loadPopulationDynamic(csvFile string, idColumn string) (Population, []ColumnInfo, error) {
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

	header := records[0]
	columns := make([]ColumnInfo, len(header))
	foundID := false
	for i, col := range header {
		col = strings.TrimSpace(col)
		if col == idColumn {
			foundID = true
		}
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
	if !foundID {
		return nil, nil, fmt.Errorf("ID column '%s' not found in CSV header. Available columns: %s",
			idColumn, strings.Join(header, ", "))
	}

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

func executeLuaModel(vm *LuaVM, model ModelConfig, population Population, verbose bool) (Population, error) {
	scriptInterface, ok := model.Parameters["script"]
	if !ok {
		return nil, fmt.Errorf("model '%s' missing 'script' parameter", model.Name)
	}
	script, ok := scriptInterface.(string)
	if !ok {
		return nil, fmt.Errorf("model '%s' script is not a string", model.Name)
	}
	if verbose {
		log.Printf("  ▶ Executing: %s (priority: %d)", model.Name, model.Priority)
	} else {
		log.Printf("  ▶ %s", model.Name)
	}
	popSlice := []map[string]interface{}(population)
	result, err := vm.ExecuteLuaScript(script, popSlice, model.Parameters)
	if err != nil {
		return nil, err
	}
	return Population(result), nil
}

func savePopulationDynamic(population Population, columns []ColumnInfo, outputFile string, idColumn string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	colNames := getColumnNames(columns)
	if err := writer.Write(colNames); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	sort.Slice(population, func(i, j int) bool {
		id1 := fmt.Sprintf("%v", population[i][idColumn])
		id2 := fmt.Sprintf("%v", population[j][idColumn])
		return id1 < id2
	})

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

func getColumnNames(columns []ColumnInfo) []string {
	names := make([]string, len(columns))
	for i, col := range columns {
		names[i] = col.Name
	}
	return names
}

func filterEnabledModels(models []ModelConfig) []ModelConfig {
	var enabled []ModelConfig
	for _, model := range models {
		if model.Enabled {
			enabled = append(enabled, model)
		}
	}
	return enabled
}

func sortModelsByPriority(models []ModelConfig) {
	sort.Slice(models, func(i, j int) bool {
		return models[i].Priority < models[j].Priority
	})
}
