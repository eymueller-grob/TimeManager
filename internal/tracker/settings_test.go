package tracker

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestMigrateVersionOnePreservesData(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "001", "")
	if err := s.ToggleTimer(p.ID); err != nil {
		t.Fatal(err)
	}
	legacy := s.Snapshot()
	legacy.Version = 1
	legacy.Settings = Settings{}
	legacy.PropertyDefinitions = nil
	b, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, b, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	d := reopened.Snapshot()
	if d.Version != CurrentVersion || d.Settings.Theme != "light" || d.Settings.Reminders != DefaultReminders() || d.PropertyDefinitions == nil || d.Projects[0].ID != p.ID || !d.Active.Start.Equal(legacy.Active.Start) {
		t.Fatalf("migration lost data: %#v", d)
	}
	saved, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted State
	if err := json.Unmarshal(saved, &persisted); err != nil || persisted.Version != CurrentVersion {
		t.Fatalf("migration was not persisted: %v", err)
	}
	if _, err := Open(s.path); err != nil {
		t.Fatalf("reopening migrated data: %v", err)
	}
}

func TestInvalidLegacyDataNotMigrated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	b := []byte(`{"version":1,"projects":[{"id":"1","number":"1"}]}`)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("invalid legacy data accepted")
	}
	read, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(read, b) {
		t.Fatal("invalid legacy data overwritten")
	}
}

func TestSettingsPersistenceAndFailure(t *testing.T) {
	s := newStore(t)
	for _, theme := range []string{"dark", "system", "light", "win95"} {
		settings := DefaultSettings()
		settings.Theme = theme
		if err := s.SaveSettings(settings); err != nil {
			t.Fatal(err)
		}
		reopened, err := Open(s.path)
		if err != nil || reopened.Snapshot().Settings.Theme != theme {
			t.Fatalf("theme not saved: %v", err)
		}
	}
	if err := s.SaveSettings(Settings{Theme: "invalid"}); err == nil {
		t.Fatal("invalid theme accepted")
	}
	s.path = filepath.Join(t.TempDir(), "missing", "data.json")
	if err := s.SaveSettings(Settings{Theme: "dark"}); err == nil || s.Snapshot().Settings.Theme != "win95" {
		t.Fatal("failed settings save mutated memory")
	}
}

func addDefinition(t *testing.T, s *Store, name, kind string, options []string) PropertyDefinition {
	t.Helper()
	if err := s.SavePropertyDefinition(PropertyDefinition{Name: name, Type: kind, Options: options}); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot().PropertyDefinitions
	return d[len(d)-1]
}

func TestPropertyValuesValidationAndIsolation(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	text := addDefinition(t, s, "Contact", "text", nil)
	number := addDefinition(t, s, "Rate", "number", nil)
	check := addDefinition(t, s, "Billable", "checkbox", nil)
	drop := addDefinition(t, s, "Status", "dropdown", []string{"Open", "Closed"})
	for _, test := range []struct {
		id, value string
	}{
		{number.ID, "NaN"}, {number.ID, "Inf"}, {number.ID, "abc"},
		{check.ID, "yes"}, {drop.ID, "Missing"}, {"unknown", "x"},
	} {
		t.Run(test.id+test.value, func(t *testing.T) {
			bad := p
			bad.Properties = map[string]string{test.id: test.value}
			if err := s.SaveProject(bad); err == nil {
				t.Fatal("invalid property value accepted")
			}
		})
	}
	p.Properties = map[string]string{text.ID: "Jane", number.ID: "125.5", check.ID: "false", drop.ID: "Open"}
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	p.Properties[text.ID] = "External mutation"
	d := s.Snapshot()
	if d.Projects[0].Properties[text.ID] != "Jane" {
		t.Fatal("store shares input maps")
	}
	d.Projects[0].Properties[text.ID] = "Snapshot mutation"
	d.PropertyDefinitions[3].Options[0] = "Changed"
	if s.Snapshot().Projects[0].Properties[text.ID] != "Jane" || s.Snapshot().PropertyDefinitions[3].Options[0] != "Open" {
		t.Fatal("snapshot shares maps or slices")
	}
	drop.Options = []string{"Closed"}
	if err := s.SavePropertyDefinition(drop); err == nil {
		t.Fatal("removed used dropdown option")
	}
	drop.Options = []string{"Open", "Closed"}
	drop.Name = "Stage"
	if err := s.SavePropertyDefinition(drop); err != nil {
		t.Fatal(err)
	}
	drop.Type = "text"
	if err := s.SavePropertyDefinition(drop); err == nil {
		t.Fatal("changed existing property type")
	}
}

