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

**Key Philosophy:** Models are data, not code. Change your model by editing YAML, not recompiling.

**Data Structure:**
- Population is a list of tables (each person is a table)
- Each person has key-value pairs from the CSV columns
- Example: `person.age`, `person.sex`, `person.alive`, `person.area`

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
- `hekate_stats.linear_predict(intercept, coef1, var1, coef2, var2, ...)` - Linear regression prediction
- `hekate_stats.logistic_predict(intercept, coef1, var1, coef2, var2, ...)` - Logistic regression (probability)
- `hekate_stats.linear_predict_default(intercept, default, coef1, var1, default1, ...)` - Linear with defaults for missing values

**Built-in Lua Functions:**
- `math.random()` - random number between 0 and 1
- `math.random(n)` - random integer between 1 and n
- `math.random(min, max)` - random integer between min and max
- `math.floor(x)` - round down
- `math.ceil(x)` - round up
- `tonumber(x)` - convert string to number (critical for CSV data!)

**YAML Structure:**
```yaml
simulation:
  iterations: 10
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"
  streaming_mode: false  # Set to true for large populations
  area_column: "area"    # Required if streaming_mode is true

models:
  - name: "model_name"
    type: "lua_model"
    priority: 1          # Lower numbers run first
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
      if person.gender == "female" then
        gender_score = 1
      end
      
      -- Use built-in linear regression
      local income = hekate_stats.linear_predict(
        coefs.intercept,
        coefs.age, age,
        coefs.education, edu_score,
        coefs.gender, gender_score
      )
      
      -- Add realistic noise
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
      
      -- Use built-in logistic regression
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

**YAML Parameters:**
```yaml
parameters:
  coefficients:
    intercept: -3.0
    age: 0.8
    bmi: 0.6
    smoker: 1.2
```

### 4. Mortality Model (With Safety Checks)
```lua
function transition(population, params)
  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end
    
    -- CRITICAL: Skip if already dead
    if not alive then
      goto continue
    end
    
    local age = tonumber(person.age) or 40
    local health_risk = tonumber(person.health_risk) or 0
    
    -- CRITICAL: Skip if health_risk is missing (0 means not calculated)
    if health_risk == 0 then
      person.mortality_risk = "0.0000"
      goto continue
    end
    
    -- Base mortality risk (increases with age)
    local base_risk = 0.001 + (age - 18) * 0.0005
    
    -- Health risk contribution
    local health_factor = (health_risk / 100) * 0.15
    
    -- Additional risk factors
    local smoker_risk = 0
    if person.smoker == "true" or person.smoker == true then
      smoker_risk = 0.03
    end
    
    local chronic_risk = 0
    if person.chronic_disease == "true" or person.chronic_disease == true then
      chronic_risk = 0.04
    end
    
    local mortality_risk = base_risk + health_factor + smoker_risk + chronic_risk
    
    -- Cap at reasonable level
    if mortality_risk > 0.6 then
      mortality_risk = 0.6
    end
    
    person.mortality_risk = string.format("%.4f", mortality_risk)
    
    -- Apply mortality
    if math.random() < mortality_risk then
      person.alive = false
    end
    
    ::continue::
  end
  return population
end
```

### 5. Migration Model
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
        person.previous_area = person.area
        person.area = math.random(1, num_areas)
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

### 6. Fertility Model
```lua
function transition(population, params)
  local fertility_rate = params.fertility_rate or 0.05
  local newborns = {}
  
  -- Find max ID for new IDs
  local max_id = 0
  for _, person in ipairs(population) do
    if person.id ~= nil then
      local id_num = tonumber(string.match(person.id, "%d+")) or 0
      if id_num > max_id then
        max_id = id_num
      end
    end
  end
  
  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end
    
    if alive and person.gender == "female" then
      local age = tonumber(person.age) or 0
      if age >= 15 and age < 50 then
        if math.random() < fertility_rate then
          max_id = max_id + 1
          local baby = {
            id = string.format("P%09d", max_id),
            age = 0,
            gender = math.random() < 0.5 and "female" or "male",
            area = person.area,
            alive = "true",
            mother_id = person.id
          }
          table.insert(newborns, baby)
        end
      end
    end
  end
  
  for _, baby in ipairs(newborns) do
    table.insert(population, baby)
  end
  
  return population
end
```

### 7. Education Model
```lua
function transition(population, params)
  for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
      alive = true
    end
    
    if alive then
      local age = tonumber(person.age) or 0
      if age >= 5 and age <= 18 then
        if person.education == nil or person.education == "none" then
          person.education = "primary"
        elseif person.education == "primary" and age >= 11 then
          person.education = "secondary"
        elseif person.education == "secondary" and age >= 16 then
          if math.random() < 0.3 then
            person.education = "tertiary"
          end
        end
      end
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
  random_seed: 42
  verbose: true
  id_column: "id"
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
              local age = tonumber(person.age) or 18
              person.age = age + 1
            end
          end
          return population
        end
