# TimeManager

## About

Windows desktop app built with Wails v2, Go, React, and TypeScript.
Manage colored projects and subprojects, track time from favorites, maintain
a weekly booking calendar, and export saved time to Excel.
Customize its appearance and shared project properties in Settings.

## Using the app

- **Projects:** create and edit a project number, name, customer, parent, and
  color. Numbers may repeat; names must be unique across active and archived
  projects, ignoring capitalization and surrounding spaces. Add favorites
  using the star button, or create subprojects from an existing project.
- **Copy project:** click Copy in a project's row to open a prefilled project
  editor. Enter a new unique project name, adjust any details, and save.
  The copy retains the number, customer, color, favorite flag, custom properties,
  and active parent. An archived parent is cleared so you can choose an active
  parent instead. Copies are always active. Subprojects, bookings, and timers
  are not copied; Cancel leaves saved data unchanged.
- **Settings / Appearance:** choose Light, Dark, or Follow Windows/system.
  The preference is saved locally. System mode updates when the operating
  system's color preference changes. Existing installations initially retain
  the light appearance.
- **Settings / Timer reminders:** independently enable reminders for a running
  timer (default: every 60 minutes) and no running timer (default: every 30
  minutes). Enter positive whole-minute intervals and click Save reminders.
  Preview buttons show each message without changing the timer or saving settings.
  A small native window appears at the primary monitor's bottom-right, within
  its work area above the taskbar, including while the app is hidden in the tray.
  It does not steal keyboard focus. Close dismisses it; Open TimeManager
  restores the main window without stopping or starting a timer. Messages
  disappear automatically after 15 seconds and use the saved appearance.
  Only one message window is shown at a time.
  Countdown begins at app launch and restarts when a timer starts, stops, or
  switches, or reminder settings change. Closing a popup does not disable
  later reminders. Reminders run only while the app is open, and sleep/resume
  shows at most one overdue reminder rather than replaying missed intervals.
- **Settings / Project properties:** define shared optional Text, Number,
  Checkbox, or Dropdown fields, then fill their values in each project's
  editor. Subprojects have independent values; they do not inherit from their
  parent. Unset fields remain blank in exports; checkbox fields distinguish
  unset, Yes, and No. Numbers must be finite.
  Rename definitions or edit dropdown options in Settings. Existing field
  types cannot be changed. Remove or change project values before removing a
  dropdown option they use. Deleting a definition requires confirmation and
  removes its values from all projects, including archived ones.
- **Time & calendar:** click a favorite to start tracking. Clicking it again
  stops and saves the booking. Clicking a different project atomically saves
  the previous booking and starts the next timer. The start time and elapsed
  time are always shown above the views.
  The running project has a prominent RUNNING badge and highlighted favorite
  card and directory row. The timer panel is also emphasized in light and dark
  mode; project colors are retained. Highlights move when switching timers.
  Search favorite projects by number, name, customer, or parent hierarchy.
  Search is case-insensitive; Clear favorite search restores all favorites.
  Filtering only changes the displayed cards, never favorites or the running
  timer. The timer and Stop & save remain available even if its card is hidden.
  **Edit start time** changes the running timer's local start date/time without
  stopping it or changing its project. Earlier and later starts (including
  previous days) are allowed, but future times and overlaps with saved bookings
  are rejected. Changes persist immediately and update elapsed time, calendar,
  tray, and reminder cadence. Cancel leaves the original start unchanged.
- **Calendar:** navigate weeks or select a date. Add manual bookings, or click
  an existing booking to edit its project, start/end, and notes, or delete it.
  Bookings cannot overlap or end in the future. Overnight bookings appear on
  every day they cover. Daily/weekly totals exclude the running timer.
  Use **Calendar view → Daily project totals** for one entry per project per
  day showing combined booked time as hours:minutes:seconds, without individual
  booking times or notes. Projects and subprojects remain separate, archived
  projects are included, and overnight bookings count only their portion on
  each local day. Running timers are excluded. Switch back to **Booking details**
  to edit individual bookings. This view does not change stored bookings.
