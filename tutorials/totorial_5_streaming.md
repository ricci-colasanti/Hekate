# Tutorial 5: Large-Scale Simulations with Streaming Mode

## Overview

Hekate's streaming mode allows you to simulate populations of **millions to billions** of individuals using minimal memory. Instead of loading everyone into RAM at once, Hekate processes your population **area by area** - loading, processing, and saving one area at a time.

This tutorial will show you how to use streaming mode for large-scale simulations.

---

## The Problem: Memory Limitations

Traditional microsimulation loads the entire population into memory:

```
Your Computer: 8GB RAM
Population: 10 million people
Memory needed: ~2GB per million people
Result: OUT OF MEMORY! 💥
```

### Memory Requirements Comparison

| Population | Traditional Mode | Streaming Mode |
|------------|------------------|----------------|
| 100K | 200MB | ~20MB |
| 1M | 2GB | ~20MB |
| 10M | 20GB ❌ | ~20MB ✅ |
| 100M | 200GB ❌ | ~20MB ✅ |

**With streaming mode, memory usage stays constant regardless of population size!**

---

## Prerequisites

Before starting, make sure you have:
- Completed Tutorial 1 (Basic Aging Model)
- Hekate binary downloaded and working
- A large population file (or generate one)

---

## Step 1: Generate a Large Population

Create `generate_streaming_population.py`:

```python
#!/usr/bin/env python3
"""
Generate a large population sorted by area for streaming mode
"""

import csv
import random

random.seed(42)

def generate_population(n=100000, output_file="population_streaming.csv"):
    """Generate n individuals sorted by area"""
    
    print(f"Generating {n:,} individuals...")
    
    # Define regions/areas
    areas = [f"area_{i:05d}" for i in range(1, 21)]  # 20 areas
    
    with open(output_file, 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow([
            'id', 'age', 'gender', 'income', 'alive', 'area'
        ])
        
        # Generate people area by area (already sorted!)
        for i in range(1, n + 1):
            # Choose area (distribute evenly)
            area = areas[i % len(areas)]
            
            # Generate person
            person_id = f"P{i:08d}"
            age = random.randint(18, 80)
            gender = random.choice(['male', 'female'])
            income = 20000 + age * 200 + random.randint(-5000, 5000)
            alive = random.random() < 0.95
            
            writer.writerow([
                person_id, age, gender, income, str(alive).lower(), area
            ])
            
            if i % 10000 == 0:
                print(f"  Generated {i:,} people...")
    
    print(f"✅ Generated {n:,} individuals in {output_file}")
    print(f"   Areas: {len(areas)}")

if __name__ == "__main__":
    # Generate 100,000 people (increase this for larger tests)
    generate_population(100000, "population_streaming.csv")
```

Run it:
```bash
python3 generate_streaming_population.py
```

---

## Step 2: Understanding the Streaming Configuration

### config_streaming.yaml

```yaml
simulation:
  iterations: 3                    # Run for 3 years
  population_file: population_streaming.csv
  output_file: output_streaming.csv
  random_seed: 42
  verbose: true                    # See what's happening
  id_column: id                    # Unique identifier
  area_column: area                # REQUIRED: Column to group by
  streaming_mode: true             # Enable streaming mode

models:
  # Model 1: Age everyone (Priority 1)
  - name: aging_model
    type: lua_model
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
                    person.age = person.age + 1
                end
            end
            return population
        end

  # Model 2: Predict income (Priority 2)
  - name: income_model
    type: lua_model
    priority: 2
    enabled: true
    parameters:
      coefficients:
        intercept: 20000
        age: 300
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
                    local income = hekate_stats.linear_predict(
                        coefs.intercept,
                        coefs.age, age
                    )
                    
                    person.income = math.floor(income + 0.5)
                end
            end
            
            return population
        end
```

### Key Streaming Settings

```yaml
simulation:
  streaming_mode: true      # Enable streaming
  area_column: area        # Column name to group by
```

**Important:** Your CSV must be **sorted by the area column** for streaming to work!

---

## Step 3: Run the Streaming Simulation

```bash
./hekate-linux-amd64 config_streaming.yaml
```

### What You'll See

