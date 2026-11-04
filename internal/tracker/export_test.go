package tracker

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestDateRange(t *testing.T) {
	loc := time.FixedZone("test", 2*60*60)
	start, end, err := DateRange("2026-11-03", "2026-11-03", loc)
	if err != nil {
		t.Fatal(err)
	}
	if start.Location() != loc || start.Hour() != 0 || !end.Equal(start.AddDate(0, 0, 1)) {
		t.Fatalf("unexpected date range: %v to %v", start, end)
	}
}

func TestExportDailyProjectTotals(t *testing.T) {
	loc := time.FixedZone("local", 2*60*60)
	start := time.Date(2026, 11, 3, 0, 0, 0, 0, loc)
	state := State{
		Projects: []Project{
			{ID: "root", Number: "100", Name: "Root", Customer: "=Acme", Color: "#123456", Archived: true},
			{ID: "child", Number: "100", Name: "Child", Customer: "Other", ParentID: "root", Color: "#654321"},
		},
		Bookings: []Booking{
			{ProjectID: "child", Start: start.Add(23 * time.Hour), End: start.Add(26 * time.Hour)},
			{ProjectID: "root", Start: start.Add(-2 * time.Hour), End: start.Add(time.Hour)},
			{ProjectID: "child", Start: start.Add(28 * time.Hour), End: start.Add(29 * time.Hour)},
			{ProjectID: "root", Start: start.Add(47 * time.Hour), End: start.Add(50 * time.Hour)},
			{ProjectID: "child", Start: start.Add(-3 * time.Hour), End: start},
			{ProjectID: "root", Start: start.Add(48 * time.Hour), End: start.Add(49 * time.Hour)},
		},
		Active: &Timer{ProjectID: "child", Start: start.Add(50 * time.Hour)},
	}
	data, err := Export(state, "2026-11-03", "2026-11-04", loc)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got := book.GetSheetList(); !reflect.DeepEqual(got, []string{"Bookings", "Daily project totals"}) {
		t.Fatalf("sheets = %v", got)
	}
	rows, err := book.GetRows("Daily project totals")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"Date (local)", "Customer", "Project number", "Project / subproject", "Hours", "CW"},
		{"2026-11-03", "=Acme", "100", "Root", "1.00", "CW45"},
		{"2026-11-03", "Other", "100", "Root / Child", "1.00", "CW45"},
		{"2026-11-04", "=Acme", "100", "Root", "1.00", "CW45"},
		{"2026-11-04", "Other", "100", "Root / Child", "3.00", "CW45"},
		{"", "", "", "Total hours", "6.00"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("summary rows = %#v, want %#v", rows, want)
	}
	if formula, err := book.GetCellFormula("Daily project totals", "B2"); err != nil || formula != "" {
		t.Fatalf("customer became a formula: %q, %v", formula, err)
	}
	for _, cell := range []string{"E2", "E5", "E6"} {
		if kind, err := book.GetCellType("Daily project totals", cell); err != nil ||
			(kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset) {
			t.Fatalf("%s is not numeric: %v, %v", cell, kind, err)
		}
	}
	details, err := book.GetRows("Bookings")
	if err != nil || len(details) != 6 {
		t.Fatalf("detailed bookings changed: %#v, %v", details, err)
	}
	if total, err := book.GetCellValue("Bookings", "F6"); err != nil || total != "6.00" {
		t.Fatalf("detailed total = %q, %v", total, err)
	}
}

