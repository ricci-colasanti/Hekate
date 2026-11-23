# Hekate LLM Prompt Template

Copy and paste this entire message into your LLM (ChatGPT, Claude, etc.) to get help with Hekate models.

---

## Context: Hekate Microsimulation Engine

Hekate is a self-contained demographic microsimulation engine inspired by NetLogo but built for large-scale populations. Models are defined in YAML configuration files with embedded Lua scripts.

**Core Concepts:**
- **Population**: CSV file with one row per person
- **Models**: Lua scripts that transform the population each iteration
- **YAML**: Configuration file that ties everything together
- **Streaming**: Process area-by-area for populations of any size
- **Reproducibility**: Same seed → same results, always (within a mode)
- **Validation**: Three-stage pre-flight checks catch errors before the run starts

**Key Philosophy:** Models are data, not code. Change your model by editing YAML, not recompiling.

**Data Structure:**
- Population is a list of tables (each person is a table)
- Each person has key-value pairs from the CSV columns
- Example: `person.age`, `person.sex`, `person.alive`, `person.area_id`

**Lua Script Structure:**
```lua
-- Model: runs each iteration to transform the population
function transition(population, params)
  for _, person in ipairs(population) do
    -- Your logic here
  end
  return population
end
```

---

## ⚠️ IMPORTANT: All statistical functions are bare globals

As of Hekate **0.9.2**, every statistical function is a **bare global**. Write `normal_draw(0, 1)`, **not** `hekate_stats.normal_draw(0, 1)`.

**The `hekate_stats` table no longer exists.** Any script using the old prefix will fail with:

```
attempt to index a nil value (global 'hekate_stats')
```

The namespace split is:

| What | How to call it |
|------|----------------|
| Statistical functions (`*_predict`, `*_draw`, distribution draws) | **Bare globals**, no prefix: `linear_predict(...)`, `normal_draw(...)` |
| The raw RNG (`random`, `randomseed`, `seed`) | `hekate_rng.random()`, `hekate_rng.randomseed(n)`, `hekate_rng.seed()` |
| The lookup builder | `hekate_lookup.index(rows, col1, col2, ...)` |

`math.random` and `math.randomseed` are still overridden to use Hekate's deterministic generator, so `math.random()` and `hekate_rng.random()` give identical values.

**When generating code for a user:** always emit bare globals. If a user shows you a script using `hekate_stats.X`, tell them to remove the prefix.

---

## Built-in Hekate Statistical Functions

Hekate provides **14 statistical functions** for building regression-based and distribution-based models. All are bare globals.

### Prediction functions (deterministic, no randomness)

| Function | Returns | Use Case |
|----------|---------|----------|
| `linear_predict(intercept, coef1, var1, ...)` | Float | Continuous outcomes (income) |
| `linear_predict_default(intercept, global_default, coef1, var1, default1, ...)` | Float | Continuous with nil handling |
| `logistic_predict(intercept, coef1, var1, ...)` | Float (0-1) | Binary probability |
| `poisson_predict(intercept, coef1, var1, ...)` | Float (≥0) | Expected count |
| `softmax_predict(scoreTable1, scoreTable2, ...)` | Table | Multi-category probabilities |

### Action draws (regression + sample in one call)

| Function | Returns | Use Case |
|----------|---------|----------|
| `logistic_draw(intercept, coef1, var1, ...)` | Boolean | Binary outcome whose probability comes from a regression |
| `categorical_draw(p1, p2, ..., pN)` | Integer (1..N) | Multi-category sample; probabilities auto-normalised |
| `poisson_draw(intercept, coef1, var1, ...)` | Integer (≥0) | Count whose mean comes from a regression |

### Distribution draws (sample from a named distribution)

| Function | Returns | Invalid input (Lua error) |
|----------|---------|---------------------------|
| `normal_draw(mean, sd)` | Float | `sd < 0` |
| `standard_normal_draw()` | Float | — (no arguments) |
| `uniform_draw(min, max)` | Float in [min, max) | `max <= min` |
| `exponential_draw(rate)` | Float (≥0), mean 1/rate | `rate <= 0` |
| `bernoulli_draw(p)` | Boolean | `p < 0` or `p > 1` |
| `lognormal_draw(mu, sigma)` | Float (>0) | `sigma < 0` |

### How to choose between the similar ones

| If you want... | Use |
|----------------|-----|
| A regression prediction only (no randomness) | `logistic_predict(...)` / `poisson_predict(...)` |
| A regression prediction + immediate yes/no decision | `logistic_draw(...)` |
| A yes/no decision where you already have `p` in hand | `bernoulli_draw(p)` |
| A regression prediction + count sample | `poisson_draw(...)` |
| A normal with fixed mean and sd | `normal_draw(mean, sd)` |
| A normal where mean/sd vary per person | `mean + sd * standard_normal_draw()` |
| A positive, right-skewed quantity (income, wealth) | `lognormal_draw(mu, sigma)` — `mu`/`sigma` are the log's parameters, `median = exp(mu)` |
| A continuous value with no preferred value in a range | `uniform_draw(min, max)` |
| A waiting time to the next event | `exponential_draw(rate)` — mean = 1/rate |