```
═══ Hekate: Microsimulation Engine ═══
Iterations: 3
Mode: STREAMING (area-by-area)
Area column: area

--- Using Streaming Area-by-Area Processing ---
Detected 6 columns

═══ Iteration 1/3 ═══
  ▶ aging_model
  ▶ income_model
  Processed 20 areas

═══ Iteration 2/3 ═══
  ▶ aging_model
  ▶ income_model
  Processed 20 areas

═══ Iteration 3/3 ═══
  ▶ aging_model
  ▶ income_model
  Processed 20 areas

═══ Simulation Complete ═══
Results saved to output_streaming.csv
```

### Intermediate Files

You'll see intermediate files for each year:
```
year_0.csv  → Original population
year_1.csv  → After 1 year
year_2.csv  → After 2 years  
year_3.csv  → After 3 years
output_streaming.csv → Final results (copy of year_3.csv)
```

---

## Step 4: Monitor Memory Usage

### While running, check memory usage in another terminal:

**Linux/macOS:**
```bash
# Check Hekate's memory usage
ps aux | grep hekate

# Or watch it live
watch -n 1 'ps aux | grep hekate | grep -v grep'
```

**Expected:** Memory stays around ~20-30MB regardless of population size!

---

## Step 5: Analyze the Results

Create `analyze_streaming_results.py`:

```python
#!/usr/bin/env python3
"""
Analyze streaming simulation results
"""

import csv
import statistics
from collections import defaultdict

def analyze_results(filename="output_streaming.csv"):
    print("=" * 60)
    print("STREAMING SIMULATION RESULTS")
    print("=" * 60)
    
    # Read data
    data = []
    with open(filename, 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            row['age'] = int(row['age'])
            row['income'] = int(row['income'])
            row['alive'] = row['alive'].lower() == 'true'
            data.append(row)
    
    alive = [p for p in data if p['alive']]
    dead = [p for p in data if not p['alive']]
    
    print(f"\n📊 Population: {len(data):,}")
    print(f"  Alive: {len(alive):,} ({len(alive)/len(data)*100:.1f}%)")
    print(f"  Dead:  {len(dead):,} ({len(dead)/len(data)*100:.1f}%)")
    
    # Age distribution
    print(f"\n🎂 Age Distribution:")
    age_groups = [(18, 30), (30, 50), (50, 65), (65, 90)]
    labels = ["18-29", "30-49", "50-64", "65+"]
    
    for label, (low, high) in zip(labels, age_groups):
        count = len([p for p in alive if low <= p['age'] < high])
        if count > 0:
            print(f"  {label}: {count:,} ({count/len(alive)*100:.1f}%)")
    
    # Income stats
    incomes = [p['income'] for p in alive]
    print(f"\n💰 Income Statistics:")
    print(f"  Mean:  ${statistics.mean(incomes):,.0f}")
    print(f"  Median: ${statistics.median(incomes):,.0f}")
    print(f"  Min:   ${min(incomes):,.0f}")
    print(f"  Max:   ${max(incomes):,.0f}")
    
    # Check if sorting worked (optional)
    print(f"\n📂 Checking Areas:")
    areas = sorted(set([p['area'] for p in data]))
    print(f"  Number of areas: {len(areas)}")
    for area in areas[:5]:  # Show first 5
        count = len([p for p in data if p['area'] == area])
        print(f"  {area}: {count:,} people")
    if len(areas) > 5:
        print(f"  ... and {len(areas)-5} more areas")

if __name__ == "__main__":
    analyze_results()
```

Run it:
```bash
python3 analyze_streaming_results.py
```

---

## Step 6: Scale Up Your Simulation

### Increase Population to 1 Million

Edit `generate_streaming_population.py`:
```python
generate_population(1000000, "population_streaming.csv")  # 1 million
```

### Still Only ~20MB Memory!

The beauty of streaming mode: regardless of whether you have 100,000 or 100 million people, memory usage stays the same.

### Check the File Size

```bash
ls -lh population_streaming.csv
```

| Population | File Size |
|------------|-----------|
| 100K | ~5MB |
| 1M | ~50MB |
| 10M | ~500MB |
| 100M | ~5GB |

---

## Streaming Mode Best Practices

### 1. Sort Your CSV by Area
**CRITICAL:** Streaming mode requires the CSV to be sorted by the area column.

