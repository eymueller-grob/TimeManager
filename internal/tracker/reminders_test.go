package tracker

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestReminderClockBoundariesAndRepeats(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	d := State{Settings: DefaultSettings()}
	var clock ReminderClock
	if due, reset := clock.Poll(now, d); due || !reset {
		t.Fatal("initial poll should initialize, not remind")
	}
	if due, _ := clock.Poll(now.Add(30*time.Minute-time.Nanosecond), d); due {
		t.Fatal("idle reminder fired before 30 minutes")
	}
	if due, reset := clock.Poll(now.Add(30*time.Minute), d); !due || reset {
		t.Fatal("idle reminder did not fire exactly at 30 minutes")
	}
	if due, _ := clock.Poll(now.Add(30*time.Minute), d); due {
		t.Fatal("duplicate reminder")
	}
	if due, _ := clock.Poll(now.Add(60*time.Minute), d); !due {
		t.Fatal("idle reminder did not repeat")
	}
	d.Active = &Timer{ProjectID: "p", Start: now.Add(61 * time.Minute)}
	if due, reset := clock.Poll(now.Add(61*time.Minute), d); due || !reset {
		t.Fatal("starting timer must reset cadence")
	}
	if due, _ := clock.Poll(now.Add(121*time.Minute-time.Nanosecond), d); due {
		t.Fatal("running reminder fired before 60 minutes")
	}
	if due, _ := clock.Poll(now.Add(121*time.Minute), d); !due {
		t.Fatal("running reminder not due")
	}
	if due, _ := clock.Poll(now.Add(181*time.Minute), d); !due {
		t.Fatal("running reminder did not repeat")
	}
}

func TestReminderClockTransitionsSettingsAndDisabledStates(t *testing.T) {
	now := time.Now()
	d := State{Settings: DefaultSettings(), Active: &Timer{ProjectID: "p", Start: now}}
	var clock ReminderClock
	clock.Poll(now, d)
	d.Active = &Timer{ProjectID: "q", Start: now.Add(time.Minute)}
	if due, reset := clock.Poll(now.Add(time.Minute), d); due || !reset {
		t.Fatal("switch must reset")
	}
	d.Active.Start = now.Add(2 * time.Minute)
	if _, reset := clock.Poll(now.Add(2*time.Minute), d); !reset {
		t.Fatal("new timer on same project must reset")
	}
	d.Settings.Theme = "dark"
	if _, reset := clock.Poll(now.Add(3*time.Minute), d); reset {
		t.Fatal("appearance must not reset reminders")
	}
	d.Settings.Reminders.RunningEnabled = false
	clock.Poll(now.Add(4*time.Minute), d)
	if due, _ := clock.Poll(now.Add(24*time.Hour), d); due {
		t.Fatal("disabled running reminder fired")
	}
	d.Active = nil
	if _, reset := clock.Poll(now.Add(25*time.Hour), d); !reset {
		t.Fatal("stopping must reset idle countdown")
	}
	if due, _ := clock.Poll(now.Add(25*time.Hour+30*time.Minute), d); !due {
		t.Fatal("idle reminder incorrectly disabled with running")
	}
	d.Settings.Reminders.IdleMinutes = 1
	if _, reset := clock.Poll(now.Add(26*time.Hour), d); !reset {
		t.Fatal("interval changes must reset")
	}
	if due, _ := clock.Poll(now.Add(26*time.Hour+time.Minute), d); !due {
		t.Fatal("custom one-minute interval not honored")
	}
	d.Settings.Reminders.IdleEnabled = false
	clock.Poll(now.Add(27*time.Hour), d)
	if due, _ := clock.Poll(now.Add(48*time.Hour), d); due {
		t.Fatal("disabled idle reminder fired")
	}
}

func TestReminderClockSleepDoesNotBurst(t *testing.T) {
	now := time.Now()
	d := State{Settings: DefaultSettings()}
	var clock ReminderClock
	clock.Poll(now, d)
	resume := now.Add(48 * time.Hour)
	if due, _ := clock.Poll(resume, d); !due {
		t.Fatal("overdue reminder not shown after sleep")
	}
	if due, _ := clock.Poll(resume.Add(time.Second), d); due {
		t.Fatal("missed reminders replayed in burst")
	}
	if due, _ := clock.Poll(resume.Add(30*time.Minute), d); !due {
		t.Fatal("cadence did not restart after overdue reminder")
	}
}

func TestMigrateVersionTwoPreservesCustomPropertiesAndTheme(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "1", "")
	def := addDefinition(t, s, "Contact", "text", nil)
	p.Properties = map[string]string{def.ID: "Jane"}
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	d := s.Snapshot()
	d.Version = 2
	d.Settings = Settings{Theme: "dark"}
	b, err := json.Marshal(d)
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
	result := reopened.Snapshot()
	if result.Version != 7 || result.Settings.Theme != "dark" || result.Settings.Reminders != DefaultReminders() ||
		result.Projects[0].Properties[def.ID] != "Jane" || result.PropertyDefinitions[0].ID != def.ID {
		t.Fatal("version 2 migration lost existing preferences or properties")
	}
}

func TestReminderSettingsValidationAndPersistence(t *testing.T) {
	s := newStore(t)
	for _, bad := range []ReminderSettings{
		{RunningMinutes: 0, IdleMinutes: 30}, {RunningMinutes: 60, IdleMinutes: -1}, {RunningMinutes: int((1<<63-1)/int64(time.Minute)) + 1, IdleMinutes: 30},
	} {
		settings := DefaultSettings()
		settings.Reminders = bad
		if err := s.SaveSettings(settings); err == nil {
			t.Fatal("invalid interval accepted")
		}
		if s.Snapshot().Settings.Reminders != DefaultReminders() {
			t.Fatal("failed validation changed state")
		}
	}
	settings := DefaultSettings()
	settings.Reminders = ReminderSettings{RunningEnabled: false, RunningMinutes: 90, IdleEnabled: true, IdleMinutes: 45}
	if err := s.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil || reopened.Snapshot().Settings.Reminders != settings.Reminders {
		t.Fatalf("reminder settings not persisted: %v", err)
	}
}
