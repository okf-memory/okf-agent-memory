---
type: Decision
title: Semantic Merge of Bookkeeping Files During Hub Sync
description: "During hub sync, log.md is merged by entry union and index.md by a 3-way merge per concept listing, so concurrent disjoint creates in one folder converge instead of colliding."
tags: [sync, hub, reconcile, index, log, merge]
generated: { by: agent/cli, at: "2026-10-14T00:00:00Z" }
status: stable
---

# Semantic Merge of Bookkeeping Files During Hub Sync

## Context
`okf create` and `okf update` rewrite the parent folder's `index.md`, which holds one listing line per concept (a bullet linking to the concept file, followed by its description), and append to `log.md`. The reconcile engine compares files as whole units, so two devices or agents that create *different* concepts in the same folder both change that folder's `index.md`. Only `log.md` had a semantic merge, so the shared index became a same-file collision: the remote index was adopted, the local one forked to `index.conflict-local.md`, and the canonical index silently stopped listing the local concept.

With many concurrent writers (multiple devices and agent harnesses), this bookkeeping collision is the most common conflict even when no concept file overlaps.

## Decision
`Sync` resolves collisions on bookkeeping files before applying the collision failsafe:

* **`log.md`**: union of entries grouped by date (existing `MergeLogContent`).
* **`index.md`**: `MergeIndexContent(base, local, remote)`, a 3-way merge that treats each listing line as an independent record keyed by its link target.
  * Additions and removals on either side are applied.
  * A one-sided update wins; an edit beats a concurrent deletion so no listing is lost.
  * When both sides edit the same listing differently, the remote listing wins, matching the failsafe's remote-wins rule.
  * Non-listing text (frontmatter, headings, prose) is merged as a whole; if both sides changed it differently the merge is refused and the regular `.conflict-local.md` failsafe applies.

The base version comes from the hub CAS using the conflict's base entry, which relies on the last-synced tree persisted in `<bundle>/.okf/sync-state.json` (#51, #53) so the base survives across CLI invocations.

## Consequences
`pkg/sync` stays independent of `pkg/okf`: the merge is purely line-based and does not parse concepts, and hand-written index text is preserved rather than regenerated.

# Related Concepts
- [Zero-Knowledge Vault Cryptography and Blind Sync Architecture](zero-knowledge-vault-sync.md): Extends the 3-way reconcile so bookkeeping files merge semantically instead of colliding
