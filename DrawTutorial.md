# Hekate — Temporary Tutorial: The Draw Functions

This is a **temporary tutorial** covering the nine distribution draw functions in Hekate 0.9.2, plus one worked example combining `normal_draw` with `hekate_lookup`. A proper tutorial will follow; this one is designed so you can run each example, inspect the output, and see exactly what each function does.

## What you need

Ten files in a single directory:

- `draws_population.csv` — the shared population, used by all nine draw demos
- `lookup_population.csv` — a separate population, used only by the lookup example
- `lookup_rules.csv` — the lookup table for the lookup example
- Ten configs: `01_logistic_draw.yaml` through `09_lognormal_draw.yaml`, plus `10_lookup_normal.yaml`

You can put them all in one folder. Each config is self-contained; none needs the others.

## Prerequisites

- `hekate` binary (0.9.2 or later)
- Run every command from the directory containing the files
- All configs use `random_seed: 42`, so results are reproducible

## What this tutorial covers

| # | Config | Draw function | What it produces |
|---|---|---|---|
| 1 | `01_logistic_draw.yaml` | `logistic_draw` | `died_this_year` (boolean) — age-based death decision |
| 2 | `02_categorical_draw.yaml` | `categorical_draw` | `education_pick` (string) — one of three levels |
| 3 | `03_poisson_draw.yaml` | `poisson_draw` | `children_this_year` (integer) — annual births per woman |
| 4 | `04_normal_draw.yaml` | `normal_draw` | `income_noisy` (float) — income with Gaussian noise |
| 5 | `05_standard_normal_draw.yaml` | `standard_normal_draw` | `z_score` (float) — raw N(0,1) sample |
| 6 | `06_uniform_draw.yaml` | `uniform_draw` | `random_fraction` (float) — in [0, 1) |
| 7 | `07_exponential_draw.yaml` | `exponential_draw` | `waiting_time_years` (float) — mean 5 years |
| 8 | `08_bernoulli_draw.yaml` | `bernoulli_draw` | `migrated_this_year` (boolean) — 10% hazard |
| 9 | `09_lognormal_draw.yaml` | `lognormal_draw` | `income_lognormal` (float) — right-skewed income |
| 10 | `10_lookup_normal.yaml` | `hekate_lookup.index` + `normal_draw` | `income` (float) — from per-group mean and SD |

## Important: all draws are bare globals

As of Hekate 0.9.2, **every statistical function is a bare global**. Write `normal_draw(0, 1)`, **not** `hekate_stats.normal_draw(0, 1)`. The `hekate_stats` table no longer exists — any existing script using the old prefix will fail with `attempt to index a nil value (global 'hekate_stats')` and needs updating.

The namespace split is:

- **Statistical functions** (`*_predict`, `*_draw`): **bare globals**, no prefix.
- **The raw RNG** (`random`, `randomseed`, `seed`): still namespaced as `hekate_rng.*`.
- **The lookup builder**: still namespaced as `hekate_lookup.index(...)`.

`math.random` and `math.randomseed` are still overridden to use Hekate's deterministic generator, so `math.random()` and `hekate_rng.random()` give the same values.

---

## Shared files

### `draws_population.csv`

Used by configs 01–09. Ten people, women and men, ages 2–82, all alive, all with a starting income.

```csv
id,sex,age,alive,area_id,income
1,F,25,true,1,24000
2,M,30,true,1,31000
3,F,45,true,1,42000
4,M,68,true,1,35000
5,F,82,true,1,18000
6,M,2,true,2,0
7,F,15,true,2,0
8,M,35,true,2,38000
9,F,55,true,2,46000
10,M,70,true,2,22000
```

### `lookup_population.csv`

Used only by config 10. Fifteen people, women and men, ages 2–82, all alive, all with `income = 0` (it will be overwritten by the lookup model).

```csv
id,sex,age,alive,area_id,income
1,F,25,true,1,0
2,M,30,true,1,0
3,F,45,true,1,0
4,M,68,true,1,0
5,F,82,true,1,0
6,M,2,true,2,0
7,F,15,true,2,0
8,M,35,true,2,0
9,F,55,true,2,0
10,M,70,true,2,0
11,F,38,true,3,0
12,M,42,true,3,0
13,F,29,true,3,0
14,M,51,true,3,0
15,F,64,true,3,0
```

### `lookup_rules.csv`

The lookup table for config 10. One row per `(sex, alive)` combination, giving the mean (`value`) and SD (`sd`) of a normal draw.

