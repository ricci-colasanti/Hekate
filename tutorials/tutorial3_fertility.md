# Tutorial 3: Adding Fertility with Streaming Mode and Areas

## Overview

In this tutorial, you'll learn how to add fertility to your demographic model. We'll build on the aging and mortality model from Tutorial 2, adding births to create a more complete population simulation. You'll learn how to create new individuals, assign their characteristics, track population growth, and work with area-based data using streaming mode.

## What You'll Learn

By the end of this tutorial, you'll be able to:
- Add a fertility model to your simulation
- Create new individuals (births) with appropriate characteristics
- Understand how to work with family relationships
- Write complex Lua models that modify the population
- Use streaming mode with fertility models
- Track population growth across areas

## Prerequisites

- Completion of Tutorial 2 (or equivalent knowledge)
- The `population.csv` file from Tutorial 1
- A text editor

---

## Part 1: What We're Going to Build

So far we have:
- ✅ **Aging**: Everyone gets older each year
- ✅ **Mortality**: Some people die based on their age
- ✅ **Areas**: Population is organized by geographic areas
- ✅ **Streaming Mode**: Process large populations with low memory

Now we'll add:
- **Fertility**: Women of childbearing age can give birth

### What We Want to Do

**In plain English:** "Each year, for every woman aged 15-49 who is alive, there is a 5% chance she gives birth. If she gives birth, we add a new person to the population. The new baby inherits the mother's area, is randomly male or female, and is marked as alive."

### Why This Matters

Adding fertility completes the demographic cycle:
- **Births** add people to the population
- **Aging** moves people through the life course
- **Deaths** remove people from the population

With all three models, we can simulate realistic population dynamics.

---

## Part 2: Understanding Lua Table Operations

Before we write our fertility model, let's understand how to work with tables (lists) in Lua.

### Adding Items to a Table

In Lua, `table.insert()` adds a new item to the end of a list:

```lua
-- Create a list
local newborns = {}

-- Add a new person to the list
table.insert(newborns, {
  person_id = 11,
  age = 0,
  sex = "F",
  area_id = 1,
  alive = true
})
```

### Creating a New Person

A person is just a table with properties:

```lua
local baby = {
  person_id = 11,
  age = 0,
  sex = "F",
  area_id = 1,
  alive = true
}
```

### Finding the Maximum ID

To create new unique IDs, we need to find the highest existing ID:

```lua
local max_id = 0
for _, person in ipairs(population) do
  if person.person_id > max_id then
    max_id = person.person_id
  end
end
```

Then the next ID is `max_id + 1`.

---

## Part 3: The Fertility Model in Lua

### The Complete Fertility Model

Here's the Lua script for our fertility model:

```lua
function transition(population, params)
  -- Get fertility rate from parameters
  local fertility_rate = params.fertility_rate or 0.05
  
  -- Create a list for newborns
  local newborns = {}
  
  -- Find the maximum ID
  local max_id = 0
  for _, person in ipairs(population) do
    if person.person_id ~= nil and person.person_id > max_id then
      max_id = person.person_id
    end
  end
  
  -- Process each person
  for _, person in ipairs(population) do
    -- Check if this person is a fertile woman
    if person.alive == true and person.sex == "F" then
      local age = person.age
      if age >= 15 and age < 50 then
        -- Roll the dice for fertility
        if math.random() < fertility_rate then
          -- Create a newborn
          max_id = max_id + 1
          local baby = {
            person_id = max_id,
            age = 0,
            sex = math.random() < 0.5 and "F" or "M",
            area_id = person.area_id,
            alive = true,
            previous_area = 0,
            partner_id = 0,
            mother_id = person.person_id,
            father_id = 0,
            parity = 0
          }
          table.insert(newborns, baby)
        end
      end
    end
  end
  
  -- Add newborns to population
  for _, baby in ipairs(newborns) do
    table.insert(population, baby)
  end
  
  return population
end
```

### Breaking It Down Piece by Piece

**First, in plain English:** "Go through every woman who is alive and aged 15-49. For each one, flip a coin. If it comes up heads (5% chance), create a new baby. The baby gets a new ID, age 0, a random sex (50:50), and lives in the same area as the mother. The baby is alive."

**Now let's break down each part:**

