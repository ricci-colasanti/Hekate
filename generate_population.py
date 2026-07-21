#!/usr/bin/env python3
"""
Generate 1 million synthetic individuals for Hekate microsimulation
Includes realistic demographic distributions and dependencies
SORTED BY REGION (alphabetical) for streaming mode
"""

import csv
import random
import sys
import math
from datetime import datetime

# Set random seed for reproducibility
random.seed(42)

def generate_population(n=1000000, output_file="population.csv"):
    """Generate n synthetic individuals with realistic attributes, sorted by region"""
    
    print(f"🚀 Generating {n:,} individuals sorted by region...")
    start_time = datetime.now()
    
    # Education levels with probabilities (based on realistic distributions)
    education_levels = [
        ("none", 0.08),      # 8% no education
        ("primary", 0.22),   # 22% primary
        ("secondary", 0.40), # 40% secondary
        ("tertiary", 0.30)   # 30% tertiary
    ]
    
    # Regions with probabilities - ALPHABETICAL ORDER for streaming mode!
    # This ensures the sort check passes: east < north < south < west
    regions = [
        ("east", 0.25),
        ("north", 0.25),
        ("south", 0.30),
        ("west", 0.20)
    ]
    
    # Open file and write header
    with open(output_file, 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow([
            'id', 'age', 'gender', 'education', 'income', 
            'bmi', 'smoker', 'alive', 'region', 'household_id',
            'health_risk', 'mortality_risk', 'employed', 'chronic_disease',
            'year_born'
        ])
        
        # Generate region by region (this ensures sorting)
        global_counter = 0
        progress_interval = 100000
        
        for region, region_prob in regions:
            # Calculate how many people in this region
            region_count = int(n * region_prob)
            print(f"  Generating {region_count:,} people for region: {region}")
            
            for i in range(region_count):
                global_counter += 1
                
                # Progress indicator
                if global_counter % progress_interval == 0:
                    elapsed = (datetime.now() - start_time).total_seconds()
                    rate = global_counter / elapsed
                    print(f"  Progress: {global_counter:,} / {n:,} ({((global_counter)/n)*100:.1f}%) - {rate:.0f} records/sec")
                
                # === DEMOGRAPHICS ===
                person_id = f"P{global_counter:09d}"
                
                # Age (18-90 with realistic distribution - more people in middle age)
                age = int(random.gauss(45, 18))
                age = max(18, min(90, age))
                
                # Year born (2026 - age)
                year_born = 2026 - age
                
                # Gender
                gender = random.choice(['male', 'female'])
                
                # Education (correlated with age - younger people more educated)
                age_effect = max(0, (age - 50) / 40)
                edu_weights = [
                    ("none", 0.08 + age_effect * 0.10),
                    ("primary", 0.22 + age_effect * 0.08),
                    ("secondary", 0.40 - age_effect * 0.08),
                    ("tertiary", 0.30 - age_effect * 0.10)
                ]
                total = sum(w for _, w in edu_weights)
                edu_weights = [(e, w/total) for e, w in edu_weights]
                education = weighted_choice(edu_weights)
                
                # === HEALTH INDICATORS ===
                bmi_mean = 24 + (age - 40) * 0.05
                if education in ['none', 'primary']:
                    bmi_mean += 2
                bmi = max(16, min(50, random.gauss(bmi_mean, 4.5)))
                bmi = round(bmi, 1)
                
                smoker_prob = 0.10
                if education in ['none', 'primary']:
                    smoker_prob += 0.15
                if age < 30:
                    smoker_prob += 0.10
                elif age > 60:
                    smoker_prob -= 0.05
                if gender == 'male':
                    smoker_prob += 0.05
                smoker_prob = max(0.01, min(0.60, smoker_prob))
                smoker = random.random() < smoker_prob
                
                chronic_prob = 0.02 + (age - 18) * 0.005
                if bmi > 30:
                    chronic_prob += 0.10
                if smoker:
                    chronic_prob += 0.08
                chronic_prob = min(0.80, chronic_prob)
                chronic_disease = random.random() < chronic_prob
                
                # === MORTALITY RISK (Base - will be updated by model) ===
                mortality_risk = 0.01 + (age - 18) * 0.002
                if chronic_disease:
                    mortality_risk += 0.05
                if smoker:
                    mortality_risk += 0.03
                mortality_risk = min(0.95, mortality_risk)
                
                alive = random.random() > mortality_risk
                
                # === SOCIOECONOMIC ===
                base_income = 20000 + age * 300
                if education == "tertiary":
                    base_income += 15000
                elif education == "secondary":
                    base_income += 5000
                elif education == "primary":
                    base_income += 2000
                if gender == "male":
                    base_income *= 1.15
                income = int(base_income + random.gauss(0, 5000))
                income = max(5000, income)
                
                employed_prob = 0.30
                if 25 <= age <= 65:
                    employed_prob += 0.50
                if education in ['secondary', 'tertiary']:
                    employed_prob += 0.15
                if age > 65:
                    employed_prob -= 0.40
                employed_prob = max(0.05, min(0.95, employed_prob))
                employed = random.random() < employed_prob
                
                # Region is already determined by outer loop
                # Household (group by region and random)
                household_id = f"H{random.randint(1, n//4):09d}"
                
                # Health risk (will be updated by model)
                health_risk = 0
                
                # Write row
                writer.writerow([
                    person_id,
                    age,
                    gender,
                    education,
                    income,
                    bmi,
                    str(smoker).lower(),
                    str(alive).lower(),
                    region,
                    household_id,
                    health_risk,
                    f"{mortality_risk:.4f}",
                    str(employed).lower(),
                    str(chronic_disease).lower(),
                    year_born
                ])
    
    elapsed = (datetime.now() - start_time).total_seconds()
    print(f"✅ Generated {n:,} individuals in {elapsed:.1f} seconds")
    print(f"   File: {output_file} ({get_file_size(output_file)})")
    print(f"   Sorted by: region (alphabetical: east, north, south, west)")

def weighted_choice(choices):
    """Choose from weighted options: list of (value, probability)"""
    r = random.random()
    cumulative = 0
    for value, prob in choices:
        cumulative += prob
        if r < cumulative:
            return value
    return choices[-1][0]

def get_file_size(filename):
    """Get human-readable file size"""
    import os
    size = os.path.getsize(filename)
    for unit in ['B', 'KB', 'MB', 'GB']:
        if size < 1024.0:
            return f"{size:.1f} {unit}"
        size /= 1024.0
    return f"{size:.1f} TB"

def verify_sorting(filename="population.csv", n=1000):
    """Verify the file is properly sorted by region"""
    print("\n🔍 Verifying sorting by region...")
    
    with open(filename, 'r') as f:
        reader = csv.reader(f)
        header = next(reader)
        
        # Find region column index
        try:
            region_idx = header.index('region')
        except ValueError:
            print("  ❌ 'region' column not found!")
            return False
        
        # Check first n records
        regions_found = []
        for i, row in enumerate(reader):
            if i >= n:
                break
            if len(row) > region_idx:
                regions_found.append(row[region_idx])
        
        # Check if all same region (should be "east" for first n)
        unique_regions = set(regions_found)
        if len(unique_regions) == 1:
            print(f"  ✅ All first {n} records are from region: '{list(unique_regions)[0]}'")
            return True
        else:
            print(f"  ❌ First {n} records contain multiple regions: {unique_regions}")
            return False

def preview_population(filename="population.csv", n=10):
    """Preview the first n rows to verify sorting"""
    print("\n📋 Sample of generated data (sorted by region):")
    print("-" * 130)
    with open(filename, 'r') as f:
        reader = csv.reader(f)
        header = next(reader)
        print(" | ".join(f"{h:^12}" for h in header[:15]))
        print("-" * 130)
        for i, row in enumerate(reader):
            if i < n:
                print(" | ".join(f"{v:^12}" for v in row[:15]))
            else:
                break

if __name__ == "__main__":
    # Generate 1 million individuals sorted by region
    generate_population(1000000, "population.csv")
    
    # Preview the data
    preview_population("population.csv", 10)
    
    # Verify sorting
    verify_sorting("population.csv", 1000)
    
    print("\n" + "=" * 70)
    print("🚀 Ready to run simulation with STREAMING MODE:")
    print("   ./hekate-linux config.yaml")
    print("=" * 70)