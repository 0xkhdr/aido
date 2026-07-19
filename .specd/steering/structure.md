<!-- specd:managed:steering/structure.md:v1 begin -->
# Steering: Structure

## Layout
- **main.go** — Server startup, signal handling, env config
- **internal/db/** — SQLite store, migrations, data models (Project, Task)
- **internal/handlers/** — HTTP handlers, template rendering, routes
- **internal/handlers/templates/** — Embedded HTML templates (index, fragments)
- **.specd/specs/** — Spec lifecycle (requirements, design, tasks, state)

## Naming & patterns
- Package names lowercase, plural for collections (db, handlers, templates)
- Store methods: `Open()`, `Close()`, `Migrate()`, then CRUD (Create*, Get*, Update*, Delete*)
- Handler methods prefixed with HTTP verb: `home`, `createProject`, `selectProject`, `toggleTask`
- Tests colocated: `*_test.go` in same package
- Error variables uppercase: `ErrEmptyName`, `ErrNoProject`, etc.

## Spec authoring format
- `design.md` decision contract: declare `references:`, `boundaries:`, `interfaces:`, `invariants:`, `failure:`, `integration:`, `alternatives:`, `disposition:`, `owner:`
- `tasks.md` full columns: `role`, `files`, `depends-on`, `verify`, `acceptance`, plus `refs`, `kind`, `risk`, `complexity`, `capabilities`, `context`, `evidence`, `checks`
<!-- specd:managed:steering/structure.md:v1 end -->