| Part | What it does | In plain English |
|------|--------------|------------------|
| `local fertility_rate = params.fertility_rate or 0.05` | Get rate | "Use the rate from the config, or 5% if not specified" |
| `local newborns = {}` | Create list | "Create an empty list for babies born this year" |
| `for _, person in ipairs(population) do` | Loop through people | "For each person in the population..." |
| `if person.alive == true and person.sex == "F" then` | Check woman | "...if they are a woman and alive..." |
| `if age >= 15 and age < 50 then` | Check age | "...and they're of childbearing age (15-49)..." |
| `if math.random() < fertility_rate then` | Roll dice | "...roll a random number..." |
| `max_id = max_id + 1` | New ID | "...if it's less than the fertility rate, create a new ID" |
| `local baby = { ... }` | Create baby | "...create a new baby with properties" |
| `table.insert(newborns, baby)` | Add to list | "...add the baby to the list of newborns" |
| `table.insert(population, baby)` | Add to population | "...add the baby to the population" |

---

## Part 4: Adding Fertility to Our Configuration

Here's our complete configuration with aging, mortality, and fertility:

```yaml
# config_aging_mortality_fertility.yaml
# Complete demographic model with aging, mortality, and fertility

simulation:
  iterations: 10
  population_file: "population.csv"
  output_file: "population_complete.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"
  area_column: "area_id"
  streaming_mode: false  # Use bulk mode for small population

models:
  # Model 1: Age increment (runs first)
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

  # Model 2: Mortality (runs second)
  - name: "mortality"
    type: "lua_model"
    priority: 2
    enabled: true
    description: "Age-specific mortality: 0.1% for under 30, 5% for 30+"
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive == true then
              local age = person.age
              local prob = 0
              
              if age < 30 then
                prob = 0.001
              else
                prob = 0.05
              end
              
              if math.random() < prob then
                person.alive = false
              end
            end
          end
          return population
        end

  # Model 3: Fertility (runs third)
  - name: "fertility"
    type: "lua_model"
    priority: 3
    enabled: true
    description: "Fertility: 5% chance for women aged 15-49 to give birth"
    parameters:
      fertility_rate: 0.05
      script: |
        function transition(population, params)
          local fertility_rate = params.fertility_rate
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
                    area_id = person.area_id,
                    alive = true,
                    previous_area = 0,
                    partner_id = 0,
                    mother_id = person.person_id,
                    father_id = 0,
                    parity = 0
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

---

## Part 5: Running the Complete Model

Save the configuration as `config_aging_mortality_fertility.yaml` and run it:

```bash
./hekate config_aging_mortality_fertility.yaml
```

### Expected Output

```
2024/01/15 10:00:00 ═══ Hekate: Microsimulation Engine ═══
2024/01/15 10:00:00 Iterations: 10
2024/01/15 10:00:00 Population file: population.csv
2024/01/15 10:00:00 ID column: person_id
2024/01/15 10:00:00 Mode: BULK (load all into memory)
2024/01/15 10:00:00 Models loaded: 3
2024/01/15 10:00:00 Loaded 10 individuals with 4 columns
2024/01/15 10:00:00 Enabled models: 3
2024/01/15 10:00:00   - age_increment (priority: 1)
2024/01/15 10:00:00   - mortality (priority: 2)
2024/01/15 10:00:00   - fertility (priority: 3)

2024/01/15 10:00:00 ═══ Iteration 1/10 ═══
2024/01/15 10:00:00   ▶ age_increment
2024/01/15 10:00:00   ▶ mortality
2024/01/15 10:00:00   ▶ fertility

...

2024/01/15 10:00:00 ═══ Simulation Complete ═══
2024/01/15 10:00:00 Results saved to population_complete.csv
```

---

## Part 6: Adding Streaming Mode for Larger Populations

### Create a Population with Areas for Streaming

For streaming mode, create a CSV sorted by `area_id`:

```csv
person_id,age,sex,area_id,alive,previous_area,partner_id,mother_id,father_id,parity
1,25,F,1,true,0,0,0,0,0
2,30,M,1,true,0,1,0,0,0
3,45,F,1,true,0,0,0,0,2
4,68,M,1,true,0,0,0,0,0
5,82,F,1,true,0,0,0,0,0
6,2,M,2,true,0,0,0,3,0
7,15,F,2,true,0,0,0,0,0
8,35,M,2,true,0,0,0,0,0
9,55,F,2,true,0,0,0,0,3
10,70,M,2,true,0,0,0,0,0
```

### Streaming Mode Configuration

```yaml
# config_aging_mortality_fertility_streaming.yaml
# Streaming mode with aging, mortality, and fertility

simulation:
  iterations: 10
  population_file: "population_with_areas.csv"
  output_file: "final_output.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"
  area_column: "area_id"          # REQUIRED for streaming mode
  streaming_mode: true            # Enable area-by-area processing

