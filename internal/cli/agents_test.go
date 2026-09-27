package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIAgentsLint(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-agents-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	validContent := `# AGENTS.md - DMAA Protocol v0.1 - Instructions for AI Agents in Test

> Powered by [OKF Agent Memory](https://github.com/okf-memory/okf-agent-memory) -- Open Knowledge Format (OKF) v0.2 persistent project memory for AI agents.

## 0. Project & Domain Codex
- TONE: style == direct_concise, zero_pleasantries
- FORMAT: diagrams => ASSERT(syntax == mermaid, ELSE=STOP("Diagrams must use Mermaid syntax."))
- GOAL: maintain(high_factual_integrity, domain_neutrality)

<!-- BEGIN OKF AGENT MEMORY -->
## 1. Behavioral Invariants & Constraints (RFC 2119)
- MUST execute okf_search(query=keywords, limit=3) before proposing architecture.
- NEVER scan knowledge/ via list_dir, grep_search, find, or raw file readers.
- PREFER native okf_* MCP tools OVER CLI fallback commands.

## 2. Guard Clauses & Scope Governance
- ON edit(@path/):
    IF first_visit(@path/) => okf_search(for_path=@path/)
    IF governance == "hold" => STOP("Subsystem frozen.")

## 3. Completion Pipeline (Sequential Assertion Gates)
1. IF arch_decisions => MUST okf_create(architecture/*, type="decision")
2. ASSERT(okf_validate(strict=true) == 0_err, ELSE=fix_before_exit)
<!-- END OKF AGENT MEMORY -->
`

	agentsPath := filepath.Join(tempDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(validContent), 0o644); err != nil {
		t.Fatalf("failed to write AGENTS.md: %v", err)
	}

	// Test lint execution on valid file
	err = runAgentsLint([]string{agentsPath})
	if err != nil {
		t.Errorf("expected runAgentsLint to succeed, got: %v", err)
	}

	// Test link execution
	err = runAgentsLink([]string{"--root", tempDir})
	if err != nil {
		t.Errorf("expected runAgentsLink to succeed, got: %v", err)
	}

	// Test check execution
	err = runAgentsCheck([]string{"--root", tempDir})
	if err != nil {
		t.Errorf("expected runAgentsCheck to succeed, got: %v", err)
	}
}

func TestCLIAgentsInitDomain(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "okf-agents-init-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Test init with domain=legal
	err = runAgentsInit([]string{"--domain", "legal", "--root", tempDir})
	if err != nil {
		t.Fatalf("expected runAgentsInit --domain=legal to succeed, got: %v", err)
	}

	agentsFile := filepath.Join(tempDir, "AGENTS.md")
	data, err := os.ReadFile(agentsFile)
	if err != nil {
		t.Fatalf("expected AGENTS.md to be created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Legal & Compliance") && !strings.Contains(content, "Regulatory Governance") {
		t.Errorf("expected legal domain codex content in AGENTS.md, got:\n%s", content)
	}
}
