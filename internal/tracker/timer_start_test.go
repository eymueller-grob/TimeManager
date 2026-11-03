package tracker

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestUpdateTimerStart(t *testing.T) {
	for _, test := range []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"earlier", -30 * time.Minute, ""},
		{"later", 30 * time.Minute, ""},
		{"previous day", -25 * time.Hour, ""},
		{"exact booking boundary", -47 * time.Hour, ""},
		{"overlap by nanosecond", -47*time.Hour - time.Nanosecond, "overlaps"},
		{"now", time.Hour, ""},
		{"future by nanosecond", time.Hour + time.Nanosecond, "future"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			q := addProject(t, s, "2", "")
			now := s.now()
			if err := s.SaveBooking(Booking{ProjectID: q.ID, Start: now.Add(-49 * time.Hour), End: now.Add(-48 * time.Hour)}); err != nil {
				t.Fatal(err)
			}
			s.now = func() time.Time { return now.Add(-time.Hour) }
			if err := s.ToggleTimer(p.ID); err != nil {
				t.Fatal(err)
			}
			s.now = func() time.Time { return now }
			before := s.Snapshot()
			fileBefore, err := os.ReadFile(s.path)
			if err != nil {
				t.Fatal(err)
			}
			next := before.Active.Start.Add(test.offset)
			err = s.UpdateTimerStart(*before.Active, next)
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want %q", err, test.want)
				}
				fileAfter, readErr := os.ReadFile(s.path)
				if readErr != nil || string(fileAfter) != string(fileBefore) || !reflect.DeepEqual(before, s.Snapshot()) {
					t.Fatal("rejected update changed memory or disk")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(s.path)
			if err != nil {
				t.Fatal(err)
			}
			d := reopened.Snapshot()
			if d.Active == nil || d.Active.ProjectID != p.ID || !d.Active.Start.Equal(next) || !reflect.DeepEqual(d.Bookings, before.Bookings) {
				t.Fatal("updated timer not persisted or existing bookings changed")
			}
			s.now = func() time.Time { return now.Add(time.Minute) }
			if err := s.StopTimer(); err != nil {
				t.Fatal(err)
			}
			bookings := s.Snapshot().Bookings
			if len(bookings) != 2 || !bookings[1].Start.Equal(next) {
				t.Fatal("stop did not book adjusted start")
			}
		})
	}
}

func TestUpdateTimerStartRejectsStaleAndInvalidInputs(t *testing.T) {
	for _, name := range []string{"stopped", "different project", "different start", "zero", "save failure"} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			now := s.now()
			if err := s.ToggleTimer(p.ID); err != nil {
				t.Fatal(err)
			}
			expected := *s.Snapshot().Active
			next := now.Add(-time.Minute)
			want := ""
			switch name {
			case "stopped":
				s.now = func() time.Time { return now.Add(time.Minute) }
				if err := s.StopTimer(); err != nil {
					t.Fatal(err)
				}
				want = "no timer"
			case "different project":
				expected.ProjectID = "other"
				want = "changed"
			case "different start":
				expected.Start = expected.Start.Add(-time.Nanosecond)
				want = "changed"
			case "zero":
				next = time.Time{}
				want = "required"
			case "save failure":
				s.path = s.path + string(os.PathSeparator) + "blocked.json"
				want = "change was not applied"
			}
			before := s.Snapshot()
			err := s.UpdateTimerStart(expected, next)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %v, want %q", err, want)
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("failed update changed memory")
			}
		})
	}
}
