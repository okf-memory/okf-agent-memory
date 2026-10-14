package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okf-memory/okf-agent-memory/pkg/vault"
)

// PushResult contains metadata about a successful push operation.
type PushResult struct {
	CommitHash     string
	UploadedBlobs  int
	UnchangedBlobs int
}

// PullResult contains details about files updated during a pull.
type PullResult struct {
	CommitHash   string
	UpdatedFiles []string
	DeletedFiles []string
}

// SyncResult contains the final commit hash and any detected conflicts.
type SyncResult struct {
	CommitHash string
	Conflicts  []Conflict
}

// DefaultMaxSyncRetries is the maximum number of times Sync will retry on concurrent head conflicts (HTTP 409).
const DefaultMaxSyncRetries = 5

// Engine coordinates bundle scanning, client-side encryption/decryption,
// and sync operations with the remote OKF Memory Hub.
type Engine struct {
	Client     *Client
	VaultID    string
	VaultKey   []byte
	BundlePath string
	cachedTree *vault.Tree
	cachedHead string
}

// NewEngine initializes a synchronization engine for an OKF bundle.
func NewEngine(client *Client, vaultID string, vaultKey []byte, bundlePath string) *Engine {
	engine := &Engine{
		Client:     client,
		VaultID:    vaultID,
		VaultKey:   vaultKey,
		BundlePath: bundlePath,
		cachedTree: vault.NewTree(),
	}
	// Resume from the last synced position recorded inside the bundle (if any)
	// so consecutive CLI invocations behave like one continuous session.
	// Missing or invalid state is ignored; the engine then starts empty.
	engine.loadState()
	return engine
}

// ScanBundle walks the local bundle directory and reads all non-hidden files into memory.
func (e *Engine) ScanBundle() (map[string][]byte, error) {
	files := make(map[string][]byte)

	err := filepath.Walk(e.BundlePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			return nil
		}

		name := info.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".tmp") {
			return nil
		}

		relPath, err := filepath.Rel(e.BundlePath, path)
		if err != nil {
			return err
		}
		slashPath := filepath.ToSlash(relPath)

		// #nosec G122,G304 -- path is enumerated within validated bundle directory
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("sync: failed to read %s: %w", relPath, err)
		}

		files[slashPath] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sync: bundle scan failed: %w", err)
	}

	return files, nil
}

