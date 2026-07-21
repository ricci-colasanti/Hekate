# Hekate Tutorial 1: Building an Aging Model

## Overview

In this tutorial, you'll learn how to create a simple aging model with Hekate. We'll start with a CSV population file, write a configuration that ages everyone by one year, run the simulation, and analyze the output.

## Prerequisites

- Hekate binary downloaded and in your PATH (or in the current directory)
- Basic understanding of CSV files
- A text editor (VS Code, Sublime, Notepad++, etc.)

## What You'll Learn

By the end of this tutorial, you'll be able to:
- Create a population CSV file
- Write a YAML configuration file
- Run a Hekate simulation
- Understand the basic structure of a Hekate configuration

**Important Note:** You don't need to be a programmer to use Hekate! We're using **Lua**, a simple and readable scripting language, for our models. Think of it as writing instructions in plain English:

- **Change**: "Make everyone one year older" → `person.age = person.age + 1`

No complex programming, no compilation - just simple instructions that read like plain English!

---

## Why Lua Code Inside YAML?

You might wonder: **"Why do I need to write Lua code inside a YAML configuration file?"**

### The Role of the Code

The Lua script is the **brain** of your model. It tells Hekate **what** to do with your population data. Think of it this way:

| Component | What it does | Analogy |
|-----------|--------------|---------|
| **CSV** | Stores your data | The raw ingredients |
| **YAML** | Configures the simulation | The recipe book |
| **Lua** | Defines the logic | The cooking instructions |

Without the Lua script, Hekate wouldn't know:
- How people should age
- Who should die and when
- Who should have children
- How people should move between areas

### The Advantages

**1. You Can Change Behavior Without Changing Code**

Traditional microsimulation systems require you to edit and recompile the source code to change model behavior. With Hekate, you just edit the Lua script in the YAML file:

```yaml
# Change this:
person.age = person.age + 1

# To this:
person.age = person.age + 2
```

No recompilation needed! Just edit, save, and rerun.

**2. Models Are Self-Documenting**

The Lua script is right there in the configuration file. You can read exactly what the model does:

```lua
-- This is easy to understand!
if person.alive == true then
  person.age = person.age + 1  -- Everyone alive gets older
end
```

**3. Models Are Portable**

Because the model logic is in the YAML file, you can share it with colleagues. They can run your exact model without needing to install or compile anything.

**4. Models Are Auditable**

You can version control your models (using Git, for example). Every change to the model is tracked and visible.

**5. You Can Experiment Freely**

Want to try a different aging rule? Edit the script and rerun. Want to test a new fertility model? Edit the script. No waiting for compilation, no complex development environment.

### Why Lua?

**Lua is a scripting language designed to be embedded in other applications.** It's:

| Feature | Why It Matters |
|---------|----------------|
| **Simple** | Lua has a very small set of concepts to learn. You can learn the basics in minutes. |
| **Readable** | Lua reads like plain English. `if person.alive == true then` makes immediate sense. |
| **Fast** | Lua is one of the fastest scripting languages. It can handle populations of millions. |
| **Small** | The entire Lua interpreter fits in a tiny binary. No bloat. |
| **Embedded** | Lua is designed to be embedded in applications like Hekate. It just works. |
| **Proven** | Lua is used in games, embedded systems, and scientific applications worldwide. |

**What Lua Is NOT:**

- ❌ It's not complex like Python or Java
- ❌ It's not difficult to learn
- ❌ It's not a general-purpose programming language for building applications
- ❌ You don't need to be a programmer to use it

### What You Actually Need to Know

For Hekate, you only need to know a handful of Lua concepts:

| Concept | What it does | Example |
|---------|--------------|---------|
| `function` | Defines a model | `function transition(population, params)` |
| `for` | Loops through people | `for _, person in ipairs(population) do` |
| `if` | Checks a condition | `if person.alive == true then` |
| `==` | Compares values | `person.alive == true` |
| `=` | Assigns a value | `person.age = person.age + 1` |
| `return` | Returns a result | `return population` |

**That's it!** These 6 concepts are all you need for most demographic models.

