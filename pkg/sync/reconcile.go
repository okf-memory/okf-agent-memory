package sync

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/okf-memory/okf-agent-memory/pkg/vault"
)

// Conflict describes a collision where the same file was modified incompatibly in local and remote.
type Conflict struct {
	Path        string
	BaseEntry   *vault.TreeEntry // nil if the file was newly created in both branches
	LocalEntry  vault.TreeEntry
	RemoteEntry vault.TreeEntry
}

// ReconcileResult contains the computed merged tree and any detected conflicts.
type ReconcileResult struct {
	MergedTree *vault.Tree
	Conflicts  []Conflict
}

// ConflictLocalPath computes the fork filename for saving conflicting local changes.
// e.g. "ideas.md" -> "ideas.conflict-local.md"
func ConflictLocalPath(path string) string {
	ext := filepath.Ext(path)
	if ext != "" {
		base := strings.TrimSuffix(path, ext)
		return base + ".conflict-local" + ext
	}
	return path + ".conflict-local"
}

// ConflictForkPath computes the fork filename for saving conflicting local
// changes, suffixed with a short hash of the local content's plaintext hash.
// The suffix keeps every conflicting writer's fork distinct: with one fixed
// path per file, a later device's conflict would overwrite the conflict copy a
// previous device already committed, silently dropping that device's edit from
// the current tree. The plaintext hash (not the blob hash) is used because the
// encrypted envelope is non-deterministic across devices. The fixed-path
// result of ConflictLocalPath is returned when no plaintext hash is available.
// e.g. "ideas.md" -> "ideas.conflict-local-1a2b3c4d.md"
func ConflictForkPath(path string, local vault.TreeEntry) string {
	base := ConflictLocalPath(path)
	hash := local.PlaintextHash
	if hash == "" {
		return base
	}
	if len(hash) > 8 {
		hash = hash[:8]
	}
	ext := filepath.Ext(base)
	if ext != "" {
		return strings.TrimSuffix(base, ext) + "-" + hash + ext
	}
	return base + "-" + hash
}

// Reconcile performs a 3-way merge of the file manifests: base, local, and remote.
// If both local and remote modified disjoint sets of files, it merges cleanly with 0 conflicts.
// If both modified the same file incompatibly, it reports a Conflict.
func Reconcile(base, local, remote *vault.Tree) (*ReconcileResult, error) {
	if base == nil {
		base = vault.NewTree()
	}
	if local == nil {
		local = vault.NewTree()
	}
	if remote == nil {
		remote = vault.NewTree()
	}

	merged := vault.NewTree()
	var conflicts []Conflict

	// Collect all unique paths
	allPathsMap := make(map[string]struct{})
	for p := range base.Entries {
		allPathsMap[p] = struct{}{}
	}
	for p := range local.Entries {
		allPathsMap[p] = struct{}{}
	}
	for p := range remote.Entries {
		allPathsMap[p] = struct{}{}
	}

	var allPaths []string
	for p := range allPathsMap {
		allPaths = append(allPaths, p)
	}
	sort.Strings(allPaths)

	for _, path := range allPaths {
		baseEntry, inBase := base.Entries[path]
		localEntry, inLocal := local.Entries[path]
		remoteEntry, inRemote := remote.Entries[path]

		// 1. Unchanged anywhere
		if inBase && inLocal && inRemote &&
			baseEntry.PlaintextHash == localEntry.PlaintextHash &&
			baseEntry.PlaintextHash == remoteEntry.PlaintextHash {
			merged.Entries[path] = localEntry
			continue
		}

		// 2. Added only in Local
		if !inBase && inLocal && !inRemote {
			merged.Entries[path] = localEntry
			continue
		}

		// 3. Added only in Remote
		if !inBase && !inLocal && inRemote {
			merged.Entries[path] = remoteEntry
			continue
		}

		// 4. Deleted only in Local (unchanged in Remote)
		if inBase && !inLocal && inRemote && baseEntry.PlaintextHash == remoteEntry.PlaintextHash {
			// Deleted, omit from merged
			continue
		}

		// 5. Deleted only in Remote (unchanged in Local)
		if inBase && inLocal && !inRemote && baseEntry.PlaintextHash == localEntry.PlaintextHash {
			// Deleted, omit from merged
			continue
		}

		// 6. Modified only in Local (unchanged in Remote)
		if inBase && inLocal && inRemote &&
			baseEntry.PlaintextHash != localEntry.PlaintextHash &&
			baseEntry.PlaintextHash == remoteEntry.PlaintextHash {
			merged.Entries[path] = localEntry
			continue
		}

		// 7. Modified only in Remote (unchanged in Local)
		if inBase && inLocal && inRemote &&
			baseEntry.PlaintextHash == localEntry.PlaintextHash &&
			baseEntry.PlaintextHash != remoteEntry.PlaintextHash {
			merged.Entries[path] = remoteEntry
			continue
		}

		// 8. Both modified or added
		if inLocal && inRemote {
			if localEntry.PlaintextHash == remoteEntry.PlaintextHash {
				// Both made identical change
				merged.Entries[path] = localEntry
				continue
			}

			// Incompatible modification collision!
			var basePtr *vault.TreeEntry
			if inBase {
				basePtr = &baseEntry
			}
			conflicts = append(conflicts, Conflict{
				Path:        path,
				BaseEntry:   basePtr,
				LocalEntry:  localEntry,
				RemoteEntry: remoteEntry,
			})
			// Place remote version in merged by default
			merged.Entries[path] = remoteEntry
			continue
		}

		// 9. Deleted in one, modified in the other -> conflict
		var basePtr *vault.TreeEntry
		if inBase {
			basePtr = &baseEntry
		}
		conflicts = append(conflicts, Conflict{
			Path:        path,
			BaseEntry:   basePtr,
			LocalEntry:  localEntry,
			RemoteEntry: remoteEntry,
		})
		if inRemote {
			merged.Entries[path] = remoteEntry
		} else if inLocal {
			merged.Entries[path] = localEntry
		}
	}

	return &ReconcileResult{
		MergedTree: merged,
		Conflicts:  conflicts,
	}, nil
}

// ApplyConflictFailsafe updates mergedTree according to the CLI collision policy:
// The remote version is adopted at path, and the local version is preserved at ConflictLocalPath(path).
func ApplyConflictFailsafe(merged *vault.Tree, conflicts []Conflict) {
	if merged == nil {
		return
	}
	for _, c := range conflicts {
		forked := ConflictForkPath(c.Path, c.LocalEntry)
		if c.RemoteEntry.BlobHash != "" {
			merged.Entries[c.Path] = c.RemoteEntry
		}
		if c.LocalEntry.BlobHash != "" {
			merged.Entries[forked] = c.LocalEntry
		}
	}
}
