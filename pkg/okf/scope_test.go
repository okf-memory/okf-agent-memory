package okf_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func TestScope_ResolveURI(t *testing.T) {
	// 1. Scoped bundle with okf://@: okf://@peter/django-5-rules/decisions/auth
	uri := "okf://@peter/django-5-rules/decisions/auth"
	scope, bundleID, conceptID, err := okf.ParseURI(uri)
	if err != nil {
		t.Fatalf("ParseURI failed: %v", err)
	}
	if scope != okf.ScopeVendor {
		t.Errorf("expected ScopeVendor, got %s", scope)
	}
	if bundleID != "peter/django-5-rules" {
		t.Errorf("expected peter/django-5-rules, got %s", bundleID)
	}
	if conceptID != "decisions/auth" {
		t.Errorf("expected decisions/auth, got %s", conceptID)
	}

	// 2. Direct @-reference: @peter/django-5-rules/decisions/auth
	scopeBare, bundleIDBare, conceptIDBare, err := okf.ParseURI("@peter/django-5-rules/decisions/auth")
	if err != nil {
		t.Fatalf("ParseURI bare @ failed: %v", err)
	}
	if scopeBare != okf.ScopeVendor || bundleIDBare != "peter/django-5-rules" || conceptIDBare != "decisions/auth" {
		t.Errorf("unexpected bare @ parse result: %s, %s, %s", scopeBare, bundleIDBare, conceptIDBare)
	}

	// 3. Top-level vendor bundle with okf://@: okf://@nextjs-15/decisions/routing
	uriTop := "okf://@nextjs-15/decisions/routing"
	scopeTop, bundleIDTop, conceptIDTop, err := okf.ParseURI(uriTop)
	if err != nil {
		t.Fatalf("ParseURI top-level failed: %v", err)
	}
	if scopeTop != okf.ScopeVendor {
		t.Errorf("expected ScopeVendor, got %s", scopeTop)
	}
	if bundleIDTop != "nextjs-15" {
		t.Errorf("expected nextjs-15, got %s", bundleIDTop)
	}
	if conceptIDTop != "decisions/routing" {
		t.Errorf("expected decisions/routing, got %s", conceptIDTop)
	}

	// 4. Direct @-reference top-level: @nextjs-15/decisions/routing
	scopeTopBare, bundleIDTopBare, conceptIDTopBare, err := okf.ParseURI("@nextjs-15/decisions/routing")
	if err != nil {
		t.Fatalf("ParseURI bare @ top failed: %v", err)
	}
	if scopeTopBare != okf.ScopeVendor || bundleIDTopBare != "nextjs-15" || conceptIDTopBare != "decisions/routing" {
		t.Errorf("unexpected bare @ top parse result: %s, %s, %s", scopeTopBare, bundleIDTopBare, conceptIDTopBare)
	}

	// 5. Unscoped target without @ (e.g. nextjs-15/decisions/routing) must reject ParseURI
	// so caller treats it as a local bundle concept!
	if _, _, _, err := okf.ParseURI("nextjs-15/decisions/routing"); err == nil {
		t.Errorf("expected error for unscoped target without @, but succeeded")
	}

	// 6. User scope: okf://user/preferences
	uriUser := "okf://user/preferences"
	scopeUser, _, conceptIDUser, err := okf.ParseURI(uriUser)
	if err != nil {
		t.Fatalf("ParseURI user failed: %v", err)
	}
	if scopeUser != okf.ScopeUser || conceptIDUser != "preferences" {
		t.Errorf("unexpected user scope parse result")
	}

	// 7. Markdown link normalization: @org/repo/path.md -> okf://@org/repo/path
	shorthand := "@peter/django-5-rules/decisions/auth.md"
	normURI := okf.NormalizeVendorLink(shorthand)
	if normURI != "okf://@peter/django-5-rules/decisions/auth" {
		t.Errorf("expected normalized URI, got %s", normURI)
	}

	shorthandTop := "@nextjs-15/decisions/routing.md"
	normURITop := okf.NormalizeVendorLink(shorthandTop)
	if normURITop != "okf://@nextjs-15/decisions/routing" {
		t.Errorf("expected normalized top URI, got %s", normURITop)
	}

	// 8. User and System shorthand: user:preferences/style.md -> okf://user/preferences/style
	normUser := okf.NormalizeVendorLink("user:preferences/style.md")
	if normUser != "okf://user/preferences/style" {
		t.Errorf("expected normalized user URI, got %s", normUser)
	}
	sUser, _, cUser, err := okf.ParseURI("user:preferences/style")
	if err != nil || sUser != okf.ScopeUser || cUser != "preferences/style" {
		t.Errorf("unexpected parse result for user: shorthand: %s, %s, %v", sUser, cUser, err)
	}

	normSys := okf.NormalizeVendorLink("system:compliance/soc2.md")
	if normSys != "okf://system/compliance/soc2" {
		t.Errorf("expected normalized system URI, got %s", normSys)
	}
	sSys, _, cSys, err := okf.ParseURI("system:compliance/soc2")
	if err != nil || sSys != okf.ScopeSystem || cSys != "compliance/soc2" {
		t.Errorf("unexpected parse result for system: shorthand: %s, %s, %v", sSys, cSys, err)
	}
}

