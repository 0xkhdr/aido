<!-- specd:managed:steering/tech.md:v1 begin -->
# Steering: Tech

## Stack
- **Language / runtime:** Go 1.26, stdlib only
- **Database:** SQLite (modernc.org/sqlite), WAL mode, foreign key constraints enabled
- **Build / test:** `go build ./...`, `go test ./...`
- **Dependencies:** modernc.org/sqlite (SQLite), nothing else at runtime
- **Deployment:** Single binary, containerized (Docker), compose for local dev

## Invariants (do not break without a recorded decision)
- Single static binary deployable; no separate runtime/config files
- SQLite database must survive container restarts (volume mount)
- All HTTP handlers must return appropriate status codes; /healthz always 200
- Projects and Tasks reference integrity: foreign key constraints enforced
- Embedded templates: no external HTML files at runtime
<!-- specd:managed:steering/tech.md:v1 end -->
