package tracker

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestExportISOWeeks(t *testing.T) {
	loc := time.FixedZone("local", 2*60*60)
	for _, test := range []struct {
		date, next, week, endWeek string
	}{
		{"2026-12-31", "2027-01-01", "CW53", "CW53"},
		{"2027-01-03", "2027-01-04", "CW53", "CW01"},
		{"2024-12-29", "2024-12-30", "CW52", "CW01"},
	} {
		t.Run(test.date, func(t *testing.T) {
			day, err := time.ParseInLocation("2006-01-02", test.date, loc)
			if err != nil {
				t.Fatal(err)
			}
			d := State{
				Projects:            []Project{{ID: "p", Name: "Project", Number: "1", Color: "#123456"}},
				Bookings:            []Booking{{ProjectID: "p", Start: day.Add(23 * time.Hour).UTC(), End: day.AddDate(0, 0, 1).Add(time.Hour).UTC()}},
				PropertyDefinitions: []PropertyDefinition{{ID: "text", Name: "Contact", Type: "text"}},
			}
			data, err := Export(d, test.date, test.next, loc)
			if err != nil {
				t.Fatal(err)
			}
			book, err := excelize.OpenReader(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			defer book.Close()
			for _, cell := range []struct{ sheet, cell, want string }{
				{"Bookings", "J1", "Start CW"}, {"Bookings", "K1", "End CW"},
				{"Bookings", "J2", test.week}, {"Bookings", "K2", test.endWeek},
				{"Daily project totals", "G1", "CW"},
				{"Daily project totals", "G2", test.week}, {"Daily project totals", "G3", test.endWeek},
			} {
				if got, err := book.GetCellValue(cell.sheet, cell.cell); err != nil || got != cell.want {
					t.Errorf("%s %s = %q, want %q (%v)", cell.sheet, cell.cell, got, cell.want, err)
				}
			}
			clipped, err := Export(d, test.next, test.next, loc)
			if err != nil {
				t.Fatal(err)
			}
			filtered, err := excelize.OpenReader(bytes.NewReader(clipped))
			if err != nil {
				t.Fatal(err)
			}
			defer filtered.Close()
			if got, err := filtered.GetCellValue("Bookings", "J2"); err != nil || got != test.endWeek {
				t.Fatalf("clipped start week = %q, want %q (%v)", got, test.endWeek, err)
			}
		})
	}
}