---

## Step 1: Create a Population CSV

First, let's create a small population to work with. Create a file called `population.csv`:

```csv
person_id,age,sex,area_id,alive
1,25,F,1,true
2,30,M,1,true
3,45,F,1,true
4,68,M,1,true
5,82,F,1,true
6,2,M,1,true
7,15,F,1,true
8,35,M,1,true
9,55,F,1,true
10,70,M,1,true
```

This gives us 10 individuals with various ages. The columns are:
- `person_id`: Unique identifier for each person
- `age`: Age in years
- `sex`: Gender (M/F)
- `area_id`: Geographic area (we'll use just one area for now)
- `alive`: Whether the person is alive (true/false)

**Note about `area_id`:** This column is used by Hekate's streaming mode to process populations area-by-area. We'll cover streaming mode in detail in **Tutorial 5: Large-Scale Simulations with Streaming Mode**. For now, we'll use the simpler **Bulk Mode** which loads everything into memory at once.

---

## Step 2: Understanding YAML

YAML (YAML Ain't Markup Language) is a human-readable data format that Hekate uses for configuration.

**Important Rule:** YAML uses **spaces** for indentation, **never tabs**. The number of spaces doesn't matter as long as it's consistent (2 spaces is standard). 

That's it for now! We'll cover more YAML rules in the next tutorial. For now, just copy the configuration below exactly as shown.

---

## Step 3: Create the Configuration File

Create a file called `config_aging.yaml` and paste in the following:

```yaml
# config_aging.yaml
# A simple aging model configuration

simulation:
  iterations: 5                    # Run for 5 years
  population_file: "population.csv" # Input population
  output_file: "population_aged.csv" # Output after aging
  random_seed: 42                   # For reproducibility
  verbose: true                     # Show detailed output
  id_column: "person_id"            # REQUIRED: Unique identifier column
  streaming_mode: false             # Use bulk mode (simpler for small populations)

models:
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
```

**Note about `streaming_mode: false`:** We're using **Bulk Mode** in this tutorial. This loads the entire population into memory at once, which is simpler and faster for small populations. For large populations (1M+), we'll use Streaming Mode which processes area-by-area. We'll cover this in depth in **Tutorial 5: Large-Scale Simulations with Streaming Mode**.

---

## Step 4: Run the Simulation

Now run Hekate with your configuration:

```bash
# If hekate is in your PATH
hekate config_aging.yaml

# Or if hekate is in the current directory
./hekate config_aging.yaml
```

### Expected Output

You should see output similar to this:

```
2024/01/15 10:00:00 ═══ Hekate: Microsimulation Engine ═══
2024/01/15 10:00:00 Iterations: 5
2024/01/15 10:00:00 Population file: population.csv
2024/01/15 10:00:00 ID column: person_id
2024/01/15 10:00:00 Mode: BULK (load all into memory)
2024/01/15 10:00:00 Models loaded: 1
2024/01/15 10:00:00 Loaded 10 individuals with 4 columns
2024/01/15 10:00:00 Columns: [person_id age sex area_id alive]
2024/01/15 10:00:00 Enabled models: 1
2024/01/15 10:00:00   - age_increment (priority: 1)

2024/01/15 10:00:00 ═══ Iteration 1/5 ═══
2024/01/15 10:00:00   ▶ age_increment

2024/01/15 10:00:00 ═══ Iteration 2/5 ═══
2024/01/15 10:00:00   ▶ age_increment

2024/01/15 10:00:00 ═══ Iteration 3/5 ═══
2024/01/15 10:00:00   ▶ age_increment

2024/01/15 10:00:00 ═══ Iteration 4/5 ═══
2024/01/15 10:00:00   ▶ age_increment

2024/01/15 10:00:00 ═══ Iteration 5/5 ═══
2024/01/15 10:00:00   ▶ age_increment

2024/01/15 10:00:00 ═══ Simulation Complete ═══
2024/01/15 10:00:00 Results saved to population_aged.csv
```

---

## Step 5: Examine the Output

Open `population_aged.csv`:

```csv
person_id,age,sex,area_id,alive
1,30,F,1,true
2,35,M,1,true
3,50,F,1,true
4,73,M,1,true
5,87,F,1,true
6,7,M,1,true
7,20,F,1,true
8,40,M,1,true
9,60,F,1,true
10,75,M,1,true
```

Notice that everyone has aged exactly 5 years:
- Person 1: 25 → 30
- Person 2: 30 → 35
- Person 3: 45 → 50
- Person 4: 68 → 73
- Person 5: 82 → 87
- Person 6: 2 → 7
- Person 7: 15 → 20
- Person 8: 35 → 40
- Person 9: 55 → 60
- Person 10: 70 → 75

---

## Understanding Your Configuration

Now let's break down what the configuration does. The structure is simple - two main sections:

### 1. The Simulation Section

```yaml
simulation:
  iterations: 5                    # Run for 5 years
  population_file: "population.csv" # Input population
  output_file: "population_aged.csv" # Output after aging
  random_seed: 42                   # For reproducibility
  verbose: true                     # Show detailed output
  id_column: "person_id"            # REQUIRED: Unique identifier column
  streaming_mode: false             # Use bulk mode (simpler for small populations)
```

This tells Hekate:
- **How many times** to run the model (`iterations: 5`)
- **Where to read** the population from (`population_file`)
- **Where to save** the results (`output_file`)
- **What random seed** to use for reproducible results (`random_seed: 42`)
- **Whether to show detailed output** (`verbose: true`)
- **Which column is the unique identifier** (`id_column: "person_id"`)
- **Whether to use streaming mode** (`streaming_mode: false`)

**Why is `id_column` required?** Every individual in your population needs a unique identifier. This is how Hekate keeps track of each person throughout the simulation. When people age, die, or have children, Hekate uses this ID to ensure it's updating the right person. Without a unique ID column, Hekate wouldn't be able to distinguish between individuals or link family members together. The `id_column` tells Hekate which column in your CSV contains these unique identifiers. Ordering the output by this column is a secondary benefit that makes it easier to compare results across different runs.

**Why `streaming_mode: false`?** We're using **Bulk Mode** in this tutorial. This is the simplest mode - it loads the entire population into memory, processes all people, then saves the results. It's perfect for small populations like our 10-person example.

For **large populations** (1M+ people), we use **Streaming Mode** (`streaming_mode: true`). This processes the population area-by-area, keeping memory usage low regardless of population size. We'll cover Streaming Mode in detail in **Tutorial 5: Large-Scale Simulations with Streaming Mode**.

### 2. The Models Section

```yaml
models:
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
```

This defines our aging model:

- **`name`**: A descriptive name for the model
- **`type`**: The type of model (`lua_model` means it's a Lua script)
- **`priority`**: The order to run models (lower numbers run first)
- **`enabled`**: Whether this model is active (`true` or `false`)
- **`description`**: Human-readable description
- **`parameters.script`**: The Lua code that does the work

---

## Understanding the Lua Script (In Detail)

Let's look at our model's Lua script:

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

**First, what does this Lua do in plain English?**

> "Go through everyone in the population. For each person who is alive, take their current age, add 1 to it, and save that new age back to the population."

### Model Functions

Each individual model is described as a Lua function. All model functions are called **`transition`** functions. This is a requirement of Hekate.

### Why Are They Called `transition` Functions?

In demographic modeling, a **transition** is a change from one state to another. For example:
- **Age transition**: Moving from age 25 to age 26
- **Mortality transition**: Moving from "alive" to "dead"
- **Migration transition**: Moving from "area 1" to "area 2"

Each year, Hekate runs your `transition` functions to apply these changes to your population. That's why they're called transition functions - they describe how people **transition** from one state to another.

### How Hekate Finds Your Transition Function

When Hekate runs your model, it needs to know which function to execute. Rather than guessing, Hekate looks for a function with the exact name `transition`. This is similar to how:

- A car needs a steering wheel (it knows exactly where to look)
- A TV remote needs a power button (it knows exactly what to press)
- A recipe needs ingredients (it knows exactly what to add)

By requiring a specific function name, Hekate can reliably find and run your model every year.

### What Happens If You Use a Different Name?

If you call your model function something else, Hekate won't find it and will throw an error:

```
ERROR: script must define a 'transition' function
```

**Example:**

```lua
-- ❌ WRONG - Hekate won't find this
function my_model(population, params)
  for _, person in ipairs(population) do
    person.age = person.age + 1
  end
  return population
end

-- ✅ CORRECT - Hekate finds this
function transition(population, params)
  for _, person in ipairs(population) do
    person.age = person.age + 1
  end
  return population
end
```

### Summary

| Script Type | Required Function Name | What It Does |
|-------------|------------------------|--------------|
| Model | `transition(population, params)` | Runs each year, transforms the population |

**Remember:** The function name must be **exactly** `transition` - case-sensitive!

| Correct | Incorrect |
|---------|-----------|
| `function transition(population, params)` | `function Transition(population, params)` |

### Understanding `ipairs`

You might also wonder: **"What is `ipairs` and why do we use it?"**

`ipairs` is a Lua function that helps us **loop through a list in order**. Think of it like this:

```lua
for _, person in ipairs(population) do
  -- Do something with each person
end
```

**In plain English:** "For each person in the population list, one at a time, do the following..."

**Breaking down the parts:**

| Part | What it means | In plain English |
|------|---------------|------------------|
| `for` | Start a loop | "For each..." |
| `_` | The position/index (we don't need it) | "Ignore the position number" |
| `person` | The current item | "...this person..." |
| `in` | From the list | "...from the list..." |
| `ipairs(population)` | The list to loop through | "...the population list, in order" |
| `do` | Start the instructions | "...do this:" |

**Why do we use `ipairs` and not just `for`?**

Lua has two ways to loop through lists:

| Method | What it does | When to use |
|--------|--------------|-------------|
| `ipairs(list)` | Loops in order, stops at first nil | For lists like our population |
| `pairs(list)` | Loops in any order, includes all items | For dictionaries/maps |

Since our population is a simple list of people in order, we use `ipairs`.

**What about the underscore (`_`)?**

In Lua, `_` is a convention meaning "I don't need this value." When we loop with `ipairs`, we get both the position number and the person:

```lua
for index, person in ipairs(population) do
  -- index = 1, 2, 3, ...
  -- person = the person at that position
end
```

Since we only care about the person, not their position, we use `_` instead of `index`:

```lua
for _, person in ipairs(population) do
  -- We ignore the position and just work with each person
end
```

### How CSV Headers Link to `person.age`

This is a crucial question: **"How does `person.age` connect to my CSV column called `age`?"**

**The short answer:** When Hekate loads your CSV, it creates a table (Lua's version of a dictionary/object) for each person. The column names become the **keys**, and the values become the **values**.

**Here's how it works:**

**1. Your CSV file:**
```csv
person_id,age,sex,area_id,alive
1,25,F,1,true
2,30,M,1,true
```

**2. Hekate loads each row as a Lua table:**

When Hekate reads row 1, it creates a table like this:

```lua
{
  person_id = 1,
  age = 25,
  sex = "F",
  area_id = 1,
  alive = true
}
```

**3. You access these values using the column names:**

```lua
-- Access the age column
person.age        -- Returns 25

-- Access the sex column
person.sex        -- Returns "F"

-- Access the alive column
person.alive      -- Returns true
```

**The key point:** **The column names in your CSV become the property names in Lua.**

**Examples:**

| CSV Column Name | Lua Access | Value for Row 1 |
|-----------------|------------|-----------------|
| `person_id` | `person.person_id` | 1 |
| `age` | `person.age` | 25 |
| `sex` | `person.sex` | "F" |
| `area_id` | `person.area_id` | 1 |
| `alive` | `person.alive` | true |

**Important:** The column names are **case-sensitive**!

| CSV Header | Lua Access | Works? |
|------------|------------|--------|
| `age` | `person.age` | ✅ Yes |
| `age` | `person.Age` | ❌ No |
| `alive` | `person.alive` | ✅ Yes |
| `alive` | `person.Alive` | ❌ No |
| `person_id` | `person.person_id` | ✅ Yes |
| `person_id` | `person.personId` | ❌ No |

**Why does this matter?** If you mistype a column name, your script will fail. For example:

```lua
-- ❌ WRONG - CSV has 'alive' but we wrote 'Alive'
if person.Alive == true then
  person.age = person.age + 1
end

-- ✅ CORRECT - matches CSV header exactly
if person.alive == true then
  person.age = person.age + 1
end
```

### The Complete Picture

**When you write:**
```lua
person.age = person.age + 1
```

**Here's what happens step-by-step:**

1. Hekate loads the CSV and creates a `person` table
2. `person.age` accesses the value from the `age` column
3. `person.age + 1` calculates the new age
4. `person.age = ...` stores the new value back

**Example with person 1 (age 25):**

| Step | Code | What happens |
|------|------|--------------|
| 1 | `person.age` | Gets 25 from the table |
| 2 | `person.age + 1` | Calculates 26 |
| 3 | `person.age = 26` | Stores 26 back in the table |

**After the script runs:** Person 1's age is now 26!

### Visualizing the Data

**Before the model runs:**
```
Population = {
  { person_id = 1, age = 25, sex = "F", area_id = 1, alive = true },
  { person_id = 2, age = 30, sex = "M", area_id = 1, alive = true },
  ...
}
```

**After the model runs:**
```
Population = {
  { person_id = 1, age = 26, sex = "F", area_id = 1, alive = true },
  { person_id = 2, age = 31, sex = "M", area_id = 1, alive = true },
  ...
}
```

### Summary of Lua Concepts

| Concept | Explanation |
|---------|-------------|
| `transition` | **Required name** for model functions |
| `ipairs` | Loops through a list in order |
| `_` | Means "I don't need this value" |
| `person` | The current person being processed |
| `person.age` | Accesses the `age` column from the CSV |
| `person.alive` | Accesses the `alive` column from the CSV |
| Column names | Must match your CSV header exactly (case-sensitive!) |

---

## Understanding the Configuration Structure

Now let's put it all together. The configuration has two main sections:

### 1. Simulation Section
- Controls how the simulation runs
- `iterations`: How many years
- `population_file`: Where to read data from
- `output_file`: Where to save results
- `id_column`: Which column contains the unique ID (REQUIRED)
- `streaming_mode`: Whether to use bulk or streaming mode

### 2. Models Section
- Defines what happens each year
- Each model has:
  - `name`: What to call it
  - `type`: What kind of model (`lua_model`)
  - `priority`: When to run it (lower = earlier)
  - `script`: The Lua code that does the work (MUST have a `transition` function)

---


**We'll cover Streaming Mode in depth in Tutorial 5.** For now, Bulk Mode is perfect for learning and small populations.

---

## What You've Accomplished

Congratulations! You've successfully:

1. ✅ Created a population CSV file
2. ✅ Written a YAML configuration file
3. ✅ Run a Hekate simulation
4. ✅ Aged an entire population by 5 years
5. ✅ Understood how Lua connects to your CSV data
6. ✅ Learned that model functions must be called `transition`
7. ✅ Understood why the `id_column` is required
8. ✅ Learned about Bulk Mode vs Streaming Mode

You now know the basic workflow for using Hekate.

---

## Next Steps

In the next tutorial, we'll dive deeper into YAML and Lua. You'll learn:

- **YAML rules**: Why indentation matters, why you need the pipe (`|`) for multi-line strings, and how to avoid common mistakes
- **More Lua**: Working with age and conditions in your models
- **Mortality**: Adding a mortality model to make your simulation more realistic
- **Streaming Mode**: How to use it effectively for large populations (Tutorial 5)

For now, take a moment to appreciate what you've built - a working demographic microsimulation model!

---

**Tutorial 2 Preview:** We'll explore the YAML and Lua rules in detail and add a mortality model with age-specific death probabilities.
```