```csv
sex,alive,value,sd
M,true,30000,4000
M,false,0,0
F,true,26000,3500
F,false,0,0
```

**Note the `alive` column here is text** (`"true"`/`"false"`), while in `lookup_population.csv` the `alive` column is parsed as a **boolean** by Hekate. The lookup index coerces both to the same string when comparing, so `by_key[person.sex][person.alive]` works whether `person.alive` is a Lua `true` or the string `"true"`. This is intentional and one of the things the lookup builder is designed to handle.

---

## The nine draw demos

Each config below is complete. To run: `./hekate-linux-amd64 01_logistic_draw.yaml`, etc.

### 01 — `logistic_draw`

**What it does:** computes a probability from a linear predictor, then samples a boolean. Combines `logistic_predict` + `bernoulli_draw` in one call.

```yaml
# 01_logistic_draw.yaml — demonstrate logistic_draw
#
# logistic_draw(intercept, coef1, var1, ...) computes
#   p = 1 / (1 + e^(-(intercept + Σ coefᵢ·varᵢ)))
# then samples a boolean from Bernoulli(p).
#
# This example: age-based one-year mortality probability.
#   log-odds = -8.0 + 0.10 * age
#   At age 25 → p ≈ 2%      At age 70 → p ≈ 12%
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_01_logistic.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "one_year_mortality"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Age-based one-year death decision (logistic_draw)"
    parameters:
      intercept: -8.0
      age_coef: 0.10
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive then
              local age = tonumber(person.age) or 40
              person.died_this_year = logistic_draw(
                params.intercept,
                params.age_coef, age
              )
            end
          end
          return population
        end
```

**What to look for in `out_01_logistic.csv`:** almost all `died_this_year` values should be `false`. The two oldest (ids 4 and 10, ages 68 and 70) have ~12% chance; everyone else is under 5%. With ten people, expect 0–1 `true` values.

---

### 02 — `categorical_draw`

**What it does:** picks one of N categories with given probabilities. Returns the **1-based index** of the drawn category, which you map back to a name.

```yaml
# 02_categorical_draw.yaml — demonstrate categorical_draw
#
# categorical_draw(p1, p2, ..., pN) samples one of N categories and returns
# the 1-based index. Probabilities are auto-normalised if they don't sum to 1.
#
# This example: assign each living person one of three education levels.
#   primary   20%
#   secondary 50%
#   tertiary  30%
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_02_categorical.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "education_assignment"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Pick one of primary/secondary/tertiary (categorical_draw)"
    parameters:
      script: |
        function transition(population, params)
          local names = {"primary", "secondary", "tertiary"}
          for _, person in ipairs(population) do
            if person.alive then
              local idx = categorical_draw(0.2, 0.5, 0.3)
              person.education_pick = names[idx]
            end
          end
          return population
        end
```

**What to look for:** `education_pick` values distributed roughly 20% `primary`, 50% `secondary`, 30% `tertiary` across the ten people. With only ten draws, expect variation — a handful of `secondary`, a few of each of the others.

---

### 03 — `poisson_draw`

**What it does:** samples a non-negative integer count, with the mean (`λ`) computed from a linear predictor.

```yaml
# 03_poisson_draw.yaml — demonstrate poisson_draw
#
# poisson_draw(intercept, coef1, var1, ...) computes
#   λ = exp(intercept + Σ coefᵢ·varᵢ)
# then samples from Poisson(λ), returning an integer ≥ 0.
#
# This example: annual number of children per woman aged 15–50.
#   λ = exp(-4.0 + 0.10 * age)
#   At age 25 → λ ≈ 0.22/yr      At age 35 → λ ≈ 0.61/yr
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_03_poisson.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "births_this_year"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Annual number of children per fertile woman (poisson_draw)"
    parameters:
      intercept: -4.0
      age_coef: 0.10
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive and person.sex == "F" then
              local age = tonumber(person.age) or 30
              if age >= 15 and age <= 50 then
                person.children_this_year = poisson_draw(
                  params.intercept,
                  params.age_coef, age
                )
              end
            end
          end
          return population
        end
```

**What to look for:** `children_this_year` for the four women aged 15–50 (ids 1, 3, 7, 9). Most values will be `0`; occasionally a `1`. Men and out-of-range women will have the column blank.

---

### 04 — `normal_draw`

**What it does:** samples a continuous value from `N(mean, sd²)`. **Errors if `sd < 0`.**

