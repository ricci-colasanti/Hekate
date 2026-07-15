# Hekate Tutorial 2: Understanding YAML, Lua, and Adding Mortality

## Overview

In this tutorial, we'll dive deeper into YAML and Lua. You'll learn how YAML rules work, how to add a mortality model to your simulation, and how to track population changes.

## What You'll Learn

By the end of this tutorial, you'll be able to:
- Understand YAML formatting rules
- Write a mortality model using Lua
- Understand Lua in plain English

## Prerequisites

- Completion of Tutorial 1 (or equivalent knowledge)
- The `population.csv` file from Tutorial 1
- A text editor

---

## Part 1: Understanding YAML Rules

In Tutorial 1, we copied a YAML configuration and it just worked. Now let's understand **why** it worked and what the rules are.

### Rule 1: YAML Uses Spaces, Not Tabs

This is the most common mistake! YAML uses **spaces** for indentation, **never tabs**.

```yaml
# ✅ CORRECT - using spaces
simulation:
  iterations: 5
  population_file: "population.csv"

# ❌ WRONG - using tabs (invisible but will cause errors)
simulation:
	iterations: 5
	population_file: "population.csv"
```

**Why does this matter?** YAML reads indentation to understand what belongs to what. If you use tabs, YAML gets confused and throws an error.

### Rule 2: Multi-line Strings Need the Pipe (`|`)

In Tutorial 1, we used this in our model:

```yaml
script: |
  function transition(population, params)
    for _, person in ipairs(population) do
      if person.alive == true then
        person.age = person.age + 1
      end
    end
    return population
  end
```

The pipe (`|`) tells YAML: "Everything that follows, indented, is a single string value." This allows us to write clean, readable Lua scripts that span multiple lines.

**What happens without the pipe?**

```yaml
# ❌ WRONG - YAML sees three separate values
script: function transition(population, params)
  for _, person in ipairs(population) do
    if person.alive == true then
      person.age = person.age + 1
    end
  end
  return population
end
```

YAML would see this as multiple separate lines and throw an error because it expects a single value after the colon.

**Important:** The content after the pipe must be indented:

```yaml
# ✅ CORRECT - content is indented
script: |
  function transition(population, params)
    for _, person in ipairs(population) do
      if person.alive == true then
        person.age = person.age + 1
      end
    end
    return population
  end

# ❌ WRONG - content is not indented
script: |
function transition(population, params)
  for _, person in ipairs(population) do
    if person.alive == true then
      person.age = person.age + 1
    end
  end
  return population
end
```

### Rule 3: Column Names Must Match Your CSV Exactly

**This is the most important rule for Hekate!** The column names you use in your Lua scripts must exactly match the column names in your CSV file.

```csv
# CSV column names
person_id,age,sex,area,alive
```

```lua
-- ✅ CORRECT - matches CSV
if person.alive == true then
  person.age = person.age + 1
end

-- ❌ WRONG - 'Alive' doesn't match 'alive' in CSV
if person.Alive == true then
  person.Age = person.Age + 1
end
```

**Why does this matter?** Hekate loads your CSV and creates a data structure with the exact column names from your CSV. If you use `Alive` but your CSV has `alive`, Hekate won't find the column.

### Quick YAML Reference

| What you want | How to write it |
|---------------|-----------------|
| A key-value pair | `key: value` |
| A multi-line string | `key: |`<br>`  line 1`<br>`  line 2` |
| A list | `- item 1`<br>`- item 2` |
| A comment | `# This is a comment` |
| Indentation | Use 2 spaces (or 4, but be consistent) |

---

## Part 2: Understanding Lua (In Plain English)

Before we write more Lua, let's understand what Lua does in plain English.

**Lua is a simple scripting language that reads like English.**

Think of your population as a list of people, where each person has properties:

| person_id | age | sex | area | alive |
|-----------|-----|-----|------|-------|
| 1 | 25 | F | 1 | true |
| 2 | 30 | M | 1 | true |
| 3 | 45 | F | 1 | true |
| ... | ... | ... | ... | ... |

Lua lets you:

- **Loop through people**: `for _, person in ipairs(population) do`
- **Check conditions**: `if person.alive == true then`
- **Make changes**: `person.age = person.age + 1`
- **Return results**: `return population`

That's it! You don't need to know anything else about programming.

### The Lua You'll Use in This Tutorial

| Lua Concept | What it does | Example | Plain English |
|-------------|--------------|---------|---------------|
| `function` | Define a function | `function transition(population, params)` | "Here's a model that runs each year" |
| `for` | Loop through a list | `for _, person in ipairs(population) do` | "For each person in the population..." |
| `if` | Check a condition | `if person.alive == true then` | "If they are alive..." |
| `==` | Compare values | `person.alive == true` | "Is alive equal to true?" |
| `=` | Assign a value | `person.age = person.age + 1` | "Set age to age plus 1" |
| `return` | Return a value | `return population` | "Send back the updated population" |

---

## Part 3: Adding a Mortality Model

Now let's add a mortality model to our simulation. This will make our model more realistic by allowing people to die.

### What We Want to Do

**In plain English:** "Each year, check every person. If they are alive and under 30, there's a 0.1% chance they die. If they are alive and 30 or over, there's a 5% chance they die. If they die, mark them as dead."

### The Mortality Model in YAML with Lua

Here's the mortality model as it appears in your `config.yaml`:

