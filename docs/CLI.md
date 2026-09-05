# OKF CLI & MCP Reference

The `okf` executable provides deterministic parsing, in-memory BM25 search, automated bookkeeping, and bundle validation for **Open Knowledge Format (OKF) v0.2** corpora.

---

## Global Usage

```bash
okf <command> [arguments] [flags]
```

### General Flags

* `--json`: Outputs machine-readable JSON instead of human-friendly terminal formatting.
* `--strict`: Evaluates connectivity warnings (orphans, broken relative links, provenance gaps) as fatal errors.
* `--drift`: Detects discrepancies between concept frontmatter descriptions and listings inside parent `index.md` files.

---

## Commands Reference

### 1. `validate`

Audits an OKF bundle for structural conformance, graph health, and provenance.

```bash
okf validate [bundle-path] [--strict] [--drift] [--json]
```

* **Arguments**:
  * `bundle-path` (optional, default: `./knowledge` or `.`): Path to the OKF bundle root directory.
* **Exit Codes**:
  * `0`: Valid & conformant.
  * `1`: Non-conformant or failed producer gate (`--strict`).
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

Searches concepts within a bundle using fast in-memory BM25 scoring across titles, descriptions, tags, IDs, and body text.

```bash
okf search <query> [bundle-path] [--limit <N>] [--json]
```

* **Arguments**:
  * `query` (required): Search terms or keywords.
  * `bundle-path` (optional, default: `./knowledge`).
* **Flags**:
  * `--limit <N>` (default: `10`): Maximum results to return.

#### Terminal Output Example:
```text
Found 2 matching concept(s) in 'knowledge':

 1. [6.45] architecture/layers (Architecture)
    Five-tier architecture separating specification, convention, skills, tooling, and corpus.
    Matches: title, description

 2. [3.12] project/overview (Project)
    Domain-neutral persistent project-memory system for AI agents and humans.
    Matches: body
```

---

### 3. `show`

Displays the full metadata, trust provenance, graph connections (inbound/outbound), and markdown body of a concept.

```bash
okf show <concept-id> [bundle-path] [--json] [--raw]
```

* **Arguments**:
  * `concept-id` (required): Bundle-relative path without `.md` (e.g. `architecture/layers`).
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
  * `--tags`: Comma-separated list of tags.
  * `--actor`: Author string (default: `agent/cli`).
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
  [--actor "agent/<model>"] \
  [--no-log] \
  [--no-index] \
  [--json]
```

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
| `okf_search` | `query` (string), `limit` (int) | Query memory corpus via BM25 ranking. |
| `okf_show` | `concept_id` (string) | Fetch concept frontmatter, body, and graph links. |
| `okf_create` | `id`, `type`, `title`, `description`, `body`, `tags` | Create concept with automatic index & log bookkeeping. |
| `okf_update` | `id`, `title`, `description`, `body` | Update existing concept and record in log.md. |
| `okf_relate` | `source_id`, `target_id`, `description` | Link two concepts together. |
| `okf_validate` | `strict` (bool), `drift` (bool) | Verify bundle conformance. |
