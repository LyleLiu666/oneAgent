## 1. Implementation
- [x] 1.1 Add spec deltas for actionable skill.read not-found + skills help TurnContext
- [x] 1.2 Backend: improve `skill.read` not-found response (normalized id, suggestions<=5, next steps)
- [x] 1.3 Backend tests: add unit test for not-found + suggestions behavior
- [ ] 1.4 Backend: inject "技能帮助" TurnContext when user asks about available skills
- [ ] 1.5 Backend tests: add e2e test for skills help injection trigger
- [ ] 1.6 (Optional) Frontend: chat UI hints/link to `/governance/skills` when user asks about skills
- [ ] 1.7 Run `openspec validate fix-skill-read-not-found-ux --strict --no-interactive` and keep tests green
