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
- **Reproducibility**: Same seed → same results, always
- **Validation**: Pre-flight checks catch errors before the run starts

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

**Built-in Hekate Statistical Functions:**

Hekate provides 8 statistical functions for building regression-based models:

| Function | Type | Returns | Use Case |
|----------|------|---------|----------|
| `hekate_stats.linear_predict(...)` | Predict | Float | Continuous outcomes (income) |
| `hekate_stats.linear_predict_default(...)` | Predict | Float | Continuous with nil handling |
| `hekate_stats.logistic_predict(...)` | Predict | Float (0-1) | Binary probability |
| `hekate_stats.poisson_predict(...)` | Predict | Float (≥0) | Expected count |
| `hekate_stats.softmax_predict(...)` | Predict | Table | Multi-category probabilities |
| `hekate_stats.logistic_draw(...)` | Draw | Boolean | Binary outcome + sample |
| `hekate_stats.categorical_draw(...)` | Draw | Integer (1..N) | Multi-category sample |
| `hekate_stats.poisson_draw(...)` | Draw | Integer (≥0) | Count sample |

**Convention:**
- **`*_predict`** — computes a value or probability (deterministic)
- **`*_draw`** — samples a random outcome (uses the VM's dedicated RNG)

**Built-in Lua Functions:**
- `math.random()` - random number between 0 and 1 (automatically seeded by Hekate)
- `math.random(n)` - random integer between 1 and n
- `math.random(min, max)` - random integer between min and max
- `math.floor(x)` - round down
- `math.ceil(x)` - round up
- `tonumber(x)` - convert string to number (critical for CSV data!)
- `tostring(x)` - convert to string
- `string.format("%.4f", x)` - format numbers with precision

**YAML Structure:**
```yaml
simulation:
  iterations: 10
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"
  streaming_mode: false      # Set to true for large populations
  area_column: "area_id"     # Required if streaming_mode is true
  strict_mode: false         # Set to true to treat warnings as errors
  skip_validation: false     # Not recommended

models:
  - name: "model_name"
    type: "lua_model"
    priority: 1              # Lower numbers run first
    enabled: true
    description: "What this model does"
    parameters:
      rate: 0.05
      script: |
        function transition(population, params)
          -- Your model logic here
          return population
        end
```

---

## ⭐ Reproducibility Guarantee

Hekate guarantees **scientifically reproducible results**. Given the same `random_seed` and the same input, Hekate will produce **byte-for-byte identical output** every time, on every machine.

### How It Works

Hekate uses a **dedicated random number generator** for each Lua VM instance:

```
seed = random_seed + (iteration × 1000) + model_index
```

This means:
- **No dependency on system time** or global random state
- **Independent randomness** between different models
- **Fully reproducible** results across runs, machines, and time

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
| **Use `math.random()` or `hekate_stats.*_draw()`** | Automatically mapped to the VM's dedicated RNG |
| **Avoid external randomness** | No `os.time()`, network calls, or filesystem state in models |
| **Never call `math.randomseed()`** | Hekate handles seeding automatically |
| **Use the same version of Hekate** | Future versions may change behavior |

### Verifying Reproducibility

```bash
# Run the simulation 4 times
for i in 1 2 3 4; do
    ./hekate config.yaml
    cp output.csv run$i.csv
done

# Compare all runs
diff run1.csv run2.csv
diff run1.csv run3.csv
diff run1.csv run4.csv
# All should show no differences
```

### Important Note for Lua Scripts

**`math.random()` works reliably in Hekate** — it is automatically mapped to the VM's dedicated RNG. You do **not** need to call `math.randomseed()` yourself.

**Do NOT call `math.randomseed()` in your Lua scripts.** Hekate handles seeding automatically. If you call it, you may break reproducibility.

```lua
-- ✅ CORRECT - Just use math.random()
if math.random() < 0.05 then
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

Hekate validates your configuration **before** the simulation starts. This catches two classes of errors in milliseconds:

### 1. Lua Syntax Errors

Compiles every script in a throwaway VM. Catches:
- Missing `end` keywords
- Unbalanced brackets
- Typos in Lua syntax
- Missing `transition` function

### 2. Column Name Errors

Scans scripts for `person.COLUMN` references and checks them against the CSV header. Catches:
- Typos (`person.agge` → suggests `person.age`)
- Columns that don't exist
- Case sensitivity issues

### What This Means for You

- **Fast feedback**: fix errors in seconds, not hours
- **Fuzzy suggestions**: the validator suggests the closest column name
- **Fail-fast**: aborts before wasting compute time
- **Detailed logs**: writes `hekate_validation.log` on failure

### Example Output

When you have a typo, Hekate tells you:

```
❌ mortality                      (priority: 2)
     └─ column 'agge' not found (did you mean 'age'?)
```

Fix it, re-run, and it works. Without this, you'd only find out hours into the run.

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
      -- Always convert CSV strings to numbers!
      local age = tonumber(person.age) or 18
      person.age = age + 1
    end
  end
  return population
end
```

### 2. Income Model (Using Linear Regression)

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
      
      -- Convert categorical variables to numeric
      local edu_score = 0
      if person.education == "tertiary" then
        edu_score = 3
      elseif person.education == "secondary" then
        edu_score = 2
      elseif person.education == "primary" then
        edu_score = 1
      end
      
      local gender_score = 0
      if person.sex == "F" then
        gender_score = 1
      end
      
      -- Use built-in linear regression
      local income = hekate_stats.linear_predict(
        coefs.intercept,
        coefs.age, age,
        coefs.education, edu_score,
        coefs.gender, gender_score
      )
      
      -- Add realistic noise (uses reproducible RNG)
      local noise = 1 + (math.random() * 0.1 - 0.05)
      person.income = math.floor(income * noise + 0.5)
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
    age: 300
    education: 5000
    gender: -3000
```

### 3. Health Risk Model (Using Logistic Regression)

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
      
      -- BMI categories
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
      
      -- Compute risk probability
      local risk = hekate_stats.logistic_predict(
        coefs.intercept,
        coefs.age, age / 10,
        coefs.bmi, bmi_score,
        coefs.smoker, smoker_score
      )
      
      person.health_risk = math.floor(risk * 100 + 0.5)
    end
  end
  return population
end
```

### 4. Mortality Model (Using Logistic Draw)

The simplest form of mortality — logistic regression + immediate decision:

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
      
      -- Combined predict + sample in one call
      if hekate_stats.logistic_draw(
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

### 5. Fertility Model (Using Poisson Draw)

Counts of children per woman per year:

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
        -- Married indicator
        local married = 0
        if person.partner_id ~= nil and tonumber(person.partner_id) ~= 0 then
          married = 1
        end
        
        -- Sample number of children this year
        local new_children = hekate_stats.poisson_draw(
          coefs.intercept,
          coefs.age, age,
          coefs.married, married
        )
        
        -- Update parity
        local current_parity = tonumber(person.parity) or 0
        person.parity = current_parity + new_children
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

### 6. Education Model (Using Softmax + Categorical Draw)

Multi-category outcomes with probabilities that sum to 1:

```lua
function transition(population, params)
  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end
    
    if alive then
      local age = tonumber(person.age) or 0
      
      -- Education decision at age 16
      if age == 16 and person.education ~= "tertiary" then
        -- Scores for each category (higher = more likely)
        local primary_score   = {0.5, 0.1, age, -0.2, 0}
        local secondary_score = {1.0, 0.3, age,  0.5, 0}
        local tertiary_score  = {0.2, 0.5, age,  0.8, 0}
        
        -- Get probabilities via softmax
        local probs = hekate_stats.softmax_predict(
          primary_score,
          secondary_score,
          tertiary_score
        )
        
        -- probs is now a table like {0.15, 0.55, 0.30}
        
        -- Sample a category (returns 1, 2, or 3)
        local chosen = hekate_stats.categorical_draw(
          probs[1], probs[2], probs[3]
        )
        
        local education_levels = {"primary", "secondary", "tertiary"}
        person.education = education_levels[chosen]
      end
    end
  end
  return population
end
```

### 7. Migration Model (Age-Based)

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
      elseif age >= 18 and age < 35 then
        prob = rates.adult_18_34
      elseif age >= 35 and age < 65 then
        prob = rates.adult_35_64
      else
        prob = rates.elderly_65_plus
      end
      
      if math.random() < prob then
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
  random_seed: 42                       # REQUIRED for reproducibility
  verbose: false                        # Set to false for speed
  id_column: "person_id"
  streaming_mode: true                  # Enable streaming
  area_column: "area_id"                # Must be sorted in CSV

models:
  # Your models here (same as above)
```

**Important:** The CSV file must be **sorted by `area_column`** for streaming mode:

```bash
# Sort the CSV by area_id
(head -n1 population.csv && tail -n+2 population.csv | sort -t, -k4 -n) > population_sorted.csv
```

### Full Model Template (With All Features)

```yaml
simulation:
  iterations: 5
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42                   # REQUIRED for reproducibility
  verbose: true
  id_column: "person_id"
  streaming_mode: false
  strict_mode: false                # Set true for CI / strict warnings

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

  # Model 2: Income
  - name: "income"
    type: "lua_model"
    priority: 2
    enabled: true
    parameters:
      coefficients:
        intercept: 15000
        age: 300
        education: 5000
        gender: -3000
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
              local gender_score = 0
              if person.sex == "F" then gender_score = 1 end
              
              person.income = hekate_stats.linear_predict(
                coefs.intercept,
                coefs.age, age,
                coefs.education, edu_score,
                coefs.gender, gender_score
              )
            end
          end
          return population
        end

  # Model 3: Mortality
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
              
              if hekate_stats.logistic_draw(
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

  # Model 4: Fertility
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
                
                local new_children = hekate_stats.poisson_draw(
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

### 1. ALWAYS Use tonumber() for CSV Data

CSV values are strings. Always convert:

```lua
local age = tonumber(person.age) or 0  -- ✅ Correct
-- local age = person.age  -- ❌ WRONG! Will fail comparisons
```

### 2. Handle Both String and Boolean for "alive"

CSV booleans come as strings, but Lua scripts may set them as booleans:

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

### 4. Use goto continue for Safety Checks (Lua 5.2+)

```lua
if health_risk == 0 then
  person.mortality_risk = "0.0000"
  goto continue  -- Skip to end of loop
end
-- ... rest of logic ...
::continue::
```

### 5. Models Run in Priority Order

- Lower numbers run first
- Aging should ALWAYS be priority 1
- Models that depend on others should have higher priority numbers

### 6. Streaming Mode Requirements

- CSV must be sorted by the `area_column`
- `area_column` must be specified in YAML
- Memory usage stays constant (~20MB) regardless of population size

### 7. Lua Comments Use `--` NOT `#`

```lua
-- This is a comment  -- ✅ Correct
-- # This is NOT a comment in Lua  -- ❌ WRONG!
```

### 8. Use math.random() Correctly

```lua
math.random()        -- Returns 0.0 to 1.0
math.random(10)      -- Returns 1 to 10
math.random(1, 100)  -- Returns 1 to 100
```

### 9. NEVER Call math.randomseed() Yourself

Hekate handles seeding automatically. Calling it yourself may break reproducibility.

### 10. ALWAYS Set random_seed for Reproducibility

Even if you don't need reproducibility right now, set a seed:

```yaml
simulation:
  random_seed: 42  # Any integer - always set this!
```

### 11. Use Column Names That Exist in Your CSV

Hekate validates `person.COLUMN` references against the CSV header. If you get an error like:

```
column 'agge' not found (did you mean 'age'?)
```

Fix the typo. The pre-flight validator saves you from hours of wasted runs.

### 12. Watch Out for Extreme Coefficients

Logistic and Poisson regressions use `exp()`, which can overflow:

```lua
-- ❌ BAD: exp(100) overflows
hekate_stats.logistic_predict(100, 1.0, age)

-- ✅ GOOD: intercepts are typically -10 to +10
hekate_stats.logistic_predict(-3.5, 0.1, age)
```

If you see `inf` or `nan` in your output, you've likely overflowed.

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
4. "I want to predict income using linear regression with age and education. Show me how to use `hekate_stats.linear_predict()`"
5. "How do I calculate health risk using logistic regression with BMI and smoking status?"
6. "I need a Poisson regression model for fertility with age and marital status"
7. "How do I use `hekate_stats.softmax_predict` and `hekate_stats.categorical_draw` to model education transitions?"
8. "What's the difference between `logistic_predict` and `logistic_draw`? When should I use each?"

### Complex Models
9. "I want migration to depend on distance between areas. How do I implement this?"
10. "I need a household formation model where young adults leave their parents' household."
11. "How do I model income where it depends on age, education, and sex?"

### Large-Scale Models
12. "How do I set up streaming mode for a population of 10 million people?"
13. "What's the best way to optimize my model for large populations?"

### Reproducibility
14. "How do I ensure my model produces reproducible results?"
15. "Can I run the same model twice and get identical outputs?"
16. "What could cause my results to be non-reproducible?"

### Validation
17. "I got an error: 'column 'agge' not found (did you mean 'age'?)'. How do I fix it?"
18. "What is `strict_mode` and when should I use it?"

### Advanced
19. "How do I add a new column to each person when they get married?"
20. "I want to track the number of children each woman has (parity)."
21. "How do I model international migration where people leave permanently?"

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
      elseif age >= 18 and age < 35 then
        prob = rates.adult_18_34
      elseif age >= 35 and age < 65 then
        prob = rates.adult_35_64
      else
        prob = rates.elderly_65_plus
      end
      
      if math.random() < prob then
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
              elseif age >= 18 and age < 35 then
                prob = rates.adult_18_34
              elseif age >= 35 and age < 65 then
                prob = rates.adult_35_64
              else
                prob = rates.elderly_65_plus
              end
              
              if math.random() < prob then
                person.previous_area = person.area_id
                person.area_id = math.random(1, num_areas)
              end
            end
          end
          return population
        end
```

**Reproducibility Note:** Remember to set `random_seed: 42` (or any integer) in the `simulation:` section to ensure reproducible results. This migration model uses `math.random()`, which is automatically seeded by Hekate for reproducibility.

---

## Common Reproducibility Pitfalls

Here are things that can break reproducibility. If your results vary between runs, check for these:

| Pitfall | Why It Breaks Reproducibility | Fix |
|---------|-------------------------------|-----|
| **Missing `random_seed`** | Auto-seeded with time | Always set `random_seed` |
| **Calling `math.randomseed()`** | Overrides Hekate's seeding | Remove all `math.randomseed()` calls |
| **Using `os.time()`** | Depends on current time | Use `current_year()` if needed |
| **Using `os.clock()`** | Depends on CPU time | Avoid |
| **Reading external files** | May change between runs | Keep data in the population |
| **Using `pairs()` on unordered tables** | Iteration order is non-deterministic | Use `ipairs()` for ordered lists |
| **Different Hekate versions** | Behavior may change | Pin your version |

### Checking Your Model for Reproducibility Issues

Run this quick check:

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

**Streaming mode is fully compatible with reproducibility.** The seed derivation accounts for area processing:

```
seed = random_seed + (iteration × 100000) + (area_index × 1000) + model_index
```

This means:
- Each (year, area, model) combination gets a unique seed
- Results are reproducible whether you use bulk mode or streaming mode
- The only difference is the order in which areas are processed

**Example:**
```yaml
simulation:
  streaming_mode: true
  area_column: "area_id"
  random_seed: 42
```

Both bulk and streaming modes will produce the same results for each area, but streaming mode writes them area-by-area instead of all at once.

---

## Pre-Flight Validation in Detail

Hekate validates your configuration **before** the simulation starts. Here's what it checks:

### Stage 1: Lua Syntax Validation

- Compiles every script in a throwaway VM
- Verifies a `transition(population, params)` function exists
- Warns about common mistakes (missing `tonumber()`, `math.randomseed()` calls)

### Stage 2: Runtime Column Validation

- Reads the CSV header
- Scans scripts for `person.COLUMN` and `person["COLUMN"]` references
- Warns about columns that don't exist in the CSV
- Suggests the closest match using fuzzy matching

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
       └─ line 12: consider using tonumber(person.age) — CSV values are strings
```

**With errors:**

```
  ❌ mortality                      (priority: 2)
       └─ column 'agge' not found (did you mean 'age'?)
```

### Skipping Validation (Not Recommended)

You can disable validation with:

```yaml
simulation:
  skip_validation: true
```

But this is **not recommended** — you'll only discover errors when they crash the run at runtime.

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
| [Tutorial 4](../tutorials/totorial_4_regression.md) | Regression and Statistical Functions | ⭐⭐⭐ Advanced |
| [Tutorial 5](../tutorials/totorial_5_streaming.md) | Large-Scale Simulations with Streaming Mode | ⭐⭐⭐⭐ Advanced |
| [Tutorial 6](../tutorials/tutorial6_stats.md) | Using Hekate's Statistical Toolkit | ⭐⭐⭐⭐ Advanced |
