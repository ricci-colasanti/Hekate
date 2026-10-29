// ============================================================================
// hekate_stats.go - Statistical Functions Exposed to Lua
// ============================================================================
//
// These functions are exposed to Lua as `hekate_stats.*`. They provide
// commonly-needed statistical operations implemented in Go for speed and
// numerical accuracy.
//
// FUNCTION CATEGORIES
// -------------------
//   *_predict  - Pure functions that compute a value or probability.
//                No randomness, no side effects.
//   *_draw     - Random sampling functions that use the VM's dedicated RNG
//                (vm.rng) to preserve reproducibility.
//
// REPRODUCIBILITY
// ---------------
// Functions ending in "_draw" MUST use vm.rng, never the global math/rand
// and never math/rand directly. Otherwise they will break Hekate's
// reproducibility guarantee.
//
// ============================================================================

package main

import (
	"math"

	lua "github.com/yuin/gopher-lua"
)

// HekateStats is a receiver for the statistical functions.
//
// It holds a reference to the LuaVM so that draw functions can access the
// dedicated random source (vm.rng) for reproducibility.
type HekateStats struct {
	vm *LuaVM
}

// ============================================================================
// PREDICTION FUNCTIONS (no randomness)
// ============================================================================

// LinearPredict computes: y = intercept + coef₁×var₁ + coef₂×var₂ + ...
//
// Lua signature:
//
//	hekate_stats.linear_predict(intercept, coef1, var1, coef2, var2, ...)
//
// Example (predicting income):
//
//	local income = hekate_stats.linear_predict(
//	    25000,                  -- intercept
//	    500, person.age,        -- age effect
//	    3000, edu_score,        -- education effect
//	    -2000, gender_score     -- gender effect
//	)
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

// LinearPredictDefault is like LinearPredict but substitutes a default
// value when a variable is nil (missing).
//
// Lua signature:
//
//	hekate_stats.linear_predict_default(intercept, global_default,
//	    coef1, var1, default1, coef2, var2, default2, ...)
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

// LogisticPredict computes: p = 1 / (1 + e^(-linear))
//
// Returns a probability between 0 and 1. Useful for modelling binary
// outcomes (die/survive, get disease/don't, migrate/stay).
//
// Lua signature:
//
//	hekate_stats.logistic_predict(intercept, coef1, var1, coef2, var2, ...)
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

// PoissonPredict computes: lambda = exp(intercept + coef₁×var₁ + ...)
//
// Returns the expected count for a Poisson-distributed outcome. Useful for
// modelling count data: number of children, doctor visits, migrations, etc.
//
// Lua signature:
//
//	hekate_stats.poisson_predict(intercept, coef1, var1, coef2, var2, ...)
//
// Example (expected number of children):
//
//	local lambda = hekate_stats.poisson_predict(
//	    -1.5,                    -- intercept (base rate)
//	    0.05, person.age,        -- age effect
//	    0.5, married_score,      -- married effect
//	    -0.3, urban_score        -- urban effect
//	)
func (h *HekateStats) PoissonPredict(L *lua.LState) int {
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

	L.Push(lua.LNumber(math.Exp(linear)))
	return 1
}

// SoftmaxPredict computes softmax over multiple linear combinations.
//
// Takes a variable number of score tables, each of which represents the
// linear combination for one category:
//
//	score_k = base_k + coef_k1 × var_k1 + coef_k2 × var_k2 + ...
//
// Returns a table of probabilities (one per category) that sum to 1.0.
//
// Lua signature:
//
//	hekate_stats.softmax_predict(
//	    {base1, coef1a, var1a, coef1b, var1b, ...},  -- category 1
//	    {base2, coef2a, var2a, coef2b, var2b, ...},  -- category 2
//	    {base3, coef3a, var3a, coef3b, var3b, ...}   -- category 3
//	)
//
// Example (education outcome):
//
//	local probs = hekate_stats.softmax_predict(
//	    {0.5, 0.1, age, -0.2, parent_edu},   -- primary
//	    {1.0, 0.3, age, 0.5, parent_edu},    -- secondary
//	    {0.2, 0.5, age, 0.8, parent_edu}     -- tertiary
//	)
//	-- probs is a table like {0.15, 0.55, 0.30}
func (h *HekateStats) SoftmaxPredict(L *lua.LState) int {
	numCategories := L.GetTop()
	if numCategories == 0 {
		result := L.NewTable()
		L.Push(result)
		return 1
	}

	// Compute linear score for each category
	scores := make([]float64, numCategories)
	maxScore := -math.MaxFloat64

	for catIdx := 1; catIdx <= numCategories; catIdx++ {
		tbl := L.CheckTable(catIdx)

		// First element is the base/intercept
		base := tbl.RawGetInt(1)
		linear := 0.0
		if base.Type() == lua.LTNumber {
			linear = float64(base.(lua.LNumber))
		}

		// Remaining elements come in (coef, var) pairs
		// We iterate over the table using RawGetInt since it's a list
		length := tbl.Len()
		for i := 2; i+1 <= length; i += 2 {
			coefVal := tbl.RawGetInt(i)
			varVal := tbl.RawGetInt(i + 1)
			if coefVal.Type() == lua.LTNumber && varVal.Type() == lua.LTNumber {
				coef := float64(coefVal.(lua.LNumber))
				varNum := float64(varVal.(lua.LNumber))
				linear += coef * varNum
			}
		}

		scores[catIdx-1] = linear
		if linear > maxScore {
			maxScore = linear
		}
	}

	// Compute softmax with numerical stability (subtract max to avoid overflow)
	sumExp := 0.0
	for i := range scores {
		scores[i] = math.Exp(scores[i] - maxScore)
		sumExp += scores[i]
	}

	// Normalize and return as a Lua table
	result := L.NewTable()
	for i := range scores {
		result.Append(lua.LNumber(scores[i] / sumExp))
	}
	L.Push(result)
	return 1
}

