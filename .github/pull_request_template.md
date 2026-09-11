> [!IMPORTANT]
> **1. Target Branch:** All feature and bugfix pull requests must target the **`develop`** branch. Direct PRs to `main` are rejected.  
> **2. Issue-First Policy:** Pull requests must be preceded by an approved GitHub Issue. Unsolicited PRs or automated agent-generated PRs submitted without prior discussion will be closed.

## Related Issue
<!-- Mandatory: Link the approved issue this PR addresses (e.g. Fixes #123, Resolves #456). -->
<!-- Minor documentation typo fixes do not require an issue; please write "Minor doc fix". -->
Fixes #

## Description
<!-- Briefly describe the goal and changes introduced by this pull request. Do NOT leave this empty. -->

## Motivation & Context
<!-- Why is this change needed? What issue does it resolve? -->

## Changes Made
- [ ] 

## Verification & Pre-Submission Checklist
<!-- Please ensure all of the following checks pass locally: -->
- [ ] `make test` passes with 100% test success
- [ ] `make validate` passes on `knowledge/` (0 errors, 0 warnings, 0 orphans)
- [ ] `make validate-examples` passes on all sample corpora
- [ ] `make fmt` and `make vet` show clean code
- [ ] Any architectural or workflow changes are documented in `knowledge/`
