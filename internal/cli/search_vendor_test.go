package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func captureOutput(f func()) string {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = stdout
	}()

	f()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestSearch_VendorLayeringAndShadowing(t *testing.T) {
	workDir := t.TempDir()
	t.Setenv("OKF_USER_DIR", filepath.Join(workDir, "user"))
	t.Setenv("OKF_SYSTEM_DIR", filepath.Join(workDir, "system"))
	origDir, _ := os.Getwd()
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// 1. Create primary bundle in ./knowledge
	projDir := filepath.Join(workDir, "knowledge")
	if err := os.MkdirAll(filepath.Join(projDir, "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "index.md"), []byte("# Project Index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shadowedProj := `---
type: Decision
title: Local Shadowed Decision
governance: constraint
---
Local project decision overrides vendor
`
	if err := os.WriteFile(filepath.Join(projDir, "decisions", "shadowed.md"), []byte(shadowedProj), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Create vendor bundle in .okf/vendor/peter/django-5-rules
	vendorDir := filepath.Join(workDir, ".okf", "vendor", "peter", "django-5-rules")
	if err := os.MkdirAll(filepath.Join(vendorDir, "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "index.md"), []byte("# Vendor Index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shadowedVendor := `---
type: Decision
title: Vendor Shadowed Decision
---
Vendor rule should be shadowed
`
	if err := os.WriteFile(filepath.Join(vendorDir, "decisions", "shadowed.md"), []byte(shadowedVendor), 0o644); err != nil {
		t.Fatal(err)
	}
	vendorAuth := `---
type: Decision
title: Vendor Django Auth
---
Use session authentication in Django 5
`
	if err := os.WriteFile(filepath.Join(vendorDir, "decisions", "auth.md"), []byte(vendorAuth), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Search for "shadowed" -> only project decision returned (vendor shadowed)
	searchCmd, ok := FindCommand("search")
	if !ok {
		t.Fatalf("search command not found")
	}

	out := captureOutput(func() {
		searchCmd.Run([]string{"shadowed", "--json"})
	})

	if !strings.Contains(out, "Local Shadowed Decision") {
		t.Errorf("expected local decision in search output, got: %s", out)
	}
	if strings.Contains(out, "Vendor Shadowed Decision") {
		t.Errorf("expected vendor decision to be shadowed, but was present: %s", out)
	}

	// 4. Search for "django session" -> vendor concept returned as @peter/django-5-rules/decisions/auth
	outVendor := captureOutput(func() {
		searchCmd.Run([]string{"django", "--json"})
	})
	if !strings.Contains(outVendor, "@peter/django-5-rules/decisions/auth") {
		t.Errorf("expected vendor concept @ID in search output, got: %s", outVendor)
	}

	// 5. Test show @peter/django-5-rules/decisions/auth
	showCmd, ok := FindCommand("show")
	if !ok {
		t.Fatalf("show command not found")
	}
	outShow := captureOutput(func() {
		showCmd.Run([]string{"@peter/django-5-rules/decisions/auth"})
	})
	if !strings.Contains(outShow, "Vendor Django Auth") {
		t.Errorf("expected vendor title in show output, got: %s", outShow)
	}

	// 6. Test show @peter/... shorthand with .md
	outShowShort := captureOutput(func() {
		showCmd.Run([]string{"@peter/django-5-rules/decisions/auth.md"})
	})
	if !strings.Contains(outShowShort, "Vendor Django Auth") {
		t.Errorf("expected vendor title with shorthand show, got: %s", outShowShort)
	}

	// 7. Test show okf://@peter/django-5-rules/decisions/auth
	outShowURI := captureOutput(func() {
		showCmd.Run([]string{"okf://@peter/django-5-rules/decisions/auth"})
	})
	if !strings.Contains(outShowURI, "Vendor Django Auth") {
		t.Errorf("expected vendor title with URI show, got: %s", outShowURI)
	}

	// 8. Test top-level vendor bundle @nextjs-15/decisions/routing
	nextjsDir := filepath.Join(workDir, ".okf", "vendor", "nextjs-15")
	if err := os.MkdirAll(filepath.Join(nextjsDir, "decisions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nextjsDir, "index.md"), []byte("# Next.js Index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nextjsRouting := "---\ntype: Decision\ntitle: Next.js App Router\n---\nUse app router"
	if err := os.WriteFile(filepath.Join(nextjsDir, "decisions", "routing.md"), []byte(nextjsRouting), 0o644); err != nil {
		t.Fatal(err)
	}

	outNextjs := captureOutput(func() {
		showCmd.Run([]string{"@nextjs-15/decisions/routing"})
	})
	if !strings.Contains(outNextjs, "Next.js App Router") {
		t.Errorf("expected top-level vendor title with @nextjs-15, got: %s", outNextjs)
	}

	// 9. Verify that 'okf show nextjs-15/decisions/routing' WITHOUT @ does NOT look in vendor,
	// but searches the local 'knowledge' bundle (where it does not exist)
	origExit := exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = origExit }()

	// We capture stderr as well
	rErr, wErr, _ := os.Pipe()
	stderr := os.Stderr
	os.Stderr = wErr
	defer func() { os.Stderr = stderr }()

	showCmd.Run([]string{"nextjs-15/decisions/routing"})
	_ = wErr.Close()
	var errBuf bytes.Buffer
	_, _ = io.Copy(&errBuf, rErr)

	if !strings.Contains(errBuf.String(), "not found in 'knowledge'") {
		t.Errorf("expected target without @ to search in 'knowledge' bundle, got stderr: %s", errBuf.String())
	}
	if !exitCalled {
		t.Errorf("expected exitFunc to be called for missing concept")
	}

	// 10. Verify --scope flag behavior
	// --scope project should NOT return vendor concepts
	outScopeProject := captureOutput(func() {
		searchCmd.Run([]string{"django", "--scope", "project", "--json"})
	})
	if strings.Contains(outScopeProject, "@peter/django-5-rules") {
		t.Errorf("expected --scope project to exclude vendor concepts, got: %s", outScopeProject)
	}

	// --scope vendor should return vendor concepts even when searched specifically
	outScopeVendor := captureOutput(func() {
		searchCmd.Run([]string{"django", "--scope", "vendor", "--json"})
	})
	if !strings.Contains(outScopeVendor, "@peter/django-5-rules/decisions/auth") {
		t.Errorf("expected --scope vendor to include vendor concepts, got: %s", outScopeVendor)
	}

	// --scope vendor for "shadowed" returns vendor concept because local layer is excluded!
	outVendorShadowed := captureOutput(func() {
		searchCmd.Run([]string{"shadowed", "--scope", "vendor", "--json"})
	})
	if !strings.Contains(outVendorShadowed, "Vendor Shadowed Decision") {
		t.Errorf("expected --scope vendor to find vendor shadowed decision, got: %s", outVendorShadowed)
	}
}

func TestSearch_AllScopeCombinationsAndLayering(t *testing.T) {
	tempRoot := t.TempDir()

	// 1. Setup isolated directories
	projDir := filepath.Join(tempRoot, "proj")
	userDir := filepath.Join(tempRoot, "user")
	sysDir := filepath.Join(tempRoot, "sys")

	t.Setenv("OKF_USER_DIR", userDir)
	t.Setenv("OKF_SYSTEM_DIR", sysDir)

	writeConcept := func(dir, relPath, title, body string) {
		full := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf("---\ntype: Decision\ntitle: %s\ndescription: %s description\nstatus: stable\n---\n# %s\n%s\n", title, title, title, body)
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Layer 4: System Layer (/etc/okf) - Priority 10
	writeConcept(sysDir, "index.md", "System Index", "System index")
	writeConcept(sysDir, "decisions/system-only.md", "System Only Concept", "Uniquely exists in system layer")
	writeConcept(sysDir, "decisions/shared.md", "System Shared Concept", "Shared concept across all 4 layers")
	writeConcept(sysDir, "decisions/user-sys.md", "System UserSys Concept", "Common agreement between user and system")

	// Layer 3: User Layer (~/.okf) - Priority 50
	writeConcept(userDir, "index.md", "User Index", "User index")
	writeConcept(userDir, "decisions/user-only.md", "User Only Concept", "Uniquely exists in user layer")
	writeConcept(userDir, "decisions/shared.md", "User Shared Concept", "Shared concept across all 4 layers")
	writeConcept(userDir, "decisions/user-sys.md", "User UserSys Concept", "Common agreement between user and system")

	// Layer 2: Vendor Layer (.okf/vendor/react-19 and .okf/vendor/tools/linter) - Priority 70
	vendorRoot := filepath.Join(projDir, ".okf", "vendor")
	vendorLinter := filepath.Join(vendorRoot, "tools", "linter")
	writeConcept(vendorLinter, "index.md", "Linter Index", "Linter index")
	writeConcept(vendorLinter, "decisions/vendor-only.md", "Vendor Only Concept", "Uniquely exists in vendor layer")
	writeConcept(vendorLinter, "decisions/shared.md", "Vendor Shared Concept", "Shared concept across all 4 layers")

	vendorReact := filepath.Join(vendorRoot, "react-19")
	writeConcept(vendorReact, "index.md", "React Index", "React index")
	writeConcept(vendorReact, "decisions/routing.md", "React Routing Concept", "React 19 router")

	// Layer 1: Project Layer (knowledge/) - Priority 100
	projKnowledge := filepath.Join(projDir, "knowledge")
	writeConcept(projKnowledge, "index.md", "Project Index", "Project index")
	writeConcept(projKnowledge, "decisions/proj-only.md", "Project Only Concept", "Uniquely exists in project layer")
	writeConcept(projKnowledge, "decisions/shared.md", "Project Shared Concept", "Shared concept across all 4 layers")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	searchCmd, ok := FindCommand("search")
	if !ok {
		t.Fatalf("search command not found")
	}
	showCmd, ok := FindCommand("show")
	if !ok {
		t.Fatalf("show command not found")
	}

	runSearchJSON := func(args ...string) []okf.SearchResult {
		fullArgs := append(args, "--json")
		out := captureOutput(func() {
			searchCmd.Run(fullArgs)
		})
		var res []okf.SearchResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("failed to unmarshal search output %q: %v", out, err)
		}
		return res
	}

	// -------------------------------------------------------------------------
	// A. Scope: project
	// -------------------------------------------------------------------------
	res := runSearchJSON("Only", "--scope", "project")
	if len(res) != 1 {
		t.Fatalf("--scope project for 'Only': expected 1 result, got %d", len(res))
	}
	if res[0].ConceptID != "decisions/proj-only" || res[0].Scope != okf.ScopeProject || res[0].Priority != okf.PriorityProject {
		t.Errorf("--scope project: unexpected result: %+v", res[0])
	}

	// -------------------------------------------------------------------------
	// B. Scope: vendor
	// -------------------------------------------------------------------------
	resVendor := runSearchJSON("Only", "--scope", "vendor")
	if len(resVendor) != 1 {
		t.Fatalf("--scope vendor for 'Only': expected 1 result, got %d", len(resVendor))
	}
	if resVendor[0].ConceptID != "@tools/linter/decisions/vendor-only" || resVendor[0].Scope != okf.ScopeVendor || resVendor[0].Priority != okf.PriorityVendor {
		t.Errorf("--scope vendor: unexpected result: %+v", resVendor[0])
	}

	// -------------------------------------------------------------------------
	// C. Scope: user
	// -------------------------------------------------------------------------
	resUser := runSearchJSON("Only", "--scope", "user")
	if len(resUser) != 1 {
		t.Fatalf("--scope user for 'Only': expected 1 result, got %d", len(resUser))
	}
	if resUser[0].ConceptID != "user:decisions/user-only" || resUser[0].Scope != okf.ScopeUser || resUser[0].Priority != okf.PriorityUser {
		t.Errorf("--scope user: unexpected result: %+v", resUser[0])
	}

	// -------------------------------------------------------------------------
	// D. Scope: system
	// -------------------------------------------------------------------------
	resSys := runSearchJSON("Only", "--scope", "system")
	if len(resSys) != 1 {
		t.Fatalf("--scope system for 'Only': expected 1 result, got %d", len(resSys))
	}
	if resSys[0].ConceptID != "system:decisions/system-only" || resSys[0].Scope != okf.ScopeSystem || resSys[0].Priority != okf.PrioritySystem {
		t.Errorf("--scope system: unexpected result: %+v", resSys[0])
	}

	// -------------------------------------------------------------------------
	// E. Scope: all (Default) - Multi-Layer Shadowing
	// -------------------------------------------------------------------------
	// 1. Search 'shared' across all layers -> Project MUST shadow Vendor, User, System
	resShared := runSearchJSON("Shared", "--scope", "all")
	if len(resShared) != 1 {
		t.Fatalf("--scope all for 'Shared': expected exactly 1 result due to shadowing, got %d: %+v", len(resShared), resShared)
	}
	if resShared[0].ConceptID != "decisions/shared" || resShared[0].Scope != okf.ScopeProject || resShared[0].Title != "Project Shared Concept" {
		t.Errorf("expected Project to shadow all lower layers for 'shared', got: %+v", resShared[0])
	}

	// 2. Search 'user-sys' -> User MUST shadow System (Prio 50 > 10)
	resUserSys := runSearchJSON("UserSys", "--scope", "all")
	if len(resUserSys) != 1 {
		t.Fatalf("--scope all for 'UserSys': expected exactly 1 result due to shadowing, got %d: %+v", len(resUserSys), resUserSys)
	}
	if resUserSys[0].ConceptID != "user:decisions/user-sys" || resUserSys[0].Scope != okf.ScopeUser || resUserSys[0].Title != "User UserSys Concept" {
		t.Errorf("expected User to shadow System, got: %+v", resUserSys[0])
	}

	// 3. Multi-Layer Ranking Order: Project (100) > Vendor (70) > User (50) > System (10)
	resAllOnly := runSearchJSON("Only", "--scope", "all")
	if len(resAllOnly) != 4 {
		t.Fatalf("--scope all for 'Only': expected 4 results (1 per layer), got %d: %+v", len(resAllOnly), resAllOnly)
	}
	expectedOrder := []struct {
		scope okf.Scope
		prio  int
		id    string
	}{
		{okf.ScopeProject, okf.PriorityProject, "decisions/proj-only"},
		{okf.ScopeVendor, okf.PriorityVendor, "@tools/linter/decisions/vendor-only"},
		{okf.ScopeUser, okf.PriorityUser, "user:decisions/user-only"},
		{okf.ScopeSystem, okf.PrioritySystem, "system:decisions/system-only"},
	}
	for i, exp := range expectedOrder {
		if resAllOnly[i].Scope != exp.scope || resAllOnly[i].Priority != exp.prio || resAllOnly[i].ConceptID != exp.id {
			t.Errorf("rank %d mismatch: want scope=%s prio=%d id=%s; got scope=%s prio=%d id=%s",
				i, exp.scope, exp.prio, exp.id, resAllOnly[i].Scope, resAllOnly[i].Priority, resAllOnly[i].ConceptID)
		}
	}

	// -------------------------------------------------------------------------
	// F. Resolution in okf show across all scopes
	// -------------------------------------------------------------------------
	showCases := []struct {
		target   string
		expected string
	}{
		{"decisions/proj-only", "Project Only Concept"},
		{"@tools/linter/decisions/vendor-only", "Vendor Only Concept"},
		{"@react-19/decisions/routing", "React Routing Concept"},
		{"user:decisions/user-only", "User Only Concept"},
		{"okf://user/decisions/user-only", "User Only Concept"},
		{"system:decisions/system-only", "System Only Concept"},
		{"okf://system/decisions/system-only", "System Only Concept"},
	}
	for _, tc := range showCases {
		out := captureOutput(func() {
			showCmd.Run([]string{tc.target})
		})
		if !strings.Contains(out, tc.expected) {
			t.Errorf("okf show %s: expected to contain %q, got output:\n%s", tc.target, tc.expected, out)
		}
	}

	// -------------------------------------------------------------------------
	// G. Negative & Guard clause testing
	// -------------------------------------------------------------------------
	origExit := exitFunc
	exitCalled := false
	exitCode := 0
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}
	defer func() { exitFunc = origExit }()

	rErr, wErr, _ := os.Pipe()
	stderr := os.Stderr
	os.Stderr = wErr
	defer func() { os.Stderr = stderr }()

	// Invalid scope
	searchCmd.Run([]string{"foo", "--scope", "invalid-layer"})
	_ = wErr.Close()
	var errBuf bytes.Buffer
	_, _ = io.Copy(&errBuf, rErr)

	if !exitCalled || exitCode != 1 {
		t.Errorf("expected exit 1 on invalid --scope, got called=%v code=%d", exitCalled, exitCode)
	}
	if !strings.Contains(errBuf.String(), "invalid --scope 'invalid-layer'") {
		t.Errorf("expected invalid --scope error message, got: %s", errBuf.String())
	}
}
