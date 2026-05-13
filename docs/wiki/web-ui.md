# Web UI

## How the embedded server works

The web server is implemented in `internal/web/server.go`. It uses `//go:embed static` to compile the entire `internal/web/static/` directory (HTML, CSS, JS) into the binary. No external static files are needed at runtime.

```go
//go:embed static
var staticFiles embed.FS
```

The server is started with `backlog web` (default port 8080). It auto-opens the browser unless `--no-browser` is passed. The `web.New(db, actor)` constructor takes the open database connection and the resolved actor, and wires all routes.

## Static file serving and SPA routing

- `/static/` — serves the embedded `static/` directory verbatim (CSS, JS, images)
- `/` — serves `static/index.html` for the SPA root; any other path returns 404

The SPA handles all navigation client-side via a `navigate()` function that updates `state.page` and calls `render()`. There is no server-side routing beyond the two rules above.

## API surface

All API endpoints are under `/api/`. Responses are JSON. Errors return `{"error": "..."}` with an appropriate HTTP status code.

**Full API reference:** [website/api.html](https://mazen160.github.io/backlog/api.html). It documents every route registered in `routes()` (projects, tasks, labels, plans, docs, memory, attachments, activity), with request bodies, response shapes, error cases, and curl examples. Treat the section below as a navigation aid only.

| Resource | Routes |
|---|---|
| Projects | `GET/POST /api/projects`, `PATCH /api/projects/{alias}/{archive\|unarchive}`, `DELETE /api/projects/{alias}` |
| Tasks | `GET/POST /api/tasks`, `GET/PATCH /api/tasks/{id}`, `PATCH /api/tasks/{id}/{status\|archive\|unarchive}`, `POST /api/tasks/{id}/{comments\|plans\|labels}`, `DELETE /api/tasks/{id}/labels/{name}` |
| Labels | `GET/POST /api/labels` |
| Plans | `GET /api/plans`, `GET/PATCH /api/plans/{id}`, `GET /api/plans/{id}/history` |
| Docs | `GET/POST /api/docs`, `GET/PATCH/DELETE /api/docs/{id}`, `GET /api/docs/{id}/history` |
| Memory | `GET/POST /api/memory`, `PATCH/DELETE /api/memory/{id}`, `PATCH /api/memory/{id}/append` |
| Attachments | `GET/POST /api/attachments`, `GET /api/attachments/{id}` |
| Activity | `GET /api/activity` |

## SPA architecture

The SPA is implemented as vanilla JavaScript in `internal/web/static/app.js`. There are no build steps, no bundler, and no framework dependencies (except EasyMDE for the markdown editor).

**Global state object**

```js
const state = {
  page: 'tasks',       // current view
  project: '',         // selected project alias
  projects: [],        // loaded project list
  tasks: [],           // loaded task list
  filters: { search, status, type, priority },
  sorts: [{ field, dir }],
  taskView: 'list',    // 'list' | 'board' | 'grid'
  kanbanGroupBy: 'status',
  activityKindFilter: '',
  // …
};
```

**`render()` dispatch**

`render()` reads `state.page` and calls the appropriate rendering function:

```
'tasks'       → renderTasks()
'task-detail' → renderTaskDetail()
'docs'        → renderDocs()
'memory'      → renderMemory()
'activity'    → renderActivity()
'projects'    → renderProjects()
'attachments' → renderAttachments()
'plans'       → renderPlans()
```

**`navigate(page, stateOverrides)`**

Sets `state.page` (and any additional state keys), updates the active sidebar item, and calls `render()`. Does not change the browser URL (no history API).

**API client**

A small `api` object wraps `fetch()` and provides `api.get(path)`, `api.post(path, body)`, `api.patch(path, body)`, and `api.del(path)`. All methods prepend `/api/` and parse the response as JSON.

**Markdown editor**

EasyMDE is loaded from a CDN and wrapped by `createMDE(textareaEl, opts)`. It is used in the new-task modal, plan editor, doc editor, and memory editor.

## View modes

The task list supports three view modes, toggled by buttons in the toolbar:

**List** (default) — standard table view. Rows show task ref, title, type badge, priority, status, and assignee. Clicking a row navigates to task detail.

**Board (Kanban)** — columnar card view. The `kanbanGroupBy` state controls how columns are generated:
- `status` — three columns: todo, doing, done
- `priority` — five columns: P1–P5
- `type` — one column per task type present in current filter
- `assignee` — one column per distinct assignee

Cards can be moved between columns with left/right arrow buttons. The move calls `PATCH /api/tasks/{id}/status` or `PATCH /api/tasks/{id}` depending on the groupBy dimension.

**Grid** — Notion-style inline-editable table. Every cell (title, type, priority, assignee, status) is editable in place by clicking. Changes are sent to `PATCH /api/tasks/{id}` immediately on blur or Enter.

## Activity timeline

The Activity page fetches from `GET /api/activity` filtered by the selected project and optionally by entity kind. Events are rendered newest-first as a vertical timeline showing timestamp, entity kind, summary, and actor.

The page does not auto-refresh — it loads once on navigation.

## Phosphor design system

The UI uses a custom design language defined in `internal/web/static/style.css`:

- **Typography**: IBM Plex Mono for all text (monospace throughout)
- **Color palette**: CSS variables — `--bg` (dark), `--surface` (card background), `--border`, `--text`, `--text-muted`, `--accent` (amber `#f59e0b`)
- **Scanline texture**: a subtle repeating CSS `background-image` gradient on `body` gives a CRT-like texture
- **Components**: `.card`, `.btn`, `.btn-primary`, `.btn-ghost`, `.badge`, `.tag-chip`, `.filter-select`, `.search-input`, `.modal`, `.modal-overlay`
- **Priority colors**: CSS classes `.priority-1` through `.priority-5` with distinct background colors
- **Status badges**: `.badge-todo`, `.badge-doing`, `.badge-done`

The terminal command bar at the bottom of the screen shows the equivalent CLI command for the current view state, updated on every `render()` call.
