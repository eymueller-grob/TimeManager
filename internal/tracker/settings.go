package tracker

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Settings struct {
	RequireTemporaryProject bool             `json:"requireTemporaryProject"`
	Theme                   string           `json:"theme"`
	Reminders               ReminderSettings `json:"reminders"`
}

type ReminderSettings struct {
	RunningEnabled bool `json:"runningEnabled"`
	RunningMinutes int  `json:"runningMinutes"`
	IdleEnabled    bool `json:"idleEnabled"`
	IdleMinutes    int  `json:"idleMinutes"`
}

func DefaultReminders() ReminderSettings {
	return ReminderSettings{RunningEnabled: true, RunningMinutes: 60, IdleEnabled: true, IdleMinutes: 30}
}

func DefaultSettings() Settings {
	return Settings{Theme: "light", Reminders: DefaultReminders()}
}

func validateReminders(r ReminderSettings) error {
	if r.RunningMinutes < 1 || r.IdleMinutes < 1 {
		return errors.New("reminder intervals must be positive whole minutes")
	}
	const maxMinutes = int((1<<63 - 1) / int64(time.Minute))
	if r.RunningMinutes > maxMinutes || r.IdleMinutes > maxMinutes {
		return errors.New("reminder interval exceeds the supported time duration")
	}
	return nil
}

type PropertyDefinition struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Options []string `json:"options"`
}

func validateSettings(d State) error {
	switch d.Settings.Theme {
	case "light", "dark", "system":
	default:
		return errors.New("appearance must be light, dark, or system")
	}
	definitions := make(map[string]PropertyDefinition)
	names := make(map[string]bool)
	for _, def := range d.PropertyDefinitions {
		if def.ID == "" {
			return errors.New("property ID cannot be empty")
		}
		if _, exists := definitions[def.ID]; exists {
			return errors.New("property IDs must be unique")
		}
		name := strings.TrimSpace(def.Name)
		if name == "" {
			return errors.New("property name is required")
		}
		key := strings.ToLower(name)
		if names[key] {
			return errors.New("property names must be unique")
		}
		names[key] = true
		switch def.Type {
		case "text", "number", "checkbox":
			if len(def.Options) != 0 {
				return errors.New("only dropdown properties can have options")
			}
		case "dropdown":
			if len(def.Options) == 0 {
				return errors.New("dropdown properties require at least one option")
			}
			seen := make(map[string]bool)
			for _, option := range def.Options {
				if strings.TrimSpace(option) == "" || seen[option] {
					return errors.New("dropdown options must be nonempty and unique")
				}
				seen[option] = true
			}
		default:
			return errors.New("property type must be text, number, checkbox, or dropdown")
		}
		definitions[def.ID] = def
	}
	for _, p := range d.Projects {
		for id, value := range p.Properties {
			def, ok := definitions[id]
			if !ok {
				return fmt.Errorf("project %s references a removed or unknown property; reopen its editor", p.Number)
			}
			if err := validatePropertyValue(def, value); err != nil {
				return fmt.Errorf("project %s, property %q: %w", p.Number, def.Name, err)
			}
		}
	}
	return nil
}

func validatePropertyValue(def PropertyDefinition, value string) error {
	switch def.Type {
	case "number":
		n, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
			return errors.New("enter a finite number or clear the value")
		}
	case "checkbox":
		if value != "true" && value != "false" {
			return errors.New("checkbox value must be true or false")
		}
	case "dropdown":
		if !slices.Contains(def.Options, value) {
			return errors.New("selected dropdown option is no longer available; update the project value first")
		}
	}
	return nil
}

func (s *Store) SaveSettings(settings Settings) error {
	return s.change(func(d *State) error {
		d.Settings = settings
		return nil
	})
}

func (s *Store) SavePropertyDefinition(def PropertyDefinition) error {
	return s.change(func(d *State) error {
		def.Name = strings.TrimSpace(def.Name)
		def.Options = append([]string{}, def.Options...)
		for i := range def.Options {
			def.Options[i] = strings.TrimSpace(def.Options[i])
		}
		if def.ID == "" {
			def.ID = uuid.NewString()
			d.PropertyDefinitions = append(d.PropertyDefinitions, def)
			return nil
		}
		for i, old := range d.PropertyDefinitions {
			if old.ID == def.ID {
				if old.Type != def.Type {
					return errors.New("an existing property's type cannot be changed; create a new property instead")
				}
				d.PropertyDefinitions[i] = def
				return nil
			}
		}
		return errors.New("property definition no longer exists")
	})
}

func (s *Store) DeletePropertyDefinition(id string) error {
	return s.change(func(d *State) error {
		for i, def := range d.PropertyDefinitions {
			if def.ID == id {
				d.PropertyDefinitions = append(d.PropertyDefinitions[:i], d.PropertyDefinitions[i+1:]...)
				for i := range d.Projects {
					delete(d.Projects[i].Properties, id)
				}
				return nil
			}
		}
		return errors.New("property definition no longer exists")
	})
}

func (s *Store) MovePropertyDefinition(id string, direction int) error {
	return s.change(func(d *State) error {
		if direction != -1 && direction != 1 {
			return errors.New("property move direction must be -1 or 1")
		}
		for i, def := range d.PropertyDefinitions {
			if def.ID != id {
				continue
			}
			next := i + direction
			if next < 0 || next >= len(d.PropertyDefinitions) {
				return errors.New("property is already at the end of the list in that direction")
			}
			d.PropertyDefinitions[i], d.PropertyDefinitions[next] = d.PropertyDefinitions[next], d.PropertyDefinitions[i]
			return nil
		}
		return errors.New("property definition no longer exists")
	})
}