// Push detects local bundle changes, encrypts modified files, uploads missing blobs,
// and atomically advances the remote vault head.
func (e *Engine) Push(ctx context.Context, author vault.CommitAuthor, message string) (*PushResult, error) {
	files, err := e.ScanBundle()
	if err != nil {
		return nil, err
	}

	newTree := vault.NewTree()
	blobsToUpload := make(map[string][]byte)
	var candidateHashes []string
	unchangedCount := 0

	for path, content := range files {
		plaintextHash := vault.HashPlaintext(content)

		// Change detection: Check if cached tree already has identical plaintext
		if e.cachedTree != nil && !e.cachedTree.HasChanged(path, plaintextHash) {
			entry := e.cachedTree.Entries[path]
			newTree.Entries[path] = entry
			unchangedCount++
			continue
		}

		// Encrypt file into binary envelope
		envelopeBytes, err := vault.EncryptPayload(content, e.VaultKey)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to encrypt %s: %w", path, err)
		}

		blobHash := vault.HashBlob(envelopeBytes)
		blobsToUpload[blobHash] = envelopeBytes
		candidateHashes = append(candidateHashes, blobHash)

		newTree.Entries[path] = vault.TreeEntry{
			BlobHash:      blobHash,
			Size:          int64(len(content)),
			PlaintextHash: plaintextHash,
			ModifiedAt:    time.Now().UTC().Format(time.RFC3339),
		}
	}

	// 1. Upload missing content blobs
	if len(candidateHashes) > 0 {
		missing, err := e.Client.CheckMissingBlobs(ctx, e.VaultID, candidateHashes)
		if err != nil {
			return nil, fmt.Errorf("sync: check missing blobs failed: %w", err)
		}
		for _, h := range missing {
			if data, ok := blobsToUpload[h]; ok {
				if err := e.Client.PutBlob(ctx, e.VaultID, h, data); err != nil {
					return nil, fmt.Errorf("sync: put blob %s failed: %w", h, err)
				}
			}
		}
	}

	// 2. Encrypt and upload Tree manifest
	treeBytes, err := newTree.Serialize()
	if err != nil {
		return nil, fmt.Errorf("sync: tree serialization failed: %w", err)
	}

	treeEnvelope, err := vault.EncryptPayload(treeBytes, e.VaultKey)
	if err != nil {
		return nil, fmt.Errorf("sync: tree encryption failed: %w", err)
	}

	treeBlobHash := vault.HashBlob(treeEnvelope)
	if err := e.Client.PutBlob(ctx, e.VaultID, treeBlobHash, treeEnvelope); err != nil {
		return nil, fmt.Errorf("sync: put tree blob failed: %w", err)
	}

	// 3. Create, encrypt, and upload Commit object
	var expectedHead *string
	if e.cachedHead != "" {
		expectedHead = &e.cachedHead
	}

	commit := vault.NewCommit(expectedHead, treeBlobHash, author, message)
	commitBytes, err := commit.Serialize()
	if err != nil {
		return nil, fmt.Errorf("sync: commit serialization failed: %w", err)
	}

	commitEnvelope, err := vault.EncryptPayload(commitBytes, e.VaultKey)
	if err != nil {
		return nil, fmt.Errorf("sync: commit encryption failed: %w", err)
	}

	commitBlobHash := vault.HashBlob(commitEnvelope)
	if err := e.Client.PutBlob(ctx, e.VaultID, commitBlobHash, commitEnvelope); err != nil {
		return nil, fmt.Errorf("sync: put commit blob failed: %w", err)
	}

	// 4. Atomically advance Head
	resp, err := e.Client.Commit(ctx, e.VaultID, commitBlobHash, expectedHead)
	if err != nil {
		return nil, err
	}

	e.cachedHead = resp.Head
	e.cachedTree = newTree

	if err := e.saveState(); err != nil {
		return nil, fmt.Errorf("sync: commit %s was accepted by the hub, but persisting the sync state failed: %w", resp.Head, err)
	}

	return &PushResult{
		CommitHash:     resp.Head,
		UploadedBlobs:  len(blobsToUpload),
		UnchangedBlobs: unchangedCount,
	}, nil
}

