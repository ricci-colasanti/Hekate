# Tutorial 6: Using Hekate's Statistical Toolkit

Here's the complete tutorial, ready to save as `tutorials/tutorial6_stats.md`:

---

```markdown
# Hekate Tutorial 6: Using Hekate's Statistical Toolkit

## Overview

In this tutorial, you'll learn how to use Hekate's built-in statistical functions to build realistic, evidence-based demographic models. These functions let you translate findings from regression analyses directly into your simulation without hand-coding the math.

By the end of this tutorial, you'll be able to:

- Use **linear regression** to predict continuous outcomes like income
- Use **logistic regression** to model binary outcomes like mortality
- Use **Poisson regression** to model count outcomes like fertility
- Use **softmax** to model multi-category outcomes like education
- Sample from probability distributions reproducibly
- Combine multiple models into a realistic demographic simulation

## Prerequisites

- Completion of Tutorial 1, 2, and 3 (or equivalent knowledge)
- A working Hekate installation
- Familiarity with basic Lua (functions, loops, conditionals)
- Basic understanding of regression (coefficients, intercepts)

---

## Part 1: Why Built-in Statistics?

In earlier tutorials, we hand-coded probabilities directly in Lua:

```lua
-- Hand-coded mortality
if age < 30 then
  prob = 0.001
else
  prob = 0.05
end
```

This works for simple cases, but it breaks down when:

- The relationship depends on **many variables** (age AND sex AND income AND ...)
- You have **published regression coefficients** you want to use
- You need **statistical precision** (log-odds, Poisson means, etc.)
- You want models that are **scientifically defensible**

Hekate's statistical functions let you write:

```lua
-- Regression-based mortality
local died = hekate_stats.logistic_draw(
  -3.5,               -- intercept
  1.2, age / 10,      -- age effect (per decade)
  0.8, smoker,        -- smoking effect
  0.5, chronic        -- chronic condition effect
)
```

The coefficients come straight from a published study. The math is handled by Go. The results are reproducible.

### The Two Function Types

Hekate provides two categories of functions:

| Type | Purpose | Randomness? |
|------|---------|-------------|
| **Predict** (`*_predict`) | Compute a value or probability | No |
| **Draw** (`*_draw`) | Sample from a distribution | Yes |

Use **predict** when you want to record the probability, and **draw** when you want to make a decision.

---

## Part 2: Linear Regression — Income Model

Let's start with a familiar example: predicting income from age and education.

### The Regression Equation

A standard earnings regression looks like:

```
income = β₀ + β₁ × age + β₂ × education + β₃ × sex
```

Suppose we have these coefficients from a published study:

| Variable | Coefficient |
|----------|-------------|
| Intercept | 15,000 |
| Age | 500 |
| Education (per level) | 5,000 |
| Sex (female) | -3,000 |

### The Model in Hekate

```yaml
- name: "income"
  type: "lua_model"
  priority: 3
  enabled: true
  description: "Predict income from age, education, and sex"
  parameters:
    coefficients:
      intercept: 15000
      age: 500
      education: 5000
      sex: -3000
    script: |
      function transition(population, params)
        local coefs = params.coefficients
        
        for _, person in ipairs(population) do
          if person.alive == true or person.alive == "true" then
            local age = tonumber(person.age) or 30
            
            -- Convert education to a numeric score
            local edu_score = 0
            if person.education == "primary" then
              edu_score = 1
            elseif person.education == "secondary" then
              edu_score = 2
            elseif person.education == "tertiary" then
              edu_score = 3
            end
            
            -- Sex indicator (1 if female, 0 otherwise)
            local sex_score = 0
            if person.sex == "F" then
              sex_score = 1
            end
            
            -- Compute income using Hekate's linear regression
            local income = hekate_stats.linear_predict(
              coefs.intercept,
              coefs.age, age,
              coefs.education, edu_score,
              coefs.sex, sex_score
            )
            
            -- Add realistic variation
            local noise = 1 + (math.random() * 0.1 - 0.05)
            person.income = math.floor(income * noise + 0.5)
          end
        end
        return population
      end
```

### What Happens

For a 40-year-old woman with tertiary education:

```
income = 15000 + 500×40 + 5000×3 + (-3000)×1
       = 15000 + 20000 + 15000 - 3000
       = 47000
