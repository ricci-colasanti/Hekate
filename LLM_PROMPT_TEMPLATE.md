# Hekate LLM Prompt Template

Copy and paste this entire message into your LLM (ChatGPT, Claude, etc.) to get help with Hekate models.

---

## Context: Hekate Microsimulation Engine

Hekate is a self-contained demographic microsimulation engine. Models are defined in YAML configuration files with embedded Lua scripts.

**Core Concepts:**
- **Population**: CSV file with one row per person
- **Models**: Lua scripts that transform the population each year
- **YAML**: Configuration file that ties everything together

**Data Structure:**
- Population is a list of tables (each person is a table)
- Each person has key-value pairs from the CSV columns
- Example: `person.age`, `person.sex`, `person.alive`, `person.area`

**Lua Script Structure:**
```lua
-- Model: runs each year to transform the population
function transition(population, params)
  for _, person in ipairs(population) do
    -- Your logic here
  end
  return population
end
```

**Built-in Lua Functions:**
- `math.random()` - random number between 0 and 1
- `math.random(n)` - random integer between 1 and n
- `math.random(min, max)` - random integer between min and max
- `math.floor(x)` - round down
- `math.ceil(x)` - round up
- `math.round(x)` - round to nearest integer

**YAML Structure:**
```yaml
simulation:
  iterations: 10
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"

models:
  - name: "model_name"
    type: "lua_model"
    priority: 1
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

### 1. Aging Model
```lua
function transition(population, params)
  for _, person in ipairs(population) do
    if person.alive == true then
      person.age = person.age + 1
    end
  end
  return population
end
```

### 2. Mortality Model
```lua
function transition(population, params)
  local rates = params.mortality_rates
  for _, person in ipairs(population) do
    if person.alive == true then
      local age = person.age
      local prob = 0
      if age < 1 then
        prob = rates.infant
      elseif age >= 85 then
        prob = rates.elderly
      end
      if math.random() < prob then
        person.alive = false
      end
    end
  end
  return population
end
```

### 3. Migration Model
```lua
function transition(population, params)
  local rates = params.migration_rates
  local num_areas = params.num_areas
  
  for _, person in ipairs(population) do
    if person.alive == true then
      local age = person.age
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

### 4. Fertility Model
```lua
function transition(population, params)
  local fertility_rate = params.fertility_rate or 0.05
  local newborns = {}
  
  local max_id = 0
  for _, person in ipairs(population) do
    if person.person_id ~= nil and person.person_id > max_id then
      max_id = person.person_id
    end
  end
  
  for _, person in ipairs(population) do
    if person.alive == true and person.sex == "F" then
      local age = person.age
      if age >= 15 and age < 50 then
        if math.random() < fertility_rate then
          max_id = max_id + 1
          local baby = {
            person_id = max_id,
            age = 0,
            sex = math.random() < 0.5 and "F" or "M",
            area = person.area,
            alive = true,
            mother_id = person.person_id
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

### 5. Education Model
```lua
function transition(population, params)
  for _, person in ipairs(population) do
    if person.alive == true then
      if person.age >= 5 and person.age <= 18 then
        if person.education == nil or person.education == "none" then
          person.education = "primary"
        elseif person.education == "primary" and person.age >= 11 then
          person.education = "secondary"
        elseif person.education == "secondary" and person.age >= 16 then
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

### 6. Income Model
```lua
function transition(population, params)
  local base = params.base_income or 20000
  local education_bonus = params.education_bonus or {tertiary=15000, secondary=5000}
  
  for _, person in ipairs(population) do
    if person.alive == true and person.age >= 18 then
      local bonus = 0
      if person.education == "tertiary" then
        bonus = education_bonus.tertiary
      elseif person.education == "secondary" then
        bonus = education_bonus.secondary
      end
      local age_factor = math.min(1, (person.age - 18) / 20)
      local sex_factor = person.sex == "M" and 1.0 or 0.8
      person.income = (base + bonus) * age_factor * sex_factor
    end
  end
  return population
end
```

### 7. Household Formation Model
```lua
function transition(population, params)
  -- Add household_id if not present
  for _, person in ipairs(population) do
    if person.household_id == nil then
      person.household_id = person.person_id
    end
  end
  
  -- Young adults form new households
  for _, person in ipairs(population) do
    if person.alive == true and person.age >= 18 and person.age <= 25 then
      if math.random() < 0.05 then
        person.household_id = person.person_id
      end
    end
  end
  
  -- Children live with mothers
  for _, person in ipairs(population) do
    if person.alive == true and person.age < 18 and person.mother_id ~= nil then
      for _, mother in ipairs(population) do
        if mother.person_id == person.mother_id and mother.alive == true then
          person.household_id = mother.household_id
          break
        end
      end
    end
  end
  
  return population
end
```

---

## YAML Configuration Template

```yaml
# config.yaml
simulation:
  iterations: 10
  population_file: "population.csv"
  output_file: "output.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"

models:
  # --- MODEL 1: Aging ---
  - name: "age_increment"
    type: "lua_model"
    priority: 1
    enabled: true
    description: "Increment everyone's age by 1 year"
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive == true then
              person.age = person.age + 1
            end
          end
          return population
        end

  # --- MODEL 2: [YOUR MODEL HERE] ---
  # Add more models here
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

## Example Questions to Ask the LLM

### Basic Models
1. "I want to add a model where people get married. Women marry at age 20-30, men at 22-35. How do I write this?"
2. "I need a fertility model where fertility rates vary by age: 15-19: 2%, 20-24: 8%, 25-29: 10%, 30-34: 8%, 35-39: 4%, 40-44: 1%"
3. "How do I create an education model where children progress through primary, secondary, and tertiary education?"

### Complex Models
4. "I want migration to depend on distance between areas. How do I implement this?"
5. "I need a household formation model where young adults leave their parents' household."
6. "How do I model income where it depends on age, education, and sex?"

### Advanced
7. "How do I add a new column to each person when they get married?"
8. "I want to track the number of children each woman has (parity)."
9. "How do I model international migration where people leave permanently?"

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
    if person.alive == true then
      local age = person.age
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
            if person.alive == true then
              local age = person.age
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

---

**Copy and paste this entire message to get started!** 🚀