// Pull downloads the latest commit and tree manifest from the hub,
// downloads new blobs, decrypts them, and updates the local filesystem.
func (e *Engine) Pull(ctx context.Context) (*PullResult, error) {
	headResp, err := e.Client.GetHead(ctx, e.VaultID)
	if err != nil {
		return nil, err
	}

	if headResp.HeadCommit == "" {
		return &PullResult{}, nil
	}

	if headResp.HeadCommit == e.cachedHead {
		return &PullResult{CommitHash: e.cachedHead}, nil
	}

	// 1. Download and decrypt Commit
	commitEnvelope, err := e.Client.GetBlob(ctx, e.VaultID, headResp.HeadCommit)
	if err != nil {
		return nil, fmt.Errorf("sync: failed to get commit blob %s: %w", headResp.HeadCommit, err)
	}

	commitBytes, err := vault.DecryptPayload(commitEnvelope, e.VaultKey)
	if err != nil {
		return nil, fmt.Errorf("sync: failed to decrypt commit blob: %w", err)
	}

	commit, err := vault.ParseCommit(commitBytes)
	if err != nil {
		return nil, fmt.Errorf("sync: failed to parse commit: %w", err)
	}

	// 2. Download and decrypt Tree
	treeEnvelope, err := e.Client.GetBlob(ctx, e.VaultID, commit.TreeHash)
	if err != nil {
		return nil, fmt.Errorf("sync: failed to get tree blob %s: %w", commit.TreeHash, err)
	}

	treeBytes, err := vault.DecryptPayload(treeEnvelope, e.VaultKey)
	if err != nil {
		return nil, fmt.Errorf("sync: failed to decrypt tree blob: %w", err)
	}

	remoteTree, err := vault.ParseTree(treeBytes)
	if err != nil {
		return nil, fmt.Errorf("sync: failed to parse tree: %w", err)
	}

	// 3. Diff and update local filesystem
	diff := vault.DiffTrees(e.cachedTree, remoteTree)

	var updated []string
	toFetch := append(diff.Added, diff.Modified...)
	for _, path := range toFetch {
		entry := remoteTree.Entries[path]
		blobBytes, err := e.Client.GetBlob(ctx, e.VaultID, entry.BlobHash)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to get blob %s for %s: %w", entry.BlobHash, path, err)
		}

		plainBytes, err := vault.DecryptPayload(blobBytes, e.VaultKey)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to decrypt %s: %w", path, err)
		}

		fullPath := filepath.Join(e.BundlePath, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return nil, fmt.Errorf("sync: failed to mkdir for %s: %w", path, err)
		}

		if err := os.WriteFile(fullPath, plainBytes, 0o644); err != nil {
			return nil, fmt.Errorf("sync: failed to write %s: %w", path, err)
		}
		updated = append(updated, path)
	}

	for _, path := range diff.Deleted {
		fullPath := filepath.Join(e.BundlePath, filepath.FromSlash(path))
		_ = os.Remove(fullPath)
	}

	e.cachedHead = headResp.HeadCommit
	e.cachedTree = remoteTree

	if err := e.saveState(); err != nil {
		return nil, fmt.Errorf("sync: pulled commit %s, but persisting the sync state failed: %w", headResp.HeadCommit, err)
	}

	return &PullResult{
		CommitHash:   headResp.HeadCommit,
		UpdatedFiles: updated,
		DeletedFiles: diff.Deleted,
	}, nil
}

