# Hekate Tutorial 3: Adding Fertility

## Overview

In this tutorial, you'll learn how to add fertility to your demographic model. We'll build on the aging and mortality model from Tutorial 2, adding births to create a more complete population simulation. You'll learn how to create new individuals, assign their characteristics, track population growth, and work with family relationships.

## What You'll Learn

By the end of this tutorial, you'll be able to:
- Add a fertility model to your simulation
- Create new individuals (births) with appropriate characteristics
- Understand how to work with family relationships
- Write complex Lua models that modify the population
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
  streaming_mode: false   # Use bulk mode (simple for small populations)

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

## Part 6: Why Model Order Matters

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

## Part 7: Advanced Fertility - Age-Specific Rates

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

## Part 8: Area-Specific Fertility

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

### Why Area-Specific Fertility Matters

This allows you to model:
- **Urban vs. rural differences** in fertility
- **Regional policy impacts** (e.g., childcare subsidies)
- **Cultural differences** between areas
- **Economic factors** affecting fertility decisions

---

## Part 9: Advanced Fertility - Tracking Mother-Child Relationships

### The Problem

We want to track which mother had which child.

### What We Want to Do

**In plain English:** "When a baby is born, record the mother's ID. This allows us to track family relationships."

### The Updated CSV with Family Columns

Create a file called `population_with_family.csv`:

```csv
person_id,age,sex,area_id,alive,previous_area,partner_id,mother_id,father_id,parity
1,25,F,1,true,0,2,0,0,0
2,27,M,1,true,0,1,0,0,0
3,45,F,1,true,0,4,0,0,2
4,48,M,1,true,0,3,0,0,0
5,82,F,1,true,0,0,0,0,0
6,3,M,2,true,0,0,3,4,0
7,15,F,2,true,0,0,0,0,0
8,38,M,2,true,0,0,0,0,0
9,55,F,2,true,0,10,0,0,3
10,58,M,2,true,0,9,0,0,0
```

**New columns explained:**

| Column | Description |
|--------|-------------|
| `partner_id` | ID of partner/spouse (0 if none) |
| `mother_id` | ID of mother (0 if unknown) |
| `father_id` | ID of father (0 if unknown) |
| `parity` | Number of children a woman has had |

### The Updated Lua Script

```lua
function transition(population, params)
  local fertility_rate = params.fertility_rate or 0.05
  local newborns = {}
  local mothers_to_update = {}
  
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
            mother_id = person.person_id,     -- Track the mother
            father_id = person.partner_id or 0, -- Track the father (if known)
            parity = 0                        -- First child
          }
          table.insert(newborns, baby)
          
          -- Track that this mother needs parity update
          mothers_to_update[person.person_id] = true
        end
      end
    end
  end
  
  -- Add newborns to population
  for _, baby in ipairs(newborns) do
    table.insert(population, baby)
  end
  
  -- Update mother parity (number of children)
  for mother_id in pairs(mothers_to_update) do
    for _, person in ipairs(population) do
      if person.person_id == mother_id then
        person.parity = (person.parity or 0) + 1
        break
      end
    end
  end
  
  return population
end
```

### Understanding Parity

**Parity** is the number of children a woman has had.

- A woman who has had 2 children has parity 2
- A woman who has had 0 children has parity 0

When a woman gives birth, her parity increases by 1.

---

## Part 10: Complete Configuration

Here's the full configuration with advanced fertility:

```yaml
# config_advanced_fertility.yaml
# Complete model with advanced fertility

simulation:
  iterations: 10
  population_file: "population_with_family.csv"
  output_file: "population_advanced.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"
  streaming_mode: false   # Use bulk mode for simplicity

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
              
              if age < 1 then
                prob = 0.005
              elseif age < 18 then
                prob = 0.001
              elseif age < 65 then
                prob = 0.01
              elseif age < 85 then
                prob = 0.10
              else
                prob = 0.20
              end
              
              if math.random() < prob then
                person.alive = false
              end
            end
          end
          return population
        end

  # Model 3: Advanced Fertility with parity tracking
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
          local mothers_to_update = {}
          
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
              
              -- Area-specific fertility rates
              local base_rate = fertility_rate
              if area == 1 then
                base_rate = 0.03
              elseif area == 2 then
                base_rate = 0.08
              end
              
              if age >= 15 and age < 50 then
                if math.random() < base_rate then
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
                    father_id = person.partner_id or 0,
                    parity = 0
                  }
                  table.insert(newborns, baby)
                  mothers_to_update[person.person_id] = true
                end
              end
            end
          end
          
          for _, baby in ipairs(newborns) do
            table.insert(population, baby)
          end
          
          for mother_id in pairs(mothers_to_update) do
            for _, person in ipairs(population) do
              if person.person_id == mother_id then
                person.parity = (person.parity or 0) + 1
                break
              end
            end
          end
          
          return population
        end
```

