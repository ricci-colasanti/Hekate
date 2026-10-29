// ============================================================================
// lua_conversion.go - Lua <-> Go Value Conversion
// ============================================================================
//
// Lua and Go have different type systems. These functions bridge the two.
// The conversion is not symmetric (Lua has no integer type, only numbers),
// so some precision may be lost. For population simulation this is fine.
//
// ============================================================================

package main

import (
	lua "github.com/yuin/gopher-lua"
)

// toLuaValue converts a Go value to a Lua value.
//
// Type mapping:
//
//	nil                        -> nil
//	bool                       -> boolean
//	int/int64                  -> number (Lua only has float64 numbers)
//	float64                    -> number
//	string                     -> string
//	[]interface{}              -> table (array-style)
//	[]map[string]interface{}   -> table (array of object-tables)
//	Population                 -> table (same as []map[string]interface{})
//	map[string]interface{}     -> table (object-style, with string keys)
//
// Unsupported types are silently converted to nil.
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
	case []map[string]interface{}:
		// This is the type used for datafile rows (and for Population).
		// Each map becomes an object-style table; the slice becomes an
		// array-style table. Without this case, datafile rows would be
		// silently converted to nil, which is exactly what was happening.
		tbl := L.NewTable()
		for _, row := range v {
			tbl.Append(toLuaValue(L, row))
		}
		return tbl
	case Population:
		// Population is a named type for []map[string]interface{}, but Go's
		// type switch treats named types as distinct from their underlying
		// type, so we need an explicit case.
		tbl := L.NewTable()
		for _, row := range v {
			tbl.Append(toLuaValue(L, row))
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

// luaTableToSlice converts a Lua table (list of tables) back to a Go slice
// of maps. Used to convert the population returned by a Lua model back into
// the Go representation.
//
// Expected structure:
//
//	{
//	  { person_id = 1, age = 25, ... },
//	  { person_id = 2, age = 30, ... },
//	}
func luaTableToSlice(tbl *lua.LTable) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	tbl.ForEach(func(key lua.LValue, value lua.LValue) {
		if tblVal, ok := value.(*lua.LTable); ok {
			row := make(map[string]interface{})
			tblVal.ForEach(func(k lua.LValue, v lua.LValue) {
				// Only process string keys (skip numeric indices)
				if k.Type() == lua.LTString {
					row[k.String()] = luaValueToGo(v)
				}
			})
			result = append(result, row)
		}
	})
	return result, nil
}

// luaValueToGo converts a single Lua value to a Go value.
//
// Type mapping:
//
//	nil          -> nil
//	boolean      -> bool
//	number       -> float64
//	string       -> string
//	table (list) -> []interface{}
//	table (map)  -> map[string]interface{}
//
// The list-vs-map distinction is inferred heuristically: if ALL keys are
// numeric, we treat the table as a list.
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
		// Determine if this is a list (all numeric keys) or a map
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
			// Array-style table
			result := make([]interface{}, 0, maxIndex)
			for i := 1; i <= maxIndex; i++ {
				val := v.RawGetInt(i)
				result = append(result, luaValueToGo(val))
			}
			return result
		}

		// Object-style table
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