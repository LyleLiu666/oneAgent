## 1. Implementation
- [ ] 1.1 Add `system-file-tools` spec delta for `read_file`
- [ ] 1.2 Implement `read_file` tool (paged + byte-capped) and register it
- [ ] 1.3 Add tests: small file, large file paging, max_bytes truncation, path rules (workspace on/off)
- [ ] 1.4 Update docs / tool manuals (best-effort guidance for chunked reading)
- [ ] 1.5 Run `openspec validate add-read-file-tool --strict --no-interactive` and keep tests green

