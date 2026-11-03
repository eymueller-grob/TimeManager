# TimeManager Copilot instructions

## Project and scope

- TimeManager is a Windows desktop app using Go, Wails v2, React, TypeScript,
  and Vite. It provides persistent project, timer, and booking workflows with
  Excel export.
- Stay on Wails v2. Do not introduce Wails v3 APIs or change frameworks without
  an explicit request.
- Make focused changes, preserve existing behavior, and update the README
  when setup, commands, or user-visible behavior changes.
- Follow the installed versions in `go.mod` and `frontend/package.json`.
  Do not upgrade dependencies or add packages without a task-specific need.

## Architecture and generated files

- `main.go` owns Wails options, lifecycle registration, embedded frontend
  assets, and the list of bound Go instances.
- `app.go` contains the bound `App` facade and converts domain timestamps to
  RFC3339 strings for typed bindings. Keep bridge methods small. Domain
  validation, storage, and export live in `internal/tracker/`.
- `tray_windows.go` owns the Windows colored icon, elapsed-time tooltip,
  minimize-to-tray behavior, and restore/stop/quit actions. Start native
  window interactions only after DOM readiness.
- `frontend/src/` owns the UI. Use the typed bindings from
  `frontend/wailsjs/go/` for Go calls and `frontend/wailsjs/runtime/` for Wails
  runtime APIs. Do not create a separate HTTP server for desktop bridge calls.
- Never manually edit generated `frontend/wailsjs/` files or
  `frontend/dist/`. Regenerate bindings with `wails dev` or `wails build`
  after changing bound methods or exported data types.
- Keep `//go:embed all:frontend/dist` intact. Build frontend assets before
  Go-only validation in a clean checkout.
- Preserve build resources under `build/`; production executables in
  `build/bin/` are generated and ignored.

## Go conventions

- Use idiomatic Go, `gofmt`, explicit types, and small packages. Prefer the
  standard library and existing dependencies.
- Use the startup context for Wails runtime calls; do not call runtime APIs
  before startup. Pass contexts to cancellable operations.
- Validate user inputs at the Go boundary. Return descriptive errors using
  `(Result, error)` or `error`, and wrap underlying errors with `%w` when
  adding context. Do not silently convert failures into successful results.
- Export only methods intended to be callable through the Wails bridge.
  Register new bound instances in `main.go` when necessary.
- Bound methods may run concurrently. Protect shared mutable state, avoid
  blocking UI operations, and clean up long-lived work on shutdown.
- Add table-driven Go tests alongside changed domain code, covering normal
  behavior, invalid inputs, and relevant boundary cases.
- Keep storage mutations locked, copy-on-write, and durable before emitting
  `data-changed`. A failed save must not change in-memory state.
- Preserve `%APPDATA%\TimeManager\data.json`; never reset invalid data. Keep
  schema versions explicit and implement migrations before changing the format.
- Schema version 4 allows repeated project numbers and requires globally unique
  project names (case-insensitive, trimmed), including archived projects.
  Migration from versions 1-3 suffixes duplicate names without changing IDs or time records.
  It stores reminder settings in addition to the appearance preference, shared property
  definitions, and per-project property values. Deep-copy maps and option
  slices in snapshots/mutations. Validate values by the definition's type.
  Deleting a definition removes values atomically, including archived projects.
- Run reminder scheduling and native popups only after DOM readiness. Reminders
  must work while hidden in the tray without stealing focus or changing timers.
  Reset cadence on timer/config changes, avoid missed-reminder bursts after sleep,
  and destroy native windows/timers on shutdown. Keep scheduler tests deterministic.
- Project names are unique, project numbers may repeat, parent cycles are forbidden, and archived
  projects cannot have active children or timers. Booked projects are
  archived rather than deleted; remove/reparent children first.
- One timer runs at a time. Switching must atomically save the old booking and
  persist the new start. Normal quit confirms stop/save; crash recovery retains
  the original start and includes downtime.
- Bookings must have a positive duration, cannot overlap, and manual bookings
  cannot end in the future or overlap the active timer.
- Excel date filters are inclusive local dates; use `AddDate`, not 24-hour
  arithmetic, for day boundaries. Clip crossing bookings, include archived
  projects, exclude active timers, and export numeric hours with a total.
- Export custom properties in definition order, with numeric and Boolean
  cells for those types, blank unset values, and literal text for text/dropdowns.

## React and TypeScript conventions

- Use functional components and hooks, strict TypeScript, and typed React
  events. Do not use `any`, unchecked casts, or suppressed diagnostics to
  bypass type errors.
- Keep domain logic in Go and presentation state in React. Do not add browser
  storage or replace the atomic JSON store with a database without a requirement.
- Preserve original RFC3339 timestamps when editing only booking metadata;
  JavaScript dates lose Go's submillisecond precision. Split overnight calendar
  entries by local-day boundaries without duplicating stored bookings.
- Await bridge promises, handle rejection as `unknown`, and show meaningful
  errors in the UI. Never swallow errors or fabricate successful results.
- Represent loading state and prevent duplicate submissions when appropriate.
  Clean up event subscriptions and timers in effects.
- Appearance preferences are saved through Go, not browser storage. Apply
  Light/Dark across all views, dialogs, and the native window; System follows
  `prefers-color-scheme` and cleans up its media-query listener.
- Use accessible controls, labels, keyboard interactions, and `role="alert"`
  for visible errors. Follow existing CSS and component conventions.
- Vite emits the frontend bundle; TypeScript uses `noEmit` and bundler module
  resolution. Keep both frontend and Vite configuration type-checked.

## Validation and development

Run commands from the project root unless specified otherwise:

```powershell
wails doctor
$env:WAILS_DEV_ALLOWED_HOSTS = 'wails.localhost'
wails dev
wails build
```

For a clean checkout, restore and build frontend assets first:

```powershell
Set-Location frontend
npm ci
npm test
npm run build
Set-Location ..
go test ./...
go vet ./...
```

- `npm run typecheck` inside `frontend` checks the UI and Vite configuration.
- `npm test` inside `frontend` runs the React component tests with Vitest.
- Run `gofmt` on edited Go files. Run relevant Go tests, frontend type checking,
  and `wails build` for changes affecting the bridge or desktop packaging.
- Put Go tests beside their packages and frontend component tests beside their
  components. Cover behavior, validation failures, and relevant boundary cases.
- There is no end-to-end test runner yet. Do not claim native desktop behavior
  is verified by frontend unit tests or a Vite preview.
- Frontend-only Vite preview does not provide the native Go bridge. Use
  `wails dev` for end-to-end desktop behavior.
- Never commit secrets, local toolchains, `node_modules`, or build output.
