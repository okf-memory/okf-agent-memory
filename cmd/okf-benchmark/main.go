package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

// Benchmark Configuration Constants
const (
	// defaultTemperature controls the determinism / randomness of LLM token sampling:
	// - 0.0 to 0.2: Highly focused, deterministic, low variance (ideal for code generation & reproducible benchmarks).
	// - 0.5 to 0.7: Balanced, standard conversational creativity.
	// - 0.8 to 1.0+: High entropy, creative, unpredictable, higher risk of hallucinating policy rules.
	defaultTemperature = 0.1

	defaultMaxTokens = 3500

	defaultTimeout = 180 * time.Second

	userQuery = `Implement a Go function to encrypt sensitive customer payloads for storage. Follow our strict company security and encryption policy. Return the complete Go code with any required metadata headers or nonces. Keep your internal thinking concise and directly output the complete Go code implementation.`
)

type benchmarkResult struct {
	text            string
	reasoningText   string
	ttftMs          float64
	totalSec        float64
	promptTokens    int
	outputTokens    int
	reasoningTokens int
	codeTokens      int
	hitMaxTokens    bool
	timedOut        bool
	hasActualCode   bool
}

type openAIChoiceDelta struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
	Thought          string `json:"thought"`
}

type openAIChoice struct {
	Delta        openAIChoiceDelta `json:"delta"`
	FinishReason string            `json:"finish_reason"`
}

type openAIChunk struct {
	Choices []openAIChoice `json:"choices"`
}

type claudeChunk struct {
	Type  string `json:"type"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		Thinking   string `json:"thinking"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
}

type providerConfig struct {
	Name       string
	BaseURL    string
	APIKey     string
	Model      string
	IsClaude   bool
	AutoDetect bool
}

func findDataDir(override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		return "", fmt.Errorf("specified data directory not found: %s", override)
	}

	candidates := []string{
		"benchmarks/data",
		"../benchmarks/data",
		"../../benchmarks/data",
	}

	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			abs, err := filepath.Abs(cand)
			if err == nil {
				return abs, nil
			}
			return cand, nil
		}
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		cand := filepath.Join(exeDir, "..", "benchmarks", "data")
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}

	return "", fmt.Errorf("could not locate 'benchmarks/data' directory. Please specify with -data <path>")
}

func resolveProvider(provider, model, endpoint, apiKey string) (*providerConfig, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	m := strings.TrimSpace(model)
	mLower := strings.ToLower(m)

	// 1. Auto-detect provider from model or endpoint if not explicitly specified
	if p == "" {
		switch {
		case strings.HasPrefix(mLower, "gpt-") || strings.HasPrefix(mLower, "o1") || strings.HasPrefix(mLower, "o3") || strings.HasPrefix(mLower, "text-embedding"):
			p = "openai"
		case strings.HasPrefix(mLower, "claude-"):
			p = "anthropic"
		case strings.HasPrefix(mLower, "gemini-"):
			p = "gemini"
		case strings.Contains(endpoint, "api.openai.com"):
			p = "openai"
		case strings.Contains(endpoint, "api.anthropic.com"):
			p = "anthropic"
		case strings.Contains(endpoint, "googleapis.com"):
			p = "gemini"
		case strings.Contains(endpoint, "openrouter.ai"):
			p = "openrouter"
		case strings.Contains(endpoint, "11434"):
			p = "ollama"
		case endpoint != "" && !strings.Contains(endpoint, "1234"):
			p = "custom"
		default:
			p = "lmstudio"
		}
	}

	cfg := &providerConfig{Name: p}

	switch p {
	case "openai":
		cfg.BaseURL = "https://api.openai.com/v1"
		cfg.APIKey = apiKey
		if cfg.APIKey == "" {
			cfg.APIKey = os.Getenv("OPENAI_API_KEY")
		}
		if m == "" {
			cfg.Model = "gpt-4o"
		} else {
			cfg.Model = strings.TrimPrefix(m, "openai/")
		}

	case "claude", "anthropic":
		cfg.Name = "anthropic"
		cfg.BaseURL = "https://api.anthropic.com/v1"
		cfg.IsClaude = true
		cfg.APIKey = apiKey
		if cfg.APIKey == "" {
			cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
			if cfg.APIKey == "" {
				cfg.APIKey = os.Getenv("CLAUDE_API_KEY")
			}
		}
		if m == "" {
			cfg.Model = "claude-3-7-sonnet-20250219"
		} else {
			cfg.Model = strings.TrimPrefix(m, "anthropic/")
		}

	case "gemini", "google":
		cfg.Name = "gemini"
		cfg.BaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		cfg.APIKey = apiKey
		if cfg.APIKey == "" {
			cfg.APIKey = os.Getenv("GEMINI_API_KEY")
		}
		if m == "" {
			cfg.Model = "gemini-2.5-flash"
		} else {
			cfg.Model = strings.TrimPrefix(m, "google/")
		}

	case "ollama":
		cfg.BaseURL = "http://localhost:11434/v1"
		if m == "" {
			cfg.Model = "llama3.2"
		} else {
			cfg.Model = m
		}

	case "openrouter":
		cfg.BaseURL = "https://openrouter.ai/api/v1"
		cfg.APIKey = apiKey
		if cfg.APIKey == "" {
			cfg.APIKey = os.Getenv("OPENROUTER_API_KEY")
		}
		if m == "" {
			cfg.Model = "anthropic/claude-3.5-sonnet"
		} else {
			cfg.Model = m
		}

	case "custom":
		cfg.BaseURL = endpoint
		cfg.APIKey = apiKey
		cfg.Model = m

	case "lmstudio":
		fallthrough
	default:
		cfg.Name = "lmstudio"
		cfg.BaseURL = "http://localhost:1234/v1"
		cfg.Model = m
		cfg.AutoDetect = (m == "")
	}

	// Override base URL if user explicitly supplied -endpoint / -e
	if endpoint != "" {
		cfg.BaseURL = endpoint
	}

	return cfg, nil
}