**How to sort:**
```bash
# Linux/macOS
(head -n1 population.csv && tail -n+2 population.csv | sort -t, -k6 -n) > population_sorted.csv

# Python
import pandas as pd
df = pd.read_csv('population.csv')
df_sorted = df.sort_values('area')
df_sorted.to_csv('population_sorted.csv', index=False)
```

### 2. Choose Your Area Column Wisely

| Area Type | Example | Good For |
|-----------|---------|----------|
| Administrative | County, District | Policy analysis |
| Geographic | Region, State | Regional comparisons |
| Grid | 1km grid cells | Spatial modeling |
| Random | Area_001, Area_002 | Memory testing |

### 3. Use Meaningful Area Names

```yaml
simulation:
  area_column: "region"  # Good: north, south, east, west
  # area_column: "area_id"  # Also good: 1, 2, 3, ...
```

### 4. Monitor Performance

If streaming mode is slow:
- Check your CSV is sorted properly
- Use fewer areas (more people per area)
- Use traditional mode for smaller populations (< 1M)

---

## Performance Tips

### Tip 1: Use Fewer Areas
More areas = more files = slower. Use 10-100 areas for best performance.

### Tip 2: Batch Processing
Hekate automatically batches by area. No additional configuration needed!

### Tip 3: SSD Storage
Use SSD storage for large populations (HDD will be slower).

### Tip 4: Reduce Verbose Output
Set `verbose: false` for large simulations:
```yaml
simulation:
  verbose: false  # Faster for big populations
```

---

## When to Use Streaming vs Traditional

| Factor | Use Streaming | Use Traditional |
|--------|---------------|-----------------|
| Population Size | > 1 million | < 1 million |
| Memory Available | < 8GB | > 8GB |
| Areas | Many (>10) | Few or none |
| Speed Priority | No | Yes |
| Development | No | Yes (faster) |

---

## Troubleshooting

### "ERROR: area_column is required when streaming_mode is true"
```yaml
simulation:
  area_column: "area"  # Add this!
```

### "area column 'area' not found"
Check your CSV header. Make sure the column name matches exactly.

### Streaming mode is slow
- Ensure CSV is properly sorted by area
- Use fewer areas (10-100 recommended)
- Set `verbose: false`

### Out of disk space
Intermediate files add up. Clean up after:
```bash
rm year_*.csv  # After simulation completes
```

### Memory usage is high
Check that `streaming_mode: true` is set. If false, it loads everything at once.

---

## Summary

You've learned how to:

1. ✅ Enable streaming mode in Hekate
2. ✅ Generate large populations sorted by area
3. ✅ Run simulations with minimal memory
4. ✅ Monitor memory usage during runs
5. ✅ Analyze results from streaming simulations
6. ✅ Scale from 100K to millions of people
7. ✅ Troubleshoot common streaming issues

### Key Takeaway

**With Hekate's streaming mode, you're no longer limited by memory!** You can simulate populations of any size on standard hardware.

---

## Next Steps

- Try the full 1 million person simulation
- Add more complex models (fertility, migration, etc.)
- Compare streaming vs traditional mode performance
- Run simulations with different area counts
- Experiment with different population distributions

## Complete Example

Here's a complete setup for a 1 million person simulation:

```yaml
# config_streaming_full.yaml
simulation:
  iterations: 10
  population_file: population_1m.csv
  output_file: output_1m.csv
  random_seed: 42
  verbose: false
  id_column: id
  area_column: area
  streaming_mode: true

models:
  - name: aging_model
    type: lua_model
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
                    person.age = person.age + 1
                end
            end
            return population
        end

  - name: income_model
    type: lua_model
    priority: 2
    enabled: true
    parameters:
      coefficients:
        intercept: 20000
        age: 300
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
                    local income = hekate_stats.linear_predict(
                        coefs.intercept,
                        coefs.age, age
                    )
                    person.income = math.floor(income + 0.5)
                end
            end
            return population
        end
```

Run it:
```bash
# Generate 1 million people
python3 generate_streaming_population.py  # Modify to 1000000

# Run the simulation
./hekate-linux-amd64 config_streaming_full.yaml

# Analyze
python3 analyze_streaming_results.py
```

---

Congratulations! You now have the power to simulate populations of any size with Hekate's streaming mode! 🚀