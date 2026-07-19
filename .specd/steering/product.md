<!-- specd:managed:steering/product.md:v1 begin -->
# Steering: Product

## Thesis
- **What this product is:** Simple self-hosted project management and todo tracker
- **Who it is for:** Individual users and small teams managing lightweight task lists without external dependencies

## Principles
- **Simplicity first:** Stdlib + SQLite. No framework, no ORM, no build complexity
- **Durable storage:** Single-file SQLite with WAL mode; data survives restarts
- **Deployable:** Single static binary, containerized, configurable via env vars (DB_PATH, ADDR)
- **Constraints:** Does NOT include user auth, real-time sync, complex reporting, or multi-tenant isolation
- **Self-contained:** No external dependencies at runtime; embedded templates; no CDN or external assets

## Reference
specd's thesis: **Agent = Model + Harness.** The harness makes the plan safely delegable; every harness decision is deterministic and evidence-backed.
<!-- specd:managed:steering/product.md:v1 end -->
