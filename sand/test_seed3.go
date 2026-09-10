// test_seed3.go
package main

import (
	"fmt"
	"math/rand"
	"runtime"
)

func main() {
	fmt.Printf("Go version: %s\n\n", runtime.Version())

	// ============================================
	// Test 1: Global rand.Seed (broken in Go 1.20+)
	// ============================================
	fmt.Println("=== Test 1: Global rand.Seed ===")

	rand.Seed(43)
	v1 := rand.Float64()
	v2 := rand.Float64()
	v3 := rand.Float64()
	fmt.Printf("  Run 1: %.10f, %.10f, %.10f\n", v1, v2, v3)

	rand.Seed(43)
	w1 := rand.Float64()
	w2 := rand.Float64()
	w3 := rand.Float64()
	fmt.Printf("  Run 2: %.10f, %.10f, %.10f\n", w1, w2, w3)

	match := (v1 == w1) && (v2 == w2) && (v3 == w3)
	fmt.Printf("  Match: %v\n", match)
	if !match {
		fmt.Println("  ❌ Global rand.Seed is BROKEN (Go 1.20+ behavior)")
	} else {
		fmt.Println("  ✅ Global rand.Seed works")
	}

	// ============================================
	// Test 2: Custom rand.Rand source (always works)
	// ============================================
	fmt.Println("\n=== Test 2: Custom rand.Rand source ===")

	r1 := rand.New(rand.NewSource(43))
	a1 := r1.Float64()
	a2 := r1.Float64()
	a3 := r1.Float64()
	fmt.Printf("  Run 1: %.10f, %.10f, %.10f\n", a1, a2, a3)

	r2 := rand.New(rand.NewSource(43))
	b1 := r2.Float64()
	b2 := r2.Float64()
	b3 := r2.Float64()
	fmt.Printf("  Run 2: %.10f, %.10f, %.10f\n", b1, b2, b3)

	match2 := (a1 == b1) && (a2 == b2) && (a3 == b3)
	fmt.Printf("  Match: %v\n", match2)
	if match2 {
		fmt.Println("  ✅ Custom rand.New(rand.NewSource()) works")
	} else {
		fmt.Println("  ❌ Something is very wrong")
	}

	// ============================================
	// Test 3: Show the actual values
	// ============================================
	fmt.Println("\n=== Test 3: Known Go values for seed 43 ===")
	r := rand.New(rand.NewSource(43))
	fmt.Printf("  seed 43, first 5 values:\n")
	for i := 0; i < 5; i++ {
		fmt.Printf("    %d: %.10f\n", i+1, r.Float64())
	}

	// ============================================
	// Summary
	// ============================================
	fmt.Println("\n=== Summary ===")
	if !match && match2 {
		fmt.Println("DIAGNOSIS: Go 1.20+ global rand.Seed is broken.")
		fmt.Println("FIX: Use rand.New(rand.NewSource(seed)) for a dedicated source.")
	} else if match && match2 {
		fmt.Println("DIAGNOSIS: Both work fine. Problem is elsewhere.")
	} else {
		fmt.Println("DIAGNOSIS: Both are broken. Very unusual.")
	}
}
