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
- **Project icons:** in the project editor, enter an emoji or short text
  (press Win + . for the emoji picker) and click Use emoji, or click Upload
  image… to choose a PNG, JPEG, GIF, WebP, or SVG file (up to 5 MB). Emojis and
  text are trimmed to their visible pixels and enlarged to fill the icon. Icons
  already used by any project (including archived ones) appear under "Already
  used"; click one to reuse it, and hover to see which projects use it. Every icon
  is scaled to a 64 × 64 px PNG and saved with the project; SVG files are
  rasterized, so no scripts are stored. Icons appear in favorites, the
  directory, project pickers, the timer panel, bookings, and the tray. Remove
  icon restores the colored dot. After an upload, crop the image with the Zoom
  (100–500 %), Horizontal position, and Vertical position sliders, then click
  Apply crop, Use whole image (fits the image and adds transparent padding), or
  Cancel crop. Invert colors inverts the current icon's colors and keeps
  transparency. Copies keep the icon; new subprojects inherit the parent's color
  and icon.
- **Shared color and icon:** when related projects (the top-level project and
  all its subprojects, including archived ones) have a different color or
  icon, the editor shows "Apply color and icon to all related projects", which
  is checked by default. Saving with it checked gives the whole project tree
  the same color and icon in one atomic save. Uncheck it to change only the
  edited project.
- **Copy project:** click Copy in a project's row to open a prefilled project
  editor. Enter a new unique project name, adjust any details, and save.
  The copy retains the number, customer, color, favorite flag, custom properties,
  and active parent. An archived parent is cleared so you can choose an active
  parent instead. Copies are always active. Subprojects, bookings, and timers
  are not copied; Cancel leaves saved data unchanged.
- **Settings / Appearance:** choose Light, Dark, or Follow Windows/system.
  The preference is saved locally. System mode updates when the operating
  system's color preference changes. Existing installations initially retain
  the light appearance. Navigation selections use flat backgrounds, and panels,
  calendars, and dialogs use theme borders without decorative shadows.
- **Settings / Timer reminders:** independently enable reminders for a running
  timer (default: every 60 minutes) and no running timer (default: every 30
  minutes). Enter positive whole-minute intervals and click Save reminders.
  Preview buttons show each message without changing the timer or saving settings.
  A small card in the app's style (matching the saved appearance, with the
  running project's color, icon, and live elapsed time) appears at the primary monitor's bottom-right, within
  its work area above the taskbar, including while the app is hidden in the tray.
  It does not steal keyboard focus, even when clicked. Dismiss closes it; Open TimeManager
  restores the main window without stopping or starting a timer. Messages
  disappear automatically after 15 seconds and use the saved appearance.
  Only one message window is shown at a time.
  Countdown begins at app launch and restarts when a timer starts, stops, or
  switches, or reminder settings change. Closing a popup does not disable
  later reminders. Reminders run only while the app is open, and sleep/resume
  shows at most one overdue reminder rather than replaying missed intervals.
- **Settings / Project properties:** define shared optional Text, Number,
  Checkbox, Dropdown, or Dropdown (multiple selection) fields, then fill their
  values in each project's editor. Checkbox fields are real checkboxes;
  multiple-selection fields open a dropdown where any number of options can be
  checked (or cleared with **Clear selection**). Subprojects have independent values; they do not inherit from their
  parent. Unset fields remain blank in exports; checkbox fields distinguish
  unset, Yes, and No. Numbers must be finite.
  Rename definitions or edit dropdown options in Settings. Use **Move up** and
  **Move down** to reorder properties; the order is saved immediately and used
  in project forms, project details, and both Excel sheets. Existing field
  types cannot be changed. Remove or change project values before removing a
  dropdown option they use. Deleting a definition requires confirmation and
  removes its values from all projects, including archived ones.
- **Time & calendar:** click a favorite to start tracking. Clicking it again
  stops and saves the booking. Clicking a different project atomically saves
  the previous booking and starts the next timer. The start time and elapsed
  time are always shown above the views.
  The running project has a spinning indicator beside its name and in its
  RUNNING badge (static when reduced motion is requested), plus highlighted favorite
  card and directory row. The timer panel is also emphasized in light and dark
  mode; project colors are retained. Highlights move when switching timers.
  While a timer is running or paused, the timer panel stays pinned to the top
  of the window when scrolling, in every view. The page heading is a single
  compact line with the view name and Export to Excel.
  Search favorite projects by number, name, customer, or parent hierarchy.
  Search is case-insensitive; Clear favorite search restores all favorites.
  Filtering only changes the displayed cards, never favorites or the running
  timer. The timer and Stop & save remain available even if its card is hidden.
  **Pause** saves the current segment and stops elapsed time. **Resume** starts
  a separate booking segment for the same project, excluding the paused
  interval. The paused project is remembered across restarts; starting another
  project clears the paused state.
  **Edit start time** changes the running timer's local start date/time without
  stopping it or changing its project. Earlier and later starts (including
  previous days) are allowed, but future times and overlaps with saved bookings
  are rejected. Changes persist immediately and update elapsed time, calendar,
  tray, and reminder cadence. Cancel leaves the original start unchanged.
