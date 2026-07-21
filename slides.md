---

# Hekate
## Microsimulation Engine

**NetLogo-inspired · Lua-powered · Scalable**

---

# NetLogo-Inspired

| NetLogo | Hekate |
|---------|--------|
| Single download | Single binary |
| Easy to learn | Easy to learn |
| Build models fast | Build models fast |
| ❌ ~10K agents max | ✅ Millions to billions |

**Same philosophy · Built for the big data era**

---

# Why Lua?

**Lightweight · Fast · Embeddable**

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

| Feature | Benefit |
|---------|---------|
| Tiny footprint | Fast startup |
| Easy to learn | Shallow curve |
| Embeddable | No compilation |
| Dynamic typing | Simple logic |

---

# YAML Model

```yaml
models:
  - name: aging_model
    priority: 1
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

**Models = Data · Not Code**

---

# 1M People · 5 Years

| Metric | Result |
|--------|--------|
| Population | 1,000,000 |
| Time | 40 minutes |
| Memory | ~20MB |
| Alive | 451,015 (45.1%) |
| Dead | 548,985 (54.9%) |

**Models:** Aging ✅ · Income ✅ · Health Risk ✅ · Mortality ✅

---

# Summary

**Hekate is NetLogo for Microsimulation**

- 🔮 **Models as Data** · YAML + Lua
- 🚀 **Scales** · 10 to 10 billion
- 📊 **Built-in** · Linear & Logistic Regression
- 💾 **Streaming** · Constant memory
- 🎯 **Single binary** · Runs anywhere

**Zero to model in minutes**
