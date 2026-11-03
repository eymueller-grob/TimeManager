package tracker

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID         string            `json:"id"`
	Number     string            `json:"number"`
	Name       string            `json:"name"`
	Customer   string            `json:"customer"`
	ParentID   string            `json:"parentId"`
	Color      string            `json:"color"`
	Favorite   bool              `json:"favorite"`
	Archived   bool              `json:"archived"`
	Properties map[string]string `json:"properties"`
}

type Booking struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Notes     string    `json:"notes"`
}

type Timer struct {
	ProjectID string    `json:"projectId"`
	Start     time.Time `json:"start"`
}

type State struct {
	Version             int                  `json:"version"`
	Projects            []Project            `json:"projects"`
	Bookings            []Booking            `json:"bookings"`
	Active              *Timer               `json:"active"`
	Settings            Settings             `json:"settings"`
	PropertyDefinitions []PropertyDefinition `json:"propertyDefinitions"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data State
	now  func() time.Time
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	s := &Store{path: path, now: time.Now, data: State{Version: 4, Projects: []Project{}, Bookings: []Booking{}, Settings: DefaultSettings(), PropertyDefinitions: []PropertyDefinition{}}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read saved data: %w", err)
	}
	var saved State
	if err := json.Unmarshal(b, &saved); err != nil {
		return nil, fmt.Errorf("saved data is invalid; restore a backup of %s: %w", path, err)
	}
	if err := validate(saved); err != nil {
		return nil, fmt.Errorf("saved data is invalid: %w", err)
	}
	s.data = saved
	if s.data.Version < 4 {
		if err := s.change(func(d *State) error {
			if d.Version == 1 {
				d.Settings = DefaultSettings()
				d.PropertyDefinitions = []PropertyDefinition{}
				for i := range d.Projects {
					d.Projects[i].Properties = map[string]string{}
				}
			}
			if d.Version < 3 {
				d.Settings.Reminders = DefaultReminders()
			}
			migrateProjectNames(d)
			d.Version = 4
			return nil
		}); err != nil {
			return nil, fmt.Errorf("migrate saved data to version 4: %w", err)
		}
	}
	if s.data.Projects == nil {
		s.data.Projects = []Project{}
	}
	if s.data.Bookings == nil {
		s.data.Bookings = []Booking{}
	}
	if s.data.PropertyDefinitions == nil {
		s.data.PropertyDefinitions = []PropertyDefinition{}
	}
	for i := range s.data.Projects {
		if s.data.Projects[i].Properties == nil {
			s.data.Projects[i].Properties = map[string]string{}
		}
	}
	return s, nil
}

func migrateProjectNames(d *State) {
	reserved := make(map[string]bool)
	for _, p := range d.Projects {
		reserved[strings.ToLower(strings.TrimSpace(p.Name))] = true
	}
	seen := make(map[string]bool)
	for i, p := range d.Projects {
		key := strings.ToLower(strings.TrimSpace(p.Name))
		if !seen[key] {
			seen[key] = true
			continue
		}
		for suffix := 2; ; suffix++ {
			name := fmt.Sprintf("%s (%d)", strings.TrimSpace(p.Name), suffix)
			key = strings.ToLower(name)
			if !reserved[key] {
				d.Projects[i].Name = name
				reserved[key] = true
				break
			}
		}
	}
}

func clone(d State) State {
	d.Projects = append([]Project{}, d.Projects...)
	for i := range d.Projects {
		d.Projects[i].Properties = maps.Clone(d.Projects[i].Properties)
	}
	d.PropertyDefinitions = append([]PropertyDefinition{}, d.PropertyDefinitions...)
	for i := range d.PropertyDefinitions {
		d.PropertyDefinitions[i].Options = append([]string{}, d.PropertyDefinitions[i].Options...)
	}
	d.Bookings = append([]Booking{}, d.Bookings...)
	if d.Active != nil {
		t := *d.Active
		d.Active = &t
	}
	return d
}

func (s *Store) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.data)
}

func (s *Store) change(f func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := clone(s.data)
	if err := f(&d); err != nil {
		return err
	}
	if err := validate(d); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("encode data: %w", err)
	}
	if err := WriteAtomic(s.path, b); err != nil {
		return fmt.Errorf("save data (change was not applied): %w", err)
	}
	s.data = d
	return nil
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func project(d State, id string) (Project, bool) {
	for _, p := range d.Projects {
		if p.ID == id {
			return p, true
		}
	}
	return Project{}, false
}

func validate(d State) error {
	if d.Version != 1 && d.Version != 2 && d.Version != 3 && d.Version != 4 {
		return fmt.Errorf("unsupported data version %d", d.Version)
	}
	if d.Version >= 2 {
		if err := validateSettings(d); err != nil {
			return err
		}
		if d.Version >= 3 {
			if err := validateReminders(d.Settings.Reminders); err != nil {
				return err
			}
		}
	}
	ids := make(map[string]bool)
	numbers := make(map[string]bool)
	names := make(map[string]bool)
	for _, p := range d.Projects {
		if p.ID == "" || ids[p.ID] {
			return errors.New("project IDs must be unique and nonempty")
		}
		ids[p.ID] = true
		if strings.TrimSpace(p.Number) == "" || strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Customer) == "" {
			return errors.New("project number, name, and customer are required")
		}
		key := strings.ToLower(strings.TrimSpace(p.Number))
		if d.Version < 4 && numbers[key] {
			return errors.New("project numbers must be unique (including archived projects)")
		}
		numbers[key] = true
		nameKey := strings.ToLower(strings.TrimSpace(p.Name))
		if d.Version >= 4 && names[nameKey] {
			return errors.New("project names must be unique (including archived projects)")
		}
		names[nameKey] = true
		if !colorPattern.MatchString(p.Color) {
			return errors.New("project color must be a six-digit hex color")
		}
		if p.Archived && p.Favorite {
			return errors.New("archived projects cannot be favorites")
		}
	}
	for _, p := range d.Projects {
		seen := map[string]bool{p.ID: true}
		parentID := p.ParentID
		for parentID != "" {
			if seen[parentID] {
				return errors.New("subproject hierarchy cannot contain a cycle")
			}
			seen[parentID] = true
			parent, ok := project(d, parentID)
			if !ok {
				return errors.New("parent project does not exist")
			}
			if !p.Archived && parent.Archived {
				return errors.New("an active subproject cannot have an archived parent")
			}
			parentID = parent.ParentID
		}
	}
	bookingIDs := make(map[string]bool)
	for i, b := range d.Bookings {
		if b.ID == "" || bookingIDs[b.ID] {
			return errors.New("booking IDs must be unique and nonempty")
		}
		bookingIDs[b.ID] = true
		if _, ok := project(d, b.ProjectID); !ok {
			return errors.New("booking project does not exist")
		}
		if b.Start.IsZero() || !b.End.After(b.Start) {
			return errors.New("booking end must be after its start")
		}
		for _, other := range d.Bookings[:i] {
			if b.Start.Before(other.End) && other.Start.Before(b.End) {
				return errors.New("bookings cannot overlap; edit the conflicting booking first")
			}
		}
		if d.Active != nil && b.End.After(d.Active.Start) {
			return errors.New("booking overlaps the running timer; stop the timer first")
		}
	}
	if d.Active != nil {
		p, ok := project(d, d.Active.ProjectID)
		if !ok || p.Archived || d.Active.Start.IsZero() {
			return errors.New("running timer must reference an active project and have a start time")
		}
	}
	return nil
}

func (s *Store) SaveProject(p Project) error {
	return s.change(func(d *State) error {
		p.Number = strings.TrimSpace(p.Number)
		p.Name = strings.TrimSpace(p.Name)
		p.Customer = strings.TrimSpace(p.Customer)
		p.Properties = maps.Clone(p.Properties)
		if p.Properties == nil {
			p.Properties = map[string]string{}
		}
		for id, value := range p.Properties {
			value = strings.TrimSpace(value)
			if value == "" {
				delete(p.Properties, id)
			} else {
				p.Properties[id] = value
			}
		}
		if p.ID == "" {
			if p.Archived {
				return errors.New("new projects cannot be archived")
			}
			p.ID = uuid.NewString()
			d.Projects = append(d.Projects, p)
			return nil
		}
		for i, old := range d.Projects {
			if old.ID == p.ID {
				if p.Archived != old.Archived {
					return errors.New("use Delete/Archive or Restore to change archive status")
				}
				d.Projects[i] = p
				return nil
			}
		}
		return errors.New("project no longer exists")
	})
}

func (s *Store) Favorite(id string) error {
	return s.change(func(d *State) error {
		for i, p := range d.Projects {
			if p.ID == id {
				if p.Archived {
					return errors.New("restore this project before favoriting it")
				}
				d.Projects[i].Favorite = !p.Favorite
				return nil
			}
		}
		return errors.New("project no longer exists")
	})
}

func (s *Store) DeleteProject(id string) error {
	return s.change(func(d *State) error {
		if _, ok := project(*d, id); !ok {
			return errors.New("project no longer exists")
		}
		for _, p := range d.Projects {
			if p.ParentID == id {
				return errors.New("delete, archive and move, or reparent subprojects first")
			}
		}
		if d.Active != nil && d.Active.ProjectID == id {
			return errors.New("stop this project's timer before deleting or archiving it")
		}
		used := false
		for _, b := range d.Bookings {
			if b.ProjectID == id {
				used = true
			}
		}
		for i, p := range d.Projects {
			if p.ID == id {
				if used {
					d.Projects[i].Archived = true
					d.Projects[i].Favorite = false
				} else {
					d.Projects = append(d.Projects[:i], d.Projects[i+1:]...)
				}
				return nil
			}
		}
		return nil
	})
}

func (s *Store) RestoreProject(id string) error {
	return s.change(func(d *State) error {
		for i, p := range d.Projects {
			if p.ID == id {
				d.Projects[i].Archived = false
				return nil
			}
		}
		return errors.New("project no longer exists")
	})
}

func stop(d *State, now time.Time) error {
	if d.Active == nil {
		return nil
	}
	if d.Active.ProjectID == temporaryProjectID {
		return ErrTemporaryAssignment
	}
	if !now.After(d.Active.Start) {
		return errors.New("system time is not after the timer start; correct the clock before stopping")
	}
	d.Bookings = append(d.Bookings, Booking{
		ID: uuid.NewString(), ProjectID: d.Active.ProjectID, Start: d.Active.Start, End: now,
	})
	d.Active = nil
	return nil
}

func (s *Store) ToggleTimer(id string) error {
	return s.changeTimer(id, true, false)
}

func (s *Store) StartFavoriteTimer(id string) error {
	return s.changeTimer(id, false, true)
}

func (s *Store) StartTimer(id string) error {
	return s.changeTimer(id, false, false)
}

const temporaryProjectID = "timemanager-temporary-project"

var ErrTemporaryAssignment = errors.New("assign the temporary timer to a project before ending it")

func (s *Store) FinishTemporaryTimer(expected Timer, targetID, nextID string, favoriteOnly bool) error {
	return s.change(func(d *State) error {
		if d.Active == nil || d.Active.ProjectID != temporaryProjectID ||
			expected.ProjectID != temporaryProjectID || !d.Active.Start.Equal(expected.Start) {
			return errors.New("temporary timer changed; cancel and try again")
		}
		target, ok := project(*d, targetID)
		if !ok || target.Archived || target.ID == temporaryProjectID {
			return errors.New("choose an active non-temporary project to assign this time")
		}
		now := s.now()
		d.Active.ProjectID = target.ID
		if err := stop(d, now); err != nil {
			return err
		}
		if nextID != "" {
			return changeTimer(d, nextID, now, false, favoriteOnly)
		}
		return nil
	})
}

func (s *Store) StartTemporaryTimer() error {
	return s.change(func(d *State) error {
		p, exists := project(*d, temporaryProjectID)
		if !exists {
			name := "Temporary / Unassigned"
			for suffix := 2; ; suffix++ {
				used := false
				for _, other := range d.Projects {
					if strings.EqualFold(strings.TrimSpace(other.Name), name) {
						used = true
						break
					}
				}
				if !used {
					break
				}
				name = fmt.Sprintf("Temporary / Unassigned (%d)", suffix)
			}
			p = Project{ID: temporaryProjectID, Number: "TEMP", Name: name, Customer: "Unassigned", Color: "#64748b", Properties: map[string]string{}}
			d.Projects = append(d.Projects, p)
		} else if p.Archived {
			return errors.New("restore the temporary project in Projects before starting temporary tracking")
		}
		return changeTimer(d, p.ID, s.now(), false, false)
	})
}

func (s *Store) changeTimer(id string, toggle, favoriteOnly bool) error {
	return s.change(func(d *State) error {
		return changeTimer(d, id, s.now(), toggle, favoriteOnly)
	})
}

func changeTimer(d *State, id string, now time.Time, toggle, favoriteOnly bool) error {
	p, ok := project(*d, id)
	if !ok || p.Archived {
		return errors.New("select an active project")
	}
	if favoriteOnly && !p.Favorite {
		return errors.New("project is no longer a favorite; select another project")
	}
	same := d.Active != nil && d.Active.ProjectID == id
	if !toggle && same {
		return nil
	}
	if err := stop(d, now); err != nil {
		return err
	}
	if !same {
		d.Active = &Timer{ProjectID: id, Start: now}
	}
	return nil
}

func (s *Store) StopTimer() error {
	return s.change(func(d *State) error { return stop(d, s.now()) })
}

func (s *Store) UpdateTimerStart(expected Timer, start time.Time) error {
	return s.change(func(d *State) error {
		if d.Active == nil {
			return errors.New("no timer is running")
		}
		if d.Active.ProjectID != expected.ProjectID || !d.Active.Start.Equal(expected.Start) {
			return errors.New("running timer changed; reopen the start time editor")
		}
		if start.IsZero() {
			return errors.New("timer start time is required")
		}
		if start.After(s.now()) {
			return errors.New("timer start cannot be in the future")
		}
		for _, booking := range d.Bookings {
			if booking.End.After(start) {
				return errors.New("timer start overlaps a saved booking; choose a start at or after the latest booking end")
			}
		}
		d.Active.Start = start
		return nil
	})
}

func (s *Store) SaveBooking(b Booking) error {
	return s.change(func(d *State) error {
		p, ok := project(*d, b.ProjectID)
		if !ok {
			return errors.New("select a project")
		}
		if b.End.After(s.now()) {
			return errors.New("bookings cannot end in the future")
		}
		b.Notes = strings.TrimSpace(b.Notes)
		if b.ID == "" {
			if p.Archived {
				return errors.New("cannot create a booking on an archived project")
			}
			b.ID = uuid.NewString()
			d.Bookings = append(d.Bookings, b)
			return nil
		}
		for i, old := range d.Bookings {
			if old.ID == b.ID {
				if p.Archived && old.ProjectID != b.ProjectID {
					return errors.New("cannot move a booking to an archived project")
				}
				d.Bookings[i] = b
				return nil
			}
		}
		return errors.New("booking no longer exists")
	})
}

func (s *Store) DeleteBooking(id string) error {
	return s.change(func(d *State) error {
		for i, b := range d.Bookings {
			if b.ID == id {
				d.Bookings = append(d.Bookings[:i], d.Bookings[i+1:]...)
				return nil
			}
		}
		return errors.New("booking no longer exists")
	})
}

func WriteAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".timemanager-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return replaceFile(f.Name(), path)
}
