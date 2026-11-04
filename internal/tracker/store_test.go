package tracker

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }
	return s
}

func addProject(t *testing.T, s *Store, number, parent string) Project {
	t.Helper()
	if err := s.SaveProject(Project{Number: number, Name: "Project " + number, Customer: "Customer", Color: "#16a085", ParentID: parent, Favorite: true}); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	return d.Projects[len(d.Projects)-1]
}

func TestProjectHierarchyAndValidation(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "100", "")
	child := addProject(t, s, "101", p.ID)
	grandchild := addProject(t, s, "102", child.ID)
	p.ParentID = grandchild.ID
	if err := s.SaveProject(p); err == nil {
		t.Fatal("cycle accepted")
	}
	if err := s.DeleteProject(p.ID); err == nil {
		t.Fatal("parent deletion accepted")
	}
	for _, test := range []struct {
		name string
		edit func(*Project)
	}{
		{"missing name", func(p *Project) { p.Name = " " }},
		{"missing customer", func(p *Project) { p.Customer = "" }},
		{"invalid color", func(p *Project) { p.Color = "red" }},
		{"duplicate name", func(p *Project) { p.Name = "Project 100" }},
		{"missing parent", func(p *Project) { p.ParentID = "missing" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			v := child
			test.edit(&v)
			if err := s.SaveProject(v); err == nil {
				t.Fatal("invalid project accepted")
			}
		})
	}
	p.ParentID = ""
	p.Name = "Updated"
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	if err := s.Favorite(p.ID); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Projects[0].Favorite {
		t.Fatal("favorite not toggled")
	}
	if err := s.DeleteProject(grandchild.ID); err != nil {
		t.Fatal(err)
	}
}

func TestTimerSwitchRecoveryAndArchive(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	q := addProject(t, s, "2", "")
	now := s.now()
	s.now = func() time.Time { return now }
	if err := s.ToggleTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil || reopened.Snapshot().Active == nil {
		t.Fatalf("timer not recovered: %v", err)
	}
	if err := s.DeleteProject(p.ID); err == nil {
		t.Fatal("deleted running project")
	}
	now = now.Add(35 * time.Minute)
	if err := s.ToggleTimer(q.ID); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	if len(d.Bookings) != 1 || d.Bookings[0].End.Sub(d.Bookings[0].Start) != 35*time.Minute || d.Active.ProjectID != q.ID {
		t.Fatal("switch did not atomically save previous timer")
	}
	if !d.Bookings[0].End.Equal(d.Active.Start) {
		t.Fatal("gap in switch boundary")
	}
	now = now.Add(10 * time.Minute)
	if err := s.ToggleTimer(q.ID); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().Active != nil || len(s.Snapshot().Bookings) != 2 {
		t.Fatal("second click did not stop and save")
	}
	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	d = s.Snapshot()
	if !d.Projects[0].Archived || d.Projects[0].Favorite || len(d.Bookings) != 2 {
		t.Fatal("booked project not archived safely")
	}
	if err := s.ToggleTimer(p.ID); err == nil {
		t.Fatal("archived project timer accepted")
	}
	if err := s.RestoreProject(p.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err = Open(s.path)
	if err != nil || len(reopened.Snapshot().Bookings) != 2 || reopened.Snapshot().Projects[0].Archived {
		t.Fatal("data not persisted")
	}
}

func TestPauseAndResumeSaveSeparateBookingSegments(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	now := s.now()
	s.now = func() time.Time { return now }
	if err := s.StartTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	firstStart := s.Snapshot().Active.Start
	now = now.Add(30 * time.Minute)
	if err := s.PauseTimer(); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	if d.Active != nil || d.PausedProjectID != p.ID || len(d.Bookings) != 1 ||
		!d.Bookings[0].Start.Equal(firstStart) || !d.Bookings[0].End.Equal(now) {
		t.Fatalf("pause did not save the active segment: %#v", d)
	}
	reopened, err := Open(s.path)
	if err != nil || !reflect.DeepEqual(d, reopened.Snapshot()) {
		t.Fatalf("paused timer was not persisted: %v", err)
	}
	if err := s.DeleteProject(p.ID); err == nil {
		t.Fatal("deleted project with a paused timer")
	}
	now = now.Add(20 * time.Minute)
	if err := s.ResumeTimer(); err != nil {
		t.Fatal(err)
	}
	d = s.Snapshot()
	if d.PausedProjectID != "" || d.Active == nil || d.Active.ProjectID != p.ID ||
		!d.Active.Start.Equal(now) || len(d.Bookings) != 1 {
		t.Fatalf("resume did not begin a new segment: %#v", d)
	}
	now = now.Add(10 * time.Minute)
	if err := s.PauseTimer(); err != nil {
		t.Fatal(err)
	}
	d = s.Snapshot()
	if len(d.Bookings) != 2 || d.Bookings[1].Start != d.Bookings[0].End.Add(20*time.Minute) ||
		d.Bookings[1].End.Sub(d.Bookings[1].Start) != 10*time.Minute || d.PausedProjectID != p.ID {
		t.Fatalf("paused interval was included in booked time: %#v", d)
	}
}

func TestPauseTimerFailurePreservesActiveTimer(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	if err := s.StartTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	s.now = func() time.Time { return before.Active.Start }
	if err := s.PauseTimer(); err == nil {
		t.Fatal("pause with a non-advancing clock succeeded")
	}
	if !reflect.DeepEqual(s.Snapshot(), before) {
		t.Fatal("failed pause changed timer state")
	}
}

func TestBookingsAndOverlap(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	now := s.now()
	b := Booking{ProjectID: p.ID, Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour), Notes: "work"}
	if err := s.SaveBooking(b); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []Booking{
		{ProjectID: p.ID, Start: now, End: now},
		{ProjectID: p.ID, Start: now, End: now.Add(time.Minute)},
		{ProjectID: "missing", Start: b.Start, End: b.End},
		{ProjectID: p.ID, Start: b.Start.Add(time.Minute), End: b.End.Add(time.Minute)},
	} {
		if err := s.SaveBooking(bad); err == nil {
			t.Fatal("invalid booking accepted")
		}
	}
	b = s.Snapshot().Bookings[0]
	b.Notes = "updated"
	if err := s.SaveBooking(b); err != nil {
		t.Fatal(err)
	}
	if err := s.ToggleTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now.Add(time.Hour) }
	if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now, End: now.Add(time.Minute)}); err == nil {
		t.Fatal("booking overlaps active timer")
	}
	if err := s.StopTimer(); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBooking(b.ID); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot().Bookings) != 1 {
		t.Fatal("wrong booking deleted")
	}
}

