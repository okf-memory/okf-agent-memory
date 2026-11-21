package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/okf-memory/okf-agent-memory/pkg/okf"
)

func captureStdoutStderr(t *testing.T, fn func()) (string, string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	defer func() {
		os.Stdout, os.Stderr = origOut, origErr
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()
	var outBuf, errBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, rOut)
	_, _ = io.Copy(&errBuf, rErr)
	_ = rOut.Close()
	_ = rErr.Close()
	return outBuf.String(), errBuf.String()
}

func TestCLIStaleAfterCreateUpdateAndClear(t *testing.T) {
	bundle := mutationTestBundle(t)

	// 1. Create with ISO date
	cmdCreate([]string{"decisions/adr-012", bundle, "--type", "Decision", "--title", "ADR 012", "--desc", "Use Postgres.", "--stale-after", "2027-03-01"})
	b, err := okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if b.Concepts["decisions/adr-012"].StaleAfter != "2027-03-01" {
		t.Fatalf("create stale_after = %q, want 2027-03-01", b.Concepts["decisions/adr-012"].StaleAfter)
	}

	// 2. Update with relative duration 90d
	expected90d := time.Now().UTC().Add(90 * 24 * time.Hour).Format("2006-01-02")
	cmdUpdate([]string{"decisions/adr-012", bundle, "--stale-after", "90d"})
	b, err = okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if b.Concepts["decisions/adr-012"].StaleAfter != expected90d {
		t.Fatalf("update 90d stale_after = %q, want %q", b.Concepts["decisions/adr-012"].StaleAfter, expected90d)
	}

	// 3. Update omitting --stale-after keeps existing value
	cmdUpdate([]string{"decisions/adr-012", bundle, "--desc", "Use Postgres 16."})
	b, err = okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if b.Concepts["decisions/adr-012"].StaleAfter != expected90d {
		t.Fatalf("omitted stale_after altered value: got %q, want %q", b.Concepts["decisions/adr-012"].StaleAfter, expected90d)
	}

	// 4. Update with past date succeeds and emits warning on stderr (and in JSON)
	stdout, stderr := captureStdoutStderr(t, func() {
		cmdUpdate([]string{"decisions/adr-012", bundle, "--stale-after", "2020-01-01", "--json"})
	})
	if !strings.Contains(stderr, "Warning:") || !strings.Contains(stderr, "in the past") {
		t.Errorf("expected past-date warning on stderr, got: %q", stderr)
	}
	var jsonResp map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &jsonResp); err != nil {
		t.Fatalf("failed to parse --json stdout %q: %v", stdout, err)
	}
	if !strings.Contains(jsonResp["warning"], "in the past") {
		t.Errorf("expected warning in JSON output, got: %+v", jsonResp)
	}

	// 5. Clear with --stale-after ""
	cmdUpdate([]string{"decisions/adr-012", bundle, "--stale-after", ""})
	b, err = okf.LoadBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if b.Concepts["decisions/adr-012"].StaleAfter != "" {
		t.Fatalf("cleared stale_after = %q, want empty", b.Concepts["decisions/adr-012"].StaleAfter)
	}

	// 6. Verify log.md recorded stale_after mutations
	logBytes, err := os.ReadFile(filepath.Join(bundle, "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	logStr := string(logBytes)
	if !strings.Contains(logStr, "set stale_after 2027-03-01") || !strings.Contains(logStr, "cleared stale_after") {
		t.Errorf("log.md missing stale_after entries:\n%s", logStr)
	}
}

func TestCLIStaleAfterInvalidRejectedWithoutModifyingFile(t *testing.T) {
	if mode := os.Getenv("OKF_TEST_INVALID_STALE"); mode != "" {
		bundle := os.Getenv("OKF_TEST_STALE_BUNDLE")
		if mode == "create" {
			cmdCreate([]string{"bad", bundle, "--stale-after", "not-a-date"})
		} else {
			cmdUpdate([]string{"item", bundle, "--stale-after", "not-a-date"})
		}
		return
	}

	bundle := mutationTestBundle(t)
	cmdCreate([]string{"item", bundle, "--stale-after", "2027-03-01"})
	before, err := os.ReadFile(filepath.Join(bundle, "item.md"))
	if err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"create", "update"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestCLIStaleAfterInvalidRejectedWithoutModifyingFile$")
			cmd.Env = append(os.Environ(), "OKF_TEST_INVALID_STALE="+mode, "OKF_TEST_STALE_BUNDLE="+bundle)
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "invalid stale_after") {
				t.Fatalf("expected invalid stale_after error, got %v: %s", err, output)
			}
		})
	}

	if _, err := os.Stat(filepath.Join(bundle, "bad.md")); !os.IsNotExist(err) {
		t.Fatalf("invalid create wrote bad.md")
	}
	after, err := os.ReadFile(filepath.Join(bundle, "item.md"))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("invalid update modified item.md")
	}
}