- **Temporary tracking:** click **Start temporary timer** in Time & calendar or
  the Windows tray to track time before knowing its final project. This creates
  one reusable **Temporary / Unassigned** project (number `TEMP`, customer
  `Unassigned`) on first use and atomically saves any previous timer. Clicking
  again while temporary tracking is running keeps its original start time.
  Stopping, switching projects, or quitting requires choosing an active project
  in **Assign temporary time**. Tray actions restore the app for this prompt.
  The timer keeps running until confirmation; Cancel keeps tracking without
  saving or switching. If there are no other projects, cancel and create one
  first. Assignment and any requested switch are saved atomically; failures
  leave the timer unchanged and show an error. Saved bookings can still be
  edited individually later without changing their times or notes.
  The temporary project can be edited
  in Projects, and later quick-starts reuse it even after renaming. If archived,
  restore it first; if deleted when unused, it is recreated on the next start.
  Existing projects with the default name are not reused or overwritten; a unique
  numbered name is chosen. No storage format change is required.
- **Archive/delete:** unused projects are permanently deleted; projects with
  bookings are archived and remain available in historical bookings and exports.
  Show archived projects to restore them. Reparent or remove child projects
  before deleting/archiving their parent. Stop a project's timer first.
- **Export to Excel:** select inclusive From/Through dates and choose an
  `.xlsx` file. The workbook includes customer, project number, full project
  hierarchy, local start/end, decimal hours, notes, project color, and a total.
  Each custom property adds a `Property: <name>` column. Number values are
  numeric Excel cells and checkbox values are Boolean cells; unset values
  are blank. Text values are exported as text, not formulas.
  Bookings crossing filter boundaries are clipped to the chosen local dates.
  Archived projects are included; an active timer is not included until stopped.
  The additional **Daily project totals** sheet combines bookings into one row
  per project per local day, with date, customer, project number, full hierarchy,
  numeric decimal hours, each custom property, and a grand total. Property
  columns use the same order, cell types, and blank-value handling as the
  detailed sheet. It uses the same date filter,
  splits overnight bookings at local midnight (including daylight-saving
  changes), and keeps projects and subprojects separate.
- **Windows tray:** minimizing hides the window to a project-colored play icon
  while running, or a gray stop-square icon when idle. Hover for the project name
  first, running/stopped state, live elapsed time, and project number. The same
  status appears in the tray menu; no extra status window is opened. Click
  for Open, Stop and save, **Change project**, and Quit (last).
  Change project offers three direct navigation routes to non-archived favorites:
  **All favorites → project**, **By customer → customer → project**, and
  **By top-level project → parent → project**. Entries show number, name, and customer. Choosing
  one atomically saves the current timer and starts that project without opening
  the window; if no timer is running, it starts the selected project. The running
  favorite is checked and disabled. Favorite/name changes refresh automatically.
  Top-level groups include favorite descendants at every nesting depth, even
  when the parent is not a favorite. Customer groups use each project's own
  customer. Only groups containing favorites are shown. These are navigation
  menus, not filters: expanding a group never changes the timer or saves a
  selection, and all routes remain available at once.
  **Change project → Search projects...** opens a small dedicated Windows
  search window without restoring the main app. Search all non-archived projects
  (including non-favorites) by number, name, customer, or parent hierarchy.
  Search ignores capitalization and matches all space-separated words.
  Use arrow keys and Enter, double-click a result, or click Switch project.
  Escape, Cancel, or closing the search window dismisses it without changing
  timers. Selecting the running project leaves its original start unchanged.
  Results refresh when projects change; save errors stay visible in the search
  window without discarding the running timer. Grouped menus remain favorites-only.
  Windows may place the icon in its
  hidden-icons overflow; move it into the visible tray in Windows settings.
  Windows tray icons do not support a permanently visible project-name or
  elapsed-time label beside the icon.
- **Close / Quit:** closing the window always asks whether to Run in background,
  Quit, or Cancel, even when no timer is running. Run in background hides the
  window to the tray without changing timers; reminders keep working.
  Cancel keeps the window open. Quit stops and saves any running timer before
  exiting; a save failure keeps the app open and shows the error.
  Tray Quit exits directly, also stopping and saving any running timer.

## Local data and recovery

