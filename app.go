package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"TimeManager/internal/tracker"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx                context.Context
	store              *tracker.Store
	initErr            error
	closeTray          func()
	closeReminders     func()
	closeProjectSearch func()
	openProjectSearch  func() error
	previewReminder    func(bool) error
	quitRequested      atomic.Bool
}

type Booking struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Notes     string `json:"notes"`
	Exported  bool   `json:"exported"`
}

type Timer struct {
	ProjectID string `json:"projectId"`
	Start     string `json:"start"`
}

type State struct {
	Projects            []tracker.Project            `json:"projects"`
	Bookings            []Booking                    `json:"bookings"`
	Active              *Timer                       `json:"active"`
	PausedProjectID     string                       `json:"pausedProjectId"`
	Settings            tracker.Settings             `json:"settings"`
	PropertyDefinitions []tracker.PropertyDefinition `json:"propertyDefinitions"`
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dir, err := os.UserConfigDir()
	if err == nil {
		a.store, err = tracker.Open(filepath.Join(dir, "TimeManager", "data.json"))
	}
	a.initErr = err
}

func (a *App) domReady(ctx context.Context) {
	if a.initErr != nil {
		runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type: runtime.ErrorDialog, Title: "Cannot open TimeManager data",
			Message: a.initErr.Error() + "\nYour saved file has not been overwritten.",
		})
		return
	}
	closeSearch, openSearch, err := a.startProjectSearch()
	if err != nil {
		a.showError(err)
		return
	}
	a.closeProjectSearch, a.openProjectSearch = closeSearch, openSearch
	a.closeTray = a.startTray()
	closeReminders, preview, err := a.startReminders()
	if err != nil {
		a.showError(err)
		return
	}
	a.closeReminders, a.previewReminder = closeReminders, preview
}

func (a *App) shutdown(ctx context.Context) {
	if a.closeProjectSearch != nil {
		a.closeProjectSearch()
	}
	if a.closeReminders != nil {
		a.closeReminders()
	}
	if a.closeTray != nil {
		a.closeTray()
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.store == nil {
		return false
	}
	if a.quitRequested.Swap(false) && a.store.Snapshot().Active == nil {
		return false
	}
	runtime.EventsEmit(ctx, "close-requested")
	return true
}

func (a *App) RunInBackground() error {
	if err := a.check(); err != nil {
		return err
	}
	runtime.WindowHide(a.ctx)
	return nil
}

func (a *App) QuitApplication() error {
	if err := a.check(); err != nil {
		return err
	}
	if a.store.Snapshot().Active != nil {
		if err := a.timerAction("quit", ""); err != nil {
			return fmt.Errorf("save timer before quitting: %w", err)
		}
		if a.store.Snapshot().Active != nil {
			return nil
		}
	}
	a.quitRequested.Store(true)
	runtime.Quit(a.ctx)
	return nil
}

func (a *App) check() error {
	if a.initErr != nil {
		return a.initErr
	}
	if a.store == nil {
		return errors.New("data store is not ready")
	}
	return nil
}

func (a *App) GetState() (State, error) {
	if err := a.check(); err != nil {
		return State{}, err
	}
	d := a.store.Snapshot()
	result := State{Projects: d.Projects, Bookings: []Booking{}, PausedProjectID: d.PausedProjectID, Settings: d.Settings, PropertyDefinitions: d.PropertyDefinitions}
	for _, b := range d.Bookings {
		result.Bookings = append(result.Bookings, Booking{ID: b.ID, ProjectID: b.ProjectID,
			Start: b.Start.Format(time.RFC3339Nano), End: b.End.Format(time.RFC3339Nano), Notes: b.Notes, Exported: b.Exported})
	}
	if d.Active != nil {
		result.Active = &Timer{ProjectID: d.Active.ProjectID, Start: d.Active.Start.Format(time.RFC3339Nano)}
	}
	return result, nil
}

func (a *App) changed(f func() error) error {
	if err := a.check(); err != nil {
		return err
	}
	if err := f(); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "data-changed")
	return nil
}