func TestCLIChangesAndChangeNoteParity(t *testing.T) {
	bundle := mutationTestBundle(t)

	cmdCreate([]string{"people/bob", bundle, "--type", "Person", "--title", "Bob", "--desc", "Backend lead.", "--change-note", "Created Bob profile"})
	cmdCreate([]string{"processes/oncall", bundle, "--type", "Process", "--title", "On-Call", "--desc", "Incident rotation."})
	cmdUpdate([]string{"people/bob", bundle, "--desc", "Principal backend lead.", "--change-note", "Updated role"})
	cmdRelate([]string{"people/bob", "processes/oncall", bundle, "--desc", "owns rotation", "--change-note", "Linked on-call owner"})

	stdout, stderr := captureStdoutStderr(t, func() {
		cmdChanges([]string{bundle, "--since", "1d", "--json"})
	})
	if stderr != "" {
		t.Errorf("unexpected stderr: %q", stderr)
	}

	var cliRes okf.ChangesResult
	if err := json.Unmarshal([]byte(stdout), &cliRes); err != nil {
		t.Fatalf("failed to parse okf changes --json output: %v\noutput:\n%s", err, stdout)
	}

	if cliRes.Total != 2 || len(cliRes.Changes) != 2 {
		t.Fatalf("expected 2 changes, got total=%d len=%d", cliRes.Total, len(cliRes.Changes))
	}

	var bob *okf.ConceptChange
	for i := range cliRes.Changes {
		if cliRes.Changes[i].ConceptID == "people/bob" {
			bob = &cliRes.Changes[i]
			break
		}
	}
	if bob == nil {
		t.Fatalf("people/bob missing from changes: %+v", cliRes.Changes)
	}
	if bob.Title != "Bob" || bob.Description != "Principal backend lead." {
		t.Errorf("unexpected bob metadata: %+v", bob)
	}
	if len(bob.Operations) != 3 {
		t.Fatalf("expected 3 ops for people/bob, got %+v", bob.Operations)
	}
	if bob.Operations[0].Op != "create" || bob.Operations[0].Note != "Created Bob profile" {
		t.Errorf("unexpected op[0]: %+v", bob.Operations[0])
	}
	if bob.Operations[1].Op != "update" || bob.Operations[1].Note != "Updated role" {
		t.Errorf("unexpected op[1]: %+v", bob.Operations[1])
	}
	if bob.Operations[2].Op != "relate" || bob.Operations[2].Target != "processes/oncall" || bob.Operations[2].Note != "Linked on-call owner" {
		t.Errorf("unexpected op[2]: %+v", bob.Operations[2])
	}
}

func TestCLIChangesExitCodes(t *testing.T) {
	if mode := os.Getenv("OKF_TEST_CHANGES_EXIT"); mode != "" {
		bundle := os.Getenv("OKF_TEST_CHANGES_BUNDLE")
		switch mode {
		case "missing-since":
			cmdChanges([]string{bundle})
		case "invalid-since":
			cmdChanges([]string{bundle, "--since", "not-a-time"})
		case "missing-bundle":
			cmdChanges([]string{filepath.Join(bundle, "does-not-exist"), "--since", "1d"})
		}
		return
	}

	bundle := mutationTestBundle(t)
	cases := []struct {
		mode     string
		wantCode int
	}{
		{"missing-since", 1},
		{"invalid-since", 1},
		{"missing-bundle", 2},
	}

	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestCLIChangesExitCodes$")
			cmd.Env = append(os.Environ(), "OKF_TEST_CHANGES_EXIT="+tc.mode, "OKF_TEST_CHANGES_BUNDLE="+bundle)
			err := cmd.Run()
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("expected ExitError for %s, got %v", tc.mode, err)
			}
			if exitErr.ExitCode() != tc.wantCode {
				t.Errorf("%s exit code = %d, want %d", tc.mode, exitErr.ExitCode(), tc.wantCode)
			}
		})
	}
}