// Sync performs a full synchronization cycle:
// Attempts Push; if an HTTP 409 conflict occurs, it reconciles the divergent trees,
// writes local conflict files if necessary, and pushes the merged commit.
func (e *Engine) Sync(ctx context.Context, author vault.CommitAuthor, message string) (*SyncResult, error) {
	pushRes, err := e.Push(ctx, author, message)
	if err == nil {
		return &SyncResult{
			CommitHash: pushRes.CommitHash,
		}, nil
	}

	var conflictErr *HeadConflictError
	if !errors.As(err, &conflictErr) {
		return nil, err
	}

	remoteHead := conflictErr.CurrentHead

	for attempt := 0; attempt < DefaultMaxSyncRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(10*(1<<attempt)) * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// 409 Conflict encountered: Fetch remote state
		remoteCommitBlob, err := e.Client.GetBlob(ctx, e.VaultID, remoteHead)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to fetch remote conflict head %s: %w", remoteHead, err)
		}

		remoteCommitBytes, err := vault.DecryptPayload(remoteCommitBlob, e.VaultKey)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to decrypt remote commit: %w", err)
		}

		remoteCommit, err := vault.ParseCommit(remoteCommitBytes)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to parse remote commit: %w", err)
		}

		remoteTreeBlob, err := e.Client.GetBlob(ctx, e.VaultID, remoteCommit.TreeHash)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to fetch remote tree: %w", err)
		}

		remoteTreeBytes, err := vault.DecryptPayload(remoteTreeBlob, e.VaultKey)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to decrypt remote tree: %w", err)
		}

		remoteTree, err := vault.ParseTree(remoteTreeBytes)
		if err != nil {
			return nil, fmt.Errorf("sync: failed to parse remote tree: %w", err)
		}

		// Construct current local tree from files on disk
		localFiles, err := e.ScanBundle()
		if err != nil {
			return nil, err
		}

		localTree := vault.NewTree()
		blobsToUpload := make(map[string][]byte)
		var candidateHashes []string

		for path, content := range localFiles {
			plainHash := vault.HashPlaintext(content)
			envelopeBytes, err := vault.EncryptPayload(content, e.VaultKey)
			if err != nil {
				return nil, err
			}
			blobHash := vault.HashBlob(envelopeBytes)
			blobsToUpload[blobHash] = envelopeBytes
			candidateHashes = append(candidateHashes, blobHash)

			localTree.Entries[path] = vault.TreeEntry{
				BlobHash:      blobHash,
				Size:          int64(len(content)),
				PlaintextHash: plainHash,
				ModifiedAt:    time.Now().UTC().Format(time.RFC3339),
			}
		}

		// Reconcile base, local, and remote
		reconcileRes, err := Reconcile(e.cachedTree, localTree, remoteTree)
		if err != nil {
			return nil, fmt.Errorf("sync: reconciliation failed: %w", err)
		}

		// Auto-merge bookkeeping files: append-only log.md and per-listing index.md
		var unresolvedConflicts []Conflict
		for _, c := range reconcileRes.Conflicts {
			if c.LocalEntry.BlobHash != "" && c.RemoteEntry.BlobHash != "" {
				if mergedPlain, ok := e.mergeBookkeeping(ctx, c, localFiles[c.Path]); ok {
					mergedEnv, err := vault.EncryptPayload(mergedPlain, e.VaultKey)
					if err == nil {
						mergedBlobHash := vault.HashBlob(mergedEnv)
						blobsToUpload[mergedBlobHash] = mergedEnv
						candidateHashes = append(candidateHashes, mergedBlobHash)
						localFiles[c.Path] = mergedPlain

						reconcileRes.MergedTree.Entries[c.Path] = vault.TreeEntry{
							BlobHash:      mergedBlobHash,
							Size:          int64(len(mergedPlain)),
							PlaintextHash: vault.HashPlaintext(mergedPlain),
							ModifiedAt:    time.Now().UTC().Format(time.RFC3339),
						}
						continue
					}
				}
			}
			unresolvedConflicts = append(unresolvedConflicts, c)
		}
		reconcileRes.Conflicts = unresolvedConflicts

		if len(reconcileRes.Conflicts) > 0 {
			ApplyConflictFailsafe(reconcileRes.MergedTree, reconcileRes.Conflicts)
		}

		// Update local files with remote changes and conflict files
		diff := vault.DiffTrees(localTree, reconcileRes.MergedTree)
		toFetch := append(diff.Added, diff.Modified...)
		for _, path := range toFetch {
			entry := reconcileRes.MergedTree.Entries[path]
			var plainBytes []byte
			if localData, ok := localFiles[path]; ok && vault.HashPlaintext(localData) == entry.PlaintextHash {
				plainBytes = localData
			} else if envelopeData, ok := blobsToUpload[entry.BlobHash]; ok {
				var err error
				plainBytes, err = vault.DecryptPayload(envelopeData, e.VaultKey)
				if err != nil {
					return nil, fmt.Errorf("sync: failed to decrypt local blob for %s: %w", path, err)
				}
			} else {
				blobBytes, err := e.Client.GetBlob(ctx, e.VaultID, entry.BlobHash)
				if err != nil {
					return nil, fmt.Errorf("sync: failed to fetch blob %s for %s: %w", entry.BlobHash, path, err)
				}
				plainBytes, err = vault.DecryptPayload(blobBytes, e.VaultKey)
				if err != nil {
					return nil, fmt.Errorf("sync: failed to decrypt %s: %w", path, err)
				}
			}

			fullPath := filepath.Join(e.BundlePath, filepath.FromSlash(path))
			_ = os.MkdirAll(filepath.Dir(fullPath), 0o755)
			_ = os.WriteFile(fullPath, plainBytes, 0o644)
		}

		for _, path := range diff.Deleted {
			_ = os.Remove(filepath.Join(e.BundlePath, filepath.FromSlash(path)))
		}

		// Upload new blobs
		if len(candidateHashes) > 0 {
			missing, err := e.Client.CheckMissingBlobs(ctx, e.VaultID, candidateHashes)
			if err == nil {
				for _, h := range missing {
					if d, ok := blobsToUpload[h]; ok {
						_ = e.Client.PutBlob(ctx, e.VaultID, h, d)
					}
				}
			}
		}

		// Upload merged tree
		mergedTreeBytes, _ := reconcileRes.MergedTree.Serialize()
		mergedTreeEnv, _ := vault.EncryptPayload(mergedTreeBytes, e.VaultKey)
		mergedTreeHash := vault.HashBlob(mergedTreeEnv)
		_ = e.Client.PutBlob(ctx, e.VaultID, mergedTreeHash, mergedTreeEnv)

		// Create merge commit pointing to remoteHead
		mergeCommit := vault.NewCommit(&remoteHead, mergedTreeHash, author, "Reconcile merge: "+message)
		mergeCommitBytes, _ := mergeCommit.Serialize()
		mergeCommitEnv, _ := vault.EncryptPayload(mergeCommitBytes, e.VaultKey)
		mergeCommitHash := vault.HashBlob(mergeCommitEnv)
		_ = e.Client.PutBlob(ctx, e.VaultID, mergeCommitHash, mergeCommitEnv)

		resp, err := e.Client.Commit(ctx, e.VaultID, mergeCommitHash, &remoteHead)
		if err != nil {
			var raceErr *HeadConflictError
			if errors.As(err, &raceErr) {
				// Server head advanced during reconciliation, retry with new head
				remoteHead = raceErr.CurrentHead
				e.cachedTree = remoteTree
				continue
			}
			return nil, fmt.Errorf("sync: failed to commit merged head: %w", err)
		}

		e.cachedHead = resp.Head
		e.cachedTree = reconcileRes.MergedTree

		if err := e.saveState(); err != nil {
			return nil, fmt.Errorf("sync: merged commit %s was accepted by the hub, but persisting the sync state failed: %w", resp.Head, err)
		}

		return &SyncResult{
			CommitHash: resp.Head,
			Conflicts:  reconcileRes.Conflicts,
		}, nil
	}

	return nil, fmt.Errorf("sync: exceeded maximum reconciliation retries (%d)", DefaultMaxSyncRetries)
}

