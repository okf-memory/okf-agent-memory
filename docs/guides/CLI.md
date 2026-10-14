# OKF CLI & MCP Reference

The `okf` executable provides deterministic parsing, in-memory BM25 search, automated bookkeeping, and bundle validation for **Open Knowledge Format (OKF) v0.2** corpora.

---

## Global Usage

```bash
okf <command> [arguments] [flags]
```

### General Flags

* `--json`: Outputs machine-readable JSON instead of human-friendly terminal formatting.
* `--strict`: Evaluates connectivity warnings (orphans, broken links) and provenance integrity (including superseded verifications `verified.at < generated.at`, missing actors, and v0.1 legacy syntax) as fatal gate errors.
* `--stale`: Evaluates expired lifecycle dates (`today >= stale_after`) as fatal errors. By default, stale concepts are reported as lifecycle warnings without failing `--strict`, separating CI build integrity from temporal review cycles.
* `--drift`: Detects discrepancies between concept frontmatter descriptions and parent `index.md` listings, and verifies that `code_refs` paths point to existing files/directories in the repository.

---

## Commands Reference

### 1. `validate`

Audits an OKF bundle for structural conformance, graph health, provenance integrity, and lifecycle status.

```bash
okf validate [bundle-path] [--strict] [--stale] [--drift] [--json]
```

* **Arguments**:
  * `bundle-path` (optional, default: `./knowledge` or `.`): Path to the OKF bundle root directory.
* **Flags**:
  * `--strict`: Fails the producer gate on broken links, orphans, superseded verifications, and schema discrepancies.
  * `--stale`: Fails the producer gate if any concept has reached or passed its `stale_after` date.
  * `--stale-within <duration>`: Fails the producer gate if any concept will expire within the given relative horizon (e.g. `14d`, `2w`, `3m`).
  * `--drift`: Checks whether concept listings in index files differ from concept descriptions, and verifies that `code_refs` point to valid source paths.
  * `--json`: Emits machine-readable JSON diagnostics.
* **Exit Codes**:
  * `0`: Valid & conformant (producer gate passed).
  * `1`: Non-conformant or failed producer gate (`--strict` / `--stale` / `--stale-within`).
  * `2`: File system or bundle loading error.

#### JSON Output Example:
```json
{
  "bundle_path": "knowledge",
  "declared_version": "0.2",
  "concept_count": 7,
  "errors": [],
  "warnings": [],
  "broken_links": [],
  "orphans": [],
  "stale_count": 0,
  "is_conformant": true,
  "gate_passed": true
}
```

---

### 2. `search`

Searches concepts using fast in-memory BM25 scoring across titles, descriptions, tags, IDs, and body text, filters by frontmatter metadata predicates, discovers concepts governing a specific file path via `code_refs`, or filters across discrete memory scopes.

```bash
okf search [query] [bundle-path] \
  [--scope <all|project|bundle|vendor|user|system>] \
  [--for-path <file-or-dir>] \
  [--filter <predicate>] \
  [--stale-within <duration>] \
  [--limit <N>] \
  [--json]
```

* **Arguments**:
  * `query` (optional when `--for-path`, `--filter`, or `--stale-within` is provided): Search terms or keywords.
  * `bundle-path` (optional, default: `./knowledge` or `.`).
