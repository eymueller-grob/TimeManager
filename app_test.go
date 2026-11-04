package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"TimeManager/internal/tracker"
)

func TestAppRequiresReadyStore(t *testing.T) {
	app := NewApp()
	if _, err := app.GetState(); err == nil || !strings.Contains(err.Error(), "data store is not ready") {
		t.Fatalf("GetState() error = %v, want store-not-ready", err)
	}
	if err := app.SaveProject(tracker.Project{}); err == nil || !strings.Contains(err.Error(), "data store is not ready") {
		t.Fatalf("SaveProject() error = %v, want store-not-ready", err)
	}
	if err := app.SaveProjectWithRelated(tracker.Project{}); err == nil || !strings.Contains(err.Error(), "data store is not ready") {
		t.Fatalf("SaveProjectWithRelated() error = %v, want store-not-ready", err)
	}
	for name, action := range map[string]func() error{
		"RunInBackground": app.RunInBackground,
		"QuitApplication": app.QuitApplication,
	} {
		if err := action(); err == nil || !strings.Contains(err.Error(), "data store is not ready") {
			t.Fatalf("%s() error = %v, want store-not-ready", name, err)
		}
	}

	initErr := errors.New("cannot read data")
	app.initErr = initErr
	if _, err := app.GetState(); !errors.Is(err, initErr) {
		t.Fatalf("GetState() error = %v, want original initialization error", err)
	}
}

func TestAppAllowsClosingWhenDataCannotBeOpened(t *testing.T) {
	app := NewApp()
	app.initErr = errors.New("invalid saved data")
	if app.beforeClose(nil) {
		t.Fatal("initialization failure must not prevent closing")
	}
}

func TestAppAllowsApprovedQuitWithoutActiveTimer(t *testing.T) {
	store, err := tracker.Open(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}
	app.quitRequested.Store(true)
	if app.beforeClose(nil) {
		t.Fatal("approved quit must not show another close prompt")
	}
	if app.quitRequested.Load() {
		t.Fatal("quit approval was not consumed")
	}
}

func TestAppQuitSaveFailurePreservesTimer(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	store, err := tracker.Open(filepath.Join(dir, "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProject(tracker.Project{Number: "1", Name: "Project", Customer: "Customer", Color: "#123456"}); err != nil {
		t.Fatal(err)
	}
	if err := store.ToggleTimer(store.Snapshot().Projects[0].ID); err != nil {
		t.Fatal(err)
	}
	before := store.Snapshot()
	if err := os.Rename(dir, dir+"-backup"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("block data directory"), 0600); err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}
	if err := app.QuitApplication(); err == nil || !strings.Contains(err.Error(), "save timer before quitting") {
		t.Fatalf("QuitApplication() error = %v, want save failure", err)
	}
	after := store.Snapshot()
	if after.Active == nil || *after.Active != *before.Active || len(after.Bookings) != len(before.Bookings) {
		t.Fatal("failed quit changed the timer or bookings")
	}
	if app.quitRequested.Load() {
		t.Fatal("failed save must not authorize quitting")
	}
}

func TestAppTemporaryQuitSaveFailurePreservesTimer(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	store, err := tracker.Open(filepath.Join(dir, "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.StartTemporaryTimer(); err != nil {
		t.Fatal(err)
	}
	before := store.Snapshot()
	if err := os.Rename(dir, dir+"-backup"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("block data directory"), 0600); err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}
	if err := app.QuitApplication(); err == nil || !strings.Contains(err.Error(), "save timer before quitting") {
		t.Fatalf("temporary quit error = %v, want save failure", err)
	}
	after := store.Snapshot()
	if after.Active == nil || *after.Active != *before.Active || len(after.Bookings) != len(before.Bookings) || app.quitRequested.Load() {
		t.Fatal("failed temporary quit changed timer or authorized quitting")
	}
}

func TestAppGetStateReturnsStoreSnapshot(t *testing.T) {
	store, err := tracker.Open(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}
	state, err := app.GetState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Projects == nil || state.Bookings == nil || state.Active != nil {
		t.Fatalf("unexpected app state: %#v", state)
	}
}

func TestAppGetStateSerializesBookingsAndActiveTimer(t *testing.T) {
	store, err := tracker.Open(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProject(tracker.Project{
		Number: "1", Name: "Project", Customer: "Customer", Color: "#123456",
	}); err != nil {
		t.Fatal(err)
	}
	project := store.Snapshot().Projects[0]
	start := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := store.SaveBooking(tracker.Booking{
		ProjectID: project.ID, Start: start, End: start.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ToggleTimer(project.ID); err != nil {
		t.Fatal(err)
	}

	state, err := (&App{store: store}).GetState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Bookings) != 1 || state.Active == nil || state.Active.ProjectID != project.ID {
		t.Fatalf("unexpected bridge state: %#v", state)
	}
	if _, err := time.Parse(time.RFC3339Nano, state.Bookings[0].Start); err != nil {
		t.Fatalf("booking start is not RFC3339Nano: %q, %v", state.Bookings[0].Start, err)
	}
	if _, err := time.Parse(time.RFC3339Nano, state.Active.Start); err != nil {
		t.Fatalf("timer start is not RFC3339Nano: %q, %v", state.Active.Start, err)
	}
}

func TestAppSaveBookingRejectsInvalidTimestamps(t *testing.T) {
	store, err := tracker.Open(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}
	tests := []struct {
		name    string
		booking Booking
		want    string
	}{
		{name: "invalid start", booking: Booking{Start: "not-a-time", End: time.Now().Format(time.RFC3339)}, want: "invalid booking start"},
		{name: "invalid end", booking: Booking{Start: time.Now().Format(time.RFC3339), End: "not-a-time"}, want: "invalid booking end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := app.SaveBooking(tt.booking); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("SaveBooking() error = %v, want text %q", err, tt.want)
			}
		})
	}
}

func TestAppUpdateTimerStartRejectsInvalidTimestamps(t *testing.T) {
	for _, test := range []struct {
		name, original, next, want string
	}{
		{"original", "bad", time.Now().Format(time.RFC3339Nano), "invalid original timer start"},
		{"next", time.Now().Format(time.RFC3339Nano), "bad", "invalid timer start"},
		{"not ready", time.Now().Format(time.RFC3339Nano), time.Now().Format(time.RFC3339Nano), "data store is not ready"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := NewApp().UpdateTimerStart(Timer{ProjectID: "1", Start: test.original}, test.next)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAppDoesNotApplyRejectedMutation(t *testing.T) {
	store, err := tracker.Open(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store}
	err = app.SaveProject(tracker.Project{})
	if err == nil || !strings.Contains(err.Error(), "number, name, and customer are required") {
		t.Fatalf("SaveProject() error = %v, want project validation error", err)
	}
	if len(store.Snapshot().Projects) != 0 {
		t.Fatal("rejected mutation changed persisted state")
	}
}
