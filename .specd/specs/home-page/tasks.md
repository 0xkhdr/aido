# Tasks — home-page

> Atomic DAG. Six required columns plus routing/trace/evidence intent.
> Stack: Go 1.26 + htmx + modernc.org/sqlite. Module `aido`.

| id | role | files | depends-on | verify | acceptance | refs | kind | risk | complexity | capabilities | context | evidence | checks |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| T1 | craftsman | internal/db/db.go | - | go test ./internal/db/... | R1.1,R1.2,R1.3,R1.4,R4.2,R4.3 | R1.1,R1.2,R1.3,R1.4,R4.2,R4.3 | feature | medium | standard | read,write | design.md#data-model, design.md#go-layer | tests | whitespace/empty project name rejected; CreateTask rejects empty title and missing/unknown project_id; migration adds project_id and backfills orphan tasks to default project; EnsureDefaultProject idempotent |
| T2 | craftsman | internal/handlers/handlers.go, main.go | T1 | go build ./... && go test ./internal/handlers/... | R1.2,R2.1,R2.2,R2.3,R3.2,R3.3,R3.4,R4.1 | R1,R1.2,R2,R2.1,R2.2,R2.3,R3,R3.2,R3.3,R3.4,R4,R4.1 | feature | medium | standard | read,write | design.md#go-layer, design.md#architecture | tests | select unknown/foreign project id rejected, active unchanged; empty composer submit creates no task; default project active when none selected; task list scoped to active project only |
| T3 | craftsman | internal/handlers/templates/index.html, internal/handlers/templates/sidebar.html, internal/handlers/templates/main.html, internal/handlers/templates/composer.html, internal/handlers/templates/list.html | T2 | go build ./... && go vet ./... | R2.1,R3.1,R3.5,R5.1,R5.2 | R2,R2.1,R3,R3.1,R3.5,R5,R5.1,R5.2 | feature | low | standard | read,write | design.md#templates, design.md#ui-ux | tests | two-pane layout renders; active project visibly marked in sidebar; composer centered and bound to active project; empty-state shown when project has no tasks; single shared style across panes |
| T4 | validator | internal/handlers/handlers.go | T3 | go build ./... && go run . & sleep 1; curl -sf localhost:8080/ >/dev/null; kill %1 | R2.1,R3.1,R3.5,R5.1 | R5,R5.1 | verification | low | standard | read | design.md#architecture | manual | home page boots and serves two-pane shell with sidebar, composer, and task list without error |

<!-- Field values example (not runnable): id=T<n>; role=craftsman; files=<paths>; depends-on=-; verify=<command>; acceptance=<criterion IDs>; refs=R1.1; kind=feature; risk=medium; complexity=standard; capabilities=read,write; context=<sources>; evidence=tests; checks=<negative/edge>. -->