func TestSaveFailureLeavesStateUnchanged(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	original := s.path
	s.path = filepath.Join(s.path, "no-directory", "data.json")
	if err := s.Favorite(p.ID); err == nil {
		t.Fatal("write failure not surfaced")
	}
	if !s.Snapshot().Projects[0].Favorite {
		t.Fatal("failed write changed memory")
	}
	s.path = original
	d := s.Snapshot()
	d.Projects[0].Name = "external mutation"
	if s.Snapshot().Projects[0].Name == "external mutation" {
		t.Fatal("snapshot not isolated")
	}
}

func TestCorruptFileIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	data := []byte(`{"version":900}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("corrupt data accepted")
	}
	read, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, read) {
		t.Fatal("corrupt file overwritten")
	}
}

func TestConcurrentMutations(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			if err := s.Favorite(p.ID); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if !s.Snapshot().Projects[0].Favorite {
		t.Fatal("lost concurrent mutation")
	}
}

func TestExcelDateFilterAndContent(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "001", "")
	p.Customer = "=1+1"
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return day.Add(72 * time.Hour) }
	for _, b := range []Booking{
		{ProjectID: p.ID, Start: day.Add(-time.Hour), End: day.Add(time.Hour)},
		{ProjectID: p.ID, Start: day.Add(23 * time.Hour), End: day.Add(25 * time.Hour)},
		{ProjectID: p.ID, Start: day.Add(26 * time.Hour), End: day.Add(27 * time.Hour)},
	} {
		if err := s.SaveBooking(b); err != nil {
			t.Fatal(err)
		}
	}
	data, err := Export(s.Snapshot(), "2026-11-02", "2026-11-02", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for cell, want := range map[string]string{"A2": "=1+1", "B2": "001", "F2": "1.00", "F3": "1.00", "E4": "Total hours", "F4": "2.00", "H2": "#16a085"} {
		got, err := f.GetCellValue("Bookings", cell)
		if err != nil || got != want {
			t.Errorf("%s = %q, want %q (%v)", cell, got, want, err)
		}
	}
	if formula, err := f.GetCellFormula("Bookings", "A2"); err != nil || formula != "" {
		t.Fatal("customer value interpreted as formula")
	}
	if _, err := Export(s.Snapshot(), "2026-11-03", "2026-11-02", time.UTC); err == nil {
		t.Fatal("reversed filter accepted")
	}
}

func TestDateRangeDST(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	a, z, err := DateRange("2026-10-25", "2026-10-25", loc)
	if err != nil || z.Sub(a) != 25*time.Hour {
		t.Fatalf("DST day range incorrect: %v %v", z.Sub(a), err)
	}
}
