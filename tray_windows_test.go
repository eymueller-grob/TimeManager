package main

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"TimeManager/internal/tracker"
)

func TestTrayProjectOptions(t *testing.T) {
	d := tracker.State{
		Projects: []tracker.Project{
			{ID: "b", Number: "200", Name: "Research", Customer: "Customer", Favorite: true},
			{ID: "a", Number: "100", Name: "R&D", Customer: "A&B", Favorite: true},
			{ID: "not-favorite", Number: "300", Name: "Other"},
			{ID: "archived", Number: "400", Name: "Old", Favorite: true, Archived: true},
		},
		Active: &tracker.Timer{ProjectID: "a"},
	}
	want := []trayProjectOption{
		{id: "a", title: "100 - R&&D (A&&B)", running: true},
		{id: "b", title: "200 - Research (Customer)"},
	}
	if got := trayProjectOptions(d); !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %#v, want %#v", got, want)
	}
	d.Projects[1].Favorite = false
	d.Projects[0].Name = "Renamed"
	d.Active = nil
	if got := trayProjectOptions(d); !reflect.DeepEqual(got, []trayProjectOption{{id: "b", title: "200 - Renamed (Customer)"}}) {
		t.Fatalf("updated options = %#v", got)
	}
	if got := trayProjectOptions(tracker.State{}); len(got) != 0 {
		t.Fatalf("empty favorites = %#v", got)
	}
}

func TestTrayProjectGroups(t *testing.T) {
	d := tracker.State{Projects: []tracker.Project{
		{ID: "root", Number: "1", Name: "Root", Customer: "A&B"},
		{ID: "child", Number: "1", Name: "Child", Customer: "Other", ParentID: "root", Favorite: true},
		{ID: "grandchild", Number: "2", Name: "Grandchild", Customer: "A&B", ParentID: "child", Favorite: true},
		{ID: "second", Number: "1", Name: "Second root", Customer: "Other", Favorite: true},
		{ID: "archived", Number: "3", Name: "Archived", Customer: "Old", Archived: true},
		{ID: "empty", Number: "4", Name: "No favorites", Customer: "Unused"},
	}, Active: &tracker.Timer{ProjectID: "grandchild"}}
	groups := trayProjectGroups(d)
	for _, test := range []struct {
		name string
		key  trayProjectGroupKey
		want []string
	}{
		{"all", trayProjectGroupKey{kind: "all"}, []string{"child", "second", "grandchild"}},
		{"customer", trayProjectGroupKey{"customer", "Other"}, []string{"child", "second"}},
		{"own customer not inherited", trayProjectGroupKey{"customer", "A&B"}, []string{"grandchild"}},
		{"top level and descendants", trayProjectGroupKey{"project", "root"}, []string{"child", "grandchild"}},
		{"root itself", trayProjectGroupKey{"project", "second"}, []string{"second"}},
		{"archived excluded", trayProjectGroupKey{"customer", "Old"}, []string{}},
		{"empty group omitted", trayProjectGroupKey{"project", "empty"}, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ids := make([]string, 0)
			for _, group := range groups {
				if group.key != test.key {
					continue
				}
				for _, option := range group.projects {
					ids = append(ids, option.id)
					if option.running != (option.id == "grandchild") {
						t.Fatal("group changed running marker")
					}
				}
			}
			if !reflect.DeepEqual(ids, test.want) {
				t.Fatalf("IDs = %v, want %v", ids, test.want)
			}
		})
	}
	want := []struct {
		key   trayProjectGroupKey
		title string
	}{
		{key: trayProjectGroupKey{kind: "all"}, title: "All favorites"},
		{key: trayProjectGroupKey{"customer", "A&B"}, title: "A&B"},
		{key: trayProjectGroupKey{"customer", "Other"}, title: "Other"},
		{key: trayProjectGroupKey{"project", "root"}, title: "1 - Root"},
		{key: trayProjectGroupKey{"project", "second"}, title: "1 - Second root"},
	}
	if len(groups) != len(want) {
		t.Fatalf("groups = %#v", groups)
	}
	for i, expected := range want {
		if groups[i].key != expected.key || groups[i].title != expected.title {
			t.Fatalf("group %d = %#v, want %#v", i, groups[i], expected)
		}
	}
	d.Projects[0].Name = "Renamed"
	if got := trayProjectGroups(d); got[3].title != "1 - Renamed" || got[3].key.value != "root" {
		t.Fatal("renaming lost stable top-level group identity")
	}
	d.Projects[1].Customer = "New customer"
	d.Projects[1].ParentID = "second"
	d.Projects[2].Favorite = false
	updated := trayProjectGroups(d)
	for _, group := range updated {
		if group.key == (trayProjectGroupKey{"customer", "A&B"}) || group.key == (trayProjectGroupKey{"project", "root"}) {
			t.Fatal("stale customer or root group retained")
		}
	}
	if got := trayProjectGroups(tracker.State{}); !reflect.DeepEqual(got, []trayProjectGroup{{key: trayProjectGroupKey{kind: "all"}, title: "All favorites", projects: []trayProjectOption{}}}) {
		t.Fatalf("empty groups = %#v", got)
	}
}