func TestExportDailyTotalsDaylightSaving(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, date string
		hours      float64
	}{
		{"spring forward", "2026-03-29", 23},
		{"fall back", "2026-10-25", 25},
	} {
		t.Run(test.name, func(t *testing.T) {
			start, end, err := DateRange(test.date, test.date, loc)
			if err != nil {
				t.Fatal(err)
			}
			state := State{
				Projects: []Project{{ID: "p", Number: "1", Name: "Project", Color: "#123456"}},
				Bookings: []Booking{{ProjectID: "p", Start: start.Add(-time.Hour), End: end.Add(time.Hour)}},
			}
			data, err := Export(state, test.date, test.date, loc)
			if err != nil {
				t.Fatal(err)
			}
			book, err := excelize.OpenReader(strings.NewReader(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			defer book.Close()
			rows, err := book.GetRows("Daily project totals")
			if err != nil || len(rows) != 3 {
				t.Fatalf("DST day split incorrectly: %#v, %v", rows, err)
			}
			for _, cell := range []string{"E2", "E3"} {
				value, err := book.GetCellValue("Daily project totals", cell)
				hours, parseErr := strconv.ParseFloat(value, 64)
				if err != nil || parseErr != nil || hours != test.hours {
					t.Fatalf("%s = %q, want %v hours: %v, %v", cell, value, test.hours, err, parseErr)
				}
			}
		})
	}
}

func TestExportDailySummaryCustomProperties(t *testing.T) {
	start := time.Date(2026, 11, 3, 23, 0, 0, 0, time.UTC)
	state := State{
		PropertyDefinitions: []PropertyDefinition{
			{ID: "text", Name: "Contact", Type: "text"},
			{ID: "number", Name: "Rate", Type: "number"},
			{ID: "check", Name: "Billable", Type: "checkbox"},
			{ID: "drop", Name: "Stage", Type: "dropdown", Options: []string{"Open", "Closed"}},
		},
		Projects: []Project{
			{ID: "p", Number: "1", Name: "Project", Color: "#123456", Archived: true,
				Properties: map[string]string{"text": "=1+1", "number": "125.5", "check": "false", "drop": "Open"}},
			{ID: "q", Number: "1", Name: "Unset", Color: "#123456"},
		},
		Bookings: []Booking{
			{ProjectID: "p", Start: start, End: start.Add(2 * time.Hour)},
			{ProjectID: "q", Start: start.Add(2 * time.Hour), End: start.Add(3 * time.Hour)},
		},
	}
	data, err := Export(state, "2026-11-03", "2026-11-04", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	const sheet = "Daily project totals"
	for cell, want := range map[string]string{
		"F1": "Property: Contact", "G1": "Property: Rate", "H1": "Property: Billable", "I1": "Property: Stage",
		"F2": "=1+1", "G2": "125.5", "H2": "FALSE", "I2": "Open",
		"F3": "=1+1", "G3": "125.5", "H3": "FALSE", "I3": "Open",
		"F4": "", "G4": "", "H4": "", "I4": "",
		"D5": "Total hours", "E5": "3.00", "F5": "", "I5": "",
	} {
		got, err := book.GetCellValue(sheet, cell)
		if err != nil || got != want {
			t.Errorf("%s = %q, want %q (%v)", cell, got, want, err)
		}
	}
	if formula, err := book.GetCellFormula(sheet, "F2"); err != nil || formula != "" {
		t.Fatalf("text property became formula: %q, %v", formula, err)
	}
	if kind, err := book.GetCellType(sheet, "G2"); err != nil ||
		(kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset) {
		t.Fatalf("number property not numeric: %v, %v", kind, err)
	}
	if kind, err := book.GetCellType(sheet, "H2"); err != nil || kind != excelize.CellTypeBool {
		t.Fatalf("checkbox property not Boolean: %v, %v", kind, err)
	}
	if first, err := book.GetCellStyle(sheet, "A1"); err != nil {
		t.Fatal(err)
	} else if last, err := book.GetCellStyle(sheet, "I1"); err != nil || last != first {
		t.Fatalf("property header style differs: %d != %d, %v", last, first, err)
	}
	state.Bookings = nil
	data, err = Export(state, "2026-11-03", "2026-11-04", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Close()
	if header, err := empty.GetCellValue(sheet, "I1"); err != nil || header != "Property: Stage" {
		t.Fatalf("empty summary missing property headers: %q, %v", header, err)
	}
}

func TestDateRangeInvalidInputs(t *testing.T) {
	loc := time.FixedZone("test", 2*60*60)
	tests := []struct {
		name, from, to, want string
	}{
		{name: "invalid start format", from: "11/03/2026", to: "2026-11-03", want: "start date must be YYYY-MM-DD"},
		{name: "invalid end date", from: "2026-11-03", to: "2026-02-30", want: "end date must be YYYY-MM-DD"},
		{name: "reversed range", from: "2026-11-04", to: "2026-11-03", want: "on or after the start date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := DateRange(tt.from, tt.to, loc); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DateRange() error = %v, want text %q", err, tt.want)
			}
		})
	}
}

