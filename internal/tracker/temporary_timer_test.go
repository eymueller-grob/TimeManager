package tracker

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestTemporaryTimerLifecycle(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	now := s.now()
	s.now = func() time.Time { return now }
	if err := s.StartTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := s.StartTemporaryTimer(); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	temp, ok := project(d, temporaryProjectID)
	if !ok || temp.Name != "Temporary / Unassigned" || d.Active.ProjectID != temp.ID ||
		len(d.Bookings) != 1 || !d.Bookings[0].End.Equal(d.Active.Start) {
		t.Fatalf("temporary switch not atomic: %#v", d)
	}
	now = now.Add(time.Minute)
	if err := s.StartTemporaryTimer(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d, s.Snapshot()) {
		t.Fatal("repeat start changed temporary timer")
	}
	temp.Name = "To assign"
	if err := s.SaveProject(temp); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishTemporaryTimer(*s.Snapshot().Active, p.ID, "", false); err != nil {
		t.Fatal(err)
	}
	d = s.Snapshot()
	booking := d.Bookings[1]
	original := booking
	booking.ProjectID = p.ID
	if err := s.SaveBooking(booking); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Snapshot().Bookings[1]
	original.ProjectID = p.ID
	if !reflect.DeepEqual(got, original) {
		t.Fatal("reassignment changed booking metadata or timestamps")
	}
	now = now.Add(time.Minute)
	if err := s.StartTemporaryTimer(); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot(); len(got.Projects) != 2 || got.Active.ProjectID != temp.ID || got.Projects[1].Name != "To assign" {
		t.Fatal("renamed temporary project not reused")
	}
}

func TestTemporaryTimerNameCollisionAndFailedSave(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	p.Name = "Temporary / Unassigned"
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	path := s.path
	s.path += string(os.PathSeparator) + "blocked.json"
	if err := s.StartTemporaryTimer(); err == nil {
		t.Fatal("failed save accepted")
	}
	if !reflect.DeepEqual(before, s.Snapshot()) {
		t.Fatal("failed save created project or timer")
	}
	s.path = path
	if err := s.StartTemporaryTimer(); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	if d.Projects[1].Name != "Temporary / Unassigned (2)" || d.Active.ProjectID == p.ID {
		t.Fatal("existing project overwritten or reused")
	}
}

func TestTemporaryTimerArchivedAndClockFailures(t *testing.T) {
	for _, name := range []string{"archived", "clock backwards"} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			now := s.now()
			s.now = func() time.Time { return now }
			if err := s.StartTemporaryTimer(); err != nil {
				t.Fatal(err)
			}
			if name == "archived" {
				if err := s.SaveBooking(Booking{ProjectID: temporaryProjectID, Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour)}); err != nil {
					t.Fatal(err)
				}
				p := addProject(t, s, "1", "")
				now = now.Add(time.Minute)
				if err := s.FinishTemporaryTimer(*s.Snapshot().Active, p.ID, "", false); err != nil {
					t.Fatal(err)
				}
				if err := s.DeleteProject(temporaryProjectID); err != nil {
					t.Fatal(err)
				}
			} else {
				now = now.Add(time.Minute)
				if err := s.FinishTemporaryTimer(*s.Snapshot().Active, temporaryProjectID, "", false); err == nil {
					t.Fatal("temporary assignment to itself accepted")
				}
				p := addProject(t, s, "1", "")
				if err := s.FinishTemporaryTimer(*s.Snapshot().Active, p.ID, p.ID, false); err != nil {
					t.Fatal(err)
				}

				now = now.Add(-time.Minute)
			}
			before := s.Snapshot()
			if err := s.StartTemporaryTimer(); err == nil {
				t.Fatal("invalid temporary start accepted")
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("failed temporary start changed state")
			}
		})
	}
}

func TestTemporaryTimerRequiresAssignment(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	now := s.now()
	s.now = func() time.Time { return now }
	if err := s.StartTemporaryTimer(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	before := s.Snapshot()
	for _, action := range []func() error{
		s.StopTimer,
		func() error { return s.ToggleTimer(temporaryProjectID) },
		func() error { return s.StartTimer(p.ID) },
		func() error { return s.StartFavoriteTimer(p.ID) },
	} {
		if err := action(); err != ErrTemporaryAssignment {
			t.Fatalf("ending temporary time did not require assignment: %v", err)
		}
		if !reflect.DeepEqual(before, s.Snapshot()) {
			t.Fatal("assignment request modified timer")
		}
	}
	expected := *before.Active
	if err := s.FinishTemporaryTimer(expected, p.ID, p.ID, true); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	if d.Active.ProjectID != p.ID || len(d.Bookings) != 1 || d.Bookings[0].ProjectID != p.ID ||
		!d.Bookings[0].Start.Equal(expected.Start) || !d.Bookings[0].End.Equal(d.Active.Start) {
		t.Fatal("assignment and switch not saved at same boundary")
	}
	reopened, err := Open(s.path)
	if err != nil || !reflect.DeepEqual(d, reopened.Snapshot()) {
		t.Fatalf("assignment not persisted: %v", err)
	}
	if err := s.FinishTemporaryTimer(expected, p.ID, "", false); err == nil {
		t.Fatal("stale confirmation accepted")
	}
}

func TestTemporaryAssignmentFailuresPreserveState(t *testing.T) {
	for _, name := range []string{"missing target", "archived target", "self", "missing next", "not favorite", "stale", "save failure"} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			now := s.now()
			s.now = func() time.Time { return now }
			if name == "archived target" {
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
			now = now.Add(time.Minute)
			expected := *s.Snapshot().Active
			target, next, favorite := p.ID, "", false
			switch name {
			case "missing target":
				target = "missing"
			case "self":
				target = temporaryProjectID
			case "missing next":
				next = "missing"
			case "not favorite":
				if err := s.Favorite(p.ID); err != nil {
					t.Fatal(err)
				}
				next, favorite = p.ID, true
			case "stale":
				expected.Start = expected.Start.Add(time.Second)
			case "save failure":
				s.path += string(os.PathSeparator) + "blocked.json"
			}
			before := s.Snapshot()
			if err := s.FinishTemporaryTimer(expected, target, next, favorite); err == nil {
				t.Fatal("invalid confirmation accepted")
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("failed confirmation modified state")
			}
		})
	}
}