models:
  # Model 1: Age increment
  - name: "age_increment"
    type: "lua_model"
    priority: 1
    enabled: true
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

  # Model 2: Mortality
  - name: "mortality"
    type: "lua_model"
    priority: 2
    enabled: true
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive == true then
              local age = person.age
              local prob = 0
              
              if age < 30 then
                prob = 0.001
              else
                prob = 0.05
              end
              
              if math.random() < prob then
                person.alive = false
              end
            end
          end
          return population
        end

  # Model 3: Fertility with area awareness
  - name: "fertility"
    type: "lua_model"
    priority: 3
    enabled: true
    parameters:
      fertility_rate: 0.05
      script: |
        function transition(population, params)
          local fertility_rate = params.fertility_rate
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
                    area_id = person.area_id,
                    alive = true,
                    previous_area = 0,
                    partner_id = 0,
                    mother_id = person.person_id,
                    father_id = 0,
                    parity = 0
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

### Running Streaming Mode

```bash
./hekate config_aging_mortality_fertility_streaming.yaml
```

### Expected Output with Streaming

```
2024/01/15 10:00:00 ═══ Hekate: Microsimulation Engine ═══
2024/01/15 10:00:00 Iterations: 10
2024/01/15 10:00:00 Population file: population_with_areas.csv
2024/01/15 10:00:00 ID column: person_id
2024/01/15 10:00:00 Mode: STREAMING (area-by-area)
2024/01/15 10:00:00 Area column: area_id
2024/01/15 10:00:00 Models loaded: 3
2024/01/15 10:00:00 Detected 10 columns
2024/01/15 10:00:00 
═══ Iteration 1/10 ═══
2024/01/15 10:00:00   ▶ age_increment
2024/01/15 10:00:00   ▶ mortality
2024/01/15 10:00:00   ▶ fertility
2024/01/15 10:00:00   Processed 2 areas
...
```

### Output Files in Streaming Mode

```
year_1.csv  → After year 1
year_2.csv  → After year 2
...
year_10.csv → After year 10
final_output.csv → Copy of year_10.csv
```

These are working files: Hekate writes them as it processes each year, then copies the last one to `output_file` and deletes them at the end of the run. If you want per-iteration snapshots that persist, use bulk mode with `output_every_iteration: true` (see the README).

---

## Part 7: Why Model Order Matters

The order of models is crucial. Here's why we run them in this specific order:

### Priority 1: Aging First

```yaml
priority: 1  # Runs first
```

**Why?** Women need to be the correct age for fertility. A woman who is 14 should not give birth, but if she turns 15 this year, she should be eligible.

### Priority 2: Mortality Second

```yaml
priority: 2  # Runs second
```

**Why?** Dead women shouldn't give birth. We need to remove people who die before checking fertility.

### Priority 3: Fertility Last

```yaml
priority: 3  # Runs last
```

**Why?** Newborns shouldn't be aged or killed in the same year they're born.

### Visualizing the Order

```
Start of Year
    ↓
1. AGE everyone (including women who turn 15)
    ↓
2. MORTALITY (dead people don't give birth)
    ↓
3. FERTILITY (women give birth to babies who are age 0)
    ↓
End of Year
```

---

## Part 8: Advanced Fertility - Age-Specific Rates

### The Problem

In our current model, all women 15-49 have the same 5% chance of giving birth. But in reality, fertility varies by age.

### What We Want to Do

**In plain English:** "Women in their 20s have the highest fertility, women in their 40s have much lower fertility."

### The Updated Lua Script

```lua
function transition(population, params)
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
      local rate = 0
      
      -- Age-specific fertility rates
      if age >= 15 and age < 20 then
        rate = 0.02   -- 2% chance
      elseif age >= 20 and age < 25 then
        rate = 0.08   -- 8% chance
      elseif age >= 25 and age < 30 then
        rate = 0.10   -- 10% chance (peak fertility)
      elseif age >= 30 and age < 35 then
        rate = 0.08   -- 8% chance
      elseif age >= 35 and age < 40 then
        rate = 0.04   -- 4% chance
      elseif age >= 40 and age < 45 then
        rate = 0.01   -- 1% chance
      elseif age >= 45 and age < 50 then
        rate = 0.001  -- 0.1% chance
      end
      
      if math.random() < rate then
        max_id = max_id + 1
        local baby = {
          person_id = max_id,
          age = 0,
          sex = math.random() < 0.5 and "F" or "M",
          area_id = person.area_id,
          alive = true,
          previous_area = 0,
          partner_id = 0,
          mother_id = person.person_id,
          father_id = 0,
          parity = 0
        }
        table.insert(newborns, baby)
      end
    end
  end
  
  for _, baby in ipairs(newborns) do
    table.insert(population, baby)
  end
  
  return population
end
```