```yaml
# 04_normal_draw.yaml — demonstrate normal_draw
#
# normal_draw(mean, sd) samples from N(mean, sd²).
# sd must be ≥ 0; a negative sd is an error.
#
# This example: income = N(30000 + 300*(age-18), 5000²) for alive adults.
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_04_normal.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "income_with_normal_noise"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Income from a linear mean + Gaussian noise (normal_draw)"
    parameters:
      base_mean: 30000
      age_coef: 300
      sd: 5000
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive and tonumber(person.age) >= 18 then
              local age = tonumber(person.age)
              local mean = linear_predict(
                params.base_mean,
                params.age_coef, age - 18
              )
              person.income_noisy = normal_draw(mean, params.sd)
            end
          end
          return population
        end
```

**What to look for:** `income_noisy` values clustered around the linear mean (which rises with age) with ±5,000 spread. A few values below 20,000 or above 50,000 are legitimate tail draws.

---

### 05 — `standard_normal_draw`

**What it does:** samples from `N(0, 1)` with **no arguments**. The building block for anything normally distributed. `normal_draw(m, s)` = `m + s * standard_normal_draw()`.

```yaml
# 05_standard_normal_draw.yaml — demonstrate standard_normal_draw
#
# standard_normal_draw() samples from N(0, 1) with no arguments.
#
# This example: a raw z-score per alive person.
# In a real model you'd use this to build a standardised index, or as the
# raw ingredient for any N(mean, sd) draw you want to construct by hand:
#   person.z_score = standard_normal_draw()
#   person.income  = 30000 + 5000 * person.z_score
# -- same distribution as normal_draw(30000, 5000)
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_05_standard_normal.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "z_score"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Raw N(0,1) z-score per alive person (standard_normal_draw)"
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive then
              person.z_score = standard_normal_draw()
            end
          end
          return population
        end
```

**What to look for:** `z_score` values roughly in the range −2 to +2, centred on 0. With ten draws you'll see some spread; a value of −1.8 or +1.6 is normal; a value of +4 would be very rare and worth double-checking.

---

### 06 — `uniform_draw`

**What it does:** samples a continuous value from `[min, max)`. **Errors if `max <= min`.**

```yaml
# 06_uniform_draw.yaml — demonstrate uniform_draw
#
# uniform_draw(min, max) samples from the continuous uniform distribution
# on [min, max). max must be > min; otherwise it errors.
#
# This example: a random fraction in [0, 1) per alive person.
#   For integer uniforms in [1, n], use math.random(n) instead.
#   For a symmetric noise term in [-sd, +sd], write uniform_draw(-sd, sd).
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_06_uniform.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "random_fraction"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Uniform random fraction in [0, 1) (uniform_draw)"
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive then
              person.random_fraction = uniform_draw(0.0, 1.0)
            end
          end
          return population
        end
```

**What to look for:** `random_fraction` values in [0, 1), spread evenly. Every value should be ≥ 0 and < 1. A value of exactly 0 is possible in principle but very unlikely; a value of exactly 1 is impossible by design.

---

### 07 — `exponential_draw`

**What it does:** samples a positive waiting time from `Exponential(rate)`, mean `1/rate`. **Errors if `rate <= 0`.**

```yaml
# 07_exponential_draw.yaml — demonstrate exponential_draw
#
# exponential_draw(rate) samples from Exponential(rate), mean = 1/rate.
# rate must be > 0.
#
# This example: a per-person waiting time with mean 5 years (rate 0.2).
#   Use for time-to-event quantities: time to next job change, time between
#   births, time to equipment failure, etc.
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_07_exponential.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "waiting_time"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Exponential waiting time with mean 5 years (exponential_draw)"
    parameters:
      rate: 0.2
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive then
              person.waiting_time_years = exponential_draw(params.rate)
            end
          end
          return population
        end
```

**What to look for:** `waiting_time_years` values all ≥ 0, with mean around 5. Many values will be under 5, a few above 10, occasionally one above 20 — that's the long right tail of the exponential. No negative values are possible.

---

### 08 — `bernoulli_draw`

**What it does:** returns `true` with probability `p`, where `p` is a number you already have. **Errors if `p < 0` or `p > 1`.**

**The difference from `logistic_draw`:** `logistic_draw` takes a *linear predictor* and computes `p` for you; `bernoulli_draw` takes `p` *directly*. Use `bernoulli_draw` when you have a fixed hazard, a threshold, or the output of `logistic_predict`.

