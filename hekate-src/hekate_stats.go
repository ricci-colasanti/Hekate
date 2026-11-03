// ============================================================================
// hekate_stats.go - Statistical Functions Exposed to Lua
// ============================================================================
//
// These functions are exposed to Lua as bare globals (e.g. linear_predict,
// normal_draw). They provide commonly-needed statistical operations
// implemented in Go for speed and numerical accuracy.
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
// NAMING
// ------
// All functions in this file are registered as bare globals in lua_vm.go.
// There is no `hekate_stats` table. A user script that defines a function
// with the same name will silently shadow the built-in for the rest of that
// script — standard Lua behaviour.
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
//	linear_predict(intercept, coef1, var1, coef2, var2, ...)
//
// Example (predicting income):
//
//	local income = linear_predict(
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
//	linear_predict_default(intercept, global_default,
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
//	logistic_predict(intercept, coef1, var1, coef2, var2, ...)
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
//	poisson_predict(intercept, coef1, var1, coef2, var2, ...)
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
//	softmax_predict(
//	    {base1, coef1a, var1a, coef1b, var1b, ...},  -- category 1
//	    {base2, coef2a, var2a, coef2b, var2b, ...},  -- category 2
//	    {base3, coef3a, var3a, coef3b, var3b, ...}   -- category 3
//	)
func (h *HekateStats) SoftmaxPredict(L *lua.LState) int {
	numCategories := L.GetTop()
	if numCategories == 0 {
		result := L.NewTable()
		L.Push(result)
		return 1
	}

	scores := make([]float64, numCategories)
	maxScore := -math.MaxFloat64

	for catIdx := 1; catIdx <= numCategories; catIdx++ {
		tbl := L.CheckTable(catIdx)

		base := tbl.RawGetInt(1)
		linear := 0.0
		if base.Type() == lua.LTNumber {
			linear = float64(base.(lua.LNumber))
		}

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

	sumExp := 0.0
	for i := range scores {
		scores[i] = math.Exp(scores[i] - maxScore)
		sumExp += scores[i]
	}

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
//
// Lua signature:
//
//	logistic_draw(intercept, coef1, var1, coef2, var2, ...)
func (h *HekateStats) LogisticDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("logistic_draw: vm not initialized")
		return 0
	}

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
//	categorical_draw(p1, p2, p3, ...)
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

	probs := make([]float64, n)
	total := 0.0
	for i := 1; i <= n; i++ {
		p := float64(L.CheckNumber(i))
		if p < 0 {
			p = 0
		}
		probs[i-1] = p
		total += p
	}

	if total == 0 {
		L.Push(lua.LNumber(1))
		return 1
	}

	r := h.vm.rng.Float64() * total
	cumulative := 0.0
	for i, p := range probs {
		cumulative += p
		if r < cumulative {
			L.Push(lua.LNumber(i + 1))
			return 1
		}
	}

	L.Push(lua.LNumber(n))
	return 1
}

// PoissonDraw samples from a Poisson distribution with the given mean.
//
// Lua signature:
//
//	poisson_draw(intercept, coef1, var1, coef2, var2, ...)
func (h *HekateStats) PoissonDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("poisson_draw: vm not initialized")
		return 0
	}

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

	if lambda < 30 {
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
				L.Push(lua.LNumber(k - 1))
				return 1
			}
		}
	}

	z := h.normalDraw()
	sample := lambda + math.Sqrt(lambda)*z
	if sample < 0 {
		sample = 0
	}
	L.Push(lua.LNumber(math.Round(sample)))
	return 1
}

// ============================================================================
// DISTRIBUTION DRAWS (use vm.rng for reproducibility)
// ============================================================================
//
// These expose common distributions as native Go functions so scripts don't
// hand-roll Box-Muller or inverse-CDF code. All draws use h.vm.rng, so they
// are reproducible in exactly the same way as math.random() and the other
// *_draw functions. Argument validation errors name the problem and the value.
// ============================================================================