// ============================================================================
// DRAW FUNCTIONS (use vm.rng for reproducibility)
// ============================================================================
//
// IMPORTANT: These functions MUST use h.vm.rng, never the global math/rand.
// Using the global RNG would break Hekate's reproducibility guarantee.
// ============================================================================

// LogisticDraw combines LogisticPredict with a Bernoulli sample.
//
// Returns true if the random draw is less than the computed probability.
// Returns false otherwise.
//
// This is a convenience function that eliminates the need to write:
//
//	local p = hekate_stats.logistic_predict(...)
//	if math.random() < p then ... end
//
// Lua signature:
//
//	hekate_stats.logistic_draw(intercept, coef1, var1, coef2, var2, ...)
//
// Example (mortality):
//
//	if hekate_stats.logistic_draw(
//	    -3.5,                -- intercept
//	    1.2, age/10,         -- age effect
//	    0.8, bmi_score,      -- BMI effect
//	    1.5, smoker_score    -- smoking effect
//	) then
//	    person.alive = false
//	end
func (h *HekateStats) LogisticDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("logistic_draw: vm not initialized")
		return 0
	}

	// Compute the probability (same logic as LogisticPredict)
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

	prob := 1.0 / (1.0 + math.Exp(-linear))

	// Sample using the dedicated RNG for reproducibility
	drawn := h.vm.rng.Float64() < prob

	L.Push(lua.LBool(drawn))
	return 1
}

// CategoricalDraw samples from a categorical distribution.
//
// Takes N probability arguments and returns the 1-based index of the
// drawn category. Probabilities are automatically normalized if they
// don't sum to 1.0.
//
// Lua signature:
//
//	hekate_stats.categorical_draw(p1, p2, p3, ...)
//
// Example (choosing an education level):
//
//	local idx = hekate_stats.categorical_draw(0.4, 0.5, 0.1)
//	local edu_names = {"primary", "secondary", "tertiary"}
//	person.education = edu_names[idx]
func (h *HekateStats) CategoricalDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("categorical_draw: vm not initialized")
		return 0
	}

	n := L.GetTop()
	if n == 0 {
		L.Push(lua.LNumber(1))
		return 1
	}

	// Read and validate probabilities
	probs := make([]float64, n)
	total := 0.0
	for i := 1; i <= n; i++ {
		p := float64(L.CheckNumber(i))
		if p < 0 {
			p = 0 // Treat negative probabilities as zero
		}
		probs[i-1] = p
		total += p
	}

	// If all probabilities are zero, default to the first category
	if total == 0 {
		L.Push(lua.LNumber(1))
		return 1
	}

	// Draw a uniform random value and find the corresponding category
	r := h.vm.rng.Float64() * total
	cumulative := 0.0
	for i, p := range probs {
		cumulative += p
		if r < cumulative {
			L.Push(lua.LNumber(i + 1)) // 1-based index
			return 1
		}
	}

	// Fallback (shouldn't reach here due to floating point, but safe)
	L.Push(lua.LNumber(n))
	return 1
}

// PoissonDraw samples from a Poisson distribution with the given mean.
//
// This is an alternative to calling PoissonPredict then rolling your own
// draw. Uses a numerically stable algorithm for the Poisson sample.
//
// Lua signature:
//
//	hekate_stats.poisson_draw(intercept, coef1, var1, coef2, var2, ...)
//
// Example (number of children):
//
//	local num_children = hekate_stats.poisson_draw(
//	    -1.5,
//	    0.05, person.age,
//	    0.5, married_score
//	)
func (h *HekateStats) PoissonDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("poisson_draw: vm not initialized")
		return 0
	}

	// Compute lambda (expected count)
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

	lambda := math.Exp(linear)

	// For very small lambda, use the simple algorithm (just check for zero events)
	if lambda < 30 {
		// Knuth's algorithm
		L_val := math.Exp(-lambda)
		k := 0
		p := 1.0
		for {
			k++
			p *= h.vm.rng.Float64()
			if p <= L_val {
				L.Push(lua.LNumber(k - 1))
				return 1
			}
			if k > 10000 {
				// Safety cap
				L.Push(lua.LNumber(k - 1))
				return 1
			}
		}
	}

	// For large lambda, use the normal approximation
	// Poisson(lambda) ≈ Normal(lambda, sqrt(lambda)) for large lambda
	z := h.normalDraw()
	sample := lambda + math.Sqrt(lambda)*z
	if sample < 0 {
		sample = 0
	}
	L.Push(lua.LNumber(math.Round(sample)))
	return 1
}

// normalDraw returns a sample from the standard normal distribution N(0,1)
// using the Box-Muller transform. Internal helper, not exposed to Lua.
func (h *HekateStats) normalDraw() float64 {
	u1 := h.vm.rng.Float64()
	u2 := h.vm.rng.Float64()
	// Avoid log(0)
	if u1 < 1e-10 {
		u1 = 1e-10
	}
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}