```yaml
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
              prob = 0.001  -- 0.1% chance
            else
              prob = 0.05   -- 5% chance
            end
            
            if math.random() < prob then
              person.alive = false
            end
          end
        end
        return population
      end
```

**First, in plain English:** "Go through everyone in the population. For each person who is alive, check their age. If they are under 30, flip a coin with a 0.1% chance of coming up heads. If heads, mark them as dead. If they are 30 or over, flip a coin with a 5% chance of coming up heads. If heads, mark them as dead."

**Let's break down what each part does:**

| Part | What it does | In plain English |
|------|--------------|------------------|
| `- name: "mortality"` | Names the model | "This model is called 'mortality'" |
| `type: "lua_model"` | Tells Hekate it's a Lua script | "This is a Lua model" |
| `priority: 2` | Sets execution order | "Run this after priority 1 models" |
| `enabled: true` | Activates the model | "This model is turned on" |
| `script: |` | Starts the Lua code | "Here comes the Lua script..." |
| `function transition(population, params)` | Defines the model | "This is the model that runs each year" |
| `for _, person in ipairs(population) do` | Loops through people | "For each person in the population..." |
| `if person.alive == true then` | Checks if alive | "...if they are alive..." |
| `local age = person.age` | Gets their age | "...get their age..." |
| `if age < 30 then` | Checks age group | "...if they're under 30..." |
| `prob = 0.001` | Sets probability | "...they have a 0.1% chance of dying" |
| `else` | Otherwise | "...otherwise (they're 30 or over)..." |
| `prob = 0.05` | Sets probability | "...they have a 5% chance of dying" |
| `if math.random() < prob then` | Rolls the dice | "...roll a random number..." |
| `person.alive = false` | Marks as dead | "...if it's less than the probability, mark them as dead" |
| `return population` | Returns updated population | "Send back the updated population" |

**What is `math.random()`?** In Lua, `math.random()` generates a random number between 0 and 1. So `math.random() < 0.001` means "there's a 0.1% chance this is true" (like flipping a weighted coin).

### Creating the Full Configuration

Here's our complete configuration with both aging and mortality:

```yaml
# config_aging_mortality.yaml
# Aging and mortality model

simulation:
  iterations: 5
  population_file: "population.csv"
  output_file: "population_aged_mortality.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"

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
                prob = 0.001  -- 0.1% chance
              else
                prob = 0.05   -- 5% chance
              end
              
              if math.random() < prob then
                person.alive = false
              end
            end
          end
          return population
        end
```

### Why Priority Matters

Notice we have **two models** now:
- **Priority 1**: Age increment (runs first)
- **Priority 2**: Mortality (runs second)

**Why this order?** We want people to age first, then we check if they die at their new age. Someone who turns 30 this year should now face the higher 30+ mortality rate.

### Running the Simulation

Save the configuration as `config_aging_mortality.yaml` and run it:

```bash
./hekate config_aging_mortality.yaml
```

### Expected Output

```
2024/01/15 10:00:00 ═══ Hekate: Microsimulation Engine ═══
2024/01/15 10:00:00 Iterations: 5
...
2024/01/15 10:00:00 ═══ Iteration 1/5 ═══
2024/01/15 10:00:00   ▶ age_increment
2024/01/15 10:00:00   ▶ mortality

2024/01/15 10:00:00 ═══ Iteration 2/5 ═══
2024/01/15 10:00:00   ▶ age_increment
2024/01/15 10:00:00   ▶ mortality

...

2024/01/15 10:00:00 ═══ Iteration 5/5 ═══
2024/01/15 10:00:00   ▶ age_increment
2024/01/15 10:00:00   ▶ mortality

2024/01/15 10:00:00 ═══ Simulation Complete ═══
2024/01/15 10:00:00 Results saved to population_aged_mortality.csv
```

Notice that the total population stays at 10, but some people are now marked as dead. They remain in the dataset but are excluded from future aging and mortality calculations. You can check this by examining the output CSV.

---

## Part 4: What You've Accomplished

Congratulations! You've now learned:

1. ✅ **YAML Rules**: 
   - Use spaces, not tabs
   - Use the pipe (`|`) for multi-line strings
   - Column names must match your CSV exactly

2. ✅ **Lua in Plain English**:
   - `function` = "Here's a model"
   - `for` = "For each person"
   - `if` = "If this is true"
   - `math.random()` = "Roll a dice"

3. ✅ **Mortality Model**:
   - Applied age-specific death probabilities
   - Used `math.random()` for probabilistic events
   - Understood priority ordering (age first, then mortality)

---

## Summary

In this tutorial, you've learned:

1. **YAML Rules**: 
   - Use spaces, not tabs
   - Use the pipe (`|`) for multi-line strings
   - Column names must match your CSV exactly

2. **Lua in Plain English**:
   - Lua is just instructions that read like English
   - `function` = "Here's a model"
   - `for` = "For each person"
   - `if` = "If this is true"
   - `math.random()` = "Roll a dice"

3. **Mortality Model**:
   - Applied age-specific death probabilities
   - Used `math.random()` for probabilistic events
   - Understood priority ordering (age first, then mortality)

## Next Steps

In the next tutorial, you'll learn how to:
- Add fertility (births) to your model
- Track population growth
- Work with household-level data
- Create complex models like household formation

---

**Well done for completing Tutorial 2!** You now understand YAML rules, can write Lua models, and have built a working aging and mortality model. You're ready for more complex models.
