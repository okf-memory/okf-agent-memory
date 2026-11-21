package okf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Freshness provider identifiers for StalenessInfo.Provider.
const (
	FreshnessProviderGit         = "git"
	FreshnessProviderFrontmatter = "frontmatter"
)

const (
	gitCommitMarker = "__OKF_COMMIT__\x1f"
	gitLogTimeout   = 10 * time.Second
)

// StalenessInfo holds computed provenance freshness metadata for a concept.
// These fields are purely runtime/output metadata and are never persisted into
// concept YAML frontmatter.
type StalenessInfo struct {
	LastTouchedAt string `json:"last_touched_at,omitempty"`
	Commit        string `json:"commit,omitempty"`
	Provider      string `json:"provider"` // "git" or "frontmatter"
}

type gitCommitInfo struct {
	Hash    string
	TimeStr string
	Time    time.Time
}

// FrontmatterStaleness computes StalenessInfo for a concept without invoking
// any external subprocesses. It prefers generated.at from frontmatter and falls
// back to the file's modification time (UTC RFC3339) when available.
func FrontmatterStaleness(c *Concept, bundleDir string) *StalenessInfo {
	if c != nil && c.Generated != nil && strings.TrimSpace(c.Generated.At) != "" {
		return &StalenessInfo{
			LastTouchedAt: strings.TrimSpace(c.Generated.At),
			Provider:      FreshnessProviderFrontmatter,
		}
	}
	if c != nil && bundleDir != "" && c.Path != "" {
		fullPath := filepath.Join(bundleDir, filepath.FromSlash(c.Path))
		// #nosec G304 -- fullPath is constructed from loaded bundle root and concept path
		if info, err := os.Stat(fullPath); err == nil {
			return &StalenessInfo{
				LastTouchedAt: info.ModTime().UTC().Format(time.RFC3339),
				Provider:      FreshnessProviderFrontmatter,
			}
		}
	}
	return &StalenessInfo{
		Provider: FreshnessProviderFrontmatter,
	}
}

// ResolveBundleStaleness computes StalenessInfo for every concept in b.
// When useGit is false, it uses frontmatter/mtime exclusively with zero subprocess forks.
// When useGit is true and b resides inside a Git repository, it resolves last-touched
// timestamps and short commit hashes via a single batched `git log` over the bundle
// directory, falling back cleanly to frontmatter for untracked files or non-Git environments.
func ResolveBundleStaleness(b *Bundle, useGit bool) map[string]*StalenessInfo {
	out := make(map[string]*StalenessInfo)
	if b == nil {
		return out
	}

	if !useGit || b.RootPath == "" {
		for id, c := range b.Concepts {
			out[id] = FrontmatterStaleness(c, b.RootPath)
		}
		return out
	}

	repoRoot, ok := findGitRoot(b.RootPath)
	if !ok {
		for id, c := range b.Concepts {
			out[id] = FrontmatterStaleness(c, b.RootPath)
		}
		return out
	}

	bundleAbs := resolveRealAbs(b.RootPath)
	bundleRel, err := filepath.Rel(repoRoot, bundleAbs)
	if err != nil || bundleRel == ".." || strings.HasPrefix(bundleRel, ".."+string(filepath.Separator)) {
		for id, c := range b.Concepts {
			out[id] = FrontmatterStaleness(c, b.RootPath)
		}
		return out
	}
	bundleRelSlash := filepath.ToSlash(bundleRel)
	if bundleRelSlash == "" {
		bundleRelSlash = "."
	}

	commits, gitOK := gitLogLastCommits(repoRoot, []string{bundleRelSlash})
	for id, c := range b.Concepts {
		if gitOK && c != nil && c.Path != "" {
			normPath := strings.TrimPrefix(strings.ReplaceAll(c.Path, "\\", "/"), "./")
			repoRelPath := normPath
			if bundleRelSlash != "." {
				repoRelPath = path.Join(bundleRelSlash, normPath)
			}
			if info, found := commits[repoRelPath]; found {
				out[id] = &StalenessInfo{
					LastTouchedAt: info.TimeStr,
					Commit:        info.Hash,
					Provider:      FreshnessProviderGit,
				}
				continue
			}
		}
		out[id] = FrontmatterStaleness(c, b.RootPath)
	}
	return out
}

