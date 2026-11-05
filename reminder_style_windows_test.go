package main

import (
	"image/color"
	"testing"
	"time"

	"TimeManager/internal/tracker"
)

func TestReminderContentFor(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	project := tracker.Project{ID: "p", Number: "100", Name: "Alpha", Color: "#2563eb", Icon: "data:image/png;base64,x"}
	running := tracker.State{Projects: []tracker.Project{project}, Active: &tracker.Timer{ProjectID: "p", Start: now.Add(-(time.Hour + 2*time.Minute + 3*time.Second))}}
	for _, test := range []struct {
		name             string
		state            tracker.State
		running, preview bool
		want             reminderContent
	}{
		{"idle", tracker.State{}, false, false, reminderContent{Eyebrow: "NO TIMER RUNNING", Title: "No timer is running",
			Body: "Remember to start a project timer if you are working.", Accent: reminderIdleAccent}},
		{"running project", running, true, false, reminderContent{Eyebrow: "STILL TRACKING", Title: "100 - Alpha",
			Body: "Remember to stop and save your timer when you finish working.", Elapsed: "01:02:03",
			Accent: mustTrayColor(t, "#2563eb"), Icon: project.Icon}},
		{"running preview without timer", tracker.State{}, true, true, reminderContent{Eyebrow: "PREVIEW · STILL TRACKING",
			Title: "Your timer is still running", Body: "Remember to stop and save your timer when you finish working.", Accent: reminderRunningAccent}},
		{"idle preview", tracker.State{}, false, true, reminderContent{Eyebrow: "PREVIEW · NO TIMER RUNNING", Title: "No timer is running",
			Body: "Remember to start a project timer if you are working.", Accent: reminderIdleAccent}},
		{"future start clamps to zero", tracker.State{Projects: running.Projects, Active: &tracker.Timer{ProjectID: "p", Start: now.Add(time.Minute)}}, true, false,
			reminderContent{Eyebrow: "STILL TRACKING", Title: "100 - Alpha", Body: "Remember to stop and save your timer when you finish working.",
				Elapsed: "00:00:00", Accent: mustTrayColor(t, "#2563eb"), Icon: project.Icon}},
		{"invalid project color keeps running accent", tracker.State{Projects: []tracker.Project{{ID: "p", Number: "1", Name: "X", Color: "blue"}},
			Active: &tracker.Timer{ProjectID: "p", Start: now}}, true, false,
			reminderContent{Eyebrow: "STILL TRACKING", Title: "1 - X", Body: "Remember to stop and save your timer when you finish working.",
				Elapsed: "00:00:00", Accent: reminderRunningAccent}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reminderContentFor(test.state, test.running, test.preview, now); got != test.want {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

func mustTrayColor(t *testing.T, hex string) color.NRGBA {
	t.Helper()
	c, err := parseTrayColor(hex)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestReminderHit(t *testing.T) {
	for _, test := range []struct {
		x, y int32
		want int
	}{
		{132, 120, reminderDismissID}, {219, 151, reminderDismissID}, {220, 130, 0},
		{228, 120, reminderOpenID}, {375, 151, reminderOpenID}, {376, 130, 0},
		{200, 119, 0}, {300, 152, 0}, {24, 40, 0},
	} {
		if got := reminderHit(test.x, test.y); got != test.want {
			t.Errorf("reminderHit(%d, %d) = %d, want %d", test.x, test.y, got, test.want)
		}
	}
}

func TestReminderPalettes(t *testing.T) {
	light, dark := reminderPaletteFor(false), reminderPaletteFor(true)
	win95 := reminderPaletteWin95()
	if light == dark {
		t.Fatal("light and dark palettes are identical")
	}
	for name, p := range map[string]reminderPalette{"light": light, "dark": dark, "win95": win95} {
		if p.Background == p.Foreground || p.Primary == p.PrimaryText || p.Primary == p.PrimaryHover || p.Background == p.OutlineHover {
			t.Errorf("%s palette lacks contrast or hover feedback: %+v", name, p)
		}
	}
	if got := colorRef(light.Primary); got != 0x171717 {
		t.Fatalf("colorRef = %#x", got)
	}
	if got := colorRef(reminderRunningAccent); got != 0x4aa316 {
		t.Fatalf("colorRef uses BGR order: %#x", got)
	}
}
