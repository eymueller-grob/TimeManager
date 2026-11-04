package tracker

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestPropertyOrderPersistenceAndExport(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	first := addDefinition(t, s, "First", "text", nil)
	second := addDefinition(t, s, "Second", "text", nil)
	p.Properties = map[string]string{first.ID: "one", second.ID: "two"}
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	now := s.now()
	if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-time.Hour), End: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.MovePropertyDefinition(second.ID, -1); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Snapshot()
	if got.PropertyDefinitions[0].ID != second.ID || !reflect.DeepEqual(got.Projects[0].Properties, p.Properties) {
		t.Fatal("order not persisted or property values changed")
	}
	data, err := Export(got, now.Add(-time.Hour).UTC().Format("2006-01-02"), now.UTC().Format("2006-01-02"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			t.Fatal(err)
		}
		index := -1
		for i, value := range rows[0] {
			if value == "Property: Second" {
				index = i
			}
		}
		if index < 0 || index+1 >= len(rows[0]) || rows[0][index+1] != "Property: First" ||
			len(rows) < 2 || rows[1][index] != "two" || rows[1][index+1] != "one" {
			t.Fatalf("incorrect property order or values in %s: %#v", sheet, rows)
		}
	}
	if err := s.MovePropertyDefinition(second.ID, 1); err != nil || s.Snapshot().PropertyDefinitions[0].ID != first.ID {
		t.Fatalf("moving down failed: %v", err)
	}
}

func TestInvalidPropertyMovesPreserveState(t *testing.T) {
	for _, scenario := range []string{"unknown", "direction", "first", "last", "save"} {
		t.Run(scenario, func(t *testing.T) {
			s := newStore(t)
			first := addDefinition(t, s, "First", "text", nil)
			second := addDefinition(t, s, "Second", "text", nil)
			before := s.Snapshot()
			id, direction := first.ID, 1
			switch scenario {
			case "unknown":
				id = "removed"
			case "direction":
				direction = 0
			case "first":
				direction = -1
			case "last":
				id = second.ID
			case "save":
				s.path = filepath.Join(t.TempDir(), "missing", "data.json")
			}
			if err := s.MovePropertyDefinition(id, direction); err == nil {
				t.Fatal("invalid move succeeded")
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("failed move changed state")
			}
		})
	}
}