---

## Part 11: What You've Accomplished

Congratulations! You now have a complete demographic microsimulation model with:

1. ✅ **Aging**: Everyone gets older each year
2. ✅ **Mortality**: Age-specific death probabilities
3. ✅ **Fertility**: Women of childbearing age can give birth
4. ✅ **Age-Specific Fertility**: Different rates for different age groups
5. ✅ **Area-Specific Fertility**: Different rates for different areas
6. ✅ **Mother-Child Links**: Track which mother had which child
7. ✅ **Parity Tracking**: Track how many children each woman has
8. ✅ **Population Growth**: Births and deaths change population size

### The Full Demographic Cycle

```
    ┌─────────────────────────────────────┐
    │                                     │
    │          POPULATION                 │
    │    (Distributed across areas)       │
    │                                     │
    └─────────────┬───────────────────────┘
                  │
                  ▼
    ┌─────────────────────────────────────┐
    │                                     │
    │   1. AGE (everyone gets older)      │
    │      - Same for all areas           │
    │                                     │
    └─────────────┬───────────────────────┘
                  │
                  ▼
    ┌─────────────────────────────────────┐
    │                                     │
    │   2. MORTALITY (some people die)    │
    │      - Can vary by area             │
    │                                     │
    └─────────────┬───────────────────────┘
                  │
                  ▼
    ┌─────────────────────────────────────┐
    │                                     │
    │   3. FERTILITY (some women give birth)
    │      - Varies by area and age       │
    │      - Mother-child links tracked   │
    │      - Parity updated               │
    │                                     │
    └─────────────┬───────────────────────┘
                  │
                  ▼
    ┌─────────────────────────────────────┐
    │                                     │
    │   Next year: repeat!                │
    │                                     │
    └─────────────────────────────────────┘
```

---

## Part 12: What You Can Do Next

### 1. Add Education Models
```lua
function transition(population, params)
  for _, person in ipairs(population) do
    if person.alive == true and person.age >= 5 and person.age <= 18 then
      if person.education == "none" then
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
  return population
end
```

### 2. Add Income Models
```lua
function transition(population, params)
  for _, person in ipairs(population) do
    if person.alive == true and person.age >= 18 then
      local base = 20000
      local education_bonus = 0
      if person.education == "tertiary" then
        education_bonus = 15000
      elseif person.education == "secondary" then
        education_bonus = 5000
      end
      local age_factor = math.min(1, (person.age - 18) / 20)
      local sex_factor = person.sex == "M" and 1.0 or 0.8
      person.income = (base + education_bonus) * age_factor * sex_factor
    end
  end
  return population
end
```

### 3. Add Household Formation
```lua
function transition(population, params)
  -- Young adults form households
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

## Summary of Lua Concepts Learned

| Concept | What it does | Example |
|---------|--------------|---------|
| `table.insert()` | Add to list | `table.insert(list, item)` |
| `math.random()` | Random number | `math.random()` or `math.random(1, 10)` |
| `and` / `or` | Logical operators | `if person.alive and person.sex == "F"` |
| `{}` | Create table | `{ name = "John", age = 25 }` |
| `for` loop | Iterate | `for _, person in ipairs(population) do` |
| `if-elseif-else` | Conditions | `if age < 18 then ... end` |
| `pairs()` | Iterate dictionary | `for key, value in pairs(table) do` |
| `or` | Default value | `rate = params.rate or 0.05` |

---

## Summary

In this tutorial, you've learned:

1. **Fertility Model**: How to add births to your simulation
2. **Creating New Individuals**: How to create new people with unique IDs
3. **Family Relationships**: How to track mother-child relationships
4. **Parity Tracking**: How to count how many children a woman has had
5. **Age-Specific Fertility**: Different rates for different age groups
6. **Area-Specific Fertility**: Different rates for different areas
7. **Model Priority**: Why order matters (age → mortality → fertility)
8. **Population Growth**: How births and deaths change population size

---

## Next Steps

In the next tutorial (**Tutorial 4: Using Linear Regression in Hekate**), you'll learn how to:
- Use Hekate's built-in linear regression function
- Predict income, health risk, and other continuous outcomes
- Define coefficients in YAML
- Handle categorical variables (education, gender, etc.)
- Add randomness to make predictions realistic
- Validate your model results

In **Tutorial 5**, you'll learn about Streaming Mode for large populations.

---

**Well done for completing Tutorial 3!** You now have a complete demographic simulation with aging, mortality, and fertility. You understand the full demographic cycle, can work with areas, and can track family relationships. You're ready to move on to using linear regression models in Tutorial 4!