// fetchPlaintext downloads and decrypts a blob from the hub.
func (e *Engine) fetchPlaintext(ctx context.Context, blobHash string) ([]byte, error) {
	envelope, err := e.Client.GetBlob(ctx, e.VaultID, blobHash)
	if err != nil {
		return nil, err
	}
	return vault.DecryptPayload(envelope, e.VaultKey)
}

// mergeBookkeeping resolves a collision on a bookkeeping file that has a
// semantic merge: log.md (union of entries) or index.md (3-way merge per
// listing). It returns ok=false for any other file, or when the merge cannot
// be done safely, so the caller falls back to the collision failsafe.
func (e *Engine) mergeBookkeeping(ctx context.Context, c Conflict, localPlain []byte) ([]byte, bool) {
	switch {
	case isLogFile(c.Path):
		remotePlain, err := e.fetchPlaintext(ctx, c.RemoteEntry.BlobHash)
		if err != nil {
			return nil, false
		}
		merged, err := MergeLogContent(localPlain, remotePlain)
		return merged, err == nil

	case isIndexFile(c.Path):
		remotePlain, err := e.fetchPlaintext(ctx, c.RemoteEntry.BlobHash)
		if err != nil {
			return nil, false
		}
		var basePlain []byte
		if c.BaseEntry != nil && c.BaseEntry.BlobHash != "" {
			if basePlain, err = e.fetchPlaintext(ctx, c.BaseEntry.BlobHash); err != nil {
				return nil, false
			}
		}
		return MergeIndexContent(basePlain, localPlain, remotePlain)
	}
	return nil, false
}

func isLogFile(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == "log.md" || strings.HasSuffix(clean, "/log.md")
}