**Key distinction:** `normal_draw(m, s)` is equivalent to `m + s * standard_normal_draw()`. `lognormal_draw(mu, sigma)` is `exp(N(mu, sigma²))` — its arguments are the log's parameters, not the value's.

### Convention

- **`*_predict`** — computes a value or probability (deterministic).
- **`*_draw`** — samples a random outcome (uses the VM's dedicated RNG).
- All draw functions **validate their arguments** and raise a Lua error naming the parameter and value if invalid. The error is deliberate, not a bug.

---

## Lookup Tables: `hekate_lookup.index`

When your parameters live in a CSV rather than being hard-coded, use `hekate_lookup.index` to build a nested keyed index in one call.

```lua
local by_key = hekate_lookup.index(params.datafile, "sex", "alive")
local rule = by_key[person.sex] and by_key[person.sex][person.alive]
-- rule.value, rule.sd, rule.<any column in the CSV>
```

- `params.datafile` is the loaded rows of the model's `datafile:`. It's only populated if the model declares a `datafile:` in YAML.
- Each column name adds one level of nesting, outer-to-inner.
- The leaf is **the whole row** (a reference to the same table in `params.datafile`).
- Keys are coerced to strings on both build and read, so `by_key["M"]["true"]`, `by_key["M"][true]`, and `by_key["M"]["True"]` all match the same slot. This eliminates cross-file type mismatches.
- Duplicate keys raise a Lua error naming the key and column set. Missing key columns raise a Lua error listing the columns the row *does* have.

**The lookup + draw pattern** (per-group means and SDs):

```yaml
- name: "income_from_lookup"
  datafile: "lookup_rules.csv"   # header: sex,alive,value,sd
  parameters:
    script: |
      function transition(population, params)
        local by_key = hekate_lookup.index(params.datafile, "sex", "alive")
        for _, person in ipairs(population) do
          local rule = by_key[person.sex] and by_key[person.sex][person.alive]
          if rule then
            person.income = normal_draw(rule.value, rule.sd)
          end
        end
        return population
      end
```

This is the standard pattern for income by sex, mortality by age-band, etc. It's much cleaner than a hand-rolled loop and catches duplicate keys automatically.

---

## Built-in Lua Functions

- `math.random()` - random number in (0, 1) (automatically backed by Hekate's RNG)
- `math.random(n)` - random integer in [1, n]
- `math.random(min, max)` - random integer in [min, max]
- `math.floor(x)` - round down
- `math.ceil(x)` - round up
- `math.log(x)` - natural logarithm (useful for `lognormal_draw`'s `mu`)
- `tonumber(x)` - convert string to number (often needed for CSV data)
- `tostring(x)` - convert to string
- `string.format("%.4f", x)` - format numbers with precision
- `table.insert(t, x)` - append to a list
- `log(msg)` - write `[Lua] <msg>` to Hekate's log

---

## YAML Structure

```yaml
simulation:
  iterations: 10
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"
  streaming_mode: false            # Set to true for large populations
  area_column: "area_id"           # Required if streaming_mode is true
  strict_mode: false               # Set to true to treat warnings as errors
  skip_validation: false           # Not recommended
  data_dir: "data"                 # Base for model datafile paths
  output_every_iteration: false    # Write output_N.csv per iteration (BULK ONLY)

models:
  - name: "model_name"
    type: "lua_model"
    priority: 1                    # Lower numbers run first
    enabled: true
    description: "What this model does"
    datafile: "lookup.csv"         # Optional; resolved against data_dir
    parameters:
      rate: 0.05                   # Free-form; available as params.rate
      script: |
        function transition(population, params)
          -- Your model logic here
          return population
        end
```

**Two path conventions that are different on purpose:**

| Key | Resolved against | Example |
|-----|------------------|---------|
| `population_file` | **The process CWD** | `population_file: "data/population.csv"` |
| `datafile` | **`simulation.data_dir`** | `datafile: "lookup.csv"` (with `data_dir: "data"`) |

If you set `data_dir: "data"` and write `datafile: "data/foo.csv"`, Hekate looks for `data/data/foo.csv` and fails. Tell the user this if they report a missing datafile.

**`output_every_iteration` is bulk-mode only.** In streaming mode it's ignored; streaming writes and deletes `year_N.csv` working files itself.

---

## ⭐ Reproducibility Guarantee

Hekate guarantees **scientifically reproducible results**. Given the same `random_seed` and the same input, Hekate will produce **byte-for-byte identical output** every time — *in the same execution mode*.

### The seed formulas

**Bulk mode:**
```
seed = random_seed + iteration × 1000 + model_index
```

**Streaming mode:**
```
area_seed  = random_seed + year × 100000 + area_count
model_seed = area_seed + model_index
```

`area_count` is the 0-based position of the area in the sorted CSV, not the area's identifier. Reordering the CSV changes the results.

This means:
- **No dependency on system time** or global random state
- **Independent randomness** between different models
- **Fully reproducible** results across runs, machines, and time (within a mode)

> **⚠️ Bulk and streaming produce different results for the same config.** The seed formulas differ, so switching modes changes the numbers. Both are reproducible; they are not interchangeable. Mention this if a user asks why changing `streaming_mode` changed their output.

### The Reproducibility Promise

```bash
# Run 1
./hekate config.yaml
cp output.csv run1.csv

# Run 2 (days later, on a different machine)
./hekate config.yaml
cp output.csv run2.csv

# Result:
diff run1.csv run2.csv
# (no output - files are identical)
```

### Requirements for Reproducibility

| Requirement | Why |
|-------------|-----|
| **Set `random_seed` in config.yaml** | Provides the base for all derived seeds |
| **Use `math.random()` or any `*_draw()`** | Both are backed by the VM's dedicated RNG |
| **Avoid external randomness** | No `os.time()`, network calls, or filesystem state in models |
| **Never call `math.randomseed()`** | Hekate handles seeding automatically |
| **Use the same Hekate version** | Future versions may change behaviour |
| **Use the same execution mode** | Bulk and streaming derive seeds differently |

### Important Note for Lua Scripts

**`math.random()` works reliably in Hekate** — it is automatically backed by the VM's dedicated RNG. You do **not** need to call `math.randomseed()` yourself.

```lua
-- ✅ CORRECT - Just use math.random() or a *_draw function
if bernoulli_draw(0.05) then
  person.alive = false
end

-- ❌ WRONG - Don't seed manually!
math.randomseed(42)
if math.random() < 0.05 then
  person.alive = false
end
```

---

## 🔍 Pre-Flight Validation

Hekate validates your configuration **before** the simulation starts. This catches three classes of errors in milliseconds:

### Stage 1: Lua Syntax

Compiles every script in a throwaway VM. Catches:
- Missing `end` keywords
- Unbalanced brackets
- Typos in Lua syntax
- Missing `transition` function
- Common mistakes: `#` used as a comment, `math.randomseed()` calls, reading CSVs with `io.lines()` when a `datafile:` would do

### Stage 2: Datafile Existence

For every enabled model with a `datafile:`, resolves the path against `data_dir` and checks the file exists and is a regular file. Rejects absolute paths and `..` escapes when `data_dir` is set. On failure, prints the resolved absolute path and a "did you mean" suggestion based on files actually present in `data_dir`.

### Stage 3: Column Names

Scans scripts for `person.COLUMN` and `person["COLUMN"]` references and checks them against the CSV header. Catches:
- Typos (`person.agge` → suggests `person.age`)
- Columns that don't exist
- Case sensitivity issues

When no close match is found, it prints the full list of available columns.

### What This Means for You

- **Fast feedback**: fix errors in seconds, not hours
- **Fuzzy suggestions**: the validator suggests the closest column name
- **Fail-fast**: aborts before wasting compute time
- **Detailed logs**: writes `hekate_validation.log` on failure

### Example Output

When you have a typo:

```
❌ mortality                      (priority: 2)
     └─ column 'agge' not found (did you mean 'age'?)
```

Fix it, re-run, and it works. Without this, you'd only find out hours into the run.

### Suppressing false positives

If a column is added by an earlier model in the pipeline, the validator will flag it as missing. Add this comment anywhere in the script:

```lua
-- hekate:allow-columns income, tax_bracket
```

The validator skips those columns for that model only.

---

## Example Models

### 1. Aging Model (Priority 1 - MUST RUN FIRST)

```lua
function transition(population, params)
  for _, person in ipairs(population) do
    -- Handle both string and boolean "alive" from CSV
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      local age = tonumber(person.age) or 18
      person.age = age + 1
    end
  end
  return population
end
```

### 2. Income Model (Linear Regression + Normal Draw)

Predict a mean from a regression, then sample around it. This is the canonical way to model a continuous outcome with realistic variation.

```lua
function transition(population, params)
  local coefs = params.coefficients

  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      local age = tonumber(person.age) or 30

      local edu_score = 0
      if person.education == "tertiary" then
        edu_score = 3
      elseif person.education == "secondary" then
        edu_score = 2
      elseif person.education == "primary" then
        edu_score = 1
      end

      local sex_score = 0
      if person.sex == "F" then
        sex_score = 1
      end

      local mean = linear_predict(
        coefs.intercept,
        coefs.age, age,
        coefs.education, edu_score,
        coefs.sex, sex_score
      )

      person.income = normal_draw(mean, coefs.sd)
    end
  end
  return population
end
```

**YAML Parameters:**
```yaml
parameters:
  coefficients:
    intercept: 15000
    age: 500
    education: 5000
    sex: -3000
    sd: 5000
```

### 3. Health Risk Model (Logistic Regression, prediction only)

Compute a probability and record it as a column. Useful when you want the probability itself, not a decision.

```lua
function transition(population, params)
  local coefs = params.coefficients

  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      local age = tonumber(person.age) or 40
      local bmi = tonumber(person.bmi) or 25

      local bmi_score = 0
      if bmi > 30 then
        bmi_score = 2
      elseif bmi > 25 then
        bmi_score = 1
      end

      local smoker_score = 0
      if person.smoker == "true" or person.smoker == true then
        smoker_score = 1
      end

      person.health_risk = logistic_predict(
        coefs.intercept,
        coefs.age, age / 10,
        coefs.bmi, bmi_score,
        coefs.smoker, smoker_score
      )
    end
  end
  return population
end
```

### 4. Mortality Model (Logistic Draw)

Logistic regression + immediate decision in one call.

```lua
function transition(population, params)
  local coefs = params.coefficients

  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      local age = tonumber(person.age) or 40
      local smoker_score = 0
      if person.smoker == "true" or person.smoker == true then
        smoker_score = 1
      end

      if logistic_draw(
        coefs.intercept,
        coefs.age, age,
        coefs.smoker, smoker_score
      ) then
        person.alive = false
      end
    end
  end
  return population
end
```

**YAML Parameters:**
```yaml
parameters:
  coefficients:
    intercept: -6.0
    age: 0.09
    smoker: 1.5
```

### 5. Fertility Model (Poisson Draw)

Counts of children per woman per year.

```lua
function transition(population, params)
  local coefs = params.coefficients

  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive and person.sex == "F" then
      local age = tonumber(person.age) or 30
      if age >= 15 and age <= 50 then
        local married = 0
        if person.partner_id ~= nil and tonumber(person.partner_id) ~= 0 then
          married = 1
        end

        local new_children = poisson_draw(
          coefs.intercept,
          coefs.age, age,
          coefs.married, married
        )

        person.parity = (tonumber(person.parity) or 0) + new_children
      end
    end
  end
  return population
end
```

**YAML Parameters:**
```yaml
parameters:
  coefficients:
    intercept: -4.0
    age: 0.1
    married: 0.5
```

### 6. Education Model (Softmax + Categorical Draw)

Multi-category outcomes with probabilities that sum to 1.

```lua
function transition(population, params)
  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      local age = tonumber(person.age) or 0

      if age == 16 then
        local primary_score   = {0.5, 0.1, age}
        local secondary_score = {1.0, 0.3, age}
        local tertiary_score  = {0.2, 0.5, age}

        local probs = softmax_predict(
          primary_score,
          secondary_score,
          tertiary_score
        )

        local chosen = categorical_draw(probs[1], probs[2], probs[3])
        local levels = {"primary", "secondary", "tertiary"}
        person.education = levels[chosen]
      end
    end
  end
  return population
end
```

### 7. Migration Model (Bernoulli Draw with a Fixed Hazard)

When you already have `p` and just need the yes/no decision.

```lua
function transition(population, params)
  local rates = params.migration_rates
  local num_areas = params.num_areas

  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      local age = tonumber(person.age) or 30
      local prob = 0

      if age < 18 then
        prob = rates.child_0_17
      elseif age < 35 then
        prob = rates.adult_18_34
      elseif age < 65 then
        prob = rates.adult_35_64
      else
        prob = rates.elderly_65_plus
      end

      if bernoulli_draw(prob) then
        person.previous_area = person.area_id
        person.area_id = math.random(1, num_areas)
      end
    end
  end
  return population
end
```

**YAML Parameters:**
```yaml
parameters:
  migration_rates:
    child_0_17: 0.02
    adult_18_34: 0.08
    adult_35_64: 0.03
    elderly_65_plus: 0.01
  num_areas: 5
```

### 8. Income from Lookup (Lookup + Normal Draw)

Parameters come from a CSV, not the YAML. Use this whenever you have per-group means and SDs.

```lua
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
```

**YAML:**
```yaml
- name: "income_from_lookup"
  type: "lua_model"
  priority: 2
  enabled: true
  datafile: "lookup_rules.csv"   # header: sex,alive,value,sd
  parameters:
    script: |
      -- (script above)
```

**Lookup table (`lookup_rules.csv`):**
```csv
sex,alive,value,sd
M,true,30000,4000
M,false,0,0
F,true,26000,3500
F,false,0,0
```

### 9. Weight Model (Standard Normal Draw)

Use `standard_normal_draw()` when the mean or sd varies per person and you want to write the arithmetic yourself.

```lua
function transition(population, params)
  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      local age = tonumber(person.age) or 40
      -- Mean weight rises with age up to ~50, then falls slightly
      local mean = 70 + 0.2 * math.min(age, 50) - 0.05 * math.max(0, age - 50)
      local sd = 12
      person.weight = mean + sd * standard_normal_draw()
    end
  end
  return population
end
```

### 10. Waiting Time Model (Exponential Draw)

For "how long until the next event?" questions.

```lua
function transition(population, params)
  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      -- Mean 5 years between events -> rate = 0.2
      person.waiting_time_years = exponential_draw(0.2)
    end
  end
  return population
end
```

### 11. Wealth Model (Lognormal Draw)

For positive, right-skewed quantities. `mu` and `sigma` are the parameters of the **log**, so `median = exp(mu)`.

```lua
function transition(population, params)
  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      -- Median wealth 50,000; heavy right tail
      local mu = math.log(50000)
      person.wealth = lognormal_draw(mu, 0.8)
    end
  end
  return population
end
```

---

## YAML Configuration Templates

### Basic Template

```yaml
simulation:
  iterations: 5
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42              # REQUIRED for reproducibility
  verbose: true
  id_column: "person_id"
  streaming_mode: false

models:
  - name: "aging_model"
    type: "lua_model"
    priority: 1
    enabled: true
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            local alive = false
            if person.alive == true or person.alive == "true" then
              alive = true
            end
            if alive then
              person.age = (tonumber(person.age) or 18) + 1
            end
          end
          return population
        end
```

### Streaming Mode Template (For Large Populations)

```yaml
simulation:
  iterations: 5
  population_file: "population.csv"    # Must be sorted by area_column
  output_file: "output.csv"
  random_seed: 42
  verbose: false
  id_column: "person_id"
  streaming_mode: true
  area_column: "area_id"

models:
  # Your models here (same as above)
```

**Important:** The CSV must be **sorted by `area_column`** for streaming mode. Hekate enforces this — an unsorted file aborts with a clear error.

```bash
# Sort the CSV by area_id
(head -n1 population.csv && tail -n+2 population.csv | sort -t, -k4 -n) > population_sorted.csv
```

**Streaming caveat:** `output_every_iteration` is ignored in streaming mode. Streaming writes and deletes `year_N.csv` working files itself.

### Full Model Template (With All Features)

```yaml
simulation:
  iterations: 5
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"
  streaming_mode: false
  strict_mode: false
  data_dir: "data"
  output_every_iteration: false

models:
  # Model 1: Aging (Priority 1 - ALWAYS FIRST)
  - name: "aging"
    type: "lua_model"
    priority: 1
    enabled: true
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive == true or person.alive == "true" then
              person.age = (tonumber(person.age) or 0) + 1
            end
          end
          return population
        end

  # Model 2: Income (Linear regression + normal draw)
  - name: "income"
    type: "lua_model"
    priority: 2
    enabled: true
    parameters:
      coefficients:
        intercept: 15000
        age: 300
        education: 5000
        sex: -3000
        sd: 4000
      script: |
        function transition(population, params)
          local coefs = params.coefficients
          for _, person in ipairs(population) do
            if person.alive == true or person.alive == "true" then
              local age = tonumber(person.age) or 30
              local edu_score = 0
              if person.education == "tertiary" then edu_score = 3
              elseif person.education == "secondary" then edu_score = 2
              elseif person.education == "primary" then edu_score = 1
              end
              local sex_score = 0
              if person.sex == "F" then sex_score = 1 end

              local mean = linear_predict(
                coefs.intercept,
                coefs.age, age,
                coefs.education, edu_score,
                coefs.sex, sex_score
              )
              person.income = normal_draw(mean, coefs.sd)
            end
          end
          return population
        end

  # Model 3: Mortality (Logistic draw)
  - name: "mortality"
    type: "lua_model"
    priority: 3
    enabled: true
    parameters:
      coefficients:
        intercept: -6.0
        age: 0.09
        smoker: 1.5
      script: |
        function transition(population, params)
          local coefs = params.coefficients
          for _, person in ipairs(population) do
            if person.alive == true or person.alive == "true" then
              local age = tonumber(person.age) or 40
              local smoker = 0
              if person.smoker == "true" or person.smoker == true then
                smoker = 1
              end

              if logistic_draw(
                coefs.intercept,
                coefs.age, age,
                coefs.smoker, smoker
              ) then
                person.alive = false
              end
            end
          end
          return population
        end

  # Model 4: Fertility (Poisson draw)
  - name: "fertility"
    type: "lua_model"
    priority: 4
    enabled: true
    parameters:
      coefficients:
        intercept: -4.0
        age: 0.1
        married: 0.5
      script: |
        function transition(population, params)
          local coefs = params.coefficients
          for _, person in ipairs(population) do
            if (person.alive == true or person.alive == "true") and person.sex == "F" then
              local age = tonumber(person.age) or 30
              if age >= 15 and age <= 50 then
                local married = 0
                if person.partner_id ~= nil and tonumber(person.partner_id) ~= 0 then
                  married = 1
                end

                local new_children = poisson_draw(
                  coefs.intercept,
                  coefs.age, age,
                  coefs.married, married
                )

                person.parity = (tonumber(person.parity) or 0) + new_children
              end
            end
          end
          return population
        end
```

---

## Best Practices & Common Pitfalls

### 1. Always Use tonumber() for CSV Data

CSV values arrive as strings unless Hekate has inferred the column as numeric (in which case they're already numbers). `tonumber()` is safe either way and is the idiomatic guard:

```lua
local age = tonumber(person.age) or 0
```

### 2. Handle Both String and Boolean for "alive"

CSV booleans come in as booleans once Hekate has parsed them, but scripts sometimes set them as strings. The safe pattern is:

```lua
local alive = false
if person.alive == true or person.alive == "true" then
  alive = true
end
```

### 3. Always Check "alive" Before Processing

```lua
if alive then
  -- Process this person
end
```

### 4. Models Run in Priority Order

- Lower numbers run first.
- Aging should **always** be priority 1.
- Models that depend on others should have higher priority numbers.

### 5. Streaming Mode Requirements

- CSV must be sorted by the `area_column` (Hekate enforces this).
- `area_column` must be specified in YAML.
- Memory usage stays constant (~20-50MB) regardless of population size.
- `output_every_iteration` is ignored.

### 6. Lua Comments Use `--` NOT `#`

```lua
-- This is a comment  -- ✅ Correct
-- # This is NOT a comment in Lua  -- ❌ WRONG!
```

### 7. Use math.random() Correctly

```lua
math.random()        -- Returns 0.0 to 1.0
math.random(10)      -- Returns 1 to 10
math.random(1, 100)  -- Returns 1 to 100
```

### 8. NEVER Call math.randomseed() Yourself

Hekate handles seeding automatically. Calling it yourself may break reproducibility.

### 9. ALWAYS Set random_seed for Reproducibility

Even if you don't need reproducibility right now, set a seed:

```yaml
simulation:
  random_seed: 42
```

### 10. Use Column Names That Exist in Your CSV

Hekate validates `person.COLUMN` references against the CSV header. If you get an error like:

```
column 'agge' not found (did you mean 'age'?)
```

Fix the typo. The pre-flight validator saves you from hours of wasted runs.

### 11. Watch Out for Extreme Coefficients

Logistic and Poisson regressions use `exp()`, which can overflow:

```lua
-- ❌ BAD: exp(100) overflows
logistic_predict(100, 1.0, age)

-- ✅ GOOD: intercepts are typically -10 to +10
logistic_predict(-3.5, 0.1, age)
```

If you see `inf` or `nan` in your output, you've likely overflowed.

### 12. Read the Argument-Validation Errors on the Draw Functions

The draw functions reject invalid arguments with a named error:

- `normal_draw: sd must be >= 0 (got -1.5)`
- `uniform_draw: max must be > min (got min=5, max=5)`
- `exponential_draw: rate must be > 0 (got 0)`
- `bernoulli_draw: p must be in [0, 1] (got 1.2)`
- `lognormal_draw: sigma must be >= 0 (got -0.5)`

These are deliberate, not bugs. Fix the input value.

---

## How to Use This Template

1. **Copy the entire template above** into your LLM (ChatGPT, Claude, etc.)
2. **Describe what you want to build**:
   - "I want to build a model that does [X]"
   - "My population has columns [A, B, C]"
3. **The LLM will generate** the YAML and Lua code for you
4. **Test it** by running Hekate with your new configuration
5. **Iterate** — ask follow-up questions to refine your model

---

## Example Questions to Ask the LLM

### Basic Models
1. "I want to add a model where people get married. Women marry at age 20-30, men at 22-35. How do I write this?"
2. "I need a fertility model where fertility rates vary by age: 15-19: 2%, 20-24: 8%, 25-29: 10%, 30-34: 8%, 35-39: 4%, 40-44: 1%"
3. "How do I create an education model where children progress through primary, secondary, and tertiary education?"

### Statistical Models
4. "I want to predict income using linear regression with age and education, with realistic variation. Show me how to combine `linear_predict()` and `normal_draw()`."
5. "How do I calculate health risk using logistic regression with BMI and smoking status?"
6. "I need a Poisson regression model for fertility with age and marital status"
7. "How do I use `softmax_predict` and `categorical_draw` to model education transitions?"
8. "What's the difference between `logistic_predict`, `logistic_draw`, and `bernoulli_draw`? When should I use each?"

### Distribution Draws
9. "I have a median income and want a realistic right-skewed distribution. Should I use `normal_draw` or `lognormal_draw`, and how do I parameterise it?"
10. "I need to model time-to-next-event with a mean of 4 years. How do I use `exponential_draw`?"
11. "How do I model a continuous quantity whose mean varies per person, using `standard_normal_draw`?"
12. "I want to add a small uniform jitter around a value. How do I use `uniform_draw`?"

### Lookup Tables
13. "I have a CSV of mean and SD of income by sex and education. How do I use `hekate_lookup.index` with `normal_draw` to draw individual incomes?"
14. "How do I use `hekate_lookup.index` to look up a parameter by two columns?"

### Complex Models
15. "I want migration to depend on distance between areas. How do I implement this?"
16. "I need a household formation model where young adults leave their parents' household."
17. "How do I model income where it depends on age, education, and sex, with a heavy right tail?"

### Large-Scale Models
18. "How do I set up streaming mode for a population of 10 million people?"
19. "What's the best way to optimize my model for large populations?"

### Reproducibility
20. "How do I ensure my model produces reproducible results?"
21. "Why did my results change when I switched from bulk to streaming mode?"
22. "What could cause my results to be non-reproducible?"

### Validation
23. "I got an error: 'column 'agge' not found (did you mean 'age'?)'. How do I fix it?"
24. "What is `strict_mode` and when should I use it?"

### Advanced
25. "How do I add a new column to each person when they get married?"
26. "I want to track the number of children each woman has (parity)."
27. "How do I model international migration where people leave permanently?"
28. "I'm migrating from 0.9.1. My script uses `hekate_stats.linear_predict`. What do I change?"

---

## Example: Full Conversation

**User:**
"I want to build a model where people can move between areas. The probability of moving depends on age. Young adults (18-34) are most mobile with 8% chance, children (0-17) have 2%, middle-aged (35-64) have 3%, and elderly (65+) have 1%. There are 5 areas."

**LLM Response:**

The LLM will generate this Lua script:

```lua
function transition(population, params)
  local rates = params.migration_rates
  local num_areas = params.num_areas

  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end

    if alive then
      local age = tonumber(person.age) or 30
      local prob = 0

      if age < 18 then
        prob = rates.child_0_17
      elseif age < 35 then
        prob = rates.adult_18_34
      elseif age < 65 then
        prob = rates.adult_35_64
      else
        prob = rates.elderly_65_plus
      end

      if bernoulli_draw(prob) then
        person.previous_area = person.area_id
        person.area_id = math.random(1, num_areas)
      end
    end
  end
  return population
end
```

Then the LLM will show you exactly how to add it to your `config.yaml`:

```yaml
models:
  - name: "migration"
    type: "lua_model"
    priority: 3
    enabled: true
    description: "Age-based migration between areas"
    parameters:
      migration_rates:
        child_0_17: 0.02
        adult_18_34: 0.08
        adult_35_64: 0.03
        elderly_65_plus: 0.01
      num_areas: 5
      script: |
        function transition(population, params)
          local rates = params.migration_rates
          local num_areas = params.num_areas

          for _, person in ipairs(population) do
            local alive = false
            if person.alive == true or person.alive == "true" then
              alive = true
            end

            if alive then
              local age = tonumber(person.age) or 30
              local prob = 0

              if age < 18 then
                prob = rates.child_0_17
              elseif age < 35 then
                prob = rates.adult_18_34
              elseif age < 65 then
                prob = rates.adult_35_64
              else
                prob = rates.elderly_65_plus
              end

              if bernoulli_draw(prob) then
                person.previous_area = person.area_id
                person.area_id = math.random(1, num_areas)
              end
            end
          end
          return population
        end
```

**Reproducibility Note:** Remember to set `random_seed: 42` (or any integer) in the `simulation:` section. This migration model uses `bernoulli_draw` and `math.random`, both of which are backed by Hekate's dedicated RNG for reproducibility.

---

## Common Reproducibility Pitfalls

Here are things that can break reproducibility. If your results vary between runs, check for these:

| Pitfall | Why It Breaks Reproducibility | Fix |
|---------|-------------------------------|-----|
| **Missing `random_seed`** | Auto-seeded with time | Always set `random_seed` |
| **Calling `math.randomseed()`** | Overrides Hekate's seeding | Remove all `math.randomseed()` calls |
| **Using `os.time()`** | Depends on current time | Remove it; if you need the iteration number, pass it via a model parameter |
| **Using `os.clock()`** | Depends on CPU time | Avoid |
| **Reading external files** | May change between runs | Use `datafile:` + `hekate_lookup.index` instead |
| **Using `pairs()` on unordered tables** | Iteration order is non-deterministic | Use `ipairs()` for ordered lists |
| **Switching between bulk and streaming** | Seed formulas differ | Pick one mode and stay in it |
| **Reordering the input CSV in streaming mode** | `area_count` is positional | Keep the CSV sort stable |
| **Different Hekate versions** | Behaviour may change | Pin your version |

### Checking Your Model for Reproducibility Issues

```bash
# Run twice
./hekate config.yaml
mv output.csv run1.csv
./hekate config.yaml
mv output.csv run2.csv

# Compare
if diff -q run1.csv run2.csv > /dev/null; then
    echo "✅ Reproducible"
else
    echo "❌ NOT reproducible - check the pitfalls above"
    diff run1.csv run2.csv
fi
```

---

## Streaming Mode and Reproducibility

**Streaming mode is fully reproducible, but it does not produce the same numbers as bulk mode for the same config.**

**Bulk mode seed:**
```
seed = random_seed + iteration × 1000 + model_index
```

**Streaming mode seed:**
```
area_seed  = random_seed + year × 100000 + area_count
model_seed = area_seed + model_index
```

This means:
- Within each mode, results are byte-identical across runs.
- Across modes, results differ because the seed sequences are different.
- In streaming mode, `area_count` is the 0-based **position** of the area, not its identifier. Reordering the CSV changes the results.

**Why the modes differ:** Streaming mode processes one area at a time, so it gives each area its own seed. Bulk mode gives each model one seed for the whole population. Both are defensible designs; they just aren't interchangeable. If a user switches modes and sees different numbers, this is why.

**Example:**
```yaml
simulation:
  streaming_mode: true
  area_column: "area_id"
  random_seed: 42
```

Both bulk and streaming runs are reproducible, but you should not expect them to agree with each other.

---

## Pre-Flight Validation in Detail

Hekate validates your configuration in **three stages** before the simulation starts.

### Stage 1: Lua Syntax Validation

- Compiles every script in a throwaway VM.
- Verifies a `transition(population, params)` function exists.
- Warns about common mistakes: `#` used as a comment, `math.randomseed()` calls, reading CSVs with `io.lines()`/`io.open()` when a `datafile:` would do.

### Stage 2: Datafile Existence Validation

- For every enabled model with a `datafile:`, resolves the path against `data_dir`.
- Rejects absolute paths and `..` escapes when `data_dir` is set.
- Checks the file exists and is a regular file.
- On failure, prints the user-written path, the resolved absolute path, and a "did you mean" suggestion.

### Stage 3: Column Name Validation

- Reads the CSV header.
- Scans scripts for `person.COLUMN` and `person["COLUMN"]` references.
- Flags columns that don't exist in the CSV.
- Suggests the closest match using fuzzy matching; if nothing is close, prints the full list of available columns.

### What the Output Looks Like

**Successful validation:**

```
════════════════════════════════════════════════════════
  Lua Script Pre-Flight Validation
════════════════════════════════════════════════════════

  ✅ aging                          (priority: 1)
  ✅ mortality                      (priority: 2)
  ✅ fertility                      (priority: 3)

════════════════════════════════════════════════════════
  ✅ All 3 models validated successfully
════════════════════════════════════════════════════════
```

**With warnings:**

```
  ⚠️  mortality                      (priority: 2) — 1 warning(s)
       └─ script calls math.randomseed() — Hekate seeds the RNG per-model automatically; manual seeding will override that seed.
```

**With errors:**

```
  ❌ mortality                      (priority: 2)
       └─ column 'agge' not found (did you mean 'age'?)
```

### Skipping Validation (Not Recommended)

```yaml
simulation:
  skip_validation: true
```

This disables all three stages. **Not recommended** — you'll only discover errors when they crash the run at runtime.

### Strict Mode

To treat warnings as errors (useful in CI):

```yaml
simulation:
  strict_mode: true
```

With strict mode, any warning becomes a blocking error.

---

## Tutorials

New to Hekate? Start with the tutorials:

| Tutorial | Topic | Level |
|----------|-------|-------|
| [Tutorial 1](../tutorials/tutorial1_aging.md) | Building an Aging Model | ⭐ Beginner |
| [Tutorial 2](../tutorials/tutorial2_yaml_mortality.md) | YAML and Adding Mortality | ⭐⭐ Intermediate |
| [Tutorial 3](../tutorials/tutorial3_fertility.md) | Adding Fertility | ⭐⭐⭐ Advanced |
| [Tutorial 4](../tutorials/tutorial_4_regression.md) | Regression and Statistical Functions | ⭐⭐⭐ Advanced |
| [Tutorial 5](../tutorials/tutorial_5_streaming.md) | Large-Scale Simulations with Streaming Mode | ⭐⭐⭐⭐ Advanced |
| [Tutorial 6](../tutorials/tutorial_6_stats.md) | Using Hekate's Statistical Toolkit | ⭐⭐⭐⭐ Advanced |
| [Tutorial 7](../tutorials/tutorial7_draws.md) | The Draw Functions | ⭐⭐⭐⭐ Advanced |