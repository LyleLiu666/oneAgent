## 1. Implementation
- [x] 1.1 Pin both SDKs into the repo as fixed git submodules for the default path
- [x] 1.2 Point the default backend module resolution to pinned in-repo SDK snapshots instead of developer-local SDK directories
- [x] 1.3 Add an explicit `sdk-local` Docker override for local SDK联调
- [x] 1.4 Document the recommended `go.work` local override flow and simple Docker commands

## 2. Validation
- [x] 2.1 Verify default backend module resolution against pinned in-repo SDK snapshots
- [x] 2.2 Verify default Docker build/start with pinned SDK snapshots
- [x] 2.3 Verify `sdk-local` override config/build path still resolves local SDK directories
