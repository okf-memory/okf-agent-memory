# Progressive Disclosure Benchmark: Monolith Context vs. OKF Agent Memory

> **Automated Benchmark Suite** quantifying prompt token reduction, prefill latency (Time-To-First-Token / TTFT), and instruction adherence on local Large Language Models (Gemma 26B, Gemma 12B, Qwen 35B, Llama 3) via LM Studio or any OpenAI-compatible server.

---

## 🎯 Benchmark Objective

AI coding agents often suffer from **Context Bloat** and **Attention Degradation (Lost-in-the-Middle)** when entire project architectures are dumped into monolithic context files (`CLAUDE.md`, `AGENTS.md`).

This benchmark suite measures the difference between two approaches when asking an LLM to implement an enterprise encryption payload:

1. **Run 1: Monolith Context Dump**  
   The LLM is fed the entire project documentation (~11.5k characters, ~3,000 tokens) covering architecture, databases, Kubernetes, Stripe, Redis, telemetry, and security policies.
2. **Run 2: OKF Progressive Disclosure**  
   The Go core executes an in-memory BM25 search (`< 300 µs`), isolates the single relevant concept (`security/encryption-policy`, ~500 tokens), and feeds only that atomic concept to the LLM.

---

## 📊 Live Benchmark Evidence

Measured on Apple Silicon with LM Studio:

### Gemma 26B (`google/gemma-4-26b-a4b-qat`)
| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Prompt Input Tokens** | `3,034` tokens | `603` tokens | **-80.1% context overhead** |
| **Prefill Latency (TTFT)** | `47.7 s` | `27.1 s` | **1.8x faster Time-To-First-Token** |
| **Total Turn Time** | `70.1 s` | `49.3 s` | **-20.8 s total turnaround** |
| **Policy Compliance** | 4/4 passed | 4/4 passed | 100% Consistent |

### Gemma 12B (`google/gemma-4-12b-qat`)
| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Prompt Input Tokens** | `3,034` tokens | `603` tokens | **-80.1% context overhead** |
| **Prefill Latency (TTFT)** | `50.1 s` | `45.8 s` | **1.1x faster** |
| **Policy Compliance** | 1/4 passed (attention lost) | 4/4 passed (strict adherence) | **Progressive Disclosure prevents hallucination** |

---

## 🚀 How to Reproduce Locally

The benchmark runner `okf-benchmark` is written in **100% pure Go** with zero external dependencies.

### Option A: Local LLMs (LM Studio or Ollama)
```bash
# 1. Local LM Studio (Default, listening on http://localhost:1234)
go run ./cmd/okf-benchmark

# 2. Local Ollama (listening on http://localhost:11434)
go run ./cmd/okf-benchmark -p ollama -m llama3.2
```

### Option B: Cloud Providers (OpenAI, Claude, Gemini)
```bash
# 1. OpenAI (uses OPENAI_API_KEY environment variable)
export OPENAI_API_KEY="sk-..."
go run ./cmd/okf-benchmark -p openai -m gpt-4o

# 2. Anthropic Claude (uses ANTHROPIC_API_KEY environment variable)
export ANTHROPIC_API_KEY="sk-ant-..."
go run ./cmd/okf-benchmark -p claude -m claude-3-7-sonnet-20250219

# 3. Google Gemini (uses GEMINI_API_KEY environment variable)
export GEMINI_API_KEY="AIza..."
go run ./cmd/okf-benchmark -p gemini -m gemini-2.5-flash
```

### Useful Flags
```bash
# Compare the exact generated Go code side-by-side:
go run ./cmd/okf-benchmark -o

# Or simulate in dry-run mode (no LLM or API keys required):
go run ./cmd/okf-benchmark --dry-run
```

Alternatively using Make:
```bash
make benchmark ARGS="-p openai -m gpt-4o"
make benchmark ARGS="-dry-run -o"
```

---

## 📁 Directory Layout

```
benchmarks/
├── README.md               # This guide
├── data/
│   ├── MONOLITH_DOCS.md    # Combined documentation dump (~11.5k chars)
│   └── knowledge/          # Compliant OKF v0.2 bundle (8 atomic concepts)
│       ├── index.md
│       ├── log.md
│       └── security/encryption-policy.md  # Target concept
└── results/
    ├── BENCHMARK_RESULTS_gemma-4-26b-a4b-qat.md
    └── BENCHMARK_RESULTS_google_gemma-4-12b-qat.md
```
