## 1. Implementation
- [x] 1.1 Add spec deltas for diff review artifacts + follow-up attempt flow
- [x] 1.2 Backend: generate diff artifacts for attempts (git preferred; fallback to changed files list)
- [x] 1.3 Backend: persist review comments as attempt/receipt artifacts (append-only)
- [x] 1.4 Backend: allow creating follow-up attempts from succeeded tasks with user-provided review notes
- [x] 1.5 Backend tests: diff artifact presence + review→follow-up context injection
- [x] 1.6 Frontend: add Review UI (view diff + submit comments + create follow-up attempt)
- [x] 1.7 Frontend unit tests
- [x] 1.8 Run `openspec validate add-diff-review-loop --strict --no-interactive` and keep tests green