- **Calendar:** navigate weeks or select a date. Add manual bookings, or click
  an existing booking to edit its project, start/end, and notes, or delete it.
  Each booking card also has a trash can button in its top-right corner that
  deletes the booking after confirmation (overnight bookings are deleted as a
  whole, from any day they appear on).
  Bookings cannot overlap or end in the future. Overnight bookings appear on
  every day they cover. Daily/weekly totals exclude the running timer.
  Use **Calendar view → Daily project totals** for one entry per project per
  day showing combined booked time as decimal hours (for example `2.50 h`), without individual
  booking times or notes. Projects and subprojects remain separate, archived
  projects are included, and overnight bookings count only their portion on
  each local day. Running timers are excluded. Switch back to **Booking details**
  to edit individual bookings. This view does not change stored bookings.
  Each booking has an **Exported** checkbox in its editor; it can be set or
  cleared manually. The booking editor also shows the selected project's
  custom properties read-only (unset values show "Not set"); they follow the
  project selection and are edited only in the project editor. Exported bookings are gray but remain editable and count
  toward totals. A daily project total is gray only when all its contributing
  bookings are exported; mixed totals retain their project color.
  Both views display the calendar week as **CW45** beside the date range on
  the right, without a year prefix or space. Weeks run Monday through Sunday;
  week 1 contains January 4.
  Grayed-out buttons and controls show the reason on hover, for example
  "Bookings cannot be added for future days."
  All date selectors (calendar navigation, booking start/end, timer start, and
  Excel date filters) use the shadcn Calendar with a **CW** column on every
  ISO Monday-to-Sunday row, except the booking editor's Start/End, which show
  no calendar weeks. Click a date field to open its calendar; no
  separate calendar button is needed. Use arrows to move between dates,
  Home/End for the row, Page Up/Down for months, and Enter/Space to select.
  Escape dismisses the picker without closing the surrounding editor. Dates
  can also be typed as `YYYY-MM-DD`. Booking and timer editors show separate
  date and time controls (including seconds), rather than a combined timestamp.
  Selecting a day retains the time; editing time retains the date. Both controls
  are available without opening the calendar. The selected date's week is shown
  beside each field.
- **Temporary tracking:** click **Start temporary timer** in Time & calendar or
  the Windows tray to track time before knowing its final project. This creates
  one reusable **Temporary / Unassigned** project (number `TEMP`, customer
  `Unassigned`) on first use and atomically saves any previous timer. Clicking
  again while temporary tracking is running keeps its original start time.
  While temporary tracking is running, **Set project** in the timer panel assigns
  the entire running session to a project without stopping or resetting the
  timer, and without creating a booking. Cancel leaves it temporary.
  By default, stopping, pausing, switching projects, or quitting saves temporary
  time under the temporary project. In **Settings / Temporary timer**, choose
  **Require a project before saving** to prompt for an active project before
  any of these actions, including from the tray and project search. Confirming
  assigns and saves the segment atomically; pausing resumes on the chosen project.
  Cancel keeps the original timer running. Existing temporary bookings are unchanged.
  In **Assign temporary time**, **Cancel booking** instead stops and discards
  the current temporary session without saving it. Existing bookings are kept;
  pending project switches or quitting are canceled.
  Allowing temporary bookings also works
  from the tray and when no other project exists. Saved temporary bookings can
  be reassigned individually in the booking editor later, retaining their
  original timestamps and notes. Switching is atomic; save failures leave
  the timer unchanged and show an error.
  The temporary project can be edited
  in Projects, and later quick-starts reuse it even after renaming. If archived,
  restore it first; if deleted when unused, it is recreated on the next start.
  Existing projects with the default name are not reused or overwritten; a unique
  numbered name is chosen. Temporary timers can also be paused and resumed as
  separate booking segments.
- **Archive/delete:** unused projects are permanently deleted; projects with
  bookings are archived and remain available in historical bookings and exports.
  Show archived projects to restore them. Reparent or remove child projects
  before deleting/archiving their parent. Stop a project's timer first.
- **Export to Excel:** select inclusive From/Through dates and choose an
  `.xlsx` file. The workbook includes customer, project number, full project
  hierarchy, local start/end, decimal hours, notes, project color, and a total.
  Each custom property adds a `Property: <name>` column. Number values are
  numeric Excel cells and checkbox values are Boolean cells; multiple-selection
  values are exported as comma-separated text in option order; unset values
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
  Both Excel sheets include **CW** labels after the custom property columns,
  using ISO week numbering unchanged.
  Daily totals use the local day's week; detailed bookings show separate start
  and end weeks using their clipped local timestamps (an end exactly at Monday
  midnight therefore belongs to the new week).
  Export options (both off by default) let you **Exclude already exported
  bookings** from both sheets and **Mark fully included bookings as exported
  after saving**. Crossing/partially included bookings are not marked.
  Canceling or failing to save the workbook leaves statuses unchanged. If
  status saving fails after the Excel file is saved, the error identifies the
  saved file; no statuses are changed. Bookings edited or deleted during export
  prevent status marking rather than accidentally marking changed time.
- **Windows tray:** the app shows two full-size tray icons. The project icon
  shows the running (or paused) project's picture, or a circle in its color;
  it stays empty when nothing is tracked. The status icon shows a large green
  play, amber pause, or gray stop symbol without a background circle. Hover either icon for the project name
  first, running/stopped state, live elapsed time, and project number. The same
  status appears in the tray menu; no extra status window is opened. Click
  either icon for Open, Stop and save, **Change project**, and Quit (last).
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
  Directory, favorites, and tray searches also match assigned custom property
  names and values (text, numbers, dropdown selections, and checkbox values
  as Yes/No or true/false). Unset properties and parent properties are not
  inherited into search results. Existing archive and favorite scopes remain unchanged.
  Use arrow keys and Enter, double-click a result, or click Switch project.
  Escape, Cancel, or closing the search window dismisses it without changing
  timers. Selecting the running project leaves its original start unchanged.
  Results refresh when projects change; save errors stay visible in the search
  window without discarding the running timer. Grouped menus remain favorites-only.
  Windows may place the icons in its
  hidden-icons overflow; drag both into the visible tray and arrange them with
  the project icon on the left and the status icon on the right (Windows
  decides the initial order). Each tray icon is a fixed small square; Windows
  tray icons do not support a permanently visible project-name or
  elapsed-time label beside the icon.
- **Close / Quit:** closing the window always asks whether to Run in background,
  Quit, or Cancel, even when no timer is running. Run in background hides the
  window to the tray without changing timers; reminders keep working.
  Cancel keeps the window open. Quit stops and saves any running timer before
  exiting; a save failure keeps the app open and shows the error.
  Tray Quit opens the window and shows the same confirmation before exiting.

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

Saved data uses schema version 9, which adds the multiple-selection property
type (values stored as JSON lists of options). Version 8 added optional project icons stored as
PNG data URLs (at most 64 KB and 128 px per side; invalid icons are rejected
without rewriting the file). Version 1 through 8 files are validated and atomically
migrated on opening, preserving projects, bookings, custom properties, appearance,
the original active timer start, and exported-booking status. Migration defaults to allowing temporary bookings.
Migration adds the default enabled 60/30-minute
reminders for version 1 and 2 files; existing version 3 reminder settings are retained.
For versions 1-3, duplicate project names are automatically given a suffix such as ` (2)`, skipping
names already in use; the first occurrence keeps its name. Project IDs, numbers,
bookings, and active timers remain unchanged. Migration failures leave the original
file untouched. Keep a backup before upgrading; older versions of the app do not
support version 9. Existing version 1-4 bookings migrate with Exported off; their
timestamps and other metadata are preserved.

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
- `tray_windows.go`: Windows notification icons, live tooltip, and tray actions.
- `tray_state_windows.go`: second (status) notification icon sharing the tray menu.
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

To build and package a release, run `.\scripts\package.ps1`. It runs
`wails build` and zips the executable to `build/bin/TimeManager_<version>.zip`,
using the version from `wails.json`.

The app version is defined once as `info.productVersion` in `wails.json`.
Wails embeds it in the executable's Windows version resource, and Vite injects
it into the UI, where it appears under **Settings > About**. Update that value
before building a release.

## Copilot rules

Repository-wide instructions live in
[`.github/copilot-instructions.md`](.github/copilot-instructions.md).
They cover architecture, generated files, Go conventions, strict TypeScript,
error handling, accessibility, and validation commands.
