#!/usr/bin/env bash
set -euo pipefail

# jules-flow.sh: Automate discovery, isolated verification, and merging of Google Jules security audit PRs/branches.

ACTION="${1:-review}"
REQUESTED_BRANCH="${2:-}"

get_jules_branches() {
	git branch -r --list "origin/security-audit-*" "origin/jules-*" "origin/*jules*" | tr -d ' ' | grep -v '^$' | sort -u || true
}

resolve_target_branch() {
	git fetch origin --prune 2>/dev/null || true

	if [ -n "$REQUESTED_BRANCH" ]; then
		local norm="$REQUESTED_BRANCH"
		if [[ "$norm" != origin/* ]]; then
			norm="origin/$norm"
		fi
		if ! git rev-parse --verify "$norm" >/dev/null 2>&1; then
			echo "[!] Specified Jules branch '$REQUESTED_BRANCH' (resolved as '$norm') not found." >&2
			exit 1
		fi
		echo "$norm"
		return 0
	fi

	local branches
	branches=$(get_jules_branches)
	if [ -z "$branches" ]; then
		echo ""
		return 0
	fi

	local count
	count=$(echo "$branches" | grep -c . || true)

	local latest
	latest=$(echo "$branches" | sort -V | tail -n 1)

	if [ "$count" -gt 1 ]; then
		echo "==> Note: Found $count open Jules remediation branches on origin:" >&2
		while IFS= read -r b; do
			if [ "$b" = "$latest" ]; then
				echo "  * $b (selected latest)" >&2
			else
				echo "    $b" >&2
			fi
		done <<< "$branches"
		echo "Tip: Pass BRANCH=<name> to review or merge a specific branch." >&2
		echo "" >&2
	fi

	echo "$latest"
}

case "$ACTION" in
	list)
		echo "==> Open Google Jules security audit branches on origin:"
		git fetch origin --prune 2>/dev/null || true
		branches=$(get_jules_branches)
		if [ -z "$branches" ]; then
			echo "None found."
		else
			echo "$branches"
		fi
		;;

	review)
		BRANCH=$(resolve_target_branch)
		if [ -z "$BRANCH" ]; then
			echo "[✔] No pending Jules security remediation branch found on origin."
			exit 0
		fi

		echo "==> Target Jules remediation branch: $BRANCH"
		echo ""
		echo "--- Commit Details ---"
		git log -n 1 --stat "$BRANCH"
		echo ""
		echo "--- Summary Diff against develop ---"
		git diff --stat develop..."$BRANCH"
		echo ""

		WORKTREE_DIR=$(mktemp -d -t jules-review-XXXXXX)
		cleanup() {
			git worktree remove --force "$WORKTREE_DIR" 2>/dev/null || true
			rm -rf "$WORKTREE_DIR"
		}
		trap cleanup EXIT

		echo "==> Running isolated verification in temporary worktree..."
		git worktree add -q --detach "$WORKTREE_DIR" "$BRANCH"
		(
			cd "$WORKTREE_DIR"
			make check
		)
		echo ""
		echo "[✔] Isolated verification passed for $BRANCH!"
		SHORT_NAME="${BRANCH#origin/}"
		echo "To integrate into develop: run 'make jules-merge BRANCH=$SHORT_NAME'"
		;;

	merge)
		BRANCH=$(resolve_target_branch)
		if [ -z "$BRANCH" ]; then
			echo "[!] No pending Jules remediation branch found on origin."
			exit 1
		fi

		CURRENT_BRANCH=$(git branch --show-current)
		if [ "$CURRENT_BRANCH" != "develop" ]; then
			echo "[!] Must be on 'develop' branch to merge Jules remediation. Currently on '$CURRENT_BRANCH'."
			exit 1
		fi

		SHORT_NAME="${BRANCH#origin/}"
		echo "==> Merging $BRANCH into develop..."
		git merge --no-ff "$BRANCH" -m "merge: incorporate Jules security remediation ($SHORT_NAME)"

		echo "==> Running make check on merged develop..."
		make check

		echo ""
		echo "[✔] Successfully merged $BRANCH into develop."
		echo "To push develop:                 git push origin develop"
		echo "To delete remote Jules branch:   git push origin --delete $SHORT_NAME"
		;;

	*)
		echo "Usage: $0 {list|review [branch]|merge [branch]}"
		exit 1
		;;
esac