```yaml
# 08_bernoulli_draw.yaml — demonstrate bernoulli_draw
#
# bernoulli_draw(p) returns true with probability p. p must be in [0, 1].
#
# This example: 10% chance of migrating this year.
#
# Difference from logistic_draw:
#   logistic_draw(intercept, coef, var, ...)  -- linear predictor → p → boolean
#   bernoulli_draw(p)                         -- p directly → boolean
# Use bernoulli_draw when p is already a number (fixed hazard, threshold,
# output of logistic_predict, ...).
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_08_bernoulli.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "migration"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "10% annual migration hazard (bernoulli_draw)"
    parameters:
      migration_prob: 0.10
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive then
              person.migrated_this_year = bernoulli_draw(params.migration_prob)
            end
          end
          return population
        end
```

**What to look for:** `migrated_this_year` mostly `false`, with roughly 1 in 10 `true`. With ten people, expect 0–2 `true` values.

---

### 09 — `lognormal_draw`

**What it does:** samples a positive, right-skewed value from `LogNormal(mu, sigma²)`. `mu` and `sigma` are the parameters of the **log**, so `median = exp(mu)` and `mean = exp(mu + sigma²/2)`. **Errors if `sigma < 0`.**

```yaml
# 09_lognormal_draw.yaml — demonstrate lognormal_draw
#
# lognormal_draw(mu, sigma) samples from LogNormal(mu, sigma²), i.e.
# exp(N(mu, sigma²)). sigma must be ≥ 0; the result is always > 0.
#
# IMPORTANT: mu and sigma are the parameters of the LOG, not of the value.
#   median of the result = exp(mu)
#   mean  of the result = exp(mu + sigma²/2)
#
# This example: right-skewed income with median 30,000 and sigma 0.5.
#   mu = ln(30000) ≈ 10.31
#   median ≈ 30,000     mean ≈ 30,000 * exp(0.125) ≈ 34,000
#   Long right tail: some people will have 60,000+; none below ~5,000.
# ============================================================================

simulation:
  iterations: 1
  population_file: "draws_population.csv"
  output_file: "out_09_lognormal.csv"
  random_seed: 42
  verbose: true
  id_column: "id"

models:
  - name: "income_lognormal"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Right-skewed income with median 30000 (lognormal_draw)"
    parameters:
      median: 30000
      sigma: 0.5
      script: |
        function transition(population, params)
          local mu = math.log(params.median)
          for _, person in ipairs(population) do
            if person.alive then
              person.income_lognormal = lognormal_draw(mu, params.sigma)
            end
          end
          return population
        end
```

**What to look for:** `income_lognormal` values all positive, median around 30,000, right-skewed. Most values between 15,000 and 50,000; one or two outliers above 60,000 are typical. **The distribution is visibly asymmetric** — you shouldn't see many values below 10,000, but you should see the occasional very large one. This is the shape that distinguishes lognormal from normal.

---

## The lookup + `normal_draw` example

### 10 — `10_lookup_normal.yaml`

**What it does:** combines `hekate_lookup.index` with `normal_draw`. Builds a nested keyed index of the rules table, looks up each person's `(sex, alive)` rule, and draws income from `N(rule.value, rule.sd²)`.

This is the pattern to use when you have a per-group lookup table with means and SDs (income by sex, mortality by age-band, etc.) and want to sample individual values from within each group.