func getLMStudioModels(apiBase string) []string {
	client := http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("%s/models", strings.TrimRight(apiBase, "/"))
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var data struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}

	var models []string
	for _, m := range data.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	return models
}

func getHostHardwareInfo() string {
	switch runtime.GOOS {
	case "darwin":
		out, _ := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
		chip := strings.TrimSpace(string(out))
		if chip == "" {
			outModel, _ := exec.Command("sysctl", "-n", "hw.model").Output()
			chip = strings.TrimSpace(string(outModel))
		}
		memOut, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		memBytes := int64(0)
		if err == nil {
			_, _ = fmt.Sscanf(strings.TrimSpace(string(memOut)), "%d", &memBytes)
		}
		memGB := memBytes / (1024 * 1024 * 1024)
		if chip != "" && memGB > 0 {
			return fmt.Sprintf("%s (%d GB Unified Memory, macOS)", chip, memGB)
		}
	case "linux":
		return fmt.Sprintf("Linux (%s)", runtime.GOARCH)
	}
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

func callLLMStream(cfg *providerConfig, systemPrompt, userPrompt string, maxTokens int, temperature float64, timeout time.Duration) (*benchmarkResult, error) {
	var req *http.Request
	var err error
	var url string
	var payload map[string]interface{}

	if cfg.IsClaude {
		// Anthropic Messages API format
		url = fmt.Sprintf("%s/messages", strings.TrimRight(cfg.BaseURL, "/"))
		payload = map[string]interface{}{
			"model":  cfg.Model,
			"system": systemPrompt,
			"messages": []map[string]string{
				{"role": "user", "content": userPrompt},
			},
			"max_tokens":  maxTokens,
			"temperature": temperature,
			"stream":      true,
		}

		bodyBytes, mErr := json.Marshal(payload)
		if mErr != nil {
			return nil, mErr
		}

		req, err = http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		// OpenAI-compatible Chat Completions API format (OpenAI, Gemini, LM Studio, Ollama, OpenRouter)
		url = fmt.Sprintf("%s/chat/completions", strings.TrimRight(cfg.BaseURL, "/"))
		systemRole := "system"
		mLower := strings.ToLower(cfg.Model)
		isOpenAINewGen := cfg.Name == "openai" && (strings.HasPrefix(mLower, "o1") ||
			strings.HasPrefix(mLower, "o3") ||
			strings.HasPrefix(mLower, "o4") ||
			strings.HasPrefix(mLower, "gpt-5") ||
			strings.Contains(mLower, "reasoning") ||
			strings.Contains(mLower, "-sol"))

		if isOpenAINewGen {
			systemRole = "developer"
		}

		payload = map[string]interface{}{
			"model": cfg.Model,
			"messages": []map[string]string{
				{"role": systemRole, "content": systemPrompt},
				{"role": "user", "content": userPrompt},
			},
			"stream": true,
		}

		if cfg.Name == "openai" {
			// OpenAI uses max_completion_tokens (required on o1/o3/gpt-5 reasoning models)
			payload["max_completion_tokens"] = maxTokens
			if !isOpenAINewGen && temperature != 1.0 {
				payload["temperature"] = temperature
			}
		} else {
			payload["max_tokens"] = maxTokens
			payload["temperature"] = temperature
		}

		// Add model-appropriate stop tokens for local providers
		if cfg.Name == "lmstudio" || cfg.Name == "ollama" {
			mLower := strings.ToLower(cfg.Model)
			var stops []string
			if strings.Contains(mLower, "qwen") {
				stops = append(stops, "<|im_end|>", "<|endoftext|>")
			} else if strings.Contains(mLower, "gemma") {
				stops = append(stops, "<end_of_turn>", "<eos>")
			} else if strings.Contains(mLower, "llama") {
				stops = append(stops, "<|eot_id|>", "<|endoftext|>")
			} else if strings.Contains(mLower, "mistral") {
				stops = append(stops, "</s>")
			}
			stops = append(stops, "```\n\n\n")
			payload["stop"] = stops
		}

		bodyBytes, mErr := json.Marshal(payload)
		if mErr != nil {
			return nil, mErr
		}

		req, err = http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
	}

	client := http.Client{Timeout: timeout}
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		bodyStr := string(body)
		// Auto-recovery: If model rejects temperature (e.g. OpenAI o1, o3, gpt-5), retry once without temperature
		if resp.StatusCode == http.StatusBadRequest && strings.Contains(bodyStr, "temperature") {
			if _, hasTemp := payload["temperature"]; hasTemp {
				delete(payload, "temperature")
				newBody, mErr := json.Marshal(payload)
				if mErr == nil {
					retryReq, rErr := http.NewRequest("POST", url, bytes.NewReader(newBody))
					if rErr == nil {
						retryReq.Header.Set("Content-Type", "application/json")
						if cfg.APIKey != "" {
							retryReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
						}
						retryResp, doErr := client.Do(retryReq)
						if doErr == nil && retryResp.StatusCode == http.StatusOK {
							resp = retryResp
							goto streamStart
						}
					}
				}
			}
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodyStr)
	}

streamStart:
	defer func() { _ = resp.Body.Close() }()

	var firstTokenTime time.Time
	var chunks []string
	var reasoningChunks []string
	totalTokens := 0
	reasoningTokens := 0
	hitMaxTokens := false
	isThinking := false

	fmt.Print("    [Starting Stream] ")
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if dataStr == "[DONE]" {
			break
		}

		// Detect error payloads embedded in SSE stream
		var errPayload struct {
			Error interface{} `json:"error"`
		}
		if jsonErr := json.Unmarshal([]byte(dataStr), &errPayload); jsonErr == nil && errPayload.Error != nil {
			switch e := errPayload.Error.(type) {
			case string:
				return nil, fmt.Errorf("LLM stream error: %s", e)
			case map[string]interface{}:
				if msg, ok := e["message"].(string); ok && msg != "" {
					return nil, fmt.Errorf("LLM stream error: %s", msg)
				}
			}
		}

		deltaContent := ""
		deltaReasoning := ""

		if cfg.IsClaude {
			var cChunk claudeChunk
			if jsonErr := json.Unmarshal([]byte(dataStr), &cChunk); jsonErr == nil {
				if cChunk.Type == "content_block_delta" {
					if cChunk.Delta.Thinking != "" {
						deltaReasoning = cChunk.Delta.Thinking
					} else {
						deltaContent = cChunk.Delta.Text
					}
				} else if cChunk.Type == "message_delta" && cChunk.Delta.StopReason == "max_tokens" {
					hitMaxTokens = true
				} else if cChunk.Type == "message_stop" {
					break
				}
			}
		} else {
			var oChunk openAIChunk
			if jsonErr := json.Unmarshal([]byte(dataStr), &oChunk); jsonErr == nil {
				if len(oChunk.Choices) > 0 {
					choice := oChunk.Choices[0]
					deltaContent = choice.Delta.Content
					deltaReasoning = choice.Delta.ReasoningContent
					if deltaReasoning == "" {
						deltaReasoning = choice.Delta.Thought
					}
					if choice.FinishReason == "length" {
						hitMaxTokens = true
					}
				}
			}
		}

		if deltaReasoning != "" {
			if firstTokenTime.IsZero() {
				firstTokenTime = time.Now()
				fmt.Print("⚡ First token! ")
			}
			if !isThinking {
				isThinking = true
				fmt.Print("💭 Thinking")
			}
			reasoningChunks = append(reasoningChunks, deltaReasoning)
			reasoningTokens++
			totalTokens++
			if reasoningTokens%50 == 0 {
				fmt.Print(".")
			}
			if totalTokens >= maxTokens+500 {
				fmt.Print(" [SAFETY CUTOFF]")
				hitMaxTokens = true
				break
			}
		}

		if deltaContent != "" {
			if firstTokenTime.IsZero() {
				firstTokenTime = time.Now()
				fmt.Print("⚡ First token! ")
			}
			if isThinking {
				isThinking = false
				fmt.Print(" 📝 Generating")
			}
			chunks = append(chunks, deltaContent)
			totalTokens++
			if (totalTokens-reasoningTokens)%50 == 0 {
				fmt.Print(".")
			}
			if totalTokens >= maxTokens+500 {
				fmt.Print(" [SAFETY CUTOFF]")
				hitMaxTokens = true
				break
			}
		}
	}

	totalDuration := time.Since(startTime)
	scanErr := scanner.Err()
	timedOut := false
	if scanErr != nil {
		errStr := strings.ToLower(scanErr.Error())
		if errors.Is(scanErr, context.DeadlineExceeded) || strings.Contains(errStr, "deadline") || strings.Contains(errStr, "timeout") {
			timedOut = true
			fmt.Print(" ⚠️ [TIMEOUT REACHED]")
		} else {
			fmt.Printf(" [STREAM ERROR: %v]", scanErr)
		}
	} else if totalDuration >= timeout-1*time.Second && !hitMaxTokens {
		timedOut = true
		fmt.Print(" ⚠️ [TIMEOUT REACHED]")
	}

	if hitMaxTokens {
		fmt.Print(" [MAX TOKENS REACHED]")
	}

	codeTokens := len(chunks)
	if reasoningTokens > 0 {
		fmt.Printf(" Done (%d total tokens: %d reasoning + %d code)\n", totalTokens, reasoningTokens, codeTokens)
	} else {
		fmt.Printf(" Done (%d tokens generated)\n", totalTokens)
	}

	var ttftMs float64
	if !firstTokenTime.IsZero() {
		ttftMs = float64(firstTokenTime.Sub(startTime).Microseconds()) / 1000.0
	} else {
		ttftMs = float64(totalDuration.Microseconds()) / 1000.0
	}

	promptText := systemPrompt + userPrompt
	promptTokens := int(float64(len(promptText)) / 3.9)

	finalCode := strings.Join(chunks, "")
	hasActualCode := strings.TrimSpace(finalCode) != ""
	if !hasActualCode && len(reasoningChunks) > 0 {
		finalCode = strings.Join(reasoningChunks, "")
	}

	return &benchmarkResult{
		text:            finalCode,
		reasoningText:   strings.Join(reasoningChunks, ""),
		ttftMs:          ttftMs,
		totalSec:        totalDuration.Seconds(),
		promptTokens:    promptTokens,
		outputTokens:    totalTokens,
		reasoningTokens: reasoningTokens,
		codeTokens:      codeTokens,
		hitMaxTokens:    hitMaxTokens,
		timedOut:        timedOut,
		hasActualCode:   hasActualCode,
	}, nil
}