func TestIsExternalLink(t *testing.T) {
	cases := []struct {
		href     string
		expected bool
	}{
		{"decisions/routing.md", false},
		{"../architecture/database.md", false},
		{"index.md", false},
		{"@nextjs-15/decisions/routing.md", true},
		{"@nextjs-15/decisions/routing", true},
		{"@peter/django-rules/auth.md", true},
		{"user:preferences/style.md", true},
		{"system:corp/policies.md", true},
		{"okf://user/preferences", true},
		{"okf://system/policy", true},
		{"okf://@nextjs-15/routing", true},
		{"https://example.com/docs", true},
		{"http://localhost:8080", true},
		{"mailto:admin@example.com", true},
		{"", false},
	}

	for _, tc := range cases {
		actual := okf.IsExternalLink(tc.href)
		if actual != tc.expected {
			t.Errorf("IsExternalLink(%q) = %v; want %v", tc.href, actual, tc.expected)
		}
	}
}

func TestValidate_ExternalLinksNeverFail(t *testing.T) {
	tmpDir := t.TempDir()

	indexContent := `---
okf_version: "0.2"
title: "Test Bundle"
---
# Test Bundle
- [Architecture](architecture/index.md) - Architecture docs
`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte(indexContent), 0o600); err != nil {
		t.Fatal(err)
	}

	archDir := filepath.Join(tmpDir, "architecture")
	if err := os.MkdirAll(archDir, 0o750); err != nil {
		t.Fatal(err)
	}

	archIndex := `# Architecture
- [Core](core.md) - Core architecture
`
	if err := os.WriteFile(filepath.Join(archDir, "index.md"), []byte(archIndex), 0o600); err != nil {
		t.Fatal(err)
	}

	coreContent := `---
type: Architecture
title: Core Architecture
description: Core architecture
status: stable
---
# Core Architecture

We build upon vendor standards and external rules:
- Vendor package: [@nextjs-15/decisions/routing](@nextjs-15/decisions/routing.md)
- Scoped vendor package: [@peter/django-rules/auth](@peter/django-rules/auth.md)
- User settings: [User Style](user:preferences/style.md)
- System compliance: [SOC2 Policy](system:compliance/soc2.md)
- Canonical URI: [Remote Policy](okf://system/compliance/soc2.md)
- Web reference: [Next.js Docs](https://nextjs.org/docs)
`
	if err := os.WriteFile(filepath.Join(archDir, "core.md"), []byte(coreContent), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := okf.LoadBundle(tmpDir)
	if err != nil {
		t.Fatalf("LoadBundle failed: %v", err)
	}

	res := okf.Validate(b, okf.ValidateOptions{
		Strict: true,
		Drift:  true,
	})

	if len(res.Errors) > 0 {
		t.Errorf("expected 0 errors, got: %v", res.Errors)
	}
	if len(b.BrokenLinks) > 0 {
		t.Errorf("expected 0 broken links for external references, got: %v", b.BrokenLinks)
	}
	if !res.GatePassed {
		t.Errorf("expected GatePassed=true under Strict mode with external links, but gate failed")
	}
	if !res.IsConformant {
		t.Errorf("expected IsConformant=true, but failed")
	}
}