Data is stored in `%APPDATA%\TimeManager\data.json`, separately for each Windows
user. No cloud account, network service, or browser storage is used. Changes
are written through a temporary file and atomically replace the saved file.
Write failures are shown and leave the previous state intact.

The active timer's start is persisted immediately, so it survives an unexpected
exit or restart. Elapsed time includes the period while the app/computer was
not running. Stop the timer and edit the resulting booking if that downtime
should not count. A normal quit stops and saves after confirmation.

Only one application instance is allowed. Back up `data.json` while no timer
is running and the app is closed. Invalid saved data is reported and never
silently reset or overwritten. Exported project/customer details use their
current values, including later project edits. Custom properties likewise use
current definitions and values, rather than historical snapshots.

Saved data uses schema version 4. Version 1, 2, and 3 files are validated and atomically
migrated on opening, preserving projects, bookings, custom properties, appearance,
and the original active timer start. Migration adds the default enabled 60/30-minute
reminders for version 1 and 2 files; existing version 3 reminder settings are retained.
Duplicate project names are automatically given a suffix such as ` (2)`, skipping
names already in use; the first occurrence keeps its name. Project IDs, numbers,
bookings, and active timers remain unchanged. Migration failures leave the original
file untouched. Keep a backup before upgrading; older versions of the app do not
support version 4.

## Prerequisites

- Go 1.26 or newer (see `go.mod`).
- Node.js 22.12 or newer on the 22.x line, or a newer supported LTS release,
  with npm. This project was initially built with Node.js 26 LTS.
- Wails v2 CLI matching the Go dependency:

  ```powershell
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
  wails doctor
  ```

- On Windows, the Microsoft Edge WebView2 runtime. `wails doctor` checks it.

Ensure Go, npm, and your Go binary directory (`$HOME\go\bin` by default) are on
`PATH`. The initial setup used portable Go and Node.js in
`$env:LOCALAPPDATA\TimeManagerToolchains` and added their binary directories to
the user `PATH`. Restart VS Code and open a new terminal to pick up these changes.

## Project layout

- `main.go`: desktop window, lifecycle hooks, bindings, and embedded assets.
- `app.go`: Go methods exposed to the frontend.
- `app_test.go`: Go bridge readiness tests.
- `internal/tracker/`: persistent project, timer, booking, and export logic with
  Go tests.
- `tray_windows.go`: Windows notification icon, live tooltip, and tray actions.
- `build/appicon.png` and `build/windows/icon.ico`: app and Windows installer icons.
- `reminder_windows.go`: focus-preserving native corner reminder window.
- `frontend/src/App.test.tsx`: workspace interaction tests.
- `frontend/src/SettingsView.tsx`: appearance and shared property editors.
- `frontend/src/`: React components, styles, and assets.
- `frontend/wailsjs/`: generated Go bindings and runtime helpers; do not edit.
- `wails.json`: Wails configuration and frontend build commands.
- `.github/copilot-instructions.md`: project-specific GitHub Copilot rules.

## Install and validate

From the project root, using PowerShell:

```powershell
Set-Location frontend
npm ci
npm test
npm run build
Set-Location ..
go test ./...
go vet ./...
wails build
```

Build the frontend before Go-only tests in a clean checkout because `main.go`
embeds `frontend/dist`. Use `npm run typecheck` inside `frontend` for a
type-check-only run.

## Live Development

Run these commands in the project root for a native window and frontend hot reload:

```powershell
$env:WAILS_DEV_ALLOWED_HOSTS = 'wails.localhost'
wails dev
```

Wails v2.17.1's development host guard requires explicitly trusting the native
WebView's `wails.localhost` loopback hostname. Do not use a wildcard or expose the
development server externally.

This also regenerates bindings for exposed Go methods. A standalone Vite preview
does not supply the Go bridge, so use Wails for end-to-end development.

## Building

Run `wails build` from the project root. On Windows, the executable is
`build/bin/TimeManager.exe`. Build resources under `build/` may be customized;
generated executables, frontend bundles, and dependencies are ignored by Git.

## Copilot rules

Repository-wide instructions live in
[`.github/copilot-instructions.md`](.github/copilot-instructions.md).
They cover architecture, generated files, Go conventions, strict TypeScript,
error handling, accessibility, and validation commands.