func TestProjectPath(t *testing.T) {
	state := State{Projects: []Project{
		{ID: "root", Name: "Client"},
		{ID: "child", Name: "Product", ParentID: "root"},
		{ID: "leaf", Name: "Migration", ParentID: "child"},
	}}
	if got := ProjectPath(state, "leaf"); got != "Client / Product / Migration" {
		t.Fatalf("ProjectPath() = %q", got)
	}
	if got := ProjectPath(state, "missing"); got != "" {
		t.Fatalf("ProjectPath(missing) = %q, want empty", got)
	}
}

func TestExportClipsSortsAndTotalsBookings(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 11, 3, 0, 0, 0, 0, loc)
	state := State{
		Projects: []Project{
			{ID: "root", Number: "100", Name: "Root", Customer: "Acme", Color: "#123456"},
			{ID: "child", Number: "101", Name: "Child", Customer: "Acme", ParentID: "root", Color: "#654321"},
		},
		Bookings: []Booking{
			{ID: "late", ProjectID: "root", Start: start.Add(3 * time.Hour), End: start.Add(5 * time.Hour), Notes: "later"},
			{ID: "crossing", ProjectID: "child", Start: start.Add(-time.Hour), End: start.Add(time.Hour), Notes: "crosses start"},
			{ID: "outside", ProjectID: "root", Start: start.Add(-3 * time.Hour), End: start.Add(-2 * time.Hour), Notes: "excluded"},
		},
	}
	data, err := Export(state, "2026-11-03", "2026-11-03", loc)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()

	rows, err := book.GetRows("Bookings")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want header, two bookings, and total: %#v", len(rows), rows)
	}
	if rows[0][0] != "Customer" || rows[0][7] != "Project color" {
		t.Fatalf("unexpected headers: %#v", rows[0])
	}
	if rows[1][2] != "Root / Child" || rows[1][6] != "crosses start" {
		t.Fatalf("cross-midnight booking not clipped/sorted first: %#v", rows[1])
	}
	if rows[2][2] != "Root" || rows[2][6] != "later" {
		t.Fatalf("later booking not sorted: %#v", rows[2])
	}
	total, err := book.GetCellValue("Bookings", "F4")
	if err != nil {
		t.Fatal(err)
	}
	totalHours, err := strconv.ParseFloat(total, 64)
	if err != nil || totalHours != 3 {
		t.Fatalf("total hours = %q, want 3 (%v)", total, err)
	}
	if label, err := book.GetCellValue("Bookings", "E4"); err != nil || label != "Total hours" {
		t.Fatalf("total label = %q, %v", label, err)
	}
}

func TestExportEmptyBookingsAndMissingProject(t *testing.T) {
	data, err := Export(State{
		Projects: []Project{{ID: "p", Number: "1", Name: "Running", Color: "#123456"}},
		Active:   &Timer{ProjectID: "p", Start: time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)},
	}, "2026-11-03", "2026-11-03", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	total, err := book.GetCellValue("Bookings", "F2")
	totalHours, parseErr := strconv.ParseFloat(total, 64)
	if err != nil || parseErr != nil || totalHours != 0 {
		t.Fatalf("empty export total = %q, %v, %v", total, err, parseErr)
	}
	rows, err := book.GetRows("Daily project totals")
	if err != nil || len(rows) != 2 || rows[1][3] != "Total hours" || rows[1][4] != "0.00" {
		t.Fatalf("empty summary = %#v, %v", rows, err)
	}
	book.Close()

	_, err = Export(State{
		Bookings: []Booking{{ProjectID: "missing", Start: time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)}},
	}, "2026-11-03", "2026-11-03", time.UTC)
	if err == nil || !strings.Contains(err.Error(), "missing project") {
		t.Fatalf("Export() error = %v, want missing project", err)
	}
}