```

The `math.random()` at the end adds ±5% variation so not everyone with the same characteristics earns the same income.

### Try It

1. Create a small population with varying ages and education levels
2. Run with this model
3. Check the output: income should increase with age and education

---

## Part 3: Logistic Regression — Mortality Model

Logistic regression is the workhorse of demographic modelling. It predicts probabilities for binary outcomes.

### The Regression Equation

A standard mortality model looks like:

```
log-odds of death = β₀ + β₁ × age + β₂ × smoker
```

Which translates to:

```
P(death) = 1 / (1 + exp(-(β₀ + β₁ × age + β₂ × smoker)))
```

Suppose we have these coefficients:

| Variable | Coefficient |
|----------|-------------|
| Intercept | -4.0 |
| Age (per year) | 0.08 |
| Smoker (indicator) | 1.5 |

### Using `logistic_predict` vs `logistic_draw`

Hekate gives you two options:

**Option A: `logistic_predict` — compute the probability**

```lua
local death_prob = hekate_stats.logistic_predict(
  -4.0,
  0.08, age,
  1.5, smoker_score
)
-- Now death_prob is a number between 0 and 1
-- You can log it, use it in further calculations, etc.
```

**Option B: `logistic_draw` — make the decision**

```lua
local died = hekate_stats.logistic_draw(
  -4.0,
  0.08, age,
  1.5, smoker_score
)
-- Now died is either true or false
```

### The Model in Hekate

```yaml
- name: "mortality"
  type: "lua_model"
  priority: 2
  enabled: true
  description: "Logistic mortality model with age and smoking"
  parameters:
    coefficients:
      intercept: -4.0
      age: 0.08
      smoker: 1.5
    script: |
      function transition(population, params)
        local coefs = params.coefficients
        
        for _, person in ipairs(population) do
          if person.alive == true or person.alive == "true" then
            local age = tonumber(person.age) or 40
            
            -- Convert smoker to numeric indicator
            local smoker_score = 0
            if person.smoker == "true" or person.smoker == true then
              smoker_score = 1
            end
            
            -- Compute mortality probability (for logging)
            local death_prob = hekate_stats.logistic_predict(
              coefs.intercept,
              coefs.age, age,
              coefs.smoker, smoker_score
            )
            person.mortality_risk = death_prob
            
            -- Make the decision (using the same RNG)
            local died = hekate_stats.logistic_draw(
              coefs.intercept,
              coefs.age, age,
              coefs.smoker, smoker_score
            )
            
            if died then
              person.alive = false
            end
          end
        end
        return population
      end
```

### Why Both Functions?

- **`logistic_predict`** records the probability as a column so you can analyse it
- **`logistic_draw`** makes the yes/no decision for this year

They use the **same random source**, so the results are consistent.

### Sanity Check

For a 70-year-old smoker:

```
linear = -4.0 + 0.08×70 + 1.5×1 = -4.0 + 5.6 + 1.5 = 3.1
P(death) = 1 / (1 + exp(-3.1)) ≈ 0.957
```

That's a 95.7% chance of dying — plausible for that age/risk profile.

For a 30-year-old non-smoker:

```
linear = -4.0 + 0.08×30 + 1.5×0 = -4.0 + 2.4 = -1.6
P(death) = 1 / (1 + exp(1.6)) ≈ 0.168
```

About 17% — higher than expected, but that's because our intercept is quite high for illustration.

---

## Part 4: Poisson Regression — Fertility Model

Poisson regression predicts **counts** — things like number of children, doctor visits, or migrations.

### The Regression Equation

A Poisson regression looks like:

```
log(λ) = β₀ + β₁ × age + β₂ × married
λ = exp(β₀ + β₁ × age + β₂ × married)
```

Where λ is the expected count.

Suppose:

| Variable | Coefficient |
|----------|-------------|
| Intercept | -1.5 |
| Age | 0.05 |
| Married (indicator) | 0.8 |

### The Model in Hekate

```yaml
- name: "fertility"
  type: "lua_model"
  priority: 3
  enabled: true
  description: "Poisson fertility model"
  parameters:
    coefficients:
      intercept: -1.5
      age: 0.05
      married: 0.8
    script: |
      function transition(population, params)
        local coefs = params.coefficients
        
        for _, person in ipairs(population) do
          if person.alive == true or person.alive == "true" then
            if person.sex == "F" then
              local age = tonumber(person.age) or 30
              
              -- Only women of childbearing age
              if age >= 15 and age <= 50 then
                -- Married indicator
                local married_score = 0
                if person.partner_id ~= nil and person.partner_id ~= 0 then
                  married_score = 1
                end
                
                -- Expected number of children this year
                local lambda = hekate_stats.poisson_predict(
                  coefs.intercept,
                  coefs.age, age,
                  coefs.married, married_score
                )
                person.expected_children = lambda
                
                -- Sample actual number
                local new_children = hekate_stats.poisson_draw(
                  coefs.intercept,
                  coefs.age, age,
                  coefs.married, married_score
                )
                
                -- Update parity
                local current_parity = tonumber(person.parity) or 0
                person.parity = current_parity + new_children
              end
            end
          end
        end
        return population
      end
