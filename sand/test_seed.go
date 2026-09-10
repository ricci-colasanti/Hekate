package main

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

func main() {
	for run := 1; run <= 3; run++ {
		L := lua.NewState()

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