### Understanding the Rates

| Age Group | Rate | Why |
|-----------|------|-----|
| 15-19 | 2% | Low - teenage pregnancies |
| 20-24 | 8% | Higher - peak childbearing years start |
| 25-29 | 10% | Peak fertility |
| 30-34 | 8% | Still high but declining |
| 35-39 | 4% | Declining |
| 40-44 | 1% | Low |
| 45-49 | 0.1% | Very low |

---

## Part 9: Area-Specific Fertility

### The Problem

Different areas might have different fertility rates due to cultural, economic, or policy differences.

### What We Want to Do

**In plain English:** "Area 1 has low fertility (3%), Area 2 has high fertility (8%), and other areas have moderate fertility (5%)."

### The Updated Lua Script

```lua
function transition(population, params)
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
      local area = person.area_id
      local base_rate = 0
      
      -- Area-specific fertility rates
      if area == 1 then
        base_rate = 0.03   -- 3% (low fertility)
      elseif area == 2 then
        base_rate = 0.08   -- 8% (high fertility)
      else
        base_rate = 0.05   -- 5% (moderate fertility)
      end
      
      -- Age adjustment (multiplier)
      local age_multiplier = 0
      if age >= 15 and age < 20 then
        age_multiplier = 0.4
      elseif age >= 20 and age < 25 then
        age_multiplier = 1.6
      elseif age >= 25 and age < 30 then
        age_multiplier = 2.0
      elseif age >= 30 and age < 35 then
        age_multiplier = 1.6
      elseif age >= 35 and age < 40 then
        age_multiplier = 0.8
      elseif age >= 40 and age < 45 then
        age_multiplier = 0.2
      elseif age >= 45 and age < 50 then
        age_multiplier = 0.02
      end
      
      local rate = base_rate * age_multiplier
      
      if math.random() < rate then
        max_id = max_id + 1
        local baby = {
          person_id = max_id,
          age = 0,
          sex = math.random() < 0.5 and "F" or "M",
          area_id = person.area_id,
          alive = true,
          previous_area = 0,
          partner_id = 0,
          mother_id = person.person_id,
          father_id = 0,
          parity = 0
        }
        table.insert(newborns, baby)
      end
    end
  end
  
  for _, baby in ipairs(newborns) do
    table.insert(population, baby)
  end
  
  return population
end
```

### How It Works

For each fertile woman, we compute a rate that is the product of two things:

- **The area's base rate** (3% in Area 1, 8% in Area 2, 5% elsewhere)
- **An age multiplier** reflecting the shape of the fertility schedule

For example:

- A 27-year-old woman in Area 2 has rate `0.08 × 2.0 = 0.16` (16% chance of giving birth this year).
- A 43-year-old in Area 1 has rate `0.03 × 0.2 = 0.006` (0.6% chance).
- A 17-year-old in any other area has rate `0.05 × 0.4 = 0.02` (2% chance).

This two-level structure — area-specific base rate, age-specific shape — is a common way of building realistic fertility models without hard-coding every area-age combination.

---

## What You've Accomplished

Congratulations! You've completed the full demographic cycle:

1. ✅ **Aging**: Everyone gets older each year
2. ✅ **Mortality**: People die based on age (and optionally area)
3. ✅ **Fertility**: Women of childbearing age give birth
4. ✅ **Population growth**: New individuals are created and added to the population
5. ✅ **Family links**: Newborns record their mother's ID
6. ✅ **Streaming compatibility**: The same models work in bulk and streaming modes
7. ✅ **Age-specific and area-specific rates**: You can tune fertility to match your domain

---

## Next Steps

You now have a working three-model demographic simulation. A few directions to explore:

- **Households**: Group individuals into families and track household composition
- **Migration**: Move people between areas and see how that interacts with fertility and mortality
- **Regression-based models**: Move beyond hard-coded rates and use Tutorial 6's statistical toolkit
- **Large populations**: Try the same config on 1M+ people with streaming mode (Tutorial 5)

---

**Well done for completing Tutorial 3!** You now understand births, deaths, aging, and how they fit together in a complete demographic simulation. The next tutorial takes a different tack — using published regression coefficients to drive the models instead of hand-coded rates.