```

### What This Does

For a 30-year-old married woman:

```
λ = exp(-1.5 + 0.05×30 + 0.8×1)
  = exp(-1.5 + 1.5 + 0.8)
  = exp(0.8)
  ≈ 2.2
```

She's expected to have ~2.2 children per year. `poisson_draw` samples from Poisson(2.2), giving 0, 1, 2, 3, etc. with the appropriate probabilities.

**Wait — 2.2 children per year is very high!** In reality we'd need to scale this down. For a first experiment, use smaller coefficients:

```yaml
coefficients:
  intercept: -4.0
  age: 0.1
  married: 0.5
```

For a 30-year-old married woman:

```
λ = exp(-4.0 + 0.1×30 + 0.5×1) = exp(-0.5) ≈ 0.61
```

About 0.6 children per year — more realistic.

**Tuning these coefficients is part of the modelling process.** Published studies give you starting points, but you adjust them to match your population's known fertility patterns.

---

## Part 5: Softmax and Categorical Draw — Education Model

Softmax is for **multi-category** outcomes: education level, marital status, migration destination.

### The Idea

Given a set of linear "scores" for each category, softmax converts them into probabilities that sum to 1. Then you sample a category using `categorical_draw`.

### The Model in Hekate

Let's model education transitions at age 16. Every 16-year-old chooses between:

- **Primary** (leave school early)
- **Secondary** (finish secondary school)
- **Tertiary** (go on to university)

```yaml
- name: "education"
  type: "lua_model"
  priority: 3
  enabled: true
  description: "Education transitions at age 16"
  parameters:
    script: |
      function transition(population, params)
        for _, person in ipairs(population) do
          if person.alive == true or person.alive == "true" then
            local age = tonumber(person.age) or 0
            
            -- Only trigger at age 16
            if age == 16 and person.sex == "F" then
              -- Parent's education as a numeric score
              local parent_edu = 0
              -- (In practice, look up the mother)
              
              -- Scores for each category
              -- Higher score = more likely to be chosen
              local primary_score = {0.5, 0.1, age, -0.2, parent_edu}
              local secondary_score = {1.0, 0.3, age, 0.5, parent_edu}
              local tertiary_score = {0.2, 0.5, age, 0.8, parent_edu}
              
              -- Get probabilities via softmax
              local probs = hekate_stats.softmax_predict(
                primary_score,
                secondary_score,
                tertiary_score
              )
              
              -- probs is now a table like {0.15, 0.55, 0.30}
              
              -- Sample a category
              local chosen = hekate_stats.categorical_draw(
                probs[1], probs[2], probs[3]
              )
              
              -- Map the 1-based index back to an education name
              local education_levels = {"primary", "secondary", "tertiary"}
              person.education = education_levels[chosen]
            end
          end
        end
        return population
      end
```

### Understanding Softmax

Given three scores `[s1, s2, s3]`, softmax computes:

```
p_i = exp(s_i) / (exp(s1) + exp(s2) + exp(s3))
```

For example, with scores `[0.5, 1.0, 0.2]`:

```
p1 = exp(0.5) / (exp(0.5) + exp(1.0) + exp(0.2))
   = 1.65 / (1.65 + 2.72 + 1.22)
   = 1.65 / 5.59
   ≈ 0.29

p2 ≈ 0.49
p3 ≈ 0.22
```

So the middle category is most likely — which makes sense given it has the highest score.

### `categorical_draw` in Action

```lua
local chosen = hekate_stats.categorical_draw(0.29, 0.49, 0.22)
-- chosen is 1, 2, or 3 with the given probabilities
```

### Why This Design?

- **Softmax** ensures probabilities sum to 1 (mathematically correct)
- **Categorical draw** uses `vm.random` so it's reproducible
- **Vector scores** let you encode multi-variable regressions compactly

---

## Part 6: Putting It All Together

Now let's build a **complete demographic model** with all four techniques.

### The Population

First, a small test population. Create `population_stats.csv`:

```csv
person_id,age,sex,area_id,alive,education,smoker,partner_id,parity
1,25,F,1,true,secondary,false,2,0
2,27,M,1,true,secondary,false,1,0
3,45,F,1,true,tertiary,false,4,2
4,48,M,1,true,tertiary,false,3,0
5,70,F,1,true,primary,true,0,3
6,72,M,1,true,secondary,false,0,0
7,35,F,1,true,secondary,false,8,1
8,37,M,1,true,secondary,false,7,0
9,15,F,1,true,primary,false,0,0
10,17,M,1,true,primary,false,0,0
11,60,F,1,true,primary,true,0,2
12,62,M,1,true,primary,false,0,0
```

### The Complete Config

Create `config_stats.yaml`:

```yaml
simulation:
  iterations: 20
  population_file: "population_stats.csv"
  output_file: "population_stats_output.csv"
  random_seed: 42
  verbose: true
  id_column: "person_id"

