package sync

import (
	"path/filepath"
	"regexp"
	"strings"
)

// indexListingRegex matches an index listing line such as
// "* [Title](file.md) - Description" and captures its link target.
var indexListingRegex = regexp.MustCompile(`^\s*[*-]\s+\[[^\]]*\]\(([^)\s]+)\)`)

// isIndexFile reports whether path is a bundle index (root or folder index.md).
func isIndexFile(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == "index.md" || strings.HasSuffix(clean, "/index.md")
}

type indexDoc struct {
	lines    []string          // original lines, in order
	prose    []string          // non-listing lines, in order
	listings map[string]string // link target -> listing line
	order    []string          // link targets in order of appearance
}

func parseIndex(content []byte) indexDoc {
	d := indexDoc{listings: map[string]string{}}
	if len(content) == 0 {
		return d
	}
	d.lines = strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	for _, line := range d.lines {
		if m := indexListingRegex.FindStringSubmatch(line); m != nil {
			if _, seen := d.listings[m[1]]; !seen {
				d.order = append(d.order, m[1])
			}
			d.listings[m[1]] = line
			continue
		}
		d.prose = append(d.prose, line)
	}
	return d
}

// mergeListing resolves one listing across base, local, and remote. An empty
// string means "absent". Edits beat deletions so no listing is silently lost;
// when both sides edited the same listing differently the remote one wins,
// matching the engine's remote-wins collision rule.
func mergeListing(base, local, remote string) string {
	switch {
	case local == remote:
		return local
	case local == base:
		return remote
	case remote == base:
		return local
	case local == "":
		return remote
	case remote == "":
		return local
	default:
		return remote
	}
}

// MergeIndexContent performs a 3-way merge of an OKF index.md, treating each
// listing line as an independent record keyed by its link target. This lets
// two devices that each add, update, or remove different concepts in the same
// folder converge without a collision.
//
// Non-listing text (headings, frontmatter, prose) is merged as a whole: if both
// sides changed it differently, MergeIndexContent returns ok=false and the
// caller falls back to the regular collision failsafe. base may be nil when the
// index was created independently on both sides.
func MergeIndexContent(base, local, remote []byte) (merged []byte, ok bool) {
	b, l, r := parseIndex(base), parseIndex(local), parseIndex(remote)

	prose := func(d indexDoc) string { return strings.Join(d.prose, "\n") }
	var template indexDoc
	switch {
	case prose(l) == prose(r) || prose(l) == prose(b):
		template = r
	case prose(r) == prose(b):
		template = l
	default:
		return nil, false
	}

	resolved := map[string]string{}
	var keys []string
	for _, d := range []indexDoc{r, l, b} {
		for _, k := range d.order {
			if _, done := resolved[k]; done {
				continue
			}
			resolved[k] = mergeListing(b.listings[k], l.listings[k], r.listings[k])
			keys = append(keys, k)
		}
	}

	var out []string
	emitted := map[string]bool{}
	lastListing := -1
	for _, line := range template.lines {
		if m := indexListingRegex.FindStringSubmatch(line); m != nil {
			if !emitted[m[1]] && resolved[m[1]] != "" {
				out = append(out, resolved[m[1]])
				lastListing = len(out) - 1
			}
			emitted[m[1]] = true
			continue
		}
		out = append(out, line)
	}

	// Listings the template did not contain go right after its last listing
	// (or at the end when it had none), in first-seen order.
	var extra []string
	for _, k := range keys {
		if !emitted[k] && resolved[k] != "" {
			extra = append(extra, resolved[k])
		}
	}
	if len(extra) > 0 {
		at := len(out)
		if lastListing >= 0 {
			at = lastListing + 1
		}
		out = append(out[:at], append(extra, out[at:]...)...)
	}

	return []byte(strings.Join(out, "\n") + "\n"), true
}