func TestTrayIcon(t *testing.T) {
	icon, err := trayIcon("#123456", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(icon) <= 22 || binary.LittleEndian.Uint16(icon[2:4]) != 1 || binary.LittleEndian.Uint16(icon[4:6]) != 1 {
		t.Fatalf("invalid ICO header: %v", icon[:min(len(icon), 22)])
	}
	if got := binary.LittleEndian.Uint32(icon[18:22]); got != 22 {
		t.Fatalf("PNG payload offset = %d, want 22", got)
	}
	img, err := png.Decode(bytes.NewReader(icon[22:]))
	if err != nil {
		t.Fatalf("decode tray PNG: %v", err)
	}
	if img.Bounds().Dx() != 32 || img.Bounds().Dy() != 32 {
		t.Fatalf("tray image dimensions = %v", img.Bounds())
	}
	if got := color.NRGBAModel.Convert(img.At(16, 24)).(color.NRGBA); got != (color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}) {
		t.Fatalf("background pixel color = %#v", got)
	}
	if got := color.NRGBAModel.Convert(img.At(16, 16)).(color.NRGBA); got != (color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Fatalf("play pixel color = %#v", got)
	}
}

func TestTrayIconRejectsInvalidColors(t *testing.T) {
	for _, value := range []string{"", "#fff", "123456", "#12zz56"} {
		t.Run(value, func(t *testing.T) {
			if _, err := trayIcon(value, true); err == nil {
				t.Fatalf("trayIcon(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestTrayIconStateAndContrast(t *testing.T) {
	for _, hex := range []string{"#123456", "#ffffff"} {
		for _, running := range []bool{false, true} {
			icon, err := trayIcon(hex, running)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(icon[22:]))
			if err != nil {
				t.Fatal(err)
			}
			symbol := color.NRGBA{255, 255, 255, 255}
			if hex == "#ffffff" {
				symbol = color.NRGBA{0, 0, 0, 255}
			}
			if got := color.NRGBAModel.Convert(img.At(16, 16)); got != symbol {
				t.Fatalf("symbol contrast: %s running=%v: %v", hex, running, got)
			}
			isSymbol := color.NRGBAModel.Convert(img.At(21, 11)) == symbol
			if isSymbol == running {
				t.Fatalf("play and stop silhouettes not distinct: %s running=%v", hex, running)
			}
			if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
				t.Fatal("icon corner must remain transparent")
			}
		}
	}
}

func TestTrayStatus(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	d := tracker.State{Projects: []tracker.Project{{ID: "p", Name: "R&D", Number: "100", Color: "#64748b"}}}
	hex, text, running := trayStatus(d, now)
	if hex != "#64748b" || running || !strings.Contains(text, "Stopped") {
		t.Fatalf("idle status: %s %s %v", hex, text, running)
	}
	d.Active = &tracker.Timer{ProjectID: "p", Start: now.Add(-time.Hour - 2*time.Minute - 3*time.Second)}
	hex, text, running = trayStatus(d, now)
	if hex != "#64748b" || !running || text != "R&D | Running 01:02:03 | 100" {
		t.Fatalf("running status: %s %s %v", hex, text, running)
	}
	d.Active.Start = now.Add(time.Minute)
	_, text, _ = trayStatus(d, now)
	if !strings.Contains(text, "00:00:00") {
		t.Fatalf("negative elapsed time: %s", text)
	}
	d.Active = nil
	d.PausedProjectID = "p"
	hex, text, running = trayStatus(d, now)
	if hex != "#64748b" || running || !strings.Contains(text, "Paused") {
		t.Fatalf("paused status: %s %s %v", hex, text, running)
	}
}

func TestTooltipBoundsUTF16Length(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "short", text: "Timer running", want: 13},
		{name: "exact limit with supplementary rune", text: strings.Repeat("a", 118) + "😀", want: 120},
		{name: "avoid splitting surrogate pair", text: strings.Repeat("a", 119) + "😀tail", want: 119},
		{name: "truncate long ASCII", text: strings.Repeat("b", 150), want: 120},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tooltip(tt.text)
			units := utf16.Encode([]rune(got))
			if len(units) != tt.want {
				t.Fatalf("UTF-16 length = %d, want %d", len(units), tt.want)
			}
			if strings.ContainsRune(got, '\uFFFD') {
				t.Fatalf("tooltip contains a replacement rune: %q", got)
			}
			if tt.name == "short" && got != tt.text {
				t.Fatalf("tooltip = %q, want %q", got, tt.text)
			}
		})
	}
}