```

### Streaming Mode Template (For Large Populations)
```yaml
simulation:
  iterations: 5
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42
  verbose: false          # Set to false for speed
  id_column: "id"
  streaming_mode: true    # Enable streaming
  area_column: "region"   # Must be sorted in CSV

models:
  # Your models here (same as above)
```

### Full Model Template (With All Features)
```yaml
simulation:
  iterations: 5
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42
  verbose: true
  id_column: "id"
  streaming_mode: false

models:
  # Model 1: Aging (Priority 1 - ALWAYS FIRST)
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
              local age = tonumber(person.age) or 18
              person.age = age + 1
            end
          end
          return population
        end

  # Model 2: Income Prediction
  - name: "income_predictor"
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
              local gender_score = 0
              if person.gender == "female" then
                gender_score = 1
              end
              local income = hekate_stats.linear_predict(
                coefs.intercept,
                coefs.age, age,
                coefs.education, edu_score,
                coefs.gender, gender_score
              )
              person.income = math.floor(income + 0.5)
            end
          end
          return population
        end

  # Model 3: Health Risk
  - name: "health_risk_calculator"
    type: "lua_model"
    priority: 3
    enabled: true
    parameters:
      coefficients:
        intercept: -3.0
        age: 0.8
        bmi: 0.6
        smoker: 1.2
      script: |
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

  # Model 4: Mortality
  - name: "mortality_model"
    type: "lua_model"
    priority: 4
    enabled: true
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            local alive = false
            if person.alive == true or person.alive == "true" then
              alive = true
            end
            if not alive then
              goto continue
            end
            local age = tonumber(person.age) or 40
            local health_risk = tonumber(person.health_risk) or 0
            if health_risk == 0 then
              person.mortality_risk = "0.0000"
              goto continue
            end
            local base_risk = 0.001 + (age - 18) * 0.0005
            local health_factor = (health_risk / 100) * 0.15
            local smoker_risk = 0
            if person.smoker == "true" or person.smoker == true then
              smoker_risk = 0.03
            end
            local chronic_risk = 0
            if person.chronic_disease == "true" or person.chronic_disease == true then
              chronic_risk = 0.04
            end
            local mortality_risk = base_risk + health_factor + smoker_risk + chronic_risk
            if mortality_risk > 0.6 then
              mortality_risk = 0.6
            end
            person.mortality_risk = string.format("%.4f", mortality_risk)
            if math.random() < mortality_risk then
              person.alive = false
            end
            ::continue::
          end
          return population
        end
```

---

## Best Practices & Common Pitfalls

### 1. ALWAYS Use tonumber() for CSV Data
CSV values are strings. Always convert:
```lua
local age = tonumber(person.age) or 0  -- Correct
-- local age = person.age  -- WRONG! Will fail comparisons
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

### 7. Lua Comments Use -- NOT #
```lua
-- This is a comment  -- Correct
-- local x = 5
-- # This is NOT a comment in Lua  -- WRONG!
```

### 8. Use math.random() Correctly
```lua
math.random()        -- Returns 0.0 to 1.0
math.random(10)      -- Returns 1 to 10
math.random(1, 100)  -- Returns 1 to 100
```

---

## How to Use This Template

1. **Copy the entire template above** into your LLM (ChatGPT, Claude, etc.)
2. **Describe what you want to build**:
   - "I want to build a model that does [X]"
   - "My population has columns [A, B, C]"
3. **The LLM will generate** the YAML and Lua code for you
4. **Test it** by running Hekate with your new configuration
5. **Iterate** - ask follow-up questions to refine your model

---

## Example Questions to Ask the LLM

### Basic Models
1. "I want to add a model where people get married. Women marry at age 20-30, men at 22-35. How do I write this?"
2. "I need a fertility model where fertility rates vary by age: 15-19: 2%, 20-24: 8%, 25-29: 10%, 30-34: 8%, 35-39: 4%, 40-44: 1%"
3. "How do I create an education model where children progress through primary, secondary, and tertiary education?"

### Statistical Models
4. "I want to predict income using linear regression with age and education. Show me how to use hekate_stats.linear_predict()"
5. "How do I calculate health risk using logistic regression with BMI and smoking status?"
6. "I need a model that combines multiple risk factors using logistic regression"

### Complex Models
7. "I want migration to depend on distance between areas. How do I implement this?"
8. "I need a household formation model where young adults leave their parents' household."
9. "How do I model income where it depends on age, education, and sex?"

### Large-Scale Models
10. "How do I set up streaming mode for a population of 10 million people?"
11. "What's the best way to optimize my model for large populations?"

### Advanced
12. "How do I add a new column to each person when they get married?"
13. "I want to track the number of children each woman has (parity)."
14. "How do I model international migration where people leave permanently?"

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
        person.previous_area = person.area
        person.area = math.random(1, num_areas)
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
                person.previous_area = person.area
                person.area = math.random(1, num_areas)
              end
            end
          end
          return population
        end
```