// ResolveConceptStaleness computes StalenessInfo for a single concept.
// When useGit is false, it returns frontmatter/mtime metadata with zero subprocess forks.
// When useGit is true and bundleDir is inside a Git repo, it queries `git log -n 1`
// for the concept file and falls back cleanly to frontmatter otherwise.
func ResolveConceptStaleness(c *Concept, bundleDir string, useGit bool) *StalenessInfo {
	if !useGit || c == nil || bundleDir == "" || c.Path == "" {
		return FrontmatterStaleness(c, bundleDir)
	}
	repoRoot, ok := findGitRoot(bundleDir)
	if !ok {
		return FrontmatterStaleness(c, bundleDir)
	}

	bundleAbs := resolveRealAbs(bundleDir)
	fullConceptAbs := filepath.Join(bundleAbs, filepath.FromSlash(c.Path))
	conceptRel, err := filepath.Rel(repoRoot, fullConceptAbs)
	if err != nil || conceptRel == ".." || strings.HasPrefix(conceptRel, ".."+string(filepath.Separator)) {
		return FrontmatterStaleness(c, bundleDir)
	}

	if info, found := gitLogSingleFile(repoRoot, filepath.ToSlash(conceptRel)); found {
		return &StalenessInfo{
			LastTouchedAt: info.TimeStr,
			Commit:        info.Hash,
			Provider:      FreshnessProviderGit,
		}
	}
	return FrontmatterStaleness(c, bundleDir)
}

func resolveRealAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	if realPath, err := filepath.EvalSymlinks(abs); err == nil {
		return realPath
	}
	return abs
}

// findGitRoot walks upward from startDir looking for a .git directory or file
// (supporting both standard repositories and Git worktrees).
func findGitRoot(startDir string) (string, bool) {
	if strings.TrimSpace(startDir) == "" {
		return "", false
	}
	dir := resolveRealAbs(startDir)
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// gitLogLastCommits runs a single batched `git log` command over pathspecs and returns
// the most recent commit (short hash and ISO 8601 timestamp) for each touched file path
// relative to repoRoot.
func gitLogLastCommits(repoRoot string, pathspecs []string) (map[string]gitCommitInfo, bool) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, false
	}

	args := []string{
		"-C", repoRoot,
		"-c", "core.quotepath=false",
		"log",
		"--format=" + gitCommitMarker + "%h\x1f%cI",
		"--name-only",
		"--",
	}
	for _, ps := range pathspecs {
		clean := strings.TrimSpace(ps)
		if clean == "" {
			continue
		}
		args = append(args, clean)
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitLogTimeout)
	defer cancel()

	// #nosec G204 -- fixed "git" binary with validated repo root and pathspecs terminated by "--"
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}

	commits := make(map[string]gitCommitInfo)
	var current gitCommitInfo
	hasCurrent := false

	for _, rawLine := range strings.Split(string(out), "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" {
			continue
		}
		if after, ok := strings.CutPrefix(line, gitCommitMarker); ok {
			parts := strings.SplitN(after, "\x1f", 2)
			if len(parts) == 2 {
				t, _ := parseTimestamp(parts[1])
				current = gitCommitInfo{
					Hash:    strings.TrimSpace(parts[0]),
					TimeStr: strings.TrimSpace(parts[1]),
					Time:    t,
				}
				hasCurrent = true
			} else {
				hasCurrent = false
			}
			continue
		}
		if !hasCurrent {
			continue
		}
		relPath := strings.TrimPrefix(strings.ReplaceAll(line, "\\", "/"), "./")
		if _, seen := commits[relPath]; !seen {
			commits[relPath] = current
		}
	}

	return commits, true
}

func gitLogSingleFile(repoRoot, relPath string) (gitCommitInfo, bool) {
	if _, err := exec.LookPath("git"); err != nil {
		return gitCommitInfo{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitLogTimeout)
	defer cancel()

	// #nosec G204 -- fixed "git" binary with validated repo root and path terminated by "--"
	cmd := exec.CommandContext(ctx, "git",
		"-C", repoRoot,
		"-c", "core.quotepath=false",
		"log", "-n", "1",
		"--format=%h\x1f%cI",
		"--", relPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return gitCommitInfo{}, false
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return gitCommitInfo{}, false
	}
	parts := strings.SplitN(trimmed, "\x1f", 2)
	if len(parts) != 2 {
		return gitCommitInfo{}, false
	}
	t, _ := parseTimestamp(parts[1])
	return gitCommitInfo{
		Hash:    strings.TrimSpace(parts[0]),
		TimeStr: strings.TrimSpace(parts[1]),
		Time:    t,
	}, true
}

// staticPathspecPrefix extracts the non-glob prefix of a code_ref so it can be passed
// as a safe Git pathspec. Returns "." if the ref begins with a wildcard.
func staticPathspecPrefix(ref string) string {
	norm := strings.TrimPrefix(strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(ref), "\\", "/"), "."), "/")
	if norm == "" {
		return ""
	}
	idx := strings.IndexAny(norm, "*?[{")
	if idx == -1 {
		return strings.TrimSuffix(norm, "/")
	}
	prefix := norm[:idx]
	slash := strings.LastIndex(prefix, "/")
	if slash <= 0 {
		return "."
	}
	return strings.TrimSuffix(prefix[:slash], "/")
}