func TestSearchLayered_VendorCompositeIDF(t *testing.T) {
	tmpDir := t.TempDir()
	vendorRoot := filepath.Join(tmpDir, ".okf", "vendor")

	writeConcept := func(bundle, rel, cType, title, desc, body string) {
		t.Helper()
		full := filepath.Join(vendorRoot, bundle, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		content := "---\ntype: " + cType + "\ntitle: " + title + "\ndescription: " + desc + "\n---\n\n" + body + "\n"
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, b := range []string{"nextjs-15", "django-5"} {
		dir := filepath.Join(vendorRoot, b)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		idx := "---\nokf_version: \"0.2\"\n---\n\n# " + b + "\n"
		if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(idx), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Next.js package: every concept is about Next.js
	writeConcept("nextjs-15", "decisions/routing.md", "decision", "Next.js routing",
		"Use the Next.js App Router for all routing.",
		"Next.js routing is file-based: each folder under app/ is a route segment. Use the App Router; do not mix it with the Pages Router.")
	writeConcept("nextjs-15", "decisions/data-fetching.md", "decision", "Data fetching in Next.js",
		"Fetch data in Next.js server components.",
		"In Next.js, fetch data in server components and cache it with revalidate.")
	writeConcept("nextjs-15", "decisions/images.md", "decision", "Images in Next.js",
		"Use next/image in Next.js.",
		"Next.js optimizes images via the Image component.")
	writeConcept("nextjs-15", "decisions/styling.md", "decision", "Styling in Next.js",
		"CSS modules in Next.js.",
		"Next.js supports CSS modules out of the box.")
	writeConcept("nextjs-15", "decisions/deploy.md", "decision", "Deploying Next.js",
		"Deploy Next.js on Vercel or a Node server.",
		"A Next.js app can be deployed on a standalone Node server.")

	// Django package: one concept mentions Next.js in passing
	writeConcept("django-5", "decisions/urls.md", "decision", "URL configuration",
		"Django URL patterns.",
		"Declare URL patterns in urls.py with path() and include(). Keep routing in Django for server-rendered pages.")
	writeConcept("django-5", "decisions/frontend.md", "decision", "Frontend integration",
		"Pairing Django with a JS frontend.",
		"If the frontend is Next.js, expose a REST API from Django and leave page routing to the frontend.")
	writeConcept("django-5", "decisions/orm.md", "decision", "ORM usage",
		"Use the Django ORM.",
		"Prefer select_related to avoid N+1 queries.")
	writeConcept("django-5", "decisions/auth.md", "decision", "Authentication",
		"Django auth.",
		"Use django-allauth.")
	writeConcept("django-5", "decisions/admin.md", "decision", "Admin",
		"Django admin.",
		"Restrict the admin to staff.")

	// Query: "next.js server"
	// With composite vendor IDF, the Next.js concepts containing both "next.js" and "server"
	// (@nextjs-15/decisions/data-fetching and @nextjs-15/decisions/deploy) must rank above
	// @django-5/decisions/frontend (which mentions Next.js in passing and does not contain "server").
	resServer, err := okf.SearchLayered(okf.LayeredSearchOptions{
		BundleDir:  filepath.Join(tmpDir, "none"),
		VendorRoot: vendorRoot,
		UserDir:    filepath.Join(tmpDir, "none"),
		SystemDir:  filepath.Join(tmpDir, "none"),
		Scope:      "vendor",
		SearchOpts: okf.SearchOptions{
			Query: "next.js server",
			Limit: 10,
		},
	})
	if err != nil {
		t.Fatalf("SearchLayered failed: %v", err)
	}
	if len(resServer) < 3 {
		t.Fatalf("expected at least 3 results for 'next.js server', got %d", len(resServer))
	}

	topTwo := map[string]bool{
		resServer[0].ConceptID: true,
		resServer[1].ConceptID: true,
	}
	if !topTwo["@nextjs-15/decisions/data-fetching"] || !topTwo["@nextjs-15/decisions/deploy"] {
		t.Errorf("expected @nextjs-15/decisions/data-fetching and @nextjs-15/decisions/deploy as top 2 results, got: %s (%.2f), %s (%.2f), %s (%.2f)",
			resServer[0].ConceptID, resServer[0].Score,
			resServer[1].ConceptID, resServer[1].Score,
			resServer[2].ConceptID, resServer[2].Score)
	}

	// Query: "next.js"
	// A concept whose title, description, and body are about Next.js must score higher than
	// a concept in another package that only mentions Next.js once in the body.
	resNext, err := okf.SearchLayered(okf.LayeredSearchOptions{
		BundleDir:  filepath.Join(tmpDir, "none"),
		VendorRoot: vendorRoot,
		UserDir:    filepath.Join(tmpDir, "none"),
		SystemDir:  filepath.Join(tmpDir, "none"),
		Scope:      "vendor",
		SearchOpts: okf.SearchOptions{
			Query: "next.js",
			Limit: 10,
		},
	})
	if err != nil {
		t.Fatalf("SearchLayered ('next.js') failed: %v", err)
	}
	if len(resNext) < 6 {
		t.Fatalf("expected 6 results for 'next.js', got %d", len(resNext))
	}
	if resNext[len(resNext)-1].ConceptID != "@django-5/decisions/frontend" {
		t.Errorf("expected @django-5/decisions/frontend (passing mention) to rank below dedicated @nextjs-15 concepts, got last=%s",
			resNext[len(resNext)-1].ConceptID)
	}
}

func TestIndexFrontmatterScopes_ValidationAndEnforcement(t *testing.T) {
	// 1. Reject empty or invalid scopes in root index.md
	invalidCases := []struct {
		name      string
		indexYAML string
		wantErr   string
	}{
		{
			name:      "empty flow list",
			indexYAML: "---\nokf_version: \"0.2\"\nscopes: []\n---\n# Index\n",
			wantErr:   "scopes list cannot be empty",
		},
		{
			name:      "empty scalar",
			indexYAML: "---\nokf_version: \"0.2\"\nscopes:\n---\n# Index\n",
			wantErr:   "scopes list cannot be empty",
		},
		{
			name:      "unknown scope value",
			indexYAML: "---\nokf_version: \"0.2\"\nscopes: [project, foo]\n---\n# Index\n",
			wantErr:   "invalid scope \"foo\" in index.md",
		},
		{
			name:      "disallowed 'all' in index.md scopes",
			indexYAML: "---\nokf_version: \"0.2\"\nscopes: [all]\n---\n# Index\n",
			wantErr:   "invalid scope \"all\" in index.md",
		},
		{
			name:      "removed 'bundle' alias in index.md scopes",
			indexYAML: "---\nokf_version: \"0.2\"\nscopes: [bundle]\n---\n# Index\n",
			wantErr:   "invalid scope \"bundle\" in index.md",
		},
		{
			name:      "empty item in flow list",
			indexYAML: "---\nokf_version: \"0.2\"\nscopes: [project, ]\n---\n# Index\n",
			wantErr:   "empty item in scopes list",
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(tc.indexYAML), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := okf.LoadBundle(dir)
			if err == nil {
				t.Fatalf("expected LoadBundle to fail for %s, but succeeded", tc.name)
			}
			if !containsSubstr(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}

	// 2. Enforce scopes restriction in SearchLayered and ResolveScopedConcept
	root := t.TempDir()
	projDir := filepath.Join(root, "project")
	vendorRoot := filepath.Join(root, "vendor")
	userDir := filepath.Join(root, "user")
	sysDir := filepath.Join(root, "system")

	for _, d := range []string{projDir, filepath.Join(vendorRoot, "acme"), userDir, sysDir} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "index.md"), []byte("---\nokf_version: \"0.2\"\n---\n# Index\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Restrict project bundle to [project, vendor] only (excluding user and system)
	restrictedIndex := "---\nokf_version: \"0.2\"\nscopes:\n  - project\n  - vendor\n---\n# Restricted Project\n"
	if err := os.WriteFile(filepath.Join(projDir, "index.md"), []byte(restrictedIndex), 0o600); err != nil {
		t.Fatal(err)
	}

	writeNote := func(dir, name, title string) {
		content := "---\ntype: Fact\ntitle: " + title + "\ndescription: Shared keyword alpha\n---\n# " + title + "\nShared keyword alpha\n"
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	writeNote(projDir, "proj-note", "Project Note")
	writeNote(filepath.Join(vendorRoot, "acme"), "vendor-note", "Vendor Note")
	writeNote(userDir, "user-note", "Secret Personal Note")
	writeNote(sysDir, "sys-note", "System Note")

	// A. Search with scope="all" must only return project and vendor results (user & system excluded)
	results, err := okf.SearchLayered(okf.LayeredSearchOptions{
		BundleDir:  projDir,
		VendorRoot: vendorRoot,
		UserDir:    userDir,
		SystemDir:  sysDir,
		Scope:      "all",
		SearchOpts: okf.SearchOptions{Query: "alpha", Limit: 10},
	})
	if err != nil {
		t.Fatalf("SearchLayered(all) failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (project + vendor), got %d: %+v", len(results), results)
	}
	for _, r := range results {
		if r.Scope == okf.ScopeUser || r.Scope == okf.ScopeSystem {
			t.Errorf("unexpected result from disabled scope %s: %+v", r.Scope, r)
		}
	}

	// B. Explicit search on disabled scope ("user" or "system") must return an error
	for _, blocked := range []string{"user", "system"} {
		_, err := okf.SearchLayered(okf.LayeredSearchOptions{
			BundleDir:  projDir,
			VendorRoot: vendorRoot,
			UserDir:    userDir,
			SystemDir:  sysDir,
			Scope:      blocked,
			SearchOpts: okf.SearchOptions{Query: "alpha", Limit: 10},
		})
		if err == nil || !containsSubstr(err.Error(), "disabled by bundle configuration") {
			t.Errorf("expected explicit search for scope %q to be blocked, got: %v", blocked, err)
		}
	}

	// C. ResolveScopedConcept with bare ID in user scope must NOT fall back to user/system when disabled
	if _, err := okf.ResolveScopedConcept("user-note", projDir, vendorRoot, userDir, sysDir); err == nil {
		t.Errorf("expected bare ID 'user-note' to fail when user scope is disabled")
	}

	// D. ResolveScopedConcept with explicit 'user:user-note' or 'okf://user/user-note' must fail with disabled error
	for _, ref := range []string{"user:user-note", "okf://user/user-note", "system:sys-note"} {
		_, err := okf.ResolveScopedConcept(ref, projDir, vendorRoot, userDir, sysDir)
		if err == nil || !containsSubstr(err.Error(), "disabled by bundle configuration") {
			t.Errorf("expected explicit show %q to be blocked by bundle configuration, got: %v", ref, err)
		}
	}

	// E. Allowed scopes (project and @acme/vendor-note) still resolve normally
	if res, err := okf.ResolveScopedConcept("proj-note", projDir, vendorRoot, userDir, sysDir); err != nil || res.Scope != okf.ScopeProject {
		t.Errorf("expected proj-note to resolve in project scope, got res=%+v err=%v", res, err)
	}
	if res, err := okf.ResolveScopedConcept("@acme/vendor-note", projDir, vendorRoot, userDir, sysDir); err != nil || res.Scope != okf.ScopeVendor {
		t.Errorf("expected @acme/vendor-note to resolve in vendor scope, got res=%+v err=%v", res, err)
	}
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
