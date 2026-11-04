package tracker

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestExportStatusSelection(t *testing.T) {
	loc := time.FixedZone("local", 2*60*60)
	start := time.Date(2026, 11, 3, 0, 0, 0, 0, loc)
	d := State{
		Projects: []Project{{ID: "p", Name: "Project", Number: "1", Customer: "Customer", Color: "#123456"}},
		Bookings: []Booking{
			{ID: "full", ProjectID: "p", Start: start, End: start.Add(time.Hour)},
			{ID: "marked", ProjectID: "p", Start: start.Add(time.Hour), End: start.Add(2 * time.Hour), Exported: true},
			{ID: "cross-start", ProjectID: "p", Start: start.Add(-time.Hour), End: start.Add(30 * time.Minute)},
			{ID: "cross-end", ProjectID: "p", Start: start.Add(23 * time.Hour), End: start.Add(25 * time.Hour)},
			{ID: "exact-end", ProjectID: "p", Start: start.Add(22 * time.Hour), End: start.AddDate(0, 0, 1)},
			{ID: "outside", ProjectID: "p", Start: start.Add(-2 * time.Hour), End: start},
		},
	}
	for _, exclude := range []bool{false, true} {
		selected, eligible, err := SelectExportBookings(d, "2026-11-03", "2026-11-03", loc, exclude)
		if err != nil {
			t.Fatal(err)
		}
		wantCount := 5
		if exclude {
			wantCount = 4
		}
		if len(selected.Bookings) != wantCount || len(eligible) != 2 || eligible[0].ID != "full" || eligible[1].ID != "exact-end" {
			t.Fatalf("selection: %#v eligible %#v", selected.Bookings, eligible)
		}
		if exclude {
			for _, b := range selected.Bookings {
				if b.Exported {
					t.Fatal("marked booking included")
				}
			}
		}
	}
	if _, _, err := SelectExportBookings(d, "bad", "2026-11-03", loc, false); err == nil {
		t.Fatal("invalid dates accepted")
	}
	// With no overlapping input, the filtered workbook still contains both empty sheets.
	d.Bookings = []Booking{d.Bookings[1]}
	selected, _, err := SelectExportBookings(d, "2026-11-03", "2026-11-03", loc, true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Export(selected, "2026-11-03", "2026-11-03", loc)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for _, sheet := range []string{"Bookings", "Daily project totals"} {
		rows, err := book.GetRows(sheet)
		if err != nil || len(rows) != 2 {
			t.Fatalf("excluded bookings leaked into %s: %#v %v", sheet, rows, err)
		}
	}
}

func TestMarkExportedAndManualReset(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	now := s.now()
	if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-time.Hour), End: now, Notes: "note"}); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	if err := s.MarkBookingsExported(before.Bookings); err != nil {
		t.Fatal(err)
	}
	want := clone(before)
	want.Bookings[0].Exported = true
	if !reflect.DeepEqual(s.Snapshot(), want) {
		t.Fatal("marking changed booking metadata")
	}
	reopened, err := Open(s.path)
	if err != nil || !reflect.DeepEqual(reopened.Snapshot(), want) {
		t.Fatalf("exported status not persisted: %v", err)
	}
	booking := want.Bookings[0]
	booking.Exported = false
	if err := s.SaveBooking(booking); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Snapshot(), before) {
		t.Fatal("manual reset changed booking")
	}
	booking.Exported = true
	if err := s.SaveBooking(booking); err != nil {
		t.Fatal(err)
	}
	if !s.Snapshot().Bookings[0].Exported {
		t.Fatal("manual marking failed")
	}
}

func TestMarkExportedRejectsStaleAndSaveFailure(t *testing.T) {
	for _, name := range []string{"edited", "deleted", "save failure"} {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			p := addProject(t, s, "1", "")
			now := s.now()
			for i := 0; i < 2; i++ {
				if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(time.Duration(-2+i) * time.Hour), End: now.Add(time.Duration(-1+i) * time.Hour)}); err != nil {
					t.Fatal(err)
				}
			}
			expected := s.Snapshot().Bookings
			switch name {
			case "edited":
				b := expected[1]
				b.Notes = "changed"
				if err := s.SaveBooking(b); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if err := s.DeleteBooking(expected[1].ID); err != nil {
					t.Fatal(err)
				}
			case "save failure":
				s.path += string(os.PathSeparator) + "blocked.json"
			}
			before := s.Snapshot()
			if err := s.MarkBookingsExported(expected); err == nil {
				t.Fatal("invalid marking accepted")
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("failed marking applied partial status updates")
			}
		})
	}
}

func TestMigrateExportedStatusFromVersionFour(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	now := s.now()
	if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-time.Hour), End: now, Notes: "keep"}); err != nil {
		t.Fatal(err)
	}

	legacy := s.Snapshot()
	legacy.Version = 4
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	legacy.Version = CurrentVersion
	if !reflect.DeepEqual(reopened.Snapshot(), legacy) {
		t.Fatal("migration lost booking data")
	}
	persisted, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(persisted, []byte(`"exported": false`)) {
		t.Fatal("migration not persisted")
	}
	invalid := bytes.Replace(data, []byte(`"version":4`), []byte(`"version":99`), 1)
	if err := os.WriteFile(s.path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(s.path); err == nil {
		t.Fatal("invalid data accepted")
	}
	got, err := os.ReadFile(s.path)
	if err != nil || !bytes.Equal(got, invalid) {
		t.Fatal("invalid data overwritten")
	}
}

func TestMigrateVersionFivePreservesExportStatus(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	now := s.now()
	if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-time.Hour), End: now}); err != nil {
		t.Fatal(err)
	}
	legacy := s.Snapshot()
	legacy.Bookings[0].Exported = true
	if err := s.SaveBooking(legacy.Bookings[0]); err != nil {
		t.Fatal(err)
	}
	legacy = s.Snapshot()
	legacy.Version = 5
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Snapshot()
	if got.Version != CurrentVersion || got.PausedProjectID != "" || !got.Bookings[0].Exported {
		t.Fatalf("migration lost version 5 data: %#v", got)
	}
}
