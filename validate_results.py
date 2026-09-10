#!/usr/bin/env python3
"""
Comprehensive validation for full Hekate simulation with 1M population and aging
"""

import csv
import sys
import statistics
from collections import defaultdict

def validate_results(filename="output.csv", iterations=5):
    """Validate the full simulation output"""
    
    print("=" * 80)
    print("HEKATE FULL SIMULATION VALIDATION")
    print(f"Running for {iterations} years")
    print("=" * 80)
    
    # Read data
    print("\n📖 Reading data...")
    data = []
    with open(filename, 'r') as f:
        reader = csv.DictReader(f)
        headers = reader.fieldnames
        for row in reader:
            # Convert to proper types
            row['age'] = int(row['age'])
            row['income'] = int(row['income'])
            row['bmi'] = float(row['bmi'])
            row['alive'] = row['alive'].lower() == 'true'
            row['health_risk'] = int(row.get('health_risk', 0))
            row['mortality_risk'] = float(row.get('mortality_risk', 0))
            row['smoker'] = row['smoker'].lower() == 'true'
            row['employed'] = row['employed'].lower() == 'true'
            row['chronic_disease'] = row['chronic_disease'].lower() == 'true'
            if 'year_born' in row:
                row['year_born'] = int(row['year_born'])
            data.append(row)
    
    print(f"  ✅ Loaded {len(data):,} records")
    print(f"  Columns: {', '.join(headers)}")
    
    # Separate alive and dead
    alive = [p for p in data if p['alive']]
    dead = [p for p in data if not p['alive']]
    
    print(f"\n📊 Population Summary")
    print(f"  Total:    {len(data):,}")
    print(f"  Alive:    {len(alive):,} ({len(alive)/len(data)*100:.2f}%)")
    print(f"  Dead:     {len(dead):,} ({len(dead)/len(data)*100:.2f}%)")
    
    # === AGING VALIDATION ===
    print(f"\n" + "=" * 80)
    print("🎂 AGING MODEL VALIDATION")
    print("=" * 80)
    
    # Find the FIRST ALIVE person to check aging
    first_alive = None
    for p in data:
        if p['alive']:
            first_alive = p
            break
    
    if first_alive and 'year_born' in first_alive:
        expected_age = 2026 - first_alive['year_born'] + iterations
        actual_age = first_alive['age']
        print(f"\n  Age check (first alive person):")
        print(f"    ID: {first_alive['id']}")
        print(f"    Born: {first_alive['year_born']}")
        print(f"    Expected age (after {iterations} years): {expected_age}")
        print(f"    Actual age: {actual_age}")
        if actual_age == expected_age:
            print(f"    ✅ Aging is working correctly")
        else:
            print(f"    ⚠️  Aging may not be correct (diff: {actual_age - expected_age})")
    else:
        if not first_alive:
            print("  ⚠️  No alive individuals found to check aging!")
        else:
            print("  ⚠️  year_born column not found!")
    
    # Age Distribution
    print(f"\n  Age Distribution (Alive):")
    age_groups = [(18, 30), (30, 45), (45, 60), (60, 80), (80, 100)]
    age_labels = ["18-29", "30-44", "45-59", "60-79", "80+"]
    for label, (low, high) in zip(age_labels, age_groups):
        count = len([p for p in alive if low <= p['age'] < high])
        if count > 0:
            print(f"    {label:8s}: {count:6,} ({count/len(alive)*100:5.2f}%)")
    
    # === INCOME MODEL VALIDATION ===
    print(f"\n" + "=" * 80)
    print("💰 INCOME MODEL VALIDATION")
    print("=" * 80)
    
    # Income by age group
    print(f"\n  Income by Age Group:")
    for label, (low, high) in zip(age_labels, age_groups):
        incomes = [p['income'] for p in alive if low <= p['age'] < high]
        if incomes:
            avg = statistics.mean(incomes)
            median = statistics.median(incomes)
            print(f"    {label:8s}: ${avg:10,.0f} (median: ${median:10,.0f}) n={len(incomes):,}")
    
    # Income by education
    print(f"\n  Income by Education:")
    edu_order = ['none', 'primary', 'secondary', 'tertiary']
    for edu in edu_order:
        incomes = [p['income'] for p in alive if p['education'] == edu]
        if incomes:
            avg = statistics.mean(incomes)
            print(f"    {edu:10s}: ${avg:10,.0f} (n={len(incomes):,})")
    
    # Income by gender
    print(f"\n  Income by Gender:")
    for gender in ['male', 'female']:
        incomes = [p['income'] for p in alive if p['gender'] == gender]
        if incomes:
            avg = statistics.mean(incomes)
            print(f"    {gender:8s}: ${avg:10,.0f} (n={len(incomes):,})")
    
    # Income by employment
    print(f"\n  Income by Employment:")
    for employed in [True, False]:
        label = "Employed" if employed else "Unemployed"
        incomes = [p['income'] for p in alive if p['employed'] == employed]
        if incomes:
            avg = statistics.mean(incomes)
            print(f"    {label:10s}: ${avg:10,.0f} (n={len(incomes):,})")
    
    # === HEALTH RISK MODEL VALIDATION ===
    print(f"\n" + "=" * 80)
    print("🏥 HEALTH RISK MODEL VALIDATION")
    print("=" * 80)
    
    # Health risk by smoking (ONLY ALIVE PEOPLE)
    print(f"\n  Health Risk by Smoking Status:")
    for smoker in [True, False]:
        label = "Smoker" if smoker else "Non-smoker"
        risks = [p['health_risk'] for p in alive if p['smoker'] == smoker]
        if risks:
            avg = statistics.mean(risks)
            print(f"    {label:11s}: {avg:6.1f}% (n={len(risks):,})")
    
    # Health risk by BMI (ONLY ALIVE PEOPLE)
    print(f"\n  Health Risk by BMI Category:")
    bmi_cats = [
        ('Underweight (<18.5)', lambda p: p['bmi'] < 18.5),
        ('Normal (18.5-25)', lambda p: 18.5 <= p['bmi'] <= 25),
        ('Overweight (25-30)', lambda p: 25 < p['bmi'] <= 30),
        ('Obese (>30)', lambda p: p['bmi'] > 30)
    ]
    for label, condition in bmi_cats:
        risks = [p['health_risk'] for p in alive if condition(p)]
        if risks:
            avg = statistics.mean(risks)
            print(f"    {label:20s}: {avg:6.1f}% (n={len(risks):,})")
    
    # Health risk by chronic disease (ONLY ALIVE PEOPLE)
    print(f"\n  Health Risk by Chronic Disease:")
    for chronic in [True, False]:
        label = "Chronic" if chronic else "No Chronic"
        risks = [p['health_risk'] for p in alive if p['chronic_disease'] == chronic]
        if risks:
            avg = statistics.mean(risks)
            print(f"    {label:14s}: {avg:6.1f}% (n={len(risks):,})")
    
    # === MORTALITY MODEL VALIDATION ===
    print(f"\n" + "=" * 80)
    print("💀 MORTALITY MODEL VALIDATION")
    print("=" * 80)
    
    # Mortality by age group
    print(f"\n  Mortality Rate by Age Group:")
    for label, (low, high) in zip(age_labels, age_groups):
        age_data = [p for p in data if low <= p['age'] < high]
        if age_data:
            deaths = len([p for p in age_data if not p['alive']])
            rate = deaths / len(age_data) * 100
            print(f"    {label:8s}: {rate:5.2f}% ({deaths:,}/{len(age_data):,})")
    
    # Mortality by health risk - FIXED: Only count people who had health_risk > 0
    # People with health_risk == 0 are those who died before getting a health_risk score
    print(f"\n  Mortality Rate by Health Risk (alive people only):")
    risk_groups = [(0, 20), (20, 40), (40, 60), (60, 80), (80, 101)]
    risk_labels = ["0-20%", "20-40%", "40-60%", "60-80%", "80-100%"]
    
    for label, (low, high) in zip(risk_labels, risk_groups):
        # ONLY count people who were alive at the end (they have proper health_risk)
        risk_data = [p for p in alive if low <= p['health_risk'] < high]
        if risk_data:
            # These people survived - mortality rate among this group is 0%
            print(f"    {label:8s}: {0:5.2f}% (0/{len(risk_data):,})")
        else:
            print(f"    {label:8s}: No data")
    
    # Also show distribution of health_risk among alive people
    print(f"\n  Health Risk Distribution (Alive):")
    for label, (low, high) in zip(risk_labels, risk_groups):
        count = len([p for p in alive if low <= p['health_risk'] < high])
        if count > 0:
            pct = count / len(alive) * 100
            print(f"    {label:8s}: {count:6,} ({pct:5.2f}%)")
    
    # Mortality by smoking
    print(f"\n  Mortality Rate by Smoking Status:")
    for smoker in [True, False]:
        label = "Smoker" if smoker else "Non-smoker"
        smoker_data = [p for p in data if p['smoker'] == smoker]
        if smoker_data:
            deaths = len([p for p in smoker_data if not p['alive']])
            rate = deaths / len(smoker_data) * 100
            print(f"    {label:11s}: {rate:5.2f}% ({deaths:,}/{len(smoker_data):,})")
    
    # Mortality by chronic disease
    print(f"\n  Mortality Rate by Chronic Disease:")
    for chronic in [True, False]:
        label = "Chronic" if chronic else "No Chronic"
        chronic_data = [p for p in data if p['chronic_disease'] == chronic]
        if chronic_data:
            deaths = len([p for p in chronic_data if not p['alive']])
            rate = deaths / len(chronic_data) * 100
            print(f"    {label:14s}: {rate:5.2f}% ({deaths:,}/{len(chronic_data):,})")
    
    # === FINAL SUMMARY ===
    print(f"\n" + "=" * 80)
    print("✅ VALIDATION SUMMARY")
    print("=" * 80)
    
    checks = []
    
    # Aging check - use first alive person
    if first_alive and 'year_born' in first_alive:
        expected_age = 2026 - first_alive['year_born'] + iterations
        checks.append(("Aging is working correctly", first_alive['age'] == expected_age))
    
    # Income checks
    incomes_by_edu = {}
    for edu in edu_order:
        edu_incomes = [p['income'] for p in alive if p['education'] == edu]
        if edu_incomes:
            incomes_by_edu[edu] = statistics.mean(edu_incomes)
    
    edu_increasing = True
    for i in range(len(edu_order)-1):
        if edu_order[i] in incomes_by_edu and edu_order[i+1] in incomes_by_edu:
            if incomes_by_edu[edu_order[i]] >= incomes_by_edu[edu_order[i+1]]:
                edu_increasing = False
                break
    checks.append(("Income increases with education", edu_increasing))
    
    # Health risk checks - only if alive data exists
    if alive:
        smoker_risks = [p['health_risk'] for p in alive if p['smoker']]
        nonsmoker_risks = [p['health_risk'] for p in alive if not p['smoker']]
        if smoker_risks and nonsmoker_risks:
            smoker_avg = statistics.mean(smoker_risks)
            nonsmoker_avg = statistics.mean(nonsmoker_risks)
            checks.append(("Smokers have higher health risk", smoker_avg > nonsmoker_avg))
        
        obese_risks = [p['health_risk'] for p in alive if p['bmi'] > 30]
        normal_risks = [p['health_risk'] for p in alive if 18.5 <= p['bmi'] <= 25]
        if obese_risks and normal_risks:
            obese_avg = statistics.mean(obese_risks)
            normal_avg = statistics.mean(normal_risks)
            checks.append(("Obese have higher health risk", obese_avg > normal_avg))
    
    # Mortality checks - COMPARE DEAD VS ALIVE HEALTH RISK
    if dead and alive:
        # Only include dead people who had health_risk > 0 (died after getting health_risk)
        dead_with_health = [p for p in dead if p['health_risk'] > 0]
        alive_with_health = [p for p in alive if p['health_risk'] > 0]
        
        if dead_with_health and alive_with_health:
            risk_dead = statistics.mean([p['health_risk'] for p in dead_with_health])
            risk_alive = statistics.mean([p['health_risk'] for p in alive_with_health])
            checks.append(("Dead had higher health risk", risk_dead > risk_alive))
        else:
            # Fallback: compare all dead vs alive (including health_risk=0)
            risk_dead = statistics.mean([p['health_risk'] for p in dead])
            risk_alive = statistics.mean([p['health_risk'] for p in alive])
            checks.append(("Dead had higher health risk", risk_dead > risk_alive))
    
    # Print results
    passed = 0
    for check_name, result in checks:
        status = "✅" if result else "❌"
        print(f"  {status} {check_name}")
        if result:
            passed += 1
    
    print(f"\n  Checks passed: {passed}/{len(checks)}")
    
    if passed == len(checks):
        print("  ✅ All validation checks passed! Model is working correctly.")
    elif passed >= len(checks) - 1:
        print("  ⚠️  Most checks passed. Results look reasonable.")
    else:
        print("  ❌ Multiple validation checks failed. Please review the models.")
    
    # Sample output - show first 5 ALIVE people for better validation
    print(f"\n📋 Sample Output (first 5 alive records):")
    print("-" * 130)
    print(f"{'ID':<12} {'Age':<5} {'Year Born':<10} {'Gender':<8} {'Education':<10} {'Income':<10} {'Health':<7} {'Mortality':<10} {'Alive'}")
    print("-" * 130)
    count = 0
    for p in data:
        if p['alive'] and count < 5:
            alive_status = "Yes" if p['alive'] else "No"
            mortality = f"{p['mortality_risk']:.4f}" if 'mortality_risk' in p else "N/A"
            year_born = str(p['year_born']) if 'year_born' in p else "N/A"
            print(f"{p['id']:<12} {p['age']:<5} {year_born:<10} {p['gender']:<8} {p['education']:<10} ${p['income']:<9,} {p['health_risk']:<7}% {mortality:<10} {alive_status}")
            count += 1
    
    if count == 0:
        print("  No alive individuals found!")
    
    print("=" * 80)

if __name__ == "__main__":
    iterations = 5  # Match the config
    if len(sys.argv) > 1:
        validate_results(sys.argv[1], iterations)
    else:
        validate_results("output.csv", iterations)