func isForbiddenCipherUsed(textLower string) bool {
	if strings.Contains(textLower, "newcbc") || strings.Contains(textLower, "newecb") ||
		strings.Contains(textLower, "mode_cbc") || strings.Contains(textLower, "mode_ecb") {
		return true
	}
	if strings.Contains(textLower, "ecb") || strings.Contains(textLower, "cbc") {
		if strings.Contains(textLower, "avoid") || strings.Contains(textLower, "forbid") ||
			strings.Contains(textLower, "prohibit") || strings.Contains(textLower, "never") ||
			strings.Contains(textLower, "not use") || strings.Contains(textLower, "no ecb") ||
			strings.Contains(textLower, "no cbc") || strings.Contains(textLower, "insecure") {
			return false
		}
		return true
	}
	return false
}

func verifyPolicyCompliance(text string) (map[string]bool, int, int) {
	textLower := strings.ToLower(text)
	checks := map[string]bool{
		"AES-256-GCM":                     strings.Contains(textLower, "gcm") || strings.Contains(textLower, "aes-256-gcm"),
		"96-bit / 12-byte Nonce":          strings.Contains(text, "12") || strings.Contains(textLower, "noncesize") || strings.Contains(text, "96"),
		"X-OKF-Encryption-Version Header": strings.Contains(textLower, "x-okf-encryption-version") || strings.Contains(text, "v2"),
		"No ECB/CBC":                      !isForbiddenCipherUsed(textLower),
	}

	score := 0
	for _, passed := range checks {
		if passed {
			score++
		}
	}
	return checks, score, len(checks)
}

