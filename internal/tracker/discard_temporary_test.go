package tracker

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestDiscardTemporaryTimer(t *testing.T) {
	for _, scenario := range []string{"success", "stale", "wrong-project", "no-timer", "save-failure"} {
		t.Run(scenario, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			now := s.now()
			if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-time.Hour), End: now.Add(-time.Minute)}); err != nil {
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
			expected := *s.Snapshot().Active
			switch scenario {
			case "stale":
				expected.Start = expected.Start.Add(time.Second)
			case "wrong-project":
				expected.ProjectID = p.ID
			case "no-timer":
				if err := s.DiscardTemporaryTimer(expected); err != nil {
					t.Fatal(err)
				}
			case "save-failure":
				s.path = filepath.Join(t.TempDir(), "missing", "data.json")
			}
			before := s.Snapshot()
			err := s.DiscardTemporaryTimer(expected)
			after := s.Snapshot()
			if scenario != "success" {
				if err == nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("failed discard changed state or succeeded: %v", err)
				}
				return
			}
			if err != nil || after.Active != nil || after.PausedProjectID != "" || !reflect.DeepEqual(before.Bookings, after.Bookings) {
				t.Fatalf("discard did not preserve existing bookings: %v", err)
			}
			reopened, err := Open(s.path)
			if err != nil || !reflect.DeepEqual(after, reopened.Snapshot()) {
				t.Fatalf("discard not persisted: %v", err)
			}
		})
	}
}
