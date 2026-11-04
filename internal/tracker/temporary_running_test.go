package tracker

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestAssignRunningTemporaryTimer(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	if err := s.StartTemporaryTimer(); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	expected := *before.Active
	if err := s.AssignRunningTemporaryTimer(expected, p.ID); err != nil {
		t.Fatal(err)
	}
	want := clone(before)
	want.Active.ProjectID = p.ID
	if !reflect.DeepEqual(s.Snapshot(), want) {
		t.Fatal("assignment reset timer, created bookings, or changed unrelated state")
	}
	reopened, err := Open(s.path)
	if err != nil || !reflect.DeepEqual(reopened.Snapshot(), want) {
		t.Fatalf("running assignment not persisted: %v", err)
	}
	if err := s.AssignRunningTemporaryTimer(expected, p.ID); err == nil {
		t.Fatal("stale assignment accepted")
	}
}

func TestAssignRunningTemporaryTimerFailures(t *testing.T) {
	for _, name := range []string{"missing", "archived", "self", "stale", "save failure"} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			now := s.now()
			if name == "archived" {
				if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour)}); err != nil {
					t.Fatal(err)
				}
				if err := s.DeleteProject(p.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.StartTemporaryTimer(); err != nil {
				t.Fatal(err)
			}
			before := s.Snapshot()
			expected, target := *before.Active, p.ID
			switch name {
			case "missing":
				target = "missing"
			case "self":
				target = temporaryProjectID
			case "stale":
				expected.Start = expected.Start.Add(time.Second)
			case "save failure":
				s.path += string(os.PathSeparator) + "blocked.json"
			}
			if err := s.AssignRunningTemporaryTimer(expected, target); err == nil {
				t.Fatal("invalid assignment accepted")
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("failed assignment changed state")
			}
		})
	}
}
