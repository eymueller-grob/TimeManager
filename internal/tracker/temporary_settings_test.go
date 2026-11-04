package tracker

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestRequireTemporaryProject(t *testing.T) {
	for _, action := range []string{"stop", "pause", "toggle", "start", "favorite"} {
		t.Run(action, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			p.Favorite = true
			if err := s.SaveProject(p); err != nil {
				t.Fatal(err)
			}
			settings := s.Snapshot().Settings
			settings.RequireTemporaryProject = true
			if err := s.SaveSettings(settings); err != nil {
				t.Fatal(err)
			}
			if err := s.StartTemporaryTimer(); err != nil {
				t.Fatal(err)
			}
			now := s.now().Add(time.Minute)
			s.now = func() time.Time { return now }
			before := s.Snapshot()
			actions := map[string]func() error{
				"stop": s.StopTimer, "pause": s.PauseTimer,
				"toggle":   func() error { return s.ToggleTimer(p.ID) },
				"start":    func() error { return s.StartTimer(p.ID) },
				"favorite": func() error { return s.StartFavoriteTimer(p.ID) },
			}
			if err := actions[action](); !errors.Is(err, ErrTemporaryProjectRequired) {
				t.Fatalf("expected project requirement, got %v", err)
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("blocked action changed state")
			}
			reopened, err := Open(s.path)
			if err != nil || !reflect.DeepEqual(before, reopened.Snapshot()) {
				t.Fatalf("setting or timer not persisted: %v", err)
			}
			if action == "pause" {
				if err := s.PauseTemporaryTimer(*before.Active, p.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				next := ""
				if action != "stop" {
					next = p.ID
				}
				if err := s.FinishTemporaryTimer(*before.Active, p.ID, next, action == "favorite"); err != nil {
					t.Fatal(err)
				}
			}
			got := s.Snapshot()
			if len(got.Bookings) != 1 || got.Bookings[0].ProjectID != p.ID || !got.Bookings[0].Start.Equal(before.Active.Start) {
				t.Fatalf("incorrect assigned booking: %#v", got)
			}
			if action == "pause" && (got.Active != nil || got.PausedProjectID != p.ID) {
				t.Fatal("pause did not retain assigned project")
			}
		})
	}
}

func TestMigrateVersionSixTemporarySettings(t *testing.T) {
	s := newStore(t)
	if err := s.StartTemporaryTimer(); err != nil {
		t.Fatal(err)
	}

	legacy := s.Snapshot()
	legacy.Version = 6
	b, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, b, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Snapshot()
	legacy.Version = CurrentVersion
	if got.Settings.RequireTemporaryProject || !reflect.DeepEqual(got, legacy) {
		t.Fatal("migration changed existing data or temporary timer behavior")
	}
}

func TestPauseTemporaryAssignmentFailuresPreserveState(t *testing.T) {
	for _, failure := range []string{"stale", "invalid-project", "save"} {
		t.Run(failure, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			if err := s.StartTemporaryTimer(); err != nil {
				t.Fatal(err)
			}
			before := s.Snapshot()
			expected := *before.Active
			target := p.ID
			now := s.now().Add(time.Minute)
			s.now = func() time.Time { return now }
			switch failure {
			case "stale":
				expected.Start = expected.Start.Add(time.Second)
			case "invalid-project":
				target = temporaryProjectID
			case "save":
				s.path = filepath.Join(t.TempDir(), "missing", "data.json")
			}
			if err := s.PauseTemporaryTimer(expected, target); err == nil {
				t.Fatal("expected assignment failure")
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("failed assignment changed state")
			}
		})
	}
}
