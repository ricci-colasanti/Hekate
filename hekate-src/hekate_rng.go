// ============================================================================
// hekate_rng.go - Self-Contained Lehmer RNG
// ============================================================================
//
// A minimal, fully-controlled pseudo-random number generator based on the
// Lehmer / Park-Miller multiplicative congruential generator:
//
//     state = (state * 16807) % 2147483647
//     value = state / 2147483647
//
// WHY THIS EXISTS
// ---------------
// Go 1.20+ made the global math/rand generator non-deterministic by default.
// Even a dedicated rand.Rand is fine, but it depends on Go's internal
// algorithm, which could change between Go versions. This LCG is defined
// entirely by us: same seed → same sequence, on every Go version, forever.
//
// CONSTRAINTS
// -----------
//   - Modulus 2147483647 = 2^31 - 1 (Mersenne prime, standard for Park-Miller)
//   - Multiplier 16807 = 7^5 (Park & Miller 1988 recommendation)
//   - Seed must be in [1, 2147483646]; 0 is a fixed point and must be avoided.
//
// QUALITY NOTES
// -------------
// This is a *low-quality* PRNG by modern standards (short period ~2^31,
// poor low-bit behaviour, fails several statistical tests). For a
// microsimulation engine that needs reproducibility as its primary goal,
// that trade-off is deliberate and acceptable. Do NOT use this for
// cryptographic purposes.
//
// ============================================================================

package main

import (
	"sync"

	lua "github.com/yuin/gopher-lua"
)

const (
	lcgModulus    uint64 = 2147483647 // 2^31 - 1
	lcgMultiplier uint64 = 16807      // 7^5
)

// HekateRNG is a reproducible Lehmer LCG with a mutex for safety.
//
// The mutex matters because gopher-lua runs on one goroutine per LState,
// but if you ever call the RNG from Go code concurrently, you'll want it.
type HekateRNG struct {
	mu    sync.Mutex
	state uint64
	seed  int64
}

// NewHekateRNG creates an RNG seeded from the given int64 seed.
//
// The raw seed is normalised into [1, 2147483646]:
//   - 0 is mapped to 1 (LCG would otherwise be stuck at 0 forever)
//   - negative seeds are folded into the positive range
func NewHekateRNG(seed int64) *HekateRNG {
	r := &HekateRNG{}
	r.Seed(seed)
	return r
}

// Seed resets the generator to a new state derived from `seed`.
//
// Two RNGs with the same seed will produce identical sequences.
func (r *HekateRNG) Seed(seed int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seed = seed

	// Normalise into [1, modulus-1].
	// Go's % can return negative values for negative operands, so we fix up.
	s := seed % int64(lcgModulus-1)
	if s < 0 {
		s = -s
	}
	if s == 0 {
		s = 1 // avoid the LCG's fixed point
	}
	r.state = uint64(s)
}

// nextUint advances the LCG and returns the new state.
// Caller must NOT hold the mutex.
func (r *HekateRNG) nextUint() uint64 {
	r.state = (r.state * lcgMultiplier) % lcgModulus
	return r.state
}

// Float64 returns a value in (0, 1).
//
// We divide by modulus (not modulus-1) so the result is strictly < 1,
// matching the semantics of math/rand.Float64 and Lua's math.random().
// The result is never exactly 0 because state is never 0.
func (r *HekateRNG) Float64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return float64(r.nextUint()) / float64(lcgModulus)
}

// Intn returns an integer in [0, n).
//
// Uses rejection sampling rather than modulo to avoid modulo bias.
// For n <= 0 it returns 0.
func (r *HekateRNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	limit := uint64(n)
	// Reject values in the "tail" so the distribution is uniform.
	// maxAcceptable is the largest multiple of `limit` that fits in modulus.
	maxAcceptable := lcgModulus - (lcgModulus % limit)
	for {
		v := r.nextUint()
		if v < maxAcceptable {
			return int(v % limit)
		}
	}
}

// Int63 mimics math/rand.Int63 (a non-negative int64), in case anything
// in the codebase expects that interface.
func (r *HekateRNG) Int63() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(r.nextUint())
}

// ============================================================================
// Lua Binding
// ============================================================================

// RegisterHekateRNG installs the RNG as `hekate_rng` in the Lua state, and
// also wires it into `math.random` / `math.randomseed` so existing scripts
// that use Lua's standard functions still get our deterministic generator.
func RegisterHekateRNG(L *lua.LState, rng *HekateRNG) {
	tbl := L.NewTable()

	// hekate_rng.random()              -> float in (0, 1)
	// hekate_rng.random(n)             -> int in [1, n]
	// hekate_rng.random(m, n)          -> int in [m, n]
	L.SetField(tbl, "random", L.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		switch top {
		case 0:
			L.Push(lua.LNumber(rng.Float64()))
			return 1
		case 1:
			n := L.CheckInt(1)
			if n < 1 {
				L.ArgError(1, "interval is empty")
				return 0
			}
			L.Push(lua.LNumber(rng.Intn(n) + 1))
			return 1
		default:
			m := L.CheckInt(1)
			n := L.CheckInt(2)
			if m > n {
				L.ArgError(1, "interval is empty")
				return 0
			}
			L.Push(lua.LNumber(rng.Intn(n-m+1) + m))
			return 1
		}
	}))

	// hekate_rng.randomseed(n)         -> reseed
	L.SetField(tbl, "randomseed", L.NewFunction(func(L *lua.LState) int {
		rng.Seed(int64(L.CheckInt64(1)))
		return 0
	}))

	// hekate_rng.seed()                -> returns the current seed (for debugging)
	L.SetField(tbl, "seed", L.NewFunction(func(L *lua.LState) int {
		rng.mu.Lock()
		defer rng.mu.Unlock()
		L.Push(lua.LNumber(rng.seed))
		return 1
	}))

	L.SetGlobal("hekate_rng", tbl)

	// ---- Belt and braces: override Lua's math.random ----
	mathTable, ok := L.GetGlobal("math").(*lua.LTable)
	if !ok {
		return
	}

	L.SetField(mathTable, "random", L.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		switch top {
		case 0:
			L.Push(lua.LNumber(rng.Float64()))
			return 1
		case 1:
			n := L.CheckInt(1)
			if n < 1 {
				L.ArgError(1, "interval is empty")
				return 0
			}
			L.Push(lua.LNumber(rng.Intn(n) + 1))
			return 1
		default:
			m := L.CheckInt(1)
			n := L.CheckInt(2)
			if m > n {
				L.ArgError(1, "interval is empty")
				return 0
			}
			L.Push(lua.LNumber(rng.Intn(n-m+1) + m))
			return 1
		}
	}))

	L.SetField(mathTable, "randomseed", L.NewFunction(func(L *lua.LState) int {
		rng.Seed(int64(L.CheckInt64(1)))
		return 0
	}))
}