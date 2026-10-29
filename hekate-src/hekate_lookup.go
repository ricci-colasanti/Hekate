// ============================================================================
// hekate_lookup.go - Keyed Indexes Over Datafile Rows
// ============================================================================
//
// Builds nested Lua tables keyed by one or more columns from a datafile,
// so scripts can do by_key[sex][alive] instead of hand-rolling the loop.
//
// The leaf value is the whole row (same table the engine built), so scripts
// access rule.value, rule.sd, or any other column. Sharing the row means
// mutating it via the index also mutates it in params.datafile — that's
// intentional; it's the same data.
//
// Key coercion
// ------------
// Keys are compared as strings. Every table in the index has a metatable
// whose __index coerces the requested key to a string before the raw lookup,
// so by_key[person.sex][person.alive] works whether person.sex is a string,
// bool, or number. This eliminates the cross-file type-mismatch footgun
// (e.g. datafile "1"/"0" vs population "true"/"false").
//
// Duplicate keys
// --------------
// Two rows with the same key raise a Lua error naming the key and columns.
// Silent last-wins (the behaviour of the hand-rolled loop) hides config bugs.
//
// ============================================================================

package main

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// RegisterHekateLookup registers the hekate_lookup table in the Lua state.
func RegisterHekateLookup(L *lua.LState) {
	tbl := L.NewTable()
	L.SetField(tbl, "index", L.NewFunction(hekateLookupIndex))
	L.SetGlobal("hekate_lookup", tbl)
}

// hekateLookupIndex implements:
//
//	hekate_lookup.index(rows, col1, col2, ..., colN) -> nested table
//
// rows is a Lua array-of-tables (each table = one row, keyed by column name).
// Each col argument names a column to key on, in outer-to-inner order.
// Returns a nested table N levels deep, leaf = the row.
func hekateLookupIndex(L *lua.LState) int {
	rowsTbl := L.CheckTable(1)

	// ---- Collect column names ----
	nCols := L.GetTop() - 1
	if nCols == 0 {
		L.ArgError(1, "at least one key column is required")
		return 0
	}
	cols := make([]string, nCols)
	for i := 0; i < nCols; i++ {
		col := L.CheckString(i + 2)
		if strings.TrimSpace(col) == "" {
			L.ArgError(i+2, "key column name cannot be empty")
			return 0
		}
		cols[i] = col
	}

	// ---- Build the nested index ----
	root := newCoerceTable(L)
	path := make([]string, nCols)
	var buildErr error

	rowsTbl.ForEach(func(_ lua.LValue, rowVal lua.LValue) {
		if buildErr != nil {
			return
		}
		row, ok := rowVal.(*lua.LTable)
		if !ok {
			buildErr = fmt.Errorf("rows must be an array of tables (got %s)", rowVal.Type())
			return
		}

		current := root
		for depth, col := range cols {
			cell := row.RawGetString(col)
			if cell == lua.LNil {
				buildErr = fmt.Errorf(
					"row is missing key column %q (available columns: %s)",
					col, listRowColumns(row))
				return
			}
			keyStr := coerceKeyToString(cell)
			path[depth] = keyStr

			if depth == nCols-1 {
				// Leaf level: set, or error on duplicate.
				if existing := current.RawGetString(keyStr); existing != lua.LNil {
					buildErr = fmt.Errorf("duplicate key [%s] on column %q",
						strings.Join(path, "]["), col)
					return
				}
				current.RawSetString(keyStr, row)
			} else {
				// Intermediate level: descend, creating subtables.
				next := current.RawGetString(keyStr)
				if next == lua.LNil {
					sub := newCoerceTable(L)
					current.RawSetString(keyStr, sub)
					current = sub
				} else {
					sub, ok := next.(*lua.LTable)
					if !ok {
						buildErr = fmt.Errorf("internal error: non-table at [%s]",
							strings.Join(path[:depth+1], "]["))
						return
					}
					current = sub
				}
			}
		}
	})

	if buildErr != nil {
		L.RaiseError("hekate_lookup.index: %v", buildErr)
		return 0
	}

	L.Push(root)
	return 1
}

// newCoerceTable creates a plain Lua table with an __index metatable that
// coerces the requested key to a string before the raw lookup.
//
// The builder stores keys via RawSetString, so they're already coerced.
// Reader accesses like by_key[true] hit a raw miss (the stored key is
// "true") and fall through to __index, which coerces and retries.
func newCoerceTable(L *lua.LState) *lua.LTable {
	t := L.NewTable()

	mt := L.NewTable()
	L.SetField(mt, "__index", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		key := coerceKeyToString(L.Get(2))
		L.Push(tbl.RawGetString(key))
		return 1
	}))

	L.SetMetatable(t, mt)
	return t
}

// coerceKeyToString normalises a Lua value into the string form used for
// index keys. Must be used consistently on both build and read sides.
//
// Mapping:
//
//	nil     -> ""
//	bool    -> "true" / "false"
//	number  -> "5" for integer-valued floats, "%g" otherwise
//	string  -> itself
//	other   -> "<typename>" (not meaningful as a key)
func coerceKeyToString(v lua.LValue) string {
	switch v.Type() {
	case lua.LTNil:
		return ""
	case lua.LTBool:
		if bool(v.(lua.LBool)) {
			return "true"
		}
		return "false"
	case lua.LTNumber:
		f := float64(v.(lua.LNumber))
		// Integer-valued floats format without a decimal point so that
		// a key of 5 and a key of "5" collide as expected. This assumes
		// keys fit in int64; realistic keys (IDs, ages, categories) do.
		if f == float64(int64(f)) {
			return fmt.Sprintf("%d", int64(f))
		}
		return fmt.Sprintf("%g", f)
	case lua.LTString:
		return string(v.(lua.LString))
	default:
		return fmt.Sprintf("<%s>", v.Type().String())
	}
}

// listRowColumns returns a comma-separated list of string keys in a row,
// for error messages.
func listRowColumns(row *lua.LTable) string {
	var cols []string
	row.ForEach(func(k lua.LValue, _ lua.LValue) {
		if k.Type() == lua.LTString {
			cols = append(cols, string(k.(lua.LString)))
		}
	})
	return strings.Join(cols, ", ")
}