* **Flags**:
  * `--scope <layer>` (default: `all`): Filter search to a specific memory layer:
    * `all`: Queries across Project, Vendor, User, and System layers with hierarchical shadowing and priority ranking.
    * `project` (alias `bundle`): Searches only the local project memory (`./knowledge/`).
    * `vendor`: Searches only installed external vendor bundles in `.okf/vendor/`.
    * `user`: Searches only personal developer memory in `~/.okf/` (or `$OKF_USER_DIR`).
    * `system`: Searches only enterprise/system memory in `/etc/okf/` (or `$OKF_SYSTEM_DIR`).
  * `--for-path <path>`: Filters concepts governing a specific source file or directory via `code_refs` (exact match, directory prefix, standard glob, or recursive `**` wildcard).
  * `--filter <expr>`: Filters concepts by frontmatter key-value predicates (supports `=`, `!=`, `null`/`nil` checks, and comma- or semicolon-separated clauses that must all match). Keys are the frontmatter field names exactly as written in the file (`tags`, `code_refs`, `description`, ...); there are no aliases, so `--filter desc=...` does not address the description even though `okf create --desc` and `okf update --desc` set it. `id` and `path` are also accepted; they are properties of the concept rather than frontmatter fields. Examples:
    * `--filter "type=Decision"`
    * `--filter "verified.by=human"`
    * `--filter "verified.by!=null,governance=constraint"`
    * `--filter "tags=security"`
    * `--filter "tags=ci,tags=ui"` (a comma starts a new clause, so repeat the key to require several tags; concepts must have all of them. `tags=ci,ui` is rejected because `ui` is not a clause. There is no OR operator.)
    * `--filter "topics=retrieval"` (any list-valued field, built-in or custom, matches when it contains the value; `topics!=retrieval` matches when it does not, and `topics=null` matches an empty list)
  * `--stale-within <duration>`: Filters concepts that are already stale or will expire within relative horizon (e.g. `14d`, `2w`, `3m`).
  * `--limit <N>` (default: `10`): Maximum results to return.
  * `--json`: Outputs machine-readable JSON array of matching concepts with governance tiers and matched fields.

#### Multi-Scope Priority Ranking & Shadowing

When searching with `--scope all`:
1. **Precedence Ranking**: Results are grouped by layer priority before BM25 score:
   - **`project`** (Priority 100) > **`vendor`** (Priority 70) > **`user`** (Priority 50) > **`system`** (Priority 10).
2. **Shadowing**: A concept in a higher layer strictly shadows identical concept IDs in lower layers (e.g. local `decisions/auth` overrides `@bundle/decisions/auth` or `user:decisions/auth`).

#### Governance Badges & Authority Ranking

Search results display an explicit governance badge indicating the concept's operational authority over code modifications:
* `[constraint]`: Mandatory rule/guardrail (e.g. pure Go stdlib, zero external dependencies).
* `[hold]`: Execution freeze / migration underway. Code under `code_refs` must NOT be edited without explicit human approval.
* `[context]`: Informative domain background.

When querying by `--for-path`, results are deterministically prioritized: `hold` concepts appear first (stop-the-line), followed by `constraint` (rules), then `context`.

#### Terminal Output Example:
```text
Found 2 matching concept(s) governing 'pkg/okf/types.go' in 'knowledge':

 1. [constraint] [22.00] architecture/governance-model (Decision)
    3-tier epistemic governance model (constraint, hold, context) and code-to-knowledge binding via code_refs.
    Matches: code_refs

 2. [context]    [0.31] architecture/layers (Architecture)
    Structural separation of concerns across the OKF specification, agent convention, skills, deterministic tooling, and knowledge corpus.
    Matches: title, description
```

---

### 3. `show`

Displays the full metadata, trust provenance, graph connections (inbound/outbound), and markdown body of a concept. Supports local concepts, external vendor packages, user memory, and system memory.

```bash
okf show <concept-id> [bundle-path] [--json] [--raw]
```

* **Arguments**:
  * `concept-id` (required): Unique concept identifier or scoped reference:
    * **Local Project Concept**: `decisions/auth` or `architecture/layers`
    * **Vendor Concept**: `@<bundle-id>/<concept-id>` (e.g. `@nextjs-15/decisions/routing` or `@peter/django-rules/decisions/auth`)
    * **User Concept**: `user:<concept-id>` (e.g. `user:guidelines/style`)
    * **System Concept**: `system:<concept-id>` (e.g. `system:corp/policies`)
    * **Canonical URN**: `okf://@nextjs-15/decisions/routing`, `okf://user/...`, `okf://system/...`
* **Flags**:
  * `--raw`: Emits the exact raw markdown file as stored on disk.
  * `--json`: Emits complete structured concept object.

---

### 4. `create`

Creates a new OKF concept file with valid frontmatter and automatically updates parent `index.md` and dated `log.md`.

```bash
okf create <concept-id> [bundle-path] \
  --type <Type> \
  --title "<Title>" \
  --desc "<One-sentence description>" \
  [--body "<Markdown body>"] \
  [--status draft|stable|deprecated] \
  [--tags "tag1,tag2"] \
  [--actor "agent/<model>"] \
  [--no-log] \
  [--no-index] \
  [--json]
```

* **Flags**:
  * `--type`: Normative OKF concept type (e.g. `Decision`, `Architecture`, `Fact`, `Entity`, `Runbook`).
  * `--title`: Human-readable concept title.
  * `--desc`: Exactly one concise sentence describing the concept.
  * `--body`: Markdown content following frontmatter.
  * `--status`: Lifecycle status (`draft`, `stable`, or `deprecated`; default `stable`).
  * `--tags`: Comma-separated list of tags.
  * `--actor`: Author string (default: `agent/cli`). An actor that is not human (`human`, `human:*`, `human/*`) cannot add human `verified` entries.
  * `--no-log`: Skips appending an entry to `log.md`.
  * `--no-index`: Skips updating the parent `index.md` listing.

---

### 5. `update`

Modifies an existing concept's metadata or body, updating timestamps and recording changes in `log.md`.

```bash
okf update <concept-id> [bundle-path] \
  [--title "<New Title>"] \
  [--desc "<Updated description>"] \
  [--body "<Updated body>"] \
  [--type <Type>] \
  [--status draft|stable|deprecated] \
  [--tags "tag1,tag2"] \
  [--actor "agent/<model>"] \
  [--no-log] \
  [--no-index] \
  [--json]
```

* **Flags**:
  * `--desc`: Updated one-sentence description.
  * `--title`: Updated concept title.
  * `--body`: Updated markdown body content.
  * `--type`: Updated non-empty concept type.
  * `--status`: Updated lifecycle status (`draft`, `stable`, or `deprecated`).
  * `--tags`: Replacement comma-separated tags (pass `--tags ""` to clear tags).
  * `--actor`: Author provenance identifier (default: `agent/cli`). An actor that is not human (`human`, `human:*`, `human/*`) can keep existing human `verified` entries but cannot add or change them.
  * `--no-log`: Skips appending an entry to `log.md`.
  * `--no-index`: Skips updating the parent `index.md` listing.
  * `--json`: Emit machine-readable JSON result.

Only supplied flags change the existing concept. `--type` requires a non-empty value, `--status` accepts only `draft`, `stable`, or `deprecated`, and `--tags` replaces the current tags after trimming each comma-separated value. Pass `--tags ""` to clear all tags; omit it to retain them.

---

### 6. `relate`

Adds a relative markdown link between two concepts, preventing link fragmentation and orphans.

```bash
okf relate <source-id> <target-id> [bundle-path] [--desc "<context>"] [--actor <actor>] [--json]
```

* **Example**:
  ```bash
  okf relate architecture/tooling architecture/layers knowledge --desc "Tooling implements the 5-layer architecture"
  ```

---

### 7. `init`

Initializes a bare OKF v0.2 bundle in a target directory with standard `index.md` (declaring `okf_version: "0.2"`) and `log.md`.

```bash
okf init [directory-path]
```

---

### 8. `bootstrap`

Scaffolds a complete agent memory stack into an existing or new project.

```bash
okf bootstrap [target-dir] \
  [--name "<Project Name>"] \
  [--overwrite-agents-md] \
  [--no-skill] \
  [--no-agents-md] \
  [--no-makefile] \
  [--no-bundle]
```

* **Flags**:
  * `--name`: Project name (defaults to directory name).
  * `--overwrite-agents-md`: Overwrite existing `AGENTS.md` instead of non-destructive smart-appending delimited section.
  * `--no-skill`: Skip installing `.agents/skills/okf-memory/`.
  * `--no-agents-md`: Skip creating/updating `AGENTS.md`.
  * `--no-makefile`: Skip installing convenience `Makefile`.
  * `--no-bundle`: Skip initializing `knowledge/` scaffold.

---

### 9. `mcp`

Runs an embedded Model Context Protocol (MCP) server over standard I/O (`stdio`).

```bash
okf mcp [bundle-path]
```

#### Exposed MCP Tools

| Tool Name | Parameters | Description |
| :--- | :--- | :--- |
| `okf_search` | `query` (string, opt), `for_path` (string, opt), `limit` (int) | Query memory corpus via BM25 ranking, or find concepts governing a file via `code_refs`. |
| `okf_show` | `concept_id` (string) | Fetch concept frontmatter, body, and graph links. |
| `okf_create` | `concept_id`, `type`, `title`, `description`, `body`, `status`, `tags` | Create concept with automatic index & log bookkeeping; status defaults to `stable`. |
| `okf_update` | `concept_id`, `type`, `status`, `tags`, `title`, `description`, `body` | Update only supplied fields and record in log.md; empty `tags` array clears tags. |
| `okf_relate` | `source_id`, `target_id`, `description` | Link two concepts together. |
| `okf_validate` | `strict` (bool), `drift` (bool) | Verify bundle conformance. |

MCP `tags` is an array of strings (max 50 chars per tag), such as `["auth", "security"]`. Both MCP tools accept lifecycle statuses `draft`, `stable`, and `deprecated`; updating with no `status` retains the existing value.

---

### 10. `hub`

Manages zero-knowledge end-to-end encrypted synchronization with the OKF Memory Hub and runs the embedded CAS server for self-hosting.

```bash
okf hub <subcommand> [arguments] [flags]
```

#### Authentication & URL Resolution
All hub commands (`push`, `pull`, `sync`, `init-vault`) support optional Bearer token authentication:
- **CLI Flag:** `-auth-token <token>` (highest precedence)
- **Environment Variable:** `OKF_HUB_TOKEN` (fallback)
- **Vault Configuration:** `auth_token` in `.okf-vault.json` (stored fallback)

The remote hub URL is resolved in order:
- **CLI Flag:** `-hub <url>`
- **Vault Configuration:** `hub_url` in `.okf-vault.json`
- **Default:** `http://127.0.0.1:8080`

#### Subcommands

##### A. `init-vault`
Initializes a new zero-knowledge vault, derives a 128-bit Secret Key, writes `.okf-vault.json`, and prints the Emergency Kit.

```bash
okf hub init-vault [bundle-path] [-hub <url>] [-auth-token <token>]
```

##### B. `push`
Detects local bundle changes via `plaintext_hash`, encrypts modified files into binary AES-256-GCM envelopes, deduplicates against remote CAS (`/blobs/check-missing`), uploads missing blobs, and advances the remote vault head.

```bash
okf hub push [bundle-path] [-hub <url>] [-auth-token <token>] [-password <pass>] [-secret-key <key>] [-message <msg>]
```

##### C. `pull`
Fetches the latest remote commit and tree manifest, downloads new ciphertext blobs from CAS, decrypts them locally into RAM, and updates files on disk.

```bash
okf hub pull [bundle-path] [-hub <url>] [-auth-token <token>] [-password <pass>] [-secret-key <key>]
```

##### D. `sync`
Performs a full two-way synchronization cycle (Pull + Push). If an HTTP 409 conflict occurs (concurrent updates), the 3-way reconcile engine automatically fast-forwards disjoint changes or preserves conflicting files locally as `<file>.conflict-local.md` without data loss.

Bookkeeping files are merged semantically instead of colliding: `log.md` entries are unioned, and each `index.md` is merged per listing (one `* [Title](file.md)` line per concept), so devices that add, update, or remove different concepts in the same folder converge cleanly. An index only falls back to a `.conflict-local.md` copy when both sides edited its non-listing text (headings or prose) differently; when both edited the same listing, the remote listing wins.

```bash
okf hub sync [bundle-path] [-hub <url>] [-auth-token <token>] [-password <pass>] [-secret-key <key>] [-message <msg>]
```

##### E. `serve`
Runs the embedded blind CAS and atomic head pointer server locally on the specified port.

