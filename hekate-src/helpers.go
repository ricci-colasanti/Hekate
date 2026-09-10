// ============================================================================
// helpers.go - Small Utility Functions
// ============================================================================
//
// Assorted small helpers used throughout the codebase. Nothing here does
// anything clever - it's just the common patterns extracted for readability.
//
// ============================================================================

package main

import "sort"

// GetColumnNames extracts just the names from a []ColumnInfo.
func GetColumnNames(columns []ColumnInfo) []string {
	names := make([]string, len(columns))
	for i, col := range columns {
		names[i] = col.Name
	}
	return names
}

// FilterEnabledModels returns only the models whose Enabled flag is true.
// Disabled models are ignored during simulation but remain in the config,
// which is convenient for toggling them on and off during development.
func FilterEnabledModels(models []ModelConfig) []ModelConfig {
	var enabled []ModelConfig
	for _, model := range models {
		if model.Enabled {
			enabled = append(enabled, model)
		}
	}
	return enabled
}

// SortModelsByPriority sorts models in ascending priority order (lower
// numbers run first). This matters because models often have dependencies -
// for example, aging must run before mortality so that people die at the
// correct age.
func SortModelsByPriority(models []ModelConfig) {
	sort.Slice(models, func(i, j int) bool {
		return models[i].Priority < models[j].Priority
	})
}