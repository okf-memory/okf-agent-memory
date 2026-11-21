package okf

import (
	"fmt"
	"strings"
)

// DomainCodexTemplate defines a pre-packaged domain-specific codex.
type DomainCodexTemplate struct {
	Name        string
	Title       string
	Description string
	CodexLines  []string
}

// GetAvailableDomains returns all built-in domain codex configurations.
func GetAvailableDomains() map[string]DomainCodexTemplate {
	return map[string]DomainCodexTemplate{
		"software": {
			Name:        "software",
			Title:       "Software Engineering Reference",
			Description: "Clean Architecture, TDD, Strict Typing, and CI/CD Assertion Gates",
			CodexLines: []string{
				"- TONE: style == direct_concise, zero_pleasantries",
				"- CODE: enforce(clean_architecture, strict_typing, tdd, zero_untested_code)",
				"- FORMAT: diagrams => ASSERT(syntax == mermaid, ELSE=STOP(\"All diagrams MUST use Mermaid syntax.\"))",
				"- SECURITY: crypto == ed25519, jwt == stateless, secrets => NEVER commit_to_git",
			},
		},
		"research": {
			Name:        "research",
			Title:       "Scientific & Academic Research",
			Description: "Hypothesis Falsification, Peer Review Gates, and Citation Rigor",
			CodexLines: []string{
				"- TONE: style == rigorous_academic, evidence_based, objective",
				"- INTEGRITY: verify(doi_references, data_provenance, reproducibility)",
				"- FORMAT: diagrams => ASSERT(syntax == mermaid, ELSE=STOP(\"All diagrams MUST use Mermaid syntax.\"))",
				"- ETHICS: NEVER fabricate citations, synthetic experimental values, or p-values",
			},
		},
		"legal": {
			Name:        "legal",
			Title:       "Legal & Compliance Regulatory Governance",
			Description: "GDPR/HIPAA Compliance, PII Anonymization, and Precedent Adherence",
			CodexLines: []string{
				"- TONE: style == authoritative_analytical, risk_calibrated, formal",
				"- PRIVACY: enforce(gdpr_article_17, hipaa_safe_harbor, pii_anonymization)",
				"- FORMAT: diagrams => ASSERT(syntax == mermaid, ELSE=STOP(\"All diagrams MUST use Mermaid syntax.\"))",
				"- GOVERNANCE: jurisdiction == EU_US, signed_counsel => REQUIRED for indemnity",
			},
		},
		"coaching": {
			Name:        "coaching",
			Title:       "Executive & Life Coaching Reference",
			Description: "Socratic Inquiry, ICF Ethics, Confidentiality, and Growth Tracking",
			CodexLines: []string{
				"- TONE: style == empathetic_reflective, solution_focused, non_prescriptive",
				"- ETHICS: adhere(icf_code_of_ethics, strict_client_confidentiality)",
				"- FORMAT: diagrams => ASSERT(syntax == mermaid, ELSE=STOP(\"All diagrams MUST use Mermaid syntax.\"))",
				"- METHOD: ask(powerful_open_questions), mirror(client_language), avoid(premature_advice)",
			},
		},
		"books": {
			Name:        "books",
			Title:       "Literature & Creative Lore Bible",
			Description: "Narrative Voice, Canon Continuity, Character Lore, and Timeline Integrity",
			CodexLines: []string{
				"- TONE: style == literary_analytical, objective_scholarly, nuanced",
				"- INTEGRITY: maintain(character_consistency, canon_fidelity, timeline_accuracy)",
				"- FORMAT: diagrams => ASSERT(syntax == mermaid, ELSE=STOP(\"All diagrams MUST use Mermaid syntax.\"))",
				"- SPOILERS: explicit_warning => MUST flag(major_plot_twists, ending_revelations)",
			},
		},
	}
}

// GenerateAgentsMarkdown renders an AGENTS.md document for a given domain.
func GenerateAgentsMarkdown(projectName, domainName string) (string, error) {
	domains := GetAvailableDomains()
	domain, ok := domains[strings.ToLower(domainName)]
	if !ok {
		domain = domains["software"]
	}

	projectName = strings.ReplaceAll(projectName, "\n", "")
	projectName = strings.ReplaceAll(projectName, "\r", "")
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		projectName = domain.Title
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# AGENTS.md - DMAA Protocol v0.1 - Instructions for AI Agents in `%s`\n\n", projectName)
	sb.WriteString("> Powered by [OKF Agent Memory](https://github.com/okf-memory/okf-agent-memory) -- Open Knowledge Format (OKF) v0.3 persistent project memory for AI agents.\n\n")
	fmt.Fprintf(&sb, "## 0. Project & Domain Codex (%s)\n", domain.Title)
	for _, line := range domain.CodexLines {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("\n<!-- BEGIN OKF AGENT MEMORY -->\n")
	sb.WriteString("## 1. Behavioral Invariants & Constraints (RFC 2119)\n")
	sb.WriteString("- MUST execute `okf_search(query=keywords, limit=3)` before proposing architecture, new dependencies, or substantial changes.\n")
	sb.WriteString("- NEVER scan `knowledge/` via `list_dir`, `grep_search`, `find`, or raw file readers.\n")
	sb.WriteString("- NEVER forge human verification (`verified:` is human-only; declare `generated: { by: \"<actor>\", at: \"<iso-time>\" }`).\n")
	sb.WriteString("- PREFER native `okf_*` MCP tools OVER CLI fallback commands.\n\n")
	sb.WriteString("## 2. Guard Clauses & Scope Governance\n")
	sb.WriteString("- ON edit(@path/):\n")
	sb.WriteString("    IF first_visit(@path/) => okf_search(for_path=@path/)\n")
	sb.WriteString("    IF governance == \"hold\" => STOP(\"Subsystem frozen by governance. Request explicit human confirmation.\")\n")
	sb.WriteString("    IF governance == \"constraint\" => MUST adhere to all listed invariants\n")
	sb.WriteString("    IF governance == \"context\" => proceed with awareness\n")
	sb.WriteString("- ON user_query(architecture | requirements | conventions | domain_facts):\n")
	sb.WriteString("    okf_search(query=keywords, limit=3) => evaluate summary description\n")
	sb.WriteString("    IF relevant => okf_show(concept_id) ONLY on demand\n")
	sb.WriteString("    IF updating_existing_concept => PREFER okf_update OVER okf_create\n\n")
	sb.WriteString("## 3. Completion Pipeline (Sequential Assertion Gates)\n")
	sb.WriteString("1. IF decisions_made => MUST okf_create(category/*, type=\"decision\", title=..., desc=...)\n")
	sb.WriteString("2. IF requirements_discovered => MUST okf_update(concept_id)\n")
	sb.WriteString("3. IF concepts_mutated => MUST sync(knowledge/log.md, knowledge/index.md)\n")
	sb.WriteString("4. ASSERT(okf_validate(strict=true, drift=true) == {errors: 0, warnings: 0}, ELSE=fix_before_exit)\n")
	sb.WriteString("<!-- END OKF AGENT MEMORY -->\n")

	return sb.String(), nil
}