```bash
okf hub serve [-port 8080] [-storage <dir>]
```

---

### 11. `pull`

Pulls and installs an external knowledge bundle into `.okf/vendor/<bundle-id>/` and updates `okf.lock`. If invoked without arguments, restores all bundles declared in `okf.lock`.

```bash
okf pull [<bundle-id|git-url>] [--registry <url>] [--force]
```

* **Arguments**:
  * `[<bundle-id|git-url>]`: Optional bundle ID (e.g. `nextjs-15`, `peter/django-5-rules`) or Git URL (e.g. `github.com/acme/agent-rules@v1.0.0`). When omitted, restores all bundles recorded in `okf.lock`.
* **Flags**:
  * `--registry <url>`: Override default registry endpoint (default: `https://registry.okf-memory.dev`).
  * `--force`: Overwrite existing vendor installation if already installed.

---

### 12. `restore`

Restores and verifies all vendor bundles declared in `okf.lock` into `.okf/vendor/<bundle-id>/`. Skips already installed bundles unless `--force` is specified.

```bash
okf restore [--registry <url>] [--force]
```

* **Flags**:
  * `--registry <url>`: Override default registry endpoint (default: `https://registry.okf-memory.dev`).
  * `--force`: Reinstall all bundles even if already present in `.okf/vendor/`.

---

### 13. `vendor`

Inspects and manages installed vendor bundles in `.okf/vendor/`.

```bash
okf vendor list
okf vendor remove <bundle-id>
```

* **`vendor remove <bundle-id>`**: Uninstalls the vendor bundle matching the exact `<bundle-id>`, removes its directory under `.okf/vendor/<bundle-id>/`, updates `okf.lock` (deleting `okf.lock` if no bundles remain), and prevents accidental deletion of namespace parent directories.

#### Multi-Scope Layering & Cross-Scope Linking

OKF Agent Memory organizes knowledge across four deterministic memory scopes. Higher layers strictly shadow identical concept IDs in lower layers during search, ensuring local project decisions always take precedence over external upstream packages or machine baselines:

| Scope | Link / Reference Syntax | Canonical URN | Storage Location | Priority | Precedence & Rules |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`project`** | `decisions/auth.md` | *(bundle relative)* | `./knowledge/` | **100** | **Local Project Memory.** Authoritative SSoT for current repository; strictly shadows identical IDs from vendor, user, and system layers. |
| **`vendor`** | `@nextjs-15/routing.md`<br/>`@peter/django-rules/auth.md` | `okf://@nextjs-15/routing`<br/>`okf://@peter/django-rules/auth` | `.okf/vendor/<bundle>/` | **70** | **External Packages.** Pinned dependencies pulled from OKF Registry (`registry.okf-memory.dev`) or Git. Shadows user and system. |
| **`user`** | `user:guidelines/style.md` | `okf://user/guidelines/style` | `~/.okf/` | **50** | **Personal Agent Memory.** Developer preferences and cross-project notes. Shadows system. |
| **`system`** | `system:corp/policies.md` | `okf://system/corp/policies` | `/etc/okf/` | **10** | **Enterprise / Machine Memory.** Infrastructure baselines and compliance policies. |

#### Resolution & Disambiguation Rules:
1. **Local vs. Vendor Disambiguation:**
   - Identifiers with leading `@` (e.g. `@nextjs-15/decisions/routing`) strictly resolve to `.okf/vendor/`.
   - Identifiers without `@` (e.g. `nextjs-15/decisions/routing` or `decisions/auth`) strictly resolve to the local project bundle (`knowledge/`).
2. **User & System URN Shorthands:**
   - `user:<path>` maps to `~/.okf/<path>` (or `$OKF_USER_DIR`).
   - `system:<path>` maps to `/etc/okf/<path>` (or `$OKF_SYSTEM_DIR`).
3. **Validator Immunity:**
   - Bundle validation (`okf validate --strict`) treats all external references (`@`, `user:`, `system:`, `okf://`, `https://`) as external targets. They never produce broken link errors or fail producer gates, and outbound external links prevent false-positive orphan detection.


