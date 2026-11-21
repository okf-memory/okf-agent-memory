package okf

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrPathTraversal is returned when a link or frontmatter path field attempts
// to escape the bundle root (CWE-22).
var ErrPathTraversal = errors.New("path traversal denied: target escapes bundle root")

// uriSchemeRegex matches RFC 3986 URI schemes with at least two characters
// (e.g. "https:", "http:", "mailto:", "okf:", "gs:", "urn:", "user:", "system:").
// Requiring >= 2 characters ensures Windows drive letters (e.g. "C:\..." or "C:/...")
// are NOT misclassified as URI schemes.
var uriSchemeRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]+:`)

// hasURIScheme reports whether s starts with an RFC 3986 URI scheme.
func hasURIScheme(s string) bool {
	return uriSchemeRegex.MatchString(s)
}

// isWindowsDrivePath reports whether s starts with a Windows drive prefix like "C:" or "D:/".
func isWindowsDrivePath(s string) bool {
	return len(s) >= 2 && s[1] == ':' && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
}

// IsAbsPath securely checks if a path is absolute across platforms, preventing
// path traversal evasion where Windows paths (e.g., C:\) are evaluated on POSIX,
// or POSIX paths (e.g., /etc) are evaluated on Windows.
func IsAbsPath(p string) bool {
	if filepath.IsAbs(p) {
		return true
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return true
	}
	if isWindowsDrivePath(p) {
		return true
	}
	return false
}

// ResolvedPathKind classifies the outcome of resolving a Markdown link or frontmatter path field.
type ResolvedPathKind int

const (
	PathKindEmpty     ResolvedPathKind = iota
	PathKindExternal                   // Absolute URL (has URI scheme) or scoped vendor ref (@...)
	PathKindFragment                   // Same-document #fragment anchor
	PathKindConcept                    // Resolves to a concept in the bundle (ConceptID populated)
	PathKindAsset                      // Resolves to a non-Markdown asset in the bundle (RelPath populated)
	PathKindSkippedMD                  // Resolves to a non-concept .md file in the bundle (e.g. README.md, SKILL.md)
	PathKindReserved                   // Resolves to a reserved bundle file (index.md, log.md, AGENTS.md)
	PathKindDirectory                  // Resolves to a directory in the bundle
	PathKindMissing                    // Valid in-bundle path, but no matching concept/asset/file exists
	PathKindEscapes                    // Path traversal attempt escaping bundle root (CWE-22)
)

// ResolvedPath holds the result of resolving a Markdown link or frontmatter path field
// according to OKF v0.3 §6 (with v0.2 §12.1 bundle-root fallback).
type ResolvedPath struct {
	Raw          string           // Original raw value
	Kind         ResolvedPathKind // Classification of the resolved target
	RelPath      string           // Normalized bundle-relative path (e.g. "tables/orders.md")
	ConceptID    string           // Populated when Kind == PathKindConcept (e.g. "tables/orders")
	UsedFallback bool             // True if resolved via v0.2 bundle-root fallback
	Err          error            // Non-nil when Kind == PathKindEscapes
}

// resolveConfinedRelative resolves relTarget relative to baseDir (bundle-relative, "" for root)
// while strictly enforcing CWE-22 bundle-root confinement. Any ".." step that would traverse
// above the bundle root returns ErrPathTraversal immediately.
func resolveConfinedRelative(baseDir, relTarget string) (string, error) {
	var stack []string
	if baseDir != "" && baseDir != "." {
		for _, seg := range strings.Split(baseDir, "/") {
			if seg == "" || seg == "." {
				continue
			}
			stack = append(stack, seg)
		}
	}

	for _, seg := range strings.Split(relTarget, "/") {
		switch seg {
		case "", ".":
			continue
		case "..":
			if len(stack) == 0 {
				return "", ErrPathTraversal
			}
			stack = stack[:len(stack)-1]
		default:
			stack = append(stack, seg)
		}
	}

	return strings.Join(stack, "/"), nil
}

// ResolvePath resolves a Markdown link target or frontmatter path field value from sourceRelPath
// using unified OKF v0.3 §6 rules:
//  1. Value with a URI scheme (e.g. https:, http:, mailto:) or @vendor ref -> PathKindExternal.
//  2. Value starting with '/' -> resolved from the bundle root.
//  3. Otherwise -> resolved relative to the directory of sourceRelPath.
//
// For v0.2 (or unversioned) bundles, unprefixed relative paths that do not resolve from the
// containing file's directory fall back to bundle-root resolution (OKF v0.3 §6.2 / §12.1).
// All in-bundle paths are strictly confined to the bundle root (CWE-22).
func (b *Bundle) ResolvePath(sourceRelPath, rawTarget string) ResolvedPath {
	trimmed := strings.TrimSpace(rawTarget)
	if trimmed == "" {
		return ResolvedPath{Raw: rawTarget, Kind: PathKindEmpty}
	}

	if IsExternalLink(trimmed) {
		return ResolvedPath{Raw: rawTarget, Kind: PathKindExternal}
	}

	if strings.HasPrefix(trimmed, "#") {
		return ResolvedPath{Raw: rawTarget, Kind: PathKindFragment}
	}

	// Strip #fragment and ?query before path normalization.
	cleanTarget := strings.SplitN(trimmed, "#", 2)[0]
	cleanTarget = strings.SplitN(cleanTarget, "?", 2)[0]
	if cleanTarget == "" {
		return ResolvedPath{Raw: rawTarget, Kind: PathKindFragment}
	}

	// Normalize Windows backslashes to POSIX forward slashes.
	normTarget := strings.ReplaceAll(cleanTarget, "\\", "/")

	// Percent-decode RFC 3986 sequences BEFORE confinement checks so encoded
	// traversal sequences like "%2e%2e/%2e%2e/etc/passwd" cannot bypass CWE-22 guards.
	unescaped := normTarget
	if dec, err := url.PathUnescape(normTarget); err == nil {
		unescaped = strings.ReplaceAll(dec, "\\", "/")
	}

	// Reject Windows drive paths (e.g. C:/etc/passwd) and UNC/protocol-relative paths (//host/share).
	if isWindowsDrivePath(unescaped) || strings.HasPrefix(unescaped, "//") {
		return ResolvedPath{
			Raw:  rawTarget,
			Kind: PathKindEscapes,
			Err:  fmt.Errorf("%w: %q", ErrPathTraversal, rawTarget),
		}
	}

	hasTrailingSlash := strings.HasSuffix(unescaped, "/")

	// Rule 2: Leading '/' resolves from bundle root.
	if strings.HasPrefix(unescaped, "/") {
		relPath, err := resolveConfinedRelative("", unescaped[1:])
		if err != nil {
			return ResolvedPath{
				Raw:  rawTarget,
				Kind: PathKindEscapes,
				Err:  fmt.Errorf("%w: %q", ErrPathTraversal, rawTarget),
			}
		}
		return b.classifyInBundlePath(rawTarget, relPath, hasTrailingSlash, false)
	}

	// Rule 3: Unprefixed path resolves relative to the containing file's directory.
	normSource := strings.ReplaceAll(strings.TrimSpace(sourceRelPath), "\\", "/")
	srcDir := path.Dir(normSource)
	if srcDir == "." || srcDir == "/" {
		srcDir = ""
	}
	srcDir = strings.TrimPrefix(srcDir, "/")

	relPath, err := resolveConfinedRelative(srcDir, unescaped)
	if err != nil {
		return ResolvedPath{
			Raw:  rawTarget,
			Kind: PathKindEscapes,
			Err:  fmt.Errorf("%w: %q", ErrPathTraversal, rawTarget),
		}
	}

	res := b.classifyInBundlePath(rawTarget, relPath, hasTrailingSlash, false)
	if res.Kind != PathKindMissing {
		return res
	}

	// OKF v0.3 §6.2 / §12.1: v0.2 compatibility fallback to bundle root when
	// an unprefixed path does not resolve from the containing file's directory.
	if b != nil && b.IsV02OrEarlier() && srcDir != "" &&
		!strings.HasPrefix(unescaped, "./") && !strings.HasPrefix(unescaped, "../") {
		rootRel, rootErr := resolveConfinedRelative("", unescaped)
		if rootErr == nil && rootRel != relPath {
			fb := b.classifyInBundlePath(rawTarget, rootRel, hasTrailingSlash, true)
			if fb.Kind != PathKindMissing {
				return fb
			}
		}
	}

	return res
}

func (b *Bundle) classifyInBundlePath(raw, relPath string, hasTrailingSlash, usedFallback bool) ResolvedPath {
	out := ResolvedPath{
		Raw:          raw,
		RelPath:      relPath,
		UsedFallback: usedFallback,
	}

	if relPath == "" {
		out.Kind = PathKindDirectory
		return out
	}

	if hasTrailingSlash {
		if b != nil && b.hasDirectory(relPath) {
			out.Kind = PathKindDirectory
		} else {
			out.Kind = PathKindMissing
		}
		return out
	}

	base := path.Base(relPath)
	if strings.EqualFold(base, "index.md") || strings.EqualFold(relPath, "log.md") || strings.EqualFold(relPath, "AGENTS.md") {
		out.Kind = PathKindReserved
		return out
	}

	if strings.HasSuffix(relPath, ".md") {
		cid := strings.TrimSuffix(relPath, ".md")
		if b != nil {
			if _, ok := b.Concepts[cid]; ok {
				out.Kind = PathKindConcept
				out.ConceptID = cid
				return out
			}
			if b.isSkippedMarkdown(relPath) {
				out.Kind = PathKindSkippedMD
				return out
			}
		}
		out.Kind = PathKindMissing
		return out
	}

	if b != nil {
		if b.HasAsset(relPath) {
			out.Kind = PathKindAsset
			return out
		}
		// Allow concept ID references without trailing ".md" (e.g. superseded_by: /metrics/gross-margin)
		if _, ok := b.Concepts[relPath]; ok {
			out.Kind = PathKindConcept
			out.ConceptID = relPath
			out.RelPath = relPath + ".md"
			return out
		}
		if b.hasDirectory(relPath) {
			out.Kind = PathKindDirectory
			return out
		}
	}

	out.Kind = PathKindMissing
	return out
}

func (b *Bundle) hasDirectory(relDir string) bool {
	if b == nil || relDir == "" {
		return false
	}
	prefix := strings.TrimSuffix(relDir, "/") + "/"
	for _, c := range b.Concepts {
		if strings.HasPrefix(c.Path, prefix) {
			return true
		}
	}
	for idxPath := range b.Indexes {
		if strings.HasPrefix(idxPath, prefix) {
			return true
		}
	}
	for _, asset := range b.Assets {
		if strings.HasPrefix(asset, prefix) {
			return true
		}
	}
	for _, skipped := range b.SkippedMarkdown {
		if strings.HasPrefix(skipped, prefix) {
			return true
		}
	}
	return false
}

// isPathLikeSourceResource reports whether a sources[].resource value looks like a
// URL or bundle path rather than a free-form prose scope descriptor (OKF v0.3 §5.1 / §6.2).
func isPathLikeSourceResource(val string) bool {
	s := strings.TrimSpace(val)
	if s == "" {
		return false
	}
	if IsExternalLink(s) {
		return true
	}
	// RFC 3986 URI references MUST percent-encode spaces; unencoded whitespace
	// indicates a prose scope descriptor (e.g. "all queries in BigQuery project X").
	if strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	return true
}
