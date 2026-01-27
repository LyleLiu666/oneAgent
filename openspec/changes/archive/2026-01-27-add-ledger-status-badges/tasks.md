## 1. Specs
- [x] 1.1 Add OpenSpec delta for ledger status badge behavior

## 2. Backend
- [x] 2.1 Add store helpers: digest exists check + sop proposed count + learning job presence
- [x] 2.2 Add API handler: `GET /api/ledger/status/today`
- [x] 2.3 Add server route and API test

## 3. Frontend
- [x] 3.1 Add API client method for `GET /api/ledger/status/today`
- [x] 3.2 Show badges on Ledger tabs (digest/job/sop)
- [x] 3.3 Update/extend vitest coverage for badges

## 4. Validation
- [x] 4.1 `cd backend && go test ./...`
- [x] 4.2 `cd frontend && npm test -- --run`
- [x] 4.3 `scripts/e2e_smoke_test.sh`
- [x] 4.4 `openspec validate add-ledger-status-badges --strict --no-interactive`
