## 1. Specification
- [x] 1.1 Add `system-tool-permissions` spec + deltas for affected specs
- [x] 1.2 Run `openspec validate add-tool-permissions-system --strict --no-interactive`

## 2. Backend: Policy Engine
- [x] 2.1 Define policy model (principal/role/rule/constraints) + evaluator
- [x] 2.2 Persist policies (settingsdb/config) + hot reload (best-effort)
- [x] 2.3 Enforce policy in tool mounting (`tool.Mount/Infos`) and tool execution (toolxml + task runner)
- [x] 2.4 Add structured deny errors + audit logs (trace + server logs)

## 3. Backend: Identity / Multi-user
- [x] 3.1 Add multi-token support mapping `Authorization: Bearer <token>` → `principal_id`
- [x] 3.2 Add minimal admin APIs/CLI to manage tokens/users/roles (create/list/revoke)
- [x] 3.3 Propagate `principal_id` into task attempts and subagent runs

## 4. Command Tools Hardening
- [x] 4.1 Replace `ONEAGENT_BASH_ALLOW_RM` with policy-backed command profiles (e.g., readonly/dev/full)
- [x] 4.2 Prefer allowlist over blacklist for `bash`/`run_command` executables
- [x] 4.3 Add regression tests for bypass patterns (scripts/interpreters/indirect deletes)

## 5. UI / UX
- [x] 5.1 Add Tool Permissions workbench (view/edit policies; view effective permissions)
- [x] 5.2 Show active policy snapshot in session/task detail; guide user on denied tool requests

## 6. Cleanup
- [ ] 6.1 Remove `ONEAGENT_BASH_ALLOW_RM` references (code/tests/specs/docs/doctor)
- [ ] 6.2 Ensure e2e smoke tests remain green