// checkGitCodeDrift implements relative code-drift detection (--drift-git).
// It flags concepts whose code_refs files have newer Git commits than the concept's
// own last_touched_at timestamp. Outside a Git repository, it returns silently without error.
func checkGitCodeDrift(b *Bundle, res *ValidationResult) {
	if b == nil || b.RootPath == "" {
		return
	}
	repoRoot, ok := findGitRoot(b.RootPath)
	if !ok {
		return
	}

	var conceptsWithRefs []*Concept
	for _, c := range b.Concepts {
		if c != nil && len(c.CodeRefs) > 0 {
			conceptsWithRefs = append(conceptsWithRefs, c)
		}
	}
	if len(conceptsWithRefs) == 0 {
		return
	}
	sort.Slice(conceptsWithRefs, func(i, j int) bool {
		return conceptsWithRefs[i].ID < conceptsWithRefs[j].ID
	})

	bundleAbs := resolveRealAbs(b.RootPath)
	projectRoot := filepath.Dir(bundleAbs)
	projRel, err := filepath.Rel(repoRoot, projectRoot)
	if err != nil || projRel == ".." || strings.HasPrefix(projRel, ".."+string(filepath.Separator)) {
		// Bundle lives directly at repo root or outside; use repoRoot as base.
		projRel = "."
	}
	projRelSlash := filepath.ToSlash(projRel)
	if projRelSlash == "" {
		projRelSlash = "."
	}

	// Collect deduplicated pathspecs for a single batched git log call.
	pathspecSet := make(map[string]bool)
	hasWildcardRoot := false
	for _, c := range conceptsWithRefs {
		for _, ref := range c.CodeRefs {
			refTrimmed := strings.TrimSpace(ref)
			if refTrimmed == "" {
				continue
			}
			normRef := strings.ReplaceAll(refTrimmed, "\\", "/")
			cleanRef := filepath.Clean(normRef)
			if IsAbsPath(cleanRef) || cleanRef == ".." || strings.HasPrefix(cleanRef, ".."+string(filepath.Separator)) || strings.HasPrefix(cleanRef, "../") {
				continue
			}
			prefix := staticPathspecPrefix(normRef)
			if prefix == "" {
				continue
			}
			if prefix == "." {
				if projRelSlash == "." {
					hasWildcardRoot = true
				} else {
					pathspecSet[projRelSlash] = true
				}
				continue
			}
			if projRelSlash != "." {
				prefix = path.Join(projRelSlash, prefix)
			}
			pathspecSet[prefix] = true
		}
	}

	if !hasWildcardRoot && len(pathspecSet) == 0 {
		return
	}

	var pathspecs []string
	if hasWildcardRoot {
		pathspecs = []string{"."}
	} else {
		pathspecs = make([]string, 0, len(pathspecSet))
		for ps := range pathspecSet {
			pathspecs = append(pathspecs, ps)
		}
		sort.Strings(pathspecs)
	}

	conceptStaleness := ResolveBundleStaleness(b, true)
	codeCommits, gitOK := gitLogLastCommits(repoRoot, pathspecs)
	if !gitOK || len(codeCommits) == 0 {
		return
	}

	for _, c := range conceptsWithRefs {
		st := conceptStaleness[c.ID]
		if st == nil || st.LastTouchedAt == "" {
			continue
		}
		conceptTime, ok := parseTimestamp(st.LastTouchedAt)
		if !ok {
			continue
		}

		for _, ref := range c.CodeRefs {
			refTrimmed := strings.TrimSpace(ref)
			if refTrimmed == "" {
				continue
			}
			normRef := strings.ReplaceAll(refTrimmed, "\\", "/")
			cleanRef := filepath.Clean(normRef)
			if IsAbsPath(cleanRef) || cleanRef == ".." || strings.HasPrefix(cleanRef, ".."+string(filepath.Separator)) || strings.HasPrefix(cleanRef, "../") {
				continue
			}

			var newest gitCommitInfo
			foundNewer := false
			for repoRelPath, info := range codeCommits {
				if info.Time.IsZero() {
					continue
				}
				candidate := repoRelPath
				if projRelSlash != "." {
					trimmed, hasPrefix := strings.CutPrefix(repoRelPath, projRelSlash+"/")
					if !hasPrefix {
						continue
					}
					candidate = trimmed
				}
				if !matchCodeRef(normRef, candidate) {
					continue
				}
				if info.Time.After(conceptTime) {
					if !foundNewer || info.Time.After(newest.Time) {
						newest = info
						foundNewer = true
					}
				}
			}

			if foundNewer {
				res.Warnings = append(res.Warnings, fmt.Sprintf(
					"%s: code_refs '%s' has newer Git commit (%s at %s) than concept last_touched_at (%s)",
					c.Path, refTrimmed, newest.Hash, newest.TimeStr, st.LastTouchedAt,
				))
			}
		}
	}
}