func TestMultiselectPropertyValues(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	multi := addDefinition(t, s, "Teams", "multiselect", []string{"Dev", "QA", "Ops"})
	for _, test := range []struct{ name, value string }{
		{"not JSON", "Dev"}, {"empty list", "[]"}, {"duplicate", `["Dev","Dev"]`},
		{"unknown option", `["Dev","Sales"]`}, {"not a list", `"Dev"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			bad := p
			bad.Properties = map[string]string{multi.ID: test.value}
			if err := s.SaveProject(bad); err == nil {
				t.Fatal("invalid multiselect value accepted")
			}
		})
	}
	p.Properties = map[string]string{multi.ID: `["QA","Dev"]`}
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	multi.Options = []string{"Dev", "Ops"}
	if err := s.SavePropertyDefinition(multi); err == nil {
		t.Fatal("removed used multiselect option")
	}
	multi.Options = []string{"Dev", "QA"}
	if err := s.SavePropertyDefinition(multi); err != nil {
		t.Fatalf("removing unused option: %v", err)
	}
}

func TestDefinitionDeletionIsAtomicAndIncludesArchivedProjects(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	def := addDefinition(t, s, "Contact", "text", nil)
	p.Properties = map[string]string{def.ID: "Jane"}
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	now := s.now()
	if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-time.Hour), End: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	path := s.path
	s.path = filepath.Join(t.TempDir(), "missing", "data.json")
	if err := s.DeletePropertyDefinition(def.ID); err == nil {
		t.Fatal("missing directory save succeeded")
	}
	if s.Snapshot().Projects[0].Properties[def.ID] != "Jane" || len(s.Snapshot().PropertyDefinitions) != 1 {
		t.Fatal("failed delete mutated state")
	}
	s.path = path
	if err := s.DeletePropertyDefinition(def.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	d := reopened.Snapshot()
	if len(d.PropertyDefinitions) != 0 || len(d.Projects[0].Properties) != 0 || !d.Projects[0].Archived || len(d.Bookings) != 1 {
		t.Fatal("definition deletion lost unrelated data or retained values")
	}
}

func TestDefinitionValidation(t *testing.T) {
	s := newStore(t)
	addDefinition(t, s, "Contact", "text", nil)
	for _, bad := range []PropertyDefinition{
		{Name: "", Type: "text"}, {Name: "contact", Type: "text"},
		{Name: "X", Type: "invalid"}, {Name: "X", Type: "dropdown"},
		{Name: "X", Type: "dropdown", Options: []string{"A", "A"}},
		{Name: "X", Type: "multiselect"},
		{Name: "X", Type: "multiselect", Options: []string{"A", " "}},
		{Name: "X", Type: "number", Options: []string{"A"}},
	} {
		if err := s.SavePropertyDefinition(bad); err == nil {
			t.Fatalf("invalid definition accepted: %#v", bad)
		}
	}
}

func TestExportCustomPropertiesTypes(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	text := addDefinition(t, s, "Contact", "text", nil)
	number := addDefinition(t, s, "Rate", "number", nil)
	check := addDefinition(t, s, "Billable", "checkbox", nil)
	drop := addDefinition(t, s, "Status", "dropdown", []string{"Open", "Closed"})
	multi := addDefinition(t, s, "Teams", "multiselect", []string{"Dev", "QA"})
	p.Properties = map[string]string{text.ID: "=1+1", number.ID: "125.5", check.ID: "false", drop.ID: "Open", multi.ID: `["QA","Dev"]`}
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	now := s.now()
	if err := s.SaveBooking(Booking{ProjectID: p.ID, Start: now.Add(-time.Hour), End: now}); err != nil {
		t.Fatal(err)
	}
	b, err := Export(s.Snapshot(), "2026-11-03", "2026-11-03", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for cell, want := range map[string]string{"I1": "Property: Contact", "I2": "=1+1", "J2": "125.5", "K2": "FALSE", "L2": "Open", "M2": "QA, Dev"} {
		got, err := f.GetCellValue("Bookings", cell)
		if err != nil || got != want {
			t.Errorf("%s = %q, want %q (%v)", cell, got, want, err)
		}
	}
	if formula, err := f.GetCellFormula("Bookings", "I2"); err != nil || formula != "" {
		t.Fatal("text property became formula")
	}
	// Excel permits an omitted cell type for numeric cells.
	if kind, err := f.GetCellType("Bookings", "J2"); err != nil || (kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset) {
		t.Fatalf("number property not exported as numeric cell: %v %v", kind, err)
	}
}