models:
  # ---- Priority 1: Aging ----
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

  # ---- Priority 2: Income (linear regression) ----
  - name: "income"
    type: "lua_model"
    priority: 2
    enabled: true
    parameters:
      coefficients:
        intercept: 15000
        age: 500
        education: 5000
        sex: -3000
      script: |
        function transition(population, params)
          local coefs = params.coefficients
          for _, person in ipairs(population) do
            if person.alive == true or person.alive == "true" then
              local age = tonumber(person.age) or 30
              local edu_score = 0
              if person.education == "primary" then
                edu_score = 1
              elseif person.education == "secondary" then
                edu_score = 2
              elseif person.education == "tertiary" then
                edu_score = 3
              end
              local sex_score = 0
              if person.sex == "F" then
                sex_score = 1
              end
              
              -- Income only applies to working-age adults
              if age >= 18 and age <= 65 then
                local income = hekate_stats.linear_predict(
                  coefs.intercept,
                  coefs.age, age,
                  coefs.education, edu_score,
                  coefs.sex, sex_score
                )
                local noise = 1 + (math.random() * 0.1 - 0.05)
                person.income = math.floor(income * noise + 0.5)
              else
                person.income = 0
              end
            end
          end
          return population
        end

  # ---- Priority 3: Education transitions (softmax + categorical_draw) ----
  - name: "education"
    type: "lua_model"
    priority: 3
    enabled: true
    parameters:
      script: |
        function transition(population, params)
          for _, person in ipairs(population) do
            if person.alive == true or person.alive == "true" then
              local age = tonumber(person.age) or 0
              
              -- Education decision at 16
              if age == 16 then
                local primary = {0.0, 0.2, age}
                local secondary = {1.0, 0.3, age}
                local tertiary = {-0.5, 0.5, age}
                
                local probs = hekate_stats.softmax_predict(primary, secondary, tertiary)
                local chosen = hekate_stats.categorical_draw(probs[1], probs[2], probs[3])
                person.education = ({"primary", "secondary", "tertiary"})[chosen]
              end
            end
          end
          return population
        end

  # ---- Priority 4: Fertility (Poisson regression) ----
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
            if person.alive == true or person.alive == "true" then
              if person.sex == "F" then
                local age = tonumber(person.age) or 30
                if age >= 15 and age <= 50 then
                  local married = 0
                  if person.partner_id ~= nil and person.partner_id ~= 0 then
                    married = 1
                  end
                  local new_children = hekate_stats.poisson_draw(
                    coefs.intercept,
                    coefs.age, age,
                    coefs.married, married
                  )
                  local current_parity = tonumber(person.parity) or 0
                  person.parity = current_parity + new_children
                end
              end
            end
          end
          return population
        end

  # ---- Priority 5: Mortality (logistic regression) ----
  - name: "mortality"
    type: "lua_model"
    priority: 5
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
              
              local died = hekate_stats.logistic_draw(
                coefs.intercept,
                coefs.age, age,
                coefs.smoker, smoker
              )
              
              if died then
                person.alive = false
              end
            end
          end
          return population
        end
```

### Run It

```bash
./hekate config_stats.yaml
```

### What You'll See

The output CSV will have **new columns** that weren't in the input:

| Column | Added By |
|--------|----------|
| `income` | Income model |
| `education` | Education model (may change) |
| `parity` | Fertility model |
| `alive` | Mortality model (may change) |

Over 20 years, you'll see:

- **Aging**: everyone gets older
- **Income**: working-age adults earn income based on their education
- **Education**: 16-year-olds transition to their final education level
- **Fertility**: women accumulate children
- **Mortality**: older people and smokers die more often

---

## Part 7: Reproducibility Check

The whole point of Hekate's statistical functions is that they're **reproducible**. Let's verify:

```bash
# Run twice
./hekate config_stats.yaml && mv population_stats_output.csv run1.csv
./hekate config_stats.yaml && mv population_stats_output.csv run2.csv

