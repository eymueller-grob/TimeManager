package tracker

import "time"

// ReminderClock is confined to the native reminder thread. Cadence starts at
// app launch, a timer transition, or a reminder configuration change.
type ReminderClock struct {
	initialized bool
	settings    ReminderSettings
	projectID   string
	start       time.Time
	next        time.Time
}

func (c *ReminderClock) Poll(now time.Time, d State) (due, reset bool) {
	id := ""
	var start time.Time
	if d.Active != nil {
		id, start = d.Active.ProjectID, d.Active.Start
	}
	r := d.Settings.Reminders
	reset = !c.initialized || r != c.settings || id != c.projectID || !start.Equal(c.start)
	minutes, enabled := r.IdleMinutes, r.IdleEnabled
	if d.Active != nil {
		minutes, enabled = r.RunningMinutes, r.RunningEnabled
	}
	if reset {
		c.initialized, c.settings, c.projectID, c.start = true, r, id, start
		c.next = now.Add(time.Duration(minutes) * time.Minute)
		return false, true
	}
	if !enabled {
		return false, false
	}
	if !now.Before(c.next) {
		// Resume from sleep produces one reminder, never a burst of missed ones.
		c.next = now.Add(time.Duration(minutes) * time.Minute)
		return true, false
	}
	return false, false
}
