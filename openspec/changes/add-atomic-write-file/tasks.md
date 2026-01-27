## 1. Implementation
- [ ] 1.1 Add `system-file-tools` spec delta for atomic overwrite (write temp + rename)
- [ ] 1.2 Implement shared atomic write helper (same-dir temp + rename; best-effort cleanup)
- [ ] 1.3 Migrate `write_file` overwrite path to atomic write helper
- [ ] 1.4 Migrate SBE actuator write-back path to atomic write helper
- [ ] 1.5 Add tests: overwrite atomicity, failure does not corrupt, temp cleanup
- [ ] 1.6 Run `openspec validate add-atomic-write-file --strict --no-interactive` and keep tests green

