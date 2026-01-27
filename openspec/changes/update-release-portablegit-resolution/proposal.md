# Change: Make PortableGit resolution reproducible and testable

## Why
Windows release bundles can include PortableGit (Git Bash) to deliver consistent UX.

Current risk areas:
- Resolving the PortableGit download URL depends on GitHub API rate limits and “latest” moving target.
- The release script is hard to unit-test because it executes immediately when sourced.

## What Changes
- Allow pinning PortableGit resolution via a file path (optional).
- Refactor `scripts/release_local.sh` so it can be sourced in “library mode” for fast self-tests.
- Add a lightweight self-test script and run it in CI.

## Impact
- Affected specs: `project-tooling` (release workflow robustness)
- Affected code:
  - `scripts/release_local.sh`
  - `scripts/release_local_selftest.sh`
  - `scripts/ci_build.sh`