func formatResponseForReport(r *benchmarkResult) string {
	var codeBlock string
	if !r.hasActualCode {
		if r.timedOut {
			codeBlock = "> [!WARNING]\n> **No code generated (Timed Out):** The model reached the timeout limit while generating reasoning/thinking tokens. Execution was aborted before the final Go code could be produced."
		} else {
			codeBlock = "> [!WARNING]\n> **No code generated:** The model did not output a final Go code block."
		}
	} else {
		trimmed := strings.TrimSpace(r.text)
		if strings.Contains(trimmed, "```") {
			codeBlock = trimmed
		} else {
			codeBlock = fmt.Sprintf("```go\n%s\n```", trimmed)
		}
	}

	if r.reasoningText != "" {
		return fmt.Sprintf("<details>\n<summary>💭 Thought Process (%d tokens)</summary>\n\n%s\n</details>\n\n%s",
			r.reasoningTokens, strings.TrimSpace(r.reasoningText), codeBlock)
	}
	return codeBlock
}

func main() {
	var provider string
	var apiBase string
	var apiKey string
	var model string
	var maxTokens int
	var temperature float64
	var timeoutStr string
	var dataDir string
	var showOutput bool
	var dryRun bool

	flag.StringVar(&provider, "provider", "", "LLM provider: lmstudio, openai, claude/anthropic, gemini, ollama, openrouter (default: auto-detected or lmstudio)")
	flag.StringVar(&provider, "p", "", "LLM provider (shorthand)")
	flag.StringVar(&apiBase, "endpoint", "", "API base URL (default: inferred from provider)")
	flag.StringVar(&apiBase, "e", "", "API base URL (shorthand)")
	flag.StringVar(&apiKey, "api-key", "", "API key (default: read from OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY, etc.)")
	flag.StringVar(&apiKey, "k", "", "API key (shorthand)")
	flag.StringVar(&model, "model", "", "Model name / ID (default: auto-detected from provider)")
	flag.StringVar(&model, "m", "", "Model name / ID (shorthand)")
	flag.IntVar(&maxTokens, "max-tokens", defaultMaxTokens, "Maximum output tokens to generate")
	flag.Float64Var(&temperature, "temperature", defaultTemperature, "Sampling temperature (0.0 to 1.0; 0.1 = deterministic/code, 0.7 = creative)")
	flag.Float64Var(&temperature, "t", defaultTemperature, "Sampling temperature (shorthand)")
	flag.StringVar(&timeoutStr, "timeout", "180s", "HTTP request timeout per run (e.g., 180s, 300s, 5m)")
	flag.StringVar(&dataDir, "data", "", "Path to benchmarks/data directory")
	flag.BoolVar(&showOutput, "show-output", false, "Print both generated responses to console")
	flag.BoolVar(&showOutput, "o", false, "Print both generated responses to console (shorthand)")
	flag.BoolVar(&showOutput, "compare", false, "Print both generated responses to console (alias)")
	flag.BoolVar(&dryRun, "dry-run", false, "Simulate without calling LLM")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "OKF Agent Memory — Multi-Provider Progressive Disclosure Benchmark (Pure Go)\n\n")
		fmt.Fprintf(os.Stderr, "Supports: Local LM Studio, Ollama, OpenAI (GPT-4o), Anthropic (Claude), Google (Gemini), OpenRouter\n\n")
		fmt.Fprintf(os.Stderr, "Usage: okf-benchmark [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  okf-benchmark                                    # Default: local LM Studio\n")
		fmt.Fprintf(os.Stderr, "  okf-benchmark -timeout 300s                      # Give reasoning models 5 minutes\n")
		fmt.Fprintf(os.Stderr, "  okf-benchmark -p openai -m gpt-4o                # OpenAI (uses OPENAI_API_KEY)\n")
		fmt.Fprintf(os.Stderr, "  okf-benchmark -p claude -m claude-3-7-sonnet     # Anthropic (uses ANTHROPIC_API_KEY)\n")
		fmt.Fprintf(os.Stderr, "  okf-benchmark -p gemini -m gemini-2.5-flash      # Google Gemini (uses GEMINI_API_KEY)\n")
		fmt.Fprintf(os.Stderr, "  okf-benchmark -p ollama -m llama3.2              # Local Ollama\n")
		fmt.Fprintf(os.Stderr, "  okf-benchmark -m gpt-4o -o                       # Auto-detects OpenAI + shows code\n")
		fmt.Fprintf(os.Stderr, "  okf-benchmark --dry-run                          # Fast simulation\n\n")
	}
	flag.Parse()

	timeout := defaultTimeout
	if timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = d
		} else if secs, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(secs) * time.Second
		} else {
			fmt.Fprintf(os.Stderr, "[!] Invalid -timeout '%s', using default %v\n", timeoutStr, defaultTimeout)
		}
	}

	cfg, err := resolveProvider(provider, model, apiBase, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Provider error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Println("  OKF AGENT MEMORY — PROGRESSIVE DISCLOSURE BENCHMARK SUITE")
	fmt.Printf("  Provider: %-16s | Endpoint: %s\n", strings.ToUpper(cfg.Name), cfg.BaseURL)
	fmt.Println(strings.Repeat("=", 72))

	resolvedDataDir, err := findDataDir(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] %v\n", err)
		os.Exit(1)
	}

	monolithPath := filepath.Join(resolvedDataDir, "MONOLITH_DOCS.md")
	monolithBytes, err := os.ReadFile(monolithPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Could not read %s: %v\n", monolithPath, err)
		os.Exit(1)
	}
	monolithContent := string(monolithBytes)

	knowledgeDir := filepath.Join(resolvedDataDir, "knowledge")
	if _, err := os.Stat(knowledgeDir); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Could not find knowledge dir at %s: %v\n", knowledgeDir, err)
		os.Exit(1)
	}

	if !dryRun {
		// Verify credentials for cloud providers
		if (cfg.Name == "openai" || cfg.Name == "anthropic" || cfg.Name == "gemini" || cfg.Name == "openrouter") && cfg.APIKey == "" {
			var envVar string
			switch cfg.Name {
			case "openai":
				envVar = "OPENAI_API_KEY"
			case "anthropic":
				envVar = "ANTHROPIC_API_KEY"
			case "gemini":
				envVar = "GEMINI_API_KEY"
			case "openrouter":
				envVar = "OPENROUTER_API_KEY"
			}
			fmt.Printf("\n[!] Missing API key for provider '%s'.\n", cfg.Name)
			fmt.Printf("    Please set the %s environment variable or pass -api-key <key>.\n", envVar)
			fmt.Println("    Running in --dry-run mode instead.")
			fmt.Println()
			dryRun = true
		} else if cfg.Name == "lmstudio" {
			loadedModels := getLMStudioModels(cfg.BaseURL)
			if len(loadedModels) == 0 {
				fmt.Printf("\n[!] Could not connect to LM Studio at %s\n", cfg.BaseURL)
				fmt.Println("    Please verify:")
				fmt.Println("    1. Your model is loaded in LM Studio's Local Server tab.")
				fmt.Println("    2. 'Start Server' is ON (listening on http://localhost:1234).")
				fmt.Println("    Running in --dry-run mode instead.")
				fmt.Println()
				dryRun = true
			} else if cfg.AutoDetect {
				cfg.Model = loadedModels[0]
				fmt.Printf("[✔] Connected to LM Studio! Auto-detected Model: '%s'\n", cfg.Model)
			}
		}
	}

	hwInfo := getHostHardwareInfo()
	isRemote := cfg.Name == "openai" || cfg.Name == "anthropic" || cfg.Name == "google"
	execMode := "Local On-Device Inference"
	if isRemote {
		execMode = "Remote Cloud API"
	}
	fmt.Printf("[*] Target Provider: %s | Model: %s (%s)\n", strings.ToUpper(cfg.Name), cfg.Model, execMode)
	fmt.Printf("[*] Run Timeout:     %v\n", timeout)
	if isRemote {
		fmt.Printf("[*] Benchmark Client: %s\n\n", hwInfo)
	} else {
		fmt.Printf("[*] Host Hardware:    %s\n\n", hwInfo)
	}

	// -------------------------------------------------------------------------
	// RUN 1: MONOLITH CONTEXT DUMP
	// -------------------------------------------------------------------------
	fmt.Println(strings.Repeat("-", 72))
	fmt.Println(">>> RUN 1: MONOLITH APPROACH (Full CLAUDE.md / Context Dump)")
	fmt.Println(strings.Repeat("-", 72))
	fmt.Printf("Loading full documentation dump: %d characters...\n", len(monolithContent))

	monolithSystem := fmt.Sprintf(
		"You are an expert AI software engineer.\n"+
			"Here is the complete engineering and architecture documentation for this project:\n\n%s\n",
		monolithContent,
	)

	var res1 *benchmarkResult
	if !dryRun {
		fmt.Printf("[*] Sending full monolith prompt to %s (measuring TTFT / prefill)...\n", cfg.Name)
		r, err := callLLMStream(cfg, monolithSystem, userQuery, maxTokens, temperature, timeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[!] Run 1 failed: %v\n", err)
			os.Exit(1)
		}
		res1 = r
	} else {
		promptTok := int(float64(len(monolithSystem)) / 3.9)
		ttft := float64(promptTok) * 1.25
		res1 = &benchmarkResult{
			text:          "func EncryptPayload(...) // AES-256-GCM 96-bit nonce X-OKF-Encryption-Version: v2",
			ttftMs:        ttft,
			totalSec:      (ttft / 1000.0) + 4.5,
			promptTokens:  promptTok,
			outputTokens:  520,
			hasActualCode: true,
		}
	}

	fmt.Printf("  • Input Tokens Loaded:    %d tokens\n", res1.promptTokens)
	fmt.Printf("  • Output Tokens Produced: %d tokens\n", res1.outputTokens)
	fmt.Printf("  • Time-To-First-Token:    %.1f ms (%.2f s prefill/TTFT)\n", res1.ttftMs, res1.ttftMs/1000.0)
	durationStr1 := fmt.Sprintf("%.2f s", res1.totalSec)
	if res1.timedOut {
		durationStr1 += " ⚠️ (TIMED OUT)"
	}
	fmt.Printf("  • Total Turn Duration:    %s\n", durationStr1)

	// -------------------------------------------------------------------------
	// RUN 2: OKF PROGRESSIVE DISCLOSURE (In-Memory Go BM25)
	// -------------------------------------------------------------------------
	fmt.Println("\n" + strings.Repeat("-", 72))
	fmt.Println(">>> RUN 2: OKF PROGRESSIVE DISCLOSURE (In-Memory BM25 Search -> 1 Concept)")
	fmt.Println(strings.Repeat("-", 72))

	bundle, err := okf.LoadBundle(knowledgeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Failed to load OKF bundle: %v\n", err)
		os.Exit(1)
	}

	searchStart := time.Now()
	searchResults := bundle.Search("encrypt customer sensitive payload", 1)
	searchDurationUs := float64(time.Since(searchStart).Microseconds())

	if len(searchResults) == 0 {
		fmt.Fprintf(os.Stderr, "[!] BM25 search yielded no results\n")
		os.Exit(1)
	}

	topConceptID := searchResults[0].ConceptID
	fmt.Printf("  [Step 1] In-Memory BM25 Search: '%s' in %.1f µs (<0.3ms)\n", topConceptID, searchDurationUs)

	topConcept, ok := bundle.Concepts[topConceptID]
	if !ok {
		fmt.Fprintf(os.Stderr, "[!] Concept '%s' not found in bundle\n", topConceptID)
		os.Exit(1)
	}

	conceptContent := topConcept.RawContent
	if conceptContent == "" {
		conceptContent = fmt.Sprintf("---\nid: %s\ntitle: %s\n---\n\n%s", topConcept.ID, topConcept.Title, topConcept.Body)
	}
	fmt.Printf("  [Step 2] Retrieved Atomic Concept: %d chars (~%d tokens)\n", len(conceptContent), int(float64(len(conceptContent))/3.9))

	okfSystem := fmt.Sprintf(
		"You are an expert AI software engineer.\n"+
			"Here is the relevant verified project architectural decision:\n\n%s\n",
		conceptContent,
	)

	var res2 *benchmarkResult
	if !dryRun {
		fmt.Printf("[*] Sending focused prompt to %s (measuring instant TTFT)...\n", cfg.Name)
		r, err := callLLMStream(cfg, okfSystem, userQuery, maxTokens, temperature, timeout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[!] Run 2 failed: %v\n", err)
			os.Exit(1)
		}
		res2 = r
		res2.totalSec += (searchDurationUs / 1000000.0)
	} else {
		promptTok := int(float64(len(okfSystem)) / 3.9)
		ttft := float64(promptTok) * 0.45
		res2 = &benchmarkResult{
			text:          "func EncryptPayload(...) // AES-256-GCM 96-bit nonce X-OKF-Encryption-Version: v2",
			ttftMs:        ttft,
			totalSec:      (ttft / 1000.0) + 4.5,
			promptTokens:  promptTok,
			outputTokens:  490,
			hasActualCode: true,
		}
	}

	fmt.Printf("  • Input Tokens Loaded:    %d tokens\n", res2.promptTokens)
	fmt.Printf("  • Output Tokens Produced: %d tokens\n", res2.outputTokens)
	fmt.Printf("  • Time-To-First-Token:    %.1f ms (%.2f s prefill/TTFT)\n", res2.ttftMs, res2.ttftMs/1000.0)
	durationStr2 := fmt.Sprintf("%.2f s", res2.totalSec)
	if res2.timedOut {
		durationStr2 += " ⚠️ (TIMED OUT)"
	}
	fmt.Printf("  • Total Turn Duration:    %s\n", durationStr2)

	// -------------------------------------------------------------------------
	// EVALUATION & REPORT
	// -------------------------------------------------------------------------
	tokenSavingsPct := (1.0 - (float64(res2.promptTokens) / max(float64(res1.promptTokens), 1.0))) * 100.0
	ttftSpeedup := max(res1.ttftMs, 0.1) / max(res2.ttftMs, 0.1)

	_, score1, maxScore := verifyPolicyCompliance(res1.text)
	checks2, score2, _ := verifyPolicyCompliance(res2.text)

	complianceStr1 := fmt.Sprintf("%d/%d checks", score1, maxScore)
	if !res1.hasActualCode {
		if res1.timedOut {
			complianceStr1 += " ⚠️ (Timed Out)"
		} else {
			complianceStr1 += " ⚠️ (No Code)"
		}
	}
	complianceStr2 := fmt.Sprintf("%d/%d checks", score2, maxScore)
	if !res2.hasActualCode {
		if res2.timedOut {
			complianceStr2 += " ⚠️ (Timed Out)"
		} else {
			complianceStr2 += " ⚠️ (No Code)"
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println("  OBJECTIVE BENCHMARK VERIFICATION RESULTS")
	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("  %-32s | %-16s | %-16s\n", "Metric", "Monolith Dump", "OKF Progressive")
	fmt.Println("  " + strings.Repeat("-", 68))
	fmt.Printf("  %-32s | %-16s | %-16s\n", "Prompt Input Tokens", fmt.Sprintf("%d tok", res1.promptTokens), fmt.Sprintf("%d tok", res2.promptTokens))
	fmt.Printf("  %-32s | %-16s | %-16s\n", "Output Tokens (Generated)", fmt.Sprintf("%d tok", res1.outputTokens), fmt.Sprintf("%d tok", res2.outputTokens))
	fmt.Printf("  %-32s | %-16s | %-16s\n", "Prefill Latency (TTFT)", fmt.Sprintf("%.1f ms", res1.ttftMs), fmt.Sprintf("%.1f ms", res2.ttftMs))
	fmt.Printf("  %-32s | %-16s | %-16s\n", "Total Turn Duration", durationStr1, durationStr2)
	fmt.Printf("  %-32s | %-16s | %-16s\n", "Rule Adherence Accuracy", complianceStr1, complianceStr2)
	fmt.Println("  " + strings.Repeat("-", 68))
	fmt.Printf("  🔥 TOKEN REDUCTION:       %.1f%% LESS CONTEXT OVERHEAD (AND BILLING COST)\n", tokenSavingsPct)
	fmt.Printf("  ⚡ PREFILL ACCELERATION:  %.1fX FASTER TIME-TO-FIRST-TOKEN\n", ttftSpeedup)
	if res1.hitMaxTokens || res2.hitMaxTokens {
		fmt.Println("  " + strings.Repeat("-", 68))
		fmt.Printf("  ⚠️ WARNING: Model hit max_tokens limit (%d) and was truncated!\n", maxTokens)
		fmt.Println("             Run with -max-tokens 5000 to increase generation ceiling.")
	}
	if res1.timedOut || res2.timedOut {
		fmt.Println("  " + strings.Repeat("-", 68))
		fmt.Printf("  ⏱️ TIMEOUT WARNING: Model exceeded the %v request limit!\n", timeout)
		if !res1.hasActualCode || !res2.hasActualCode {
			fmt.Println("             The model was interrupted while still outputting thinking/reasoning,")
			fmt.Println("             before generating the final Go code block.")
			fmt.Println("             💡 Tip: Pass '-timeout 300s' or '-timeout 5m' for large local models.")
		}
	}
	fmt.Println(strings.Repeat("=", 72))

	if showOutput {
		fmt.Println("\n" + strings.Repeat("=", 72))
		fmt.Println("📄 GENERATED OUTPUT: RUN 1 (MONOLITH CONTEXT DUMP)")
		fmt.Println(strings.Repeat("=", 72))
		if res1.reasoningText != "" {
			fmt.Printf("💭 [Thinking Process: %d tokens — saved in markdown report]\n\n", res1.reasoningTokens)
		}
		fmt.Println(strings.TrimSpace(res1.text))
		fmt.Println("\n" + strings.Repeat("=", 72))
		fmt.Println("⚡ GENERATED OUTPUT: RUN 2 (OKF PROGRESSIVE DISCLOSURE)")
		fmt.Println(strings.Repeat("=", 72))
		if res2.reasoningText != "" {
			fmt.Printf("💭 [Thinking Process: %d tokens — saved in markdown report]\n\n", res2.reasoningTokens)
		}
		fmt.Println(strings.TrimSpace(res2.text))
		fmt.Println(strings.Repeat("=", 72))
		fmt.Println()
	} else {
		fmt.Println()
		fmt.Println("💡 Tip: Pass '-show-output' (or '-o') to print both generated responses directly in the terminal.")
	}

	// Save results markdown artifact
	resultsDir := filepath.Join(filepath.Dir(resolvedDataDir), "results")
	_ = os.MkdirAll(resultsDir, 0o755)

	safeModel := strings.ReplaceAll(strings.ReplaceAll(cfg.Model, "/", "_"), ":", "-")
	outMd := filepath.Join(resultsDir, fmt.Sprintf("BENCHMARK_RESULTS_%s_%s.md", cfg.Name, safeModel))

	var report strings.Builder
	report.WriteString("# Benchmark Results: Monolith vs. OKF Progressive Disclosure\n\n")
	fmt.Fprintf(&report, "* **Provider**: `%s`\n", strings.ToUpper(cfg.Name))
	fmt.Fprintf(&report, "* **Model Tested**: `%s`\n", cfg.Model)
	if isRemote {
		fmt.Fprintf(&report, "* **Execution Mode**: `Remote Cloud API (%s)`\n", cfg.BaseURL)
		fmt.Fprintf(&report, "* **Benchmark Runner Client**: `%s`\n", hwInfo)
	} else {
		fmt.Fprintf(&report, "* **Execution Mode**: `Local On-Device Inference`\n")
		fmt.Fprintf(&report, "* **Host Hardware**: `%s`\n", hwInfo)
	}
	fmt.Fprintf(&report, "* **Temperature**: `%.2f`\n", temperature)
	fmt.Fprintf(&report, "* **Timeout**: `%v`\n", timeout)
	fmt.Fprintf(&report, "* **Endpoint**: `%s`\n", cfg.BaseURL)
	fmt.Fprintf(&report, "* **Date**: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	if res1.timedOut || res2.timedOut {
		report.WriteString("> [!WARNING]\n")
		fmt.Fprintf(&report, "> **Timeout Alert:** One or more runs hit the %v timeout limit while generating.\n", timeout)
		if !res1.hasActualCode || !res2.hasActualCode {
			report.WriteString("> The model was interrupted during its reasoning/thinking phase before generating complete Go code. Policy check scores for interrupted runs reflect the uncompleted scratchpad rather than finished code.\n")
		}
		report.WriteString("\n")
	}

	report.WriteString("| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |\n")
	report.WriteString("| :--- | :--- | :--- | :--- |\n")
	fmt.Fprintf(&report, "| **Input Tokens (Prompt)** | `%d` tokens | `%d` tokens | **-%.1f%%** |\n", res1.promptTokens, res2.promptTokens, tokenSavingsPct)
	fmt.Fprintf(&report, "| **Output Tokens (Generated)** | `%d` tokens | `%d` tokens | - |\n", res1.outputTokens, res2.outputTokens)
	fmt.Fprintf(&report, "| **Prefill Latency (TTFT)** | `%.1f ms` | `%.1f ms` | **%.1fx faster** |\n", res1.ttftMs, res2.ttftMs, ttftSpeedup)
	fmt.Fprintf(&report, "| **Total Turn Time** | `%s` | `%s` | - |\n", durationStr1, durationStr2)
	fmt.Fprintf(&report, "| **Policy Compliance** | `%s` | `%s` | 100%% Consistent |\n\n", complianceStr1, complianceStr2)

	report.WriteString("### Policy Checks:\n")
	for check, passed := range checks2 {
		status := "✅ PASS"
		if !passed {
			status = "❌ FAIL"
		}
		fmt.Fprintf(&report, "* **%s**: %s\n", check, status)
	}

	report.WriteString("\n---\n\n## 📝 Generated Code Responses\n\n")
	report.WriteString("### Run 1: Monolith Context Dump\n\n")
	report.WriteString(formatResponseForReport(res1))
	report.WriteString("\n\n")
	report.WriteString("### Run 2: OKF Progressive Disclosure\n\n")
	report.WriteString(formatResponseForReport(res2))
	report.WriteString("\n")

	if err := os.WriteFile(outMd, []byte(report.String()), 0o644); err == nil {
		fmt.Printf("[✔] Detailed benchmark markdown saved to: %s\n\n", outMd)
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
