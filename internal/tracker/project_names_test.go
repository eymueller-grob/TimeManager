package tracker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProjectNamesUniqueAndNumbersRepeat(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "100", "")
	for _, name := range []string{"Another project", "Child"} {
		if err := s.SaveProject(Project{Number: p.Number, Name: name, Customer: p.Customer, Color: p.Color, ParentID: p.ID}); err != nil {
			t.Fatalf("repeated number rejected: %v", err)
		}
	}
	for _, name := range []string{p.Name, "  " + p.Name + "  ", strings.ToUpper(p.Name)} {
		before := s.Snapshot()
		if err := s.SaveProject(Project{Number: "different", Name: name, Customer: "Customer", Color: p.Color}); err == nil || !strings.Contains(err.Error(), "names must be unique") {
			t.Fatalf("duplicate name %q accepted: %v", name, err)
		}
		q := before.Projects[1]
		q.Name = name
		if err := s.SaveProject(q); err == nil {
			t.Fatal("rename to duplicate name accepted")
		}
		if !reflect.DeepEqual(before, s.Snapshot()) {
			t.Fatal("failed duplicate name changed memory")
		}
	}
	if err := s.SaveProject(p); err != nil {
		t.Fatalf("unchanged name rejected: %v", err)
	}
	reopened, err := Open(s.path)
	if err != nil || len(reopened.Snapshot().Projects) != 3 {
		t.Fatalf("repeated numbers did not persist: %v", err)
	}
}

func TestArchivedProjectNamesRemainReserved(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "100", "")
	now := s.now()
	if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-time.Hour), End: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(Project{Number: "other", Name: strings.ToUpper(p.Name), Customer: "Customer", Color: p.Color}); err == nil {
		t.Fatal("archived name reused")
	}
	if err := s.SaveProject(Project{Number: p.Number, Name: "New active project", Customer: "Customer", Color: p.Color}); err != nil {
		t.Fatalf("archived number could not be reused: %v", err)
	}
}

func TestMigrateDuplicateProjectNames(t *testing.T) {
	for _, version := range []int{1, 2, 3} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "data.json")
			names := []string{"Alpha", " alpha ", "ALPHA", "Alpha (2)", "alpha (3)", "Unique"}
			legacy := State{Version: version, Settings: DefaultSettings(), PropertyDefinitions: []PropertyDefinition{}}
			legacy.Settings.Theme = "dark"
			legacy.Settings.Reminders = ReminderSettings{RunningEnabled: false, RunningMinutes: 45, IdleEnabled: true, IdleMinutes: 10}
			for i, name := range names {
				id := string(rune('a' + i))
				legacy.Projects = append(legacy.Projects, Project{ID: id, Number: id, Name: name, Customer: "Customer", Color: "#123456", Properties: map[string]string{}})
			}
			legacy.Projects[1].Archived = true
			legacy.Projects[2].ParentID = "a"
			start := time.Date(2026, 11, 3, 10, 0, 0, 123456789, time.UTC)
			legacy.Active = &Timer{ProjectID: "c", Start: start}
			legacy.Bookings = []Booking{{ID: "booking", ProjectID: "b", Start: start.Add(-2 * time.Hour), End: start.Add(-time.Hour), Notes: "preserve"}}
			if version >= 2 {
				legacy.PropertyDefinitions = []PropertyDefinition{{ID: "cost", Name: "Cost", Type: "text", Options: []string{}}}
				legacy.Projects[0].Properties["cost"] = "42"
			}
			b, err := json.Marshal(legacy)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			expected := clone(legacy)
			expected.Version = 4
			expected.Projects[1].Name = "alpha (4)"
			expected.Projects[2].Name = "ALPHA (5)"
			if version == 1 {
				expected.Settings = DefaultSettings()
			} else if version == 2 {
				expected.Settings.Reminders = DefaultReminders()
			}
			if !reflect.DeepEqual(s.Snapshot(), expected) {
				t.Fatalf("migration changed unrelated data:\ngot %#v\nwant %#v", s.Snapshot(), expected)
			}
			once, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err != nil {
				t.Fatal(err)
			}
			twice, err := os.ReadFile(path)
			if err != nil || string(once) != string(twice) {
				t.Fatal("reopening migration changed names or rewrote file")
			}
		})
	}
}

func TestVersionFourDuplicateNamesRejectedWithoutOverwriting(t *testing.T) {
	s := newStore(t)
	addProject(t, s, "1", "")
	addProject(t, s, "2", "")
	d := s.Snapshot()
	d.Projects[1].Name = d.Projects[0].Name
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(s.path); err == nil {
		t.Fatal("invalid current data accepted")
	}
	after, err := os.ReadFile(s.path)
	if err != nil || string(after) != string(b) {
		t.Fatal("invalid current data was overwritten")
	}
}
