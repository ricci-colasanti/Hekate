# Tutorial 4: Using Linear Regression in Hekate

## Overview

Hekate includes powerful built-in statistical functions that you can call directly from your Lua scripts. In this tutorial, you'll learn how to use the `linear_predict()` function to build predictive models for continuous outcomes like income, health scores, or any other numeric value.

## What You'll Learn

- How to use `hekate_stats.linear_predict()` in your Lua scripts
- How to define coefficients in YAML
- How to handle categorical variables (education, gender, etc.)
- How to add randomness to your predictions
- How to validate your model results

## Prerequisites

Before starting this tutorial, you should have:
- Completed Tutorial 1: Building an Aging Model
- Basic understanding of YAML and Lua
- Hekate binary downloaded and working

---

## Understanding Linear Regression

Linear regression models the relationship between a dependent variable (what you want to predict) and one or more independent variables (what you're using to predict):

**Formula:** `y = intercept + coef₁×var₁ + coef₂×var₂ + ...`

In Hekate, this becomes:

```lua
local prediction = hekate_stats.linear_predict(
    intercept,    -- The constant term
    coef1, var1,  -- First coefficient-variable pair
    coef2, var2,  -- Second coefficient-variable pair
    ...
)
```

---

## Step 1: Creating a Population for Income Modeling

Let's create a population with characteristics that influence income.

### Generate Population

Create a file called `generate_income_population.py`:

```python
#!/usr/bin/env python3
"""
Generate population for income modeling tutorial
"""

import csv
import random

random.seed(42)

def generate_population(n=10000):
    with open('population_income.csv', 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow([
            'id', 'age', 'gender', 'education', 'income', 
            'experience', 'alive', 'region'
        ])
        
        for i in range(n):
            person_id = f"P{i+1:06d}"
            
            # Age (20-70)
            age = random.randint(20, 70)
            
            # Gender
            gender = random.choice(['male', 'female'])
            
            # Education (categorical)
            education = random.choices(
                ['none', 'primary', 'secondary', 'tertiary'],
                weights=[0.10, 0.25, 0.35, 0.30]
            )[0]
            
            # Experience (correlated with age and education)
            experience = max(0, age - 20 - random.randint(0, 5))
            if education in ['secondary', 'tertiary']:
                experience -= 2
            
            # Income (will be overwritten by model)
            base_income = 20000 + age * 200
            if education == 'tertiary':
                base_income += 10000
            elif education == 'secondary':
                base_income += 5000
            income = int(base_income + random.gauss(0, 5000))
            income = max(10000, income)
            
            # Alive (95%)
            alive = random.random() < 0.95
            
            # Region
            region = random.choice(['north', 'south', 'east', 'west'])
            
            writer.writerow([
                person_id, age, gender, education, income,
                experience, str(alive).lower(), region
            ])
    
    print(f"Generated {n} individuals in population_income.csv")

if __name__ == "__main__":
    generate_population(10000)
```

Run it:
```bash
python3 generate_income_population.py
```

---

## Step 2: Building a Simple Income Predictor

Now let's create a model that predicts income using `linear_predict()`.

### Create config_income.yaml

```yaml
simulation:
  iterations: 1
  population_file: population_income.csv
  output_file: output_income.csv
  random_seed: 42
  verbose: true
  id_column: id
  streaming_mode: false   # Using bulk mode for simplicity

models:
  - name: income_predictor
    type: lua_model
    priority: 1
    enabled: true
    description: "Predict income using linear regression"
    parameters:
      coefficients:
        intercept: 15000
        age: 300
        education: 5000
        gender: -2500
        experience: 400
      script: |
        function transition(population, params)
            local coefs = params.coefficients
            
            for _, person in ipairs(population) do
                local alive = false
                if person.alive == true or person.alive == "true" then
                    alive = true
                end
                
                if alive then
                    -- Convert education to numeric score
                    local edu_score = 0
                    if person.education == "tertiary" then
                        edu_score = 3
                    elseif person.education == "secondary" then
                        edu_score = 2
                    elseif person.education == "primary" then
                        edu_score = 1
                    end
                    
                    -- Convert gender to numeric
                    local gender_score = 0
                    if person.gender == "female" then
                        gender_score = 1
                    end
                    
                    -- Predict income using linear regression
                    local income = hekate_stats.linear_predict(
                        coefs.intercept,
                        coefs.age, person.age,
                        coefs.education, edu_score,
                        coefs.gender, gender_score,
                        coefs.experience, person.experience
                    )
                    
                    -- Add some randomness
                    local noise = 1 + (math.random() * 0.1 - 0.05)
                    person.income = math.floor(income * noise + 0.5)
                    
                    -- Ensure minimum income
                    if person.income < 8000 then
                        person.income = 8000
                    end
                end
            end
            
            return population
        end
```

**Note:** We're using Bulk Mode (`streaming_mode: false`) here. For large populations (1M+), you can use Streaming Mode - the regression models work exactly the same way. Streaming Mode is covered in detail in Tutorial 5.

### Understanding the Model

Let's break down what's happening:

**1. Coefficients defined in YAML:**
```yaml
coefficients:
  intercept: 15000      # Base income
  age: 300              # $300 per year of age
  education: 5000       # $5000 per education level
  gender: -2500         # $2500 less for females
  experience: 400       # $400 per year of experience
```

**2. Categorical variables converted to numbers:**
```lua
-- Education: none=0, primary=1, secondary=2, tertiary=3
local edu_score = 0
if person.education == "tertiary" then
    edu_score = 3
elseif person.education == "secondary" then
    edu_score = 2
elseif person.education == "primary" then
    edu_score = 1
end

-- Gender: male=0, female=1
local gender_score = 0
if person.gender == "female" then
    gender_score = 1
end
```

**3. Linear prediction:**
```lua
local income = hekate_stats.linear_predict(
    coefs.intercept,           -- 15000
    coefs.age, person.age,     -- 300 * age
    coefs.education, edu_score, -- 5000 * edu_score
    coefs.gender, gender_score, -- -2500 * gender_score
    coefs.experience, person.experience -- 400 * experience
)
```

This computes:
```
income = 15000 + 300*age + 5000*edu_score - 2500*gender_score + 400*experience
```

---

## Step 3: Run the Simulation

```bash
./hekate config_income.yaml
```

---

## Step 4: Analyze the Results

Create `analyze_income_results.py`:

```python
#!/usr/bin/env python3
"""
Analyze income model results
"""

import csv
import statistics
from collections import defaultdict

def analyze_results():
    # Read data
    data = []
    with open('output_income.csv', 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            row['age'] = int(row['age'])
            row['income'] = int(row['income'])
            row['experience'] = int(row['experience'])
            row['alive'] = row['alive'].lower() == 'true'
            data.append(row)
    
    alive = [p for p in data if p['alive']]
    
    print("=" * 60)
    print("INCOME MODEL RESULTS")
    print("=" * 60)
    
    # Overall income
    incomes = [p['income'] for p in alive]
    print(f"\nOverall Income:")
    print(f"  Mean:  ${statistics.mean(incomes):,.0f}")
    print(f"  Median: ${statistics.median(incomes):,.0f}")
    print(f"  Min:   ${min(incomes):,.0f}")
    print(f"  Max:   ${max(incomes):,.0f}")
    
    # Income by age
    print(f"\nIncome by Age Group:")
    age_groups = [(20, 30), (30, 40), (40, 50), (50, 60), (60, 70)]
    labels = ["20-29", "30-39", "40-49", "50-59", "60-69"]
    for label, (low, high) in zip(labels, age_groups):
        incomes = [p['income'] for p in alive if low <= p['age'] < high]
        if incomes:
            print(f"  {label}: ${statistics.mean(incomes):,.0f} (n={len(incomes)})")
    
    # Income by education
    print(f"\nIncome by Education:")
    edu_order = ['none', 'primary', 'secondary', 'tertiary']
    for edu in edu_order:
        incomes = [p['income'] for p in alive if p['education'] == edu]
        if incomes:
            print(f"  {edu:10s}: ${statistics.mean(incomes):,.0f} (n={len(incomes)})")
    
    # Income by gender
    print(f"\nIncome by Gender:")
    for gender in ['male', 'female']:
        incomes = [p['income'] for p in alive if p['gender'] == gender]
        if incomes:
            print(f"  {gender:8s}: ${statistics.mean(incomes):,.0f} (n={len(incomes)})")
    
    # Income by experience
    print(f"\nIncome by Experience:")
    exp_groups = [(0, 5), (5, 10), (10, 20), (20, 30), (30, 50)]
    for low, high in exp_groups:
        incomes = [p['income'] for p in alive if low <= p['experience'] < high]
        if incomes:
            label = f"{low}-{high} yrs"
            print(f"  {label:10s}: ${statistics.mean(incomes):,.0f} (n={len(incomes)})")

if __name__ == "__main__":
    analyze_results()
```

Run it:
```bash
python3 analyze_income_results.py
```

---

## Step 5: Advanced Example - Income with Interaction Effects

You can create more complex models by adding interaction terms:

```yaml
models:
  - name: income_predictor_advanced
    type: lua_model
    priority: 1
    enabled: true
    parameters:
      coefficients:
        intercept: 15000
        age: 200
        education: 3000
        gender: -2000
        experience: 300
        age_education: 50      # Interaction: age * education
        gender_education: -1000  # Interaction: gender * education
      script: |
        function transition(population, params)
            local coefs = params.coefficients
            
            for _, person in ipairs(population) do
                local alive = false
                if person.alive == true or person.alive == "true" then
                    alive = true
                end
                
                if alive then
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
                    
                    -- Include interaction terms
                    local income = hekate_stats.linear_predict(
                        coefs.intercept,
                        coefs.age, person.age,
                        coefs.education, edu_score,
                        coefs.gender, gender_score,
                        coefs.experience, person.experience,
                        coefs.age_education, person.age * edu_score,
                        coefs.gender_education, gender_score * edu_score
                    )
                    
                    person.income = math.floor(income + 0.5)
                end
            end
            
            return population
        end
```

This model captures:
- **Age-education interaction**: Education is more valuable for younger people
- **Gender-education interaction**: The gender gap varies by education level

---

## Step 6: Using Logistic Regression for Binary Outcomes

You can also use logistic regression for binary outcomes:

```yaml
models:
  - name: employment_predictor
    type: lua_model
    priority: 2
    enabled: true
    parameters:
      coefficients:
        intercept: -1.5
        age: 0.05
        education: 0.3
        gender: -0.2
      script: |
        function transition(population, params)
            local coefs = params.coefficients
            
            for _, person in ipairs(population) do
                local alive = false
                if person.alive == true or person.alive == "true" then
                    alive = true
                end
                
                if alive then
                    local edu_score = 0
                    if person.education == "tertiary" then
                        edu_score = 3
                    elseif person.education == "secondary" then
                        edu_score = 2
                    end
                    
                    local gender_score = 0
                    if person.gender == "female" then
                        gender_score = 1
                    end
                    
                    -- Probability of being employed
                    local prob = hekate_stats.logistic_predict(
                        coefs.intercept,
                        coefs.age, person.age / 10,
                        coefs.education, edu_score,
                        coefs.gender, gender_score
                    )
                    
                    person.employed = math.random() < prob
                end
            end
            
            return population
        end
```

---

## Step 7: Full Example - Income + Health Risk Model

Here's a complete example combining both linear and logistic regression:

### config_full_example.yaml

```yaml
simulation:
  iterations: 1
  population_file: population_full.csv
  output_file: output_full.csv
  random_seed: 42
  verbose: true
  id_column: id
  streaming_mode: false

models:
  # Model 1: Predict Income (Linear Regression)
  - name: income_predictor
    type: lua_model
    priority: 1
    enabled: true
    parameters:
      coefficients:
        intercept: 15000
        age: 300
        education: 5000
        gender: -2500
        experience: 400
      script: |
        function transition(population, params)
            local coefs = params.coefficients
            
            for _, person in ipairs(population) do
                local alive = false
                if person.alive == true or person.alive == "true" then
                    alive = true
                end
                
                if alive then
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
                        coefs.age, person.age,
                        coefs.education, edu_score,
                        coefs.gender, gender_score,
                        coefs.experience, person.experience
                    )
                    
                    local noise = 1 + (math.random() * 0.1 - 0.05)
                    person.income = math.floor(income * noise + 0.5)
                    
                    if person.income < 8000 then
                        person.income = 8000
                    end
                end
            end
            
            return population
        end

  # Model 2: Predict Health Risk (Logistic Regression)
  - name: health_risk_predictor
    type: lua_model
    priority: 2
    enabled: true
    parameters:
      coefficients:
        intercept: -3.5
        age: 0.8
        bmi: 0.6
        smoker: 1.2
        income: -0.00001
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
                    local income = tonumber(person.income) or 30000
                    
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
                        coefs.smoker, smoker_score,
                        coefs.income, income / 10000
                    )
                    
                    person.health_risk = math.floor(risk * 100 + 0.5)
                    
                    -- High risk individuals may die
                    if risk > 0.7 and math.random() < 0.05 then
                        person.alive = false
                    end
                end
            end
            
            return population
        end
```

---

## Step 8: Getting Help with the LLM Prompt Template

If you need help building regression models, you can use the **LLM Prompt Template** included with Hekate:

1. Copy the [LLM Prompt Template](LLM_PROMPT_TEMPLATE.md)
2. Paste it into your preferred AI assistant (ChatGPT, Claude, etc.)
3. Describe what you want to build:
   - "I want to predict income using age, education, and gender"
   - "I need a health risk model using logistic regression"
   - "I want to predict mortality risk using age and health status"
4. Get working YAML and Lua code instantly

---

## Best Practices

### 1. Always Use tonumber()
CSV data comes in as strings. Always convert:
```lua
local age = tonumber(person.age) or 0
```

### 2. Handle Missing Values
```lua
local edu_score = 0
if person.education == "tertiary" then
    edu_score = 3
elseif person.education == "secondary" then
    edu_score = 2
else
    edu_score = 0
end
```

### 3. Add Realistic Noise
Perfect predictions look suspicious. Add noise:
```lua
local noise = 1 + (math.random() * 0.1 - 0.05)  -- ±5%
person.income = math.floor(income * noise + 0.5)
```

### 4. Validate with Python
Always validate your model results using Python or R to ensure they make sense.

### 5. Document Your Coefficients
Add comments explaining what each coefficient means:
```yaml
coefficients:
  intercept: 15000    # Base income for a 20-year-old with no education
  age: 300            # Additional income per year of age
  education: 5000     # Additional income per education level
  gender: -2500       # Gender penalty (female earns less)
```

### 6. Check Coefficient Signs
- Age should have a positive coefficient (older = higher income)
- Education should have a positive coefficient
- Gender should reflect your expected difference

### 7. Check Magnitudes
- Does a year of age add $300 to income? That's $30,000 over 100 years - reasonable
- Does a university degree add $15,000? Depends on your context

---

## Common Pitfalls and How to Avoid Them

### Pitfall 1: String vs Number Comparison
**Wrong:**
```lua
if person.age > 30 then  -- age is a string!
```

**Correct:**
```lua
local age = tonumber(person.age) or 0
if age > 30 then
```

### Pitfall 2: Missing Values
**Wrong:**
```lua
local income = hekate_stats.linear_predict(
    coefs.intercept,
    coefs.age, person.age  -- person.age might be nil
)
```

**Correct:**
```lua
local age = tonumber(person.age) or 0
local income = hekate_stats.linear_predict(
    coefs.intercept,
    coefs.age, age
)
```

### Pitfall 3: Forgetting to Handle Alive Status
**Wrong:**
```lua
for _, person in ipairs(population) do
    -- Updates everyone, including dead people!
    person.income = predict_income(person)
end
```

**Correct:**
```lua
for _, person in ipairs(population) do
    local alive = false
    if person.alive == true or person.alive == "true" then
        alive = true
    end
    
    if alive then
        person.income = predict_income(person)
    end
end
```

---

## Summary

You've learned how to:

1. ✅ Use `hekate_stats.linear_predict()` in Lua scripts
2. ✅ Define coefficients in YAML
3. ✅ Handle categorical variables (education, gender)
4. ✅ Add randomness to make predictions realistic
5. ✅ Validate your model results with Python
6. ✅ Build more complex models with interaction terms
7. ✅ Use logistic regression for binary outcomes
8. ✅ Combine multiple models in one simulation
9. ✅ Use the LLM Prompt Template for help

## Next Steps

- Try adding more variables to your model (e.g., region, marital status)
- Use logistic regression for other binary outcomes (marriage, migration)
- Combine linear regression with other models (aging, mortality)
- Run the model for multiple years to see how income changes over time
- Experiment with different coefficient values to see their impact
- **Tutorial 5: Large-Scale Simulations with Streaming Mode** - Learn how to run regression models on populations of millions

## Troubleshooting

| Error | Solution |
|-------|----------|
| `attempt to compare number with string` | Use `tonumber()` to convert string values to numbers |
| `attempt to index a nil value` | Check that the column exists in your CSV and use `if person.column_name then` |
| Predictions are too high/low | Check your coefficients and add noise to make results more realistic |
| Missing values in output | Use `or` to provide defaults: `local age = tonumber(person.age) or 0` |
| Model not running in priority order | Check your priority numbers - lower numbers run first |
