package tracker

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestStartFavoriteTimer(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	q := addProject(t, s, "2", "")
	now := s.now()
	s.now = func() time.Time { return now }
	if err := s.StartFavoriteTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	first := s.Snapshot()
	now = now.Add(time.Minute)
	if err := s.StartFavoriteTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Snapshot(), first) {
		t.Fatal("selecting current project stopped or restarted timer")
	}
	now = now.Add(time.Minute)
	if err := s.StartFavoriteTimer(q.ID); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	if d.Active == nil || d.Active.ProjectID != q.ID || len(d.Bookings) != 1 ||
		!d.Bookings[0].Start.Equal(first.Active.Start) || !d.Bookings[0].End.Equal(d.Active.Start) {
		t.Fatal("favorite switch did not atomically book old timer and start new timer")
	}
	reopened, err := Open(s.path)
	if err != nil || !reflect.DeepEqual(reopened.Snapshot(), d) {
		t.Fatalf("favorite switch not persisted: %v", err)
	}
}

func TestStartTimerAllowsNonFavoritesWithoutToggling(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	q := addProject(t, s, "2", "")
	if err := s.Favorite(q.ID); err != nil {
		t.Fatal(err)
	}
	now := s.now()
	s.now = func() time.Time { return now }
	if err := s.StartTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := s.StartTimer(q.ID); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	if before.Active.ProjectID != q.ID || len(before.Bookings) != 1 ||
		before.Bookings[0].ProjectID != p.ID || !before.Bookings[0].End.Equal(before.Active.Start) {
		t.Fatal("search switch did not atomically book the previous project")
	}
	reopened, err := Open(s.path)
	if err != nil || !reflect.DeepEqual(reopened.Snapshot(), before) {
		t.Fatalf("search switch not persisted: %v", err)
	}
	now = now.Add(time.Minute)
	if err := s.StartTimer(q.ID); err != nil {
		t.Fatal(err)
	}
	after := s.Snapshot()
	if !reflect.DeepEqual(before, after) || after.Active.ProjectID != q.ID || len(after.Bookings) != 1 {
		t.Fatal("search selection toggled or restarted the active timer")
	}
	if err := s.StartTimer("missing"); err == nil {
		t.Fatal("deleted project accepted")
	}
	if !reflect.DeepEqual(after, s.Snapshot()) {
		t.Fatal("rejected start changed state")
	}
}

func TestStartTimerRejectsInvalidOrFailedSwitch(t *testing.T) {
	for _, name := range []string{"missing", "archived", "clock backwards", "save failure"} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			q := addProject(t, s, "2", "")
			now := s.now()
			s.now = func() time.Time { return now }
			if name == "archived" {
				if err := s.SaveBooking(Booking{ProjectID: q.ID, Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour)}); err != nil {
					t.Fatal(err)
				}
				if err := s.DeleteProject(q.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.StartTimer(p.ID); err != nil {
				t.Fatal(err)
			}
			id := q.ID
			switch name {
			case "missing":
				id = "missing"
			case "clock backwards":
				now = now.Add(-time.Minute)
			case "save failure":
				s.path += string(os.PathSeparator) + "blocked.json"
			}
			if name != "clock backwards" {
				now = now.Add(time.Minute)
			}
			before := s.Snapshot()
			if err := s.StartTimer(id); err == nil {
				t.Fatal("invalid search switch accepted")
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("failed search switch changed timer or bookings")
			}
		})
	}
}

func TestStartFavoriteTimerRejectsInvalidOrFailedSwitch(t *testing.T) {
	for _, name := range []string{"missing", "not favorite", "archived", "clock backwards", "save failure"} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			q := addProject(t, s, "2", "")
			if name == "archived" {
				now := s.now()
				if err := s.SaveBooking(Booking{ProjectID: q.ID, Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour)}); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.ToggleTimer(p.ID); err != nil {
				t.Fatal(err)
			}
			id := q.ID
			switch name {
			case "missing":
				id = "missing"
			case "not favorite":
				if err := s.Favorite(q.ID); err != nil {
					t.Fatal(err)
				}
			case "archived":
				if err := s.DeleteProject(q.ID); err != nil {
					t.Fatal(err)
				}
			case "clock backwards":
				now := s.now().Add(-time.Minute)
				s.now = func() time.Time { return now }
			case "save failure":
				s.path += string(os.PathSeparator) + "blocked.json"
			}
			before := s.Snapshot()
			if name != "clock backwards" {
				now := s.now().Add(time.Minute)
				s.now = func() time.Time { return now }
			}
			if err := s.StartFavoriteTimer(id); err == nil {
				t.Fatal("invalid switch accepted")
			}
			if !reflect.DeepEqual(s.Snapshot(), before) {
				t.Fatal("failed switch changed timer or bookings")
			}
		})
	}
}
