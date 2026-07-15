package main

import (
	"encoding/csv"
	"fmt"
	"log"
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

// ModelConfig represents a single model definition in YAML
type ModelConfig struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`
	Priority    int                    `yaml:"priority"`
	Enabled     bool                   `yaml:"enabled"`
	Description string                 `yaml:"description"`
	Parameters  map[string]interface{} `yaml:"parameters"`
}

// SimulationConfig holds the full simulation configuration
type SimulationConfig struct {
	Simulation SimulationParameters `yaml:"simulation"`
	Models     []ModelConfig        `yaml:"models"`
}

// SimulationParameters holds simulation-level settings
type SimulationParameters struct {
	Iterations     int    `yaml:"iterations"`
	PopulationFile string `yaml:"population_file"`
	OutputFile     string `yaml:"output_file"`
	RandomSeed     int64  `yaml:"random_seed"`
	Verbose        bool   `yaml:"verbose"`
	IDColumn       string `yaml:"id_column"` // REQUIRED: Primary key and ordering
}

// ColumnInfo stores metadata about a column
type ColumnInfo struct {
	Name string
	Type string // "int", "string", "bool"
}

// Population is a slice of maps - fully dynamic!
type Population []map[string]interface{}

// ============================================
// Lua VM with Hekate Functions
// ============================================

// LuaVM wraps the Lua interpreter with methods for Hekate
type LuaVM struct {
	L *lua.LState
}

// NewLuaVM creates a new Lua VM with Hekate-specific functions
func NewLuaVM(randomSeed int64) *LuaVM {
	L := lua.NewState()

	// Seed Lua's random number generator for reproducibility
	if randomSeed > 0 {
		err := L.DoString(fmt.Sprintf("math.randomseed(%d)", randomSeed))
		if err != nil {
			log.Printf("Warning: Failed to seed Lua random: %v", err)
		}
	} else {
		err := L.DoString(fmt.Sprintf("math.randomseed(%d)", time.Now().UnixNano()))
		if err != nil {
			log.Printf("Warning: Failed to seed Lua random: %v", err)
		}
	}

	// Register Hekate-specific functions
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

	L.SetGlobal("now", L.NewFunction(func(L *lua.LState) int {
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

	return &LuaVM{L: L}
}

// Close closes the Lua VM
func (vm *LuaVM) Close() {
	vm.L.Close()
}

// ExecuteLuaScript executes a Lua script and returns the result
func (vm *LuaVM) ExecuteLuaScript(script string, population []map[string]interface{}, params map[string]interface{}) ([]map[string]interface{}, error) {
	// Convert population to Lua table
	luaPop := vm.L.NewTable()
	for _, person := range population {
		luaPerson := vm.L.NewTable()
		for k, v := range person {
			luaPerson.RawSetString(k, toLuaValue(vm.L, v))
		}
		luaPop.Append(luaPerson)
	}

	// Convert params to Lua table
	luaParams := vm.L.NewTable()
	for k, v := range params {
		luaParams.RawSetString(k, toLuaValue(vm.L, v))
	}

	// Register globals
	vm.L.SetGlobal("params", luaParams)
	vm.L.SetGlobal("population", luaPop)

	// Execute the script
	if err := vm.L.DoString(script); err != nil {
		return nil, fmt.Errorf("failed to execute Lua script: %w", err)
	}

	// Get the transition function
	fn := vm.L.GetGlobal("transition")
	if fn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("script must define a 'transition' function")
	}

	// Call the transition function
	if err := vm.L.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, luaPop, luaParams); err != nil {
		return nil, fmt.Errorf("failed to call transition: %w", err)
	}

	// Get the result
	result := vm.L.Get(-1)
	vm.L.Pop(1)

	// Convert Lua table back to Go slice
	resultPop, err := luaTableToSlice(result.(*lua.LTable))
	if err != nil {
		return nil, fmt.Errorf("failed to convert result: %w", err)
	}

	return resultPop, nil
}

// ============================================
// Lua Value Conversion
// ============================================

// toLuaValue converts Go values to Lua values
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

// luaTableToSlice converts a Lua table to a Go slice of maps
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

// luaValueToGo converts Lua values to Go values
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
		var listLen int
		v.ForEach(func(key lua.LValue, value lua.LValue) {
			if key.Type() != lua.LTNumber {
				isList = false
			}
			listLen++
		})

		if isList && listLen > 0 {
			result := []interface{}{}
			for i := 1; i <= listLen; i++ {
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
// Main Function
// ============================================

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <config.yaml>")
	}
	configFile := os.Args[1]

	// 1. Read and parse YAML config
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	var simConfig SimulationConfig
	if err := yaml.Unmarshal(configBytes, &simConfig); err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	// Validate ID column
	if simConfig.Simulation.IDColumn == "" {
		log.Fatal("ERROR: id_column is required in simulation section of config.yaml")
	}

	// Set random seed
	if simConfig.Simulation.RandomSeed > 0 {
		rand.Seed(simConfig.Simulation.RandomSeed)
	} else {
		rand.Seed(time.Now().UnixNano())
	}

	log.Printf("═══ Hekate: Microsimulation Engine ═══")
	log.Printf("Iterations: %d", simConfig.Simulation.Iterations)
	log.Printf("Population file: %s", simConfig.Simulation.PopulationFile)
	log.Printf("ID column: %s", simConfig.Simulation.IDColumn)
	log.Printf("Random seed: %d", simConfig.Simulation.RandomSeed)
	log.Printf("Models loaded: %d", len(simConfig.Models))

	// 2. Load population
	population, columns, err := loadPopulationDynamic(simConfig.Simulation.PopulationFile, simConfig.Simulation.IDColumn)
	if err != nil {
		log.Fatalf("Failed to load population: %v", err)
	}

	log.Printf("Loaded %d individuals with %d columns", len(population), len(columns))

	// 3. Filter and sort models
	enabledModels := filterEnabledModels(simConfig.Models)
	sortModelsByPriority(enabledModels)

	log.Printf("Enabled models: %d", len(enabledModels))
	for _, model := range enabledModels {
		log.Printf("  - %s (priority: %d)", model.Name, model.Priority)
	}

	// 4. Create Lua VM
	luaVM := NewLuaVM(simConfig.Simulation.RandomSeed)
	defer luaVM.Close()

	// 5. Run the simulation
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

	// 6. Save results
	if err := savePopulationDynamic(population, columns, simConfig.Simulation.OutputFile, simConfig.Simulation.IDColumn); err != nil {
		log.Fatalf("Failed to save population: %v", err)
	}

	log.Printf("\n═══ Simulation Complete ═══")
	log.Printf("Results saved to %s", simConfig.Simulation.OutputFile)
}

// ============================================
// Population Loading
// ============================================

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
		isKey := (col == idColumn)
		if isKey {
			foundID = true
		}
		colType := "string"
		for j := 1; j < len(records) && j < 5; j++ {
			if len(records[j]) > i {
				val := strings.TrimSpace(records[j][i])
				if val != "" {
					if _, err := strconv.Atoi(val); err == nil {
						colType = "int"
					} else if val == "true" || val == "false" || val == "True" || val == "False" {
						colType = "bool"
					}
					break
				}
			}
		}
		columns[i] = ColumnInfo{
			Name: col,
			Type: colType,
		}
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

// ============================================
// Model Execution
// ============================================

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

// ============================================
// Population Saving
// ============================================

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

// ============================================
// Helper Functions
// ============================================

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
