package main

import (
	"fmt"
	"math/rand"

	lua "github.com/yuin/gopher-lua"
)

func main() {
	for run := 1; run <= 3; run++ {
		L := lua.NewState()

		// Override math.random and math.randomseed (the fix)
		mathTable := L.GetGlobal("math").(*lua.LTable)
		L.SetField(mathTable, "random", L.NewFunction(func(L *lua.LState) int {
			top := L.GetTop()
			if top >= 1 {
				n := L.ToInt(1)
				if n > 0 {
					L.Push(lua.LNumber(rand.Intn(n) + 1))
					return 1
				}
			}
			L.Push(lua.LNumber(rand.Float64()))
			return 1
		}))
		L.SetField(mathTable, "randomseed", L.NewFunction(func(L *lua.LState) int {
			seed := int64(L.ToInt(1))
			rand.Seed(seed)
			return 0
		}))

		script := `
			math.randomseed(43)
			local values = {}
			for i = 1, 5 do
				table.insert(values, string.format("%.10f", math.random()))
			end
			print(table.concat(values, ", "))
		`

		fmt.Printf("Run %d: ", run)
		L.DoString(script)
		L.Close()
	}
}