```yaml
# 10_lookup_normal.yaml — combine hekate_lookup.index with normal_draw
#
# Builds a nested index of lookup_rules.csv keyed by (sex, alive), looks up
# each person's rule, then draws income from N(rule.value, rule.sd²).
#
# This is the pattern for per-group means and SDs:
#   by_key[person.sex][person.alive] -> {sex, alive, value, sd}
#   person.income = normal_draw(rule.value, rule.sd)
#
# The lookup index coerces both the datafile keys and the population keys to
# strings, so 'alive' as a Lua boolean (from the population CSV) matches
# 'alive' as the string "true" (from the lookup CSV) without extra work.
# ============================================================================

simulation:
  iterations: 3
  population_file: "lookup_population.csv"
  output_file: "out_10_lookup_normal.csv"
  random_seed: 42
  verbose: true
  id_column: "id"
  output_every_iteration: true

models:
  # --------------------------------------------------------------------------
  # 1. Aging — so some people age out of the alive state over iterations
  # --------------------------------------------------------------------------
  - name: "aging"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Age living people by 1 year"
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive then
              person.age = (tonumber(person.age) or 0) + 1
            end
          end
          return population
        end

  # --------------------------------------------------------------------------
  # 2. Income from lookup: hekate_lookup.index + normal_draw
  # --------------------------------------------------------------------------
  - name: "income_from_lookup"
    type: "lua_model"
    priority: 2
    enabled: true
    description: "Income = normal_draw(rule.value, rule.sd), keyed by sex|alive"
    datafile: "lookup_rules.csv"
    parameters:
      script: |
        function transition(population, params)
          local by_key = hekate_lookup.index(params.datafile, "sex", "alive")

          for _, person in ipairs(population) do
            local rule = by_key[person.sex] and by_key[person.sex][person.alive]
            if rule then
              person.income = normal_draw(rule.value, rule.sd)
            else
              person.income = 0
            end
          end
          return population
        end

  # --------------------------------------------------------------------------
  # 3. Mortality — some people die, so later iterations exercise the
  #    (sex, false) lookup rule (value=0, sd=0 → income becomes exactly 0).
  # --------------------------------------------------------------------------
  - name: "mortality"
    type: "lua_model"
    priority: 3
    enabled: true
    description: "Age-based mortality (bernoulli_draw with a fixed hazard)"
    parameters:
      base_hazard: 0.01
      age_coef: 0.0005
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive then
              local age = tonumber(person.age) or 40
              local p = params.base_hazard + params.age_coef * age
              if p > 0.5 then p = 0.5 end
              if bernoulli_draw(p) then
                person.alive = false
              end
            end
          end
          return population
        end
```

**What to look for in `out_10_lookup_normal.csv`:**

- **Iteration 1** (`out_10_lookup_normal_1.csv`): everyone alive, so everyone matches a `(sex, true)` rule. Women's income clusters around 26,000 (sd 3,500); men's around 30,000 (sd 4,000). **No one has `income = 0`.**
- **Iterations 2–3:** some people have died. Their income becomes `normal_draw(0, 0) = 0` exactly, because the `(sex, false)` rule has `value=0, sd=0`. If you see a **dead person with `income = 0`**, the lookup is working. If you see a **dead person with `income = something nonzero`**, that's expected only if they died *after* the income model ran in that iteration (model priority 2 runs before priority 3).
- **Wrong-sex income** would signal the `sex` key isn't matching. **Zero income for alive people** would signal the `alive` key isn't matching. Neither should happen.

---


## Reproducibility

Every config uses `random_seed: 42`. Run any config twice and you'll get byte-identical output:

```bash
./hekate-linux-amd64 03_poisson_draw.yaml && mv out_03_poisson.csv run1.csv
./hekate-linux-amd64 03_poisson_draw.yaml && mv out_03_poisson.csv run2.csv
diff run1.csv run2.csv && echo "DETERMINISTIC"
```

Change `random_seed` to any other integer and you'll get different but still reproducible results.

**Important:** each config uses the same seed, but they run independently — running `01_logistic_draw.yaml` and `02_categorical_draw.yaml` gives each the seed 42, but the two runs don't share a stream. The seed just ensures each config is reproducible in isolation.

## The one thing that's new in 0.9.2 and will break old scripts

**All statistical functions are bare globals.** If you have an existing config that uses `hekate_stats.linear_predict(...)` or `hekate_stats.logistic_draw(...)`, it will now fail with:

```
attempt to index a nil value (global 'hekate_stats')
```

Remove the `hekate_stats.` prefix from any such call: `hekate_stats.normal_draw(0, 1)` → `normal_draw(0, 1)`. The function arguments and behaviour are unchanged.

The `hekate_rng.*` namespace (for `random`, `randomseed`, `seed`) and `hekate_lookup.*` (for `index`) still exist and are unaffected.

## Errors you might see (and what they mean)

The new functions validate their arguments. If you see any of these, the config is passing a bad value:

- `normal_draw: sd must be >= 0 (got -1.5)` — you passed a negative SD
- `uniform_draw: max must be > min (got min=5, max=5)` — you passed equal or reversed bounds
- `exponential_draw: rate must be > 0 (got 0)` — you passed zero or negative rate
- `bernoulli_draw: p must be in [0, 1] (got 1.2)` — the probability is outside [0, 1]
- `lognormal_draw: sigma must be >= 0 (got -0.5)` — you passed a negative sigma

These are deliberate errors, not bugs. Fix the input value.

If you see `column 'X' not found in CSV` during pre-flight validation, it means a script reads `person.X` where `X` isn't in the input CSV. For columns that a model *creates* (rather than reads from the CSV), add a comment anywhere in the script:

```lua
-- hekate:allow-columns X
```

Then the validator will skip that column.