func (a *App) StartTemporaryTimer() error {
	return a.timerAction("temporary", "")
}

func (a *App) SaveProject(p tracker.Project) error {
	return a.changed(func() error { return a.store.SaveProject(p) })
}

func (a *App) SaveSettings(settings tracker.Settings) error {
	return a.changed(func() error { return a.store.SaveSettings(settings) })
}

func (a *App) TestReminder(running bool) error {
	if err := a.check(); err != nil {
		return err
	}
	if a.previewReminder == nil {
		return errors.New("reminder window is not ready")
	}
	return a.previewReminder(running)
}

func (a *App) SavePropertyDefinition(def tracker.PropertyDefinition) error {
	return a.changed(func() error { return a.store.SavePropertyDefinition(def) })
}

func (a *App) DeletePropertyDefinition(id string) error {
	return a.changed(func() error { return a.store.DeletePropertyDefinition(id) })
}

func (a *App) MovePropertyDefinition(id string, direction int) error {
	return a.changed(func() error { return a.store.MovePropertyDefinition(id, direction) })
}

func (a *App) DeleteProject(id string) error {
	return a.changed(func() error { return a.store.DeleteProject(id) })
}

func (a *App) RestoreProject(id string) error {
	return a.changed(func() error { return a.store.RestoreProject(id) })
}

func (a *App) ToggleFavorite(id string) error {
	return a.changed(func() error { return a.store.Favorite(id) })
}

func (a *App) ToggleTimer(id string) error {
	return a.timerAction("toggle", id)
}

func (a *App) StopTimer() error {
	return a.timerAction("stop", "")
}

func (a *App) PauseTimer() error {
	return a.timerAction("pause", "")
}

func (a *App) ResumeTimer() error {
	return a.changed(func() error { return a.store.ResumeTimer() })
}

type TemporaryAssignment struct {
	Timer  Timer  `json:"timer"`
	Action string `json:"action"`
	ID     string `json:"id"`
}

func (a *App) timerAction(action, id string) error {
	err := a.changed(func() error {
		switch action {
		case "temporary":
			return a.store.StartTemporaryTimer()
		case "pause":
			return a.store.PauseTimer()
		case "stop", "quit":
			return a.store.StopTimer()
		case "toggle":
			return a.store.ToggleTimer(id)
		case "favorite":
			return a.store.StartFavoriteTimer(id)
		case "start":
			return a.store.StartTimer(id)
		default:
			return errors.New("unknown timer action")
		}
	})
	if errors.Is(err, tracker.ErrTemporaryProjectRequired) {
		d := a.store.Snapshot()
		if d.Active == nil {
			return err
		}
		a.restoreWindow()
		runtime.EventsEmit(a.ctx, "temporary-assignment-requested", TemporaryAssignment{
			Timer: Timer{ProjectID: d.Active.ProjectID, Start: d.Active.Start.Format(time.RFC3339Nano)}, Action: action, ID: id,
		})
		return nil
	}
	return err
}

func (a *App) DiscardTemporaryTimer(expected Timer) error {
	start, err := time.Parse(time.RFC3339Nano, expected.Start)
	if err != nil {
		return fmt.Errorf("invalid original timer start: %w", err)
	}
	return a.changed(func() error {
		return a.store.DiscardTemporaryTimer(tracker.Timer{ProjectID: expected.ProjectID, Start: start})
	})
}