# Compare
diff run1.csv run2.csv
```

**Expected: no output.** The two runs are byte-for-byte identical.

Try changing the seed:

```yaml
simulation:
  random_seed: 999  # Different seed
```

Run again. You'll get **different results** — but the results are still reproducible (run it twice with seed 999 and you'll get identical files).

---

## Part 8: Debugging Tips

### Tip 1: Log the Predicted Values

Before you commit to a draw, log the predicted probability to check your coefficients:

```lua
local death_prob = hekate_stats.logistic_predict(
  coefs.intercept,
  coefs.age, age,
  coefs.smoker, smoker
)
log("Person " .. person.person_id .. " age " .. age .. " death prob: " .. death_prob)
```

Run once and check the range. If you see probabilities of 0.9999 or 0.0001 everywhere, your coefficients are miscalibrated.

### Tip 2: Check the Sum of Softmax

`softmax_predict` should always return values that sum to 1. If they don't, you have a bug in the score construction:

```lua
local probs = hekate_stats.softmax_predict(...)
log("Sum: " .. (probs[1] + probs[2] + probs[3]))
-- Should be 1.0
```

### Tip 3: Watch Out for Extreme Coefficients

The exponential in logistic and Poisson regressions can overflow:

```lua
-- BAD: exp(100) overflows
hekate_stats.logistic_predict(100, 1.0, age)

-- GOOD: intercepts are typically -10 to +10
hekate_stats.logistic_predict(-3.5, 0.1, age)
```

If you see `inf` or `nan` in your output, you've likely overflowed.

### Tip 4: Baseline Before You Model

Before writing the regression, compute a **baseline**. For mortality, ask:

- What's the overall annual death rate?
- What should it be for a 20-year-old? A 70-year-old?

Then choose coefficients that produce those rates. You can sanity-check with a simple test:

```lua
-- Log the probability for a few reference ages
log("Age 20: " .. hekate_stats.logistic_predict(-6.0, 0.09, 20))
log("Age 50: " .. hekate_stats.logistic_predict(-6.0, 0.09, 50))
log("Age 80: " .. hekate_stats.logistic_predict(-6.0, 0.09, 80))
```

Adjust the intercept until the numbers look right.

### Tip 5: Use the Predict Variant First

When building a new model, start with `*_predict` and log values. Once the numbers look sensible, switch to `*_draw` and let it decide.

---

## Part 9: Summary Table

| Function | Returns | When to Use |
|----------|---------|-------------|
| `linear_predict` | A number | Continuous outcomes (income, wealth) |
| `linear_predict_default` | A number (nil-safe) | Same, when data may be missing |
| `logistic_predict` | Probability (0–1) | Binary outcomes (probability only) |
| `logistic_draw` | Boolean | Binary outcomes (make the decision) |
| `poisson_predict` | Expected count (λ) | Counts (expected value only) |
| `poisson_draw` | Integer count | Counts (sample the value) |
| `softmax_predict` | Table of probabilities | Multi-category (probability only) |
| `categorical_draw` | Integer index | Multi-category (sample the category) |

---

## Part 10: Where to Go From Here

You've now seen the full statistical toolkit in action. A few directions to explore:

1. **Look up published coefficients** for your domain and use them directly
2. **Add interactions** — multiply two variables together before passing them in
3. **Combine models** — use the output of one regression as input to another
4. **Run sensitivity analysis** — change one coefficient by 10% and see how the population changes
5. **Build your own composite indicators** — e.g., a "health score" from BMI, smoking, and age

### Further Reading

- **Regressions in demography**: The standard textbooks by Preston, Heuveline, and Guillot (Demography) or Vandeschrick (Demographic Analysis)
- **Coefficient sources**: The Human Mortality Database, the World Fertility Survey, and national statistics offices all publish regression results you can use
- **Sensitivity analysis**: Sobol indices, Monte Carlo methods

### A Final Note on Scientific Modelling

The point of using regression coefficients isn't to be more "mathematical" — it's to make your model **traceable**. When you write:

```lua
local died = hekate_stats.logistic_draw(-4.0, 0.08, age)
```

You can say: *"This uses coefficients from Smith et al. (2023), Table 3, Model 2."*

If someone challenges your result, you can point to the paper. If someone wants a different assumption, they change the coefficients. The model is transparent, defensible, and easy to share.

**That's what makes Hekate's statistical toolkit worth learning.**

---

**Well done for completing Tutorial 6!** You now know how to build regression-based demographic models in Hekate. You can predict continuous, binary, count, and categorical outcomes — all reproducible, all shareable as YAML configs.