// NormalDraw samples from N(mean, sd²).
//
// Lua signature:
//
//	normal_draw(mean, sd)
//
// sd must be >= 0; a negative sd is a config bug, not something to silently
// absorb with abs().
func (h *HekateStats) NormalDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("normal_draw: vm not initialized")
		return 0
	}
	mean := float64(L.CheckNumber(1))
	sd := float64(L.CheckNumber(2))
	if sd < 0 {
		L.RaiseError("normal_draw: sd must be >= 0 (got %g)", sd)
		return 0
	}
	L.Push(lua.LNumber(mean + sd*h.normalDraw()))
	return 1
}

// StandardNormalDraw samples from N(0, 1).
//
// Lua signature:
//
//	standard_normal_draw()
func (h *HekateStats) StandardNormalDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("standard_normal_draw: vm not initialized")
		return 0
	}
	L.Push(lua.LNumber(h.normalDraw()))
	return 1
}

// UniformDraw samples from the continuous uniform distribution on [min, max).
//
// Lua signature:
//
//	uniform_draw(min, max)
//
// max must be > min. This is the continuous counterpart to math.random(m, n),
// which returns integers.
func (h *HekateStats) UniformDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("uniform_draw: vm not initialized")
		return 0
	}
	min := float64(L.CheckNumber(1))
	max := float64(L.CheckNumber(2))
	if max <= min {
		L.RaiseError("uniform_draw: max must be > min (got min=%g, max=%g)", min, max)
		return 0
	}
	L.Push(lua.LNumber(min + (max-min)*h.vm.rng.Float64()))
	return 1
}

// ExponentialDraw samples from Exponential(rate), mean 1/rate.
//
// Lua signature:
//
//	exponential_draw(rate)
//
// rate must be > 0. Uses inverse-CDF: -ln(u)/rate.
func (h *HekateStats) ExponentialDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("exponential_draw: vm not initialized")
		return 0
	}
	rate := float64(L.CheckNumber(1))
	if rate <= 0 {
		L.RaiseError("exponential_draw: rate must be > 0 (got %g)", rate)
		return 0
	}
	u := h.vm.rng.Float64()
	if u < 1e-10 {
		u = 1e-10
	}
	L.Push(lua.LNumber(-math.Log(u) / rate))
	return 1
}

// BernoulliDraw samples a boolean with probability p of true.
//
// Lua signature:
//
//	bernoulli_draw(p)
//
// p must be in [0, 1]. This is the general form of the decision that
// logistic_draw makes; use it when you already have a probability in hand.
func (h *HekateStats) BernoulliDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("bernoulli_draw: vm not initialized")
		return 0
	}
	p := float64(L.CheckNumber(1))
	if p < 0 || p > 1 {
		L.RaiseError("bernoulli_draw: p must be in [0, 1] (got %g)", p)
		return 0
	}
	L.Push(lua.LBool(h.vm.rng.Float64() < p))
	return 1
}

// LognormalDraw samples from LogNormal(mu, sigma²), i.e. exp(N(mu, sigma²)).
//
// Lua signature:
//
//	lognormal_draw(mu, sigma)
//
// mu and sigma are the parameters of the *log*. The returned value is always
// positive. sigma must be >= 0.
func (h *HekateStats) LognormalDraw(L *lua.LState) int {
	if h.vm == nil {
		L.RaiseError("lognormal_draw: vm not initialized")
		return 0
	}
	mu := float64(L.CheckNumber(1))
	sigma := float64(L.CheckNumber(2))
	if sigma < 0 {
		L.RaiseError("lognormal_draw: sigma must be >= 0 (got %g)", sigma)
		return 0
	}
	L.Push(lua.LNumber(math.Exp(mu + sigma*h.normalDraw())))
	return 1
}

// normalDraw returns a sample from the standard normal distribution N(0,1)
// using the Box-Muller transform. Internal helper, not exposed to Lua directly
// (StandardNormalDraw wraps it for scripts).
func (h *HekateStats) normalDraw() float64 {
	u1 := h.vm.rng.Float64()
	u2 := h.vm.rng.Float64()
	// Avoid log(0)
	if u1 < 1e-10 {
		u1 = 1e-10
	}
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}