func (a *App) AssignTemporaryTimer(request TemporaryAssignment, targetID string) error {
	start, err := time.Parse(time.RFC3339Nano, request.Timer.Start)
	if err != nil {
		return fmt.Errorf("invalid original timer start: %w", err)
	}
	nextID := ""
	switch request.Action {
	case "pause":
		return a.changed(func() error {
			return a.store.PauseTemporaryTimer(tracker.Timer{ProjectID: request.Timer.ProjectID, Start: start}, targetID)
		})
	case "assign-running":
		return a.changed(func() error {
			return a.store.AssignRunningTemporaryTimer(tracker.Timer{ProjectID: request.Timer.ProjectID, Start: start}, targetID)
		})
	case "stop", "quit":
	case "toggle", "favorite", "start":
		if request.ID != request.Timer.ProjectID {
			if request.ID == "" {
				return errors.New("next project is required")
			}
			nextID = request.ID
		}
	default:
		return errors.New("unknown timer action")
	}
	if err := a.changed(func() error {
		return a.store.FinishTemporaryTimer(tracker.Timer{ProjectID: request.Timer.ProjectID, Start: start}, targetID, nextID, request.Action == "favorite")
	}); err != nil {
		return err
	}
	if request.Action == "quit" {
		return a.QuitApplication()
	}
	return nil
}

func (a *App) UpdateTimerStart(expected Timer, start string) error {
	original, err := time.Parse(time.RFC3339Nano, expected.Start)
	if err != nil {
		return fmt.Errorf("invalid original timer start: %w", err)
	}
	next, err := time.Parse(time.RFC3339Nano, start)
	if err != nil {
		return fmt.Errorf("invalid timer start: %w", err)
	}
	return a.changed(func() error {
		return a.store.UpdateTimerStart(tracker.Timer{ProjectID: expected.ProjectID, Start: original}, next)
	})
}

func (a *App) SaveBooking(b Booking) error {
	start, err := time.Parse(time.RFC3339Nano, b.Start)
	if err != nil {
		return fmt.Errorf("invalid booking start: %w", err)
	}
	end, err := time.Parse(time.RFC3339Nano, b.End)
	if err != nil {
		return fmt.Errorf("invalid booking end: %w", err)
	}
	return a.changed(func() error {
		return a.store.SaveBooking(tracker.Booking{ID: b.ID, ProjectID: b.ProjectID, Start: start, End: end, Notes: b.Notes, Exported: b.Exported})
	})
}

func (a *App) DeleteBooking(id string) error {
	return a.changed(func() error { return a.store.DeleteBooking(id) })
}

type ExportOptions struct {
	MarkExported    bool `json:"markExported"`
	ExcludeExported bool `json:"excludeExported"`
}

func (a *App) ExportBookings(from, to string, options ExportOptions) (string, error) {
	if err := a.check(); err != nil {
		return "", err
	}
	selection, eligible, err := tracker.SelectExportBookings(a.store.Snapshot(), from, to, time.Local, options.ExcludeExported)
	if err != nil {
		return "", err
	}
	data, err := tracker.Export(selection, from, to, time.Local)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "Export bookings to Excel", DefaultFilename: fmt.Sprintf("TimeManager_%s_%s.xlsx", from, to),
		Filters: []runtime.FileFilter{{DisplayName: "Excel workbook (*.xlsx)", Pattern: "*.xlsx"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		path += ".xlsx"
	}
	if _, err := os.Stat(path); err == nil {
		choice, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type: runtime.QuestionDialog, Title: "Replace existing export?",
			Message: "Replace the existing file?\n" + path,
			Buttons: []string{"Yes", "No"}, DefaultButton: "No", CancelButton: "No",
		})
		if err != nil {
			return "", err
		}
		if choice != "Yes" {
			return "", nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check export destination: %w", err)
	}
	if err := tracker.WriteAtomic(path, data); err != nil {
		return "", fmt.Errorf("save Excel export: %w", err)
	}
	if options.MarkExported && len(eligible) > 0 {
		if err := a.changed(func() error { return a.store.MarkBookingsExported(eligible) }); err != nil {
			return "", fmt.Errorf("Excel file saved to %s, but exported status was not updated: %w", path, err)
		}
	}
	return path, nil
}

func (a *App) restoreWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *App) showError(err error) {
	a.restoreWindow()
	runtime.EventsEmit(a.ctx, "app-error", err.Error())
	runtime.LogError(a.ctx, err.Error())
	runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type: runtime.ErrorDialog, Title: "TimeManager", Message: err.Error(),
	})
}
