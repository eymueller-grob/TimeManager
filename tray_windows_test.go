package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

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

func testProjectIcon(t *testing.T, width, height int, fill color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}

func TestTrayStatusIcon(t *testing.T) {
	d := tracker.State{Projects: []tracker.Project{{ID: "a", Icon: "icon-a"}, {ID: "b"}}}
	if got := trayStatusIcon(d); got != "" {
		t.Fatalf("idle icon = %q", got)
	}
	d.Active = &tracker.Timer{ProjectID: "a"}
	if got := trayStatusIcon(d); got != "icon-a" {
		t.Fatalf("running icon = %q", got)
	}
	d.Active = nil
	d.PausedProjectID = "a"
	if got := trayStatusIcon(d); got != "icon-a" {
		t.Fatalf("paused icon = %q", got)
	}
	d.PausedProjectID = "b"
	if got := trayStatusIcon(d); got != "" {
		t.Fatalf("project without icon = %q", got)
	}
}

func decodeTrayICO(t *testing.T, icon []byte) image.Image {
	t.Helper()
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
	return img
}

func nrgbaAt(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func TestTrayProjectMark(t *testing.T) {
	blue := color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}
	red := color.NRGBA{R: 220, G: 20, B: 60, A: 255}
	plain := decodeTrayICO(t, must(t)(trayProjectMark("#123456", "")))
	for _, p := range []image.Point{{16, 16}, {1, 16}, {30, 16}, {16, 1}, {16, 30}} {
		if got := nrgbaAt(plain, p.X, p.Y); got != blue {
			t.Fatalf("color circle at %v = %v, want full-size %v", p, got, blue)
		}
	}
	if got := nrgbaAt(plain, 0, 0); got.A != 0 {
		t.Fatal("circle corner must remain transparent")
	}
	for _, test := range []struct {
		name          string
		width, height int
		filled        image.Point
		transparent   image.Point
	}{
		{name: "square fills slot", width: 64, height: 64, filled: image.Pt(31, 31)},
		{name: "wide keeps aspect", width: 128, height: 32, filled: image.Pt(1, 14), transparent: image.Pt(1, 1)},
		{name: "small upscales", width: 8, height: 8, filled: image.Pt(0, 0)},
	} {
		t.Run(test.name, func(t *testing.T) {
			img := decodeTrayICO(t, must(t)(trayProjectMark("#123456", testProjectIcon(t, test.width, test.height, red))))
			if got := nrgbaAt(img, test.filled.X, test.filled.Y); got != red {
				t.Fatalf("project icon pixel = %v", got)
			}
			if got := nrgbaAt(img, 16, 16); got != red {
				t.Fatalf("project icon must not have a status badge, center = %v", got)
			}
			if test.transparent != (image.Point{}) && nrgbaAt(img, test.transparent.X, test.transparent.Y).A != 0 {
				t.Fatal("letterbox area must remain transparent")
			}
		})
	}
	for _, test := range []struct{ name, hex, icon string }{
		{"empty color", "", ""},
		{"short color", "#fff", ""},
		{"missing hash", "123456", ""},
		{"invalid hex", "#12zz56", ""},
		{"invalid color with icon", "#12345", testProjectIcon(t, 4, 4, red)},
		{"invalid icon", "#123456", "data:image/png;base64,AAAA"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := trayProjectMark(test.hex, test.icon); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func must(t *testing.T) func([]byte, error) []byte {
	return func(b []byte, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
}

func TestTrayBlankIcon(t *testing.T) {
	img := decodeTrayICO(t, must(t)(trayBlankIcon()))
	for _, p := range []image.Point{{0, 0}, {16, 16}, {31, 31}} {
		if nrgbaAt(img, p.X, p.Y).A != 0 {
			t.Fatalf("blank icon pixel %v must be transparent", p)
		}
	}
}

func TestTrayStateIcon(t *testing.T) {
	for _, test := range []struct {
		state  trayState
		symbol color.NRGBA
		filled []image.Point
		empty  []image.Point
	}{
		// Large play triangle: wide base on the left, tapering to the tip.
		{trayRunning, color.NRGBA{0x16, 0xa3, 0x4a, 0xff},
			[]image.Point{{4, 3}, {4, 28}, {16, 16}, {28, 15}},
			[]image.Point{{0, 0}, {1, 16}, {20, 5}, {31, 16}}},
		// Two tall bars with a gap between them.
		{trayPaused, color.NRGBA{0xd9, 0x77, 0x06, 0xff},
			[]image.Point{{4, 2}, {12, 29}, {19, 2}, {27, 29}},
			[]image.Point{{0, 0}, {16, 16}, {1, 16}, {30, 16}}},
		// Large square.
		{trayStopped, color.NRGBA{0x64, 0x74, 0x8b, 0xff},
			[]image.Point{{3, 3}, {28, 28}, {16, 16}, {28, 3}},
			[]image.Point{{0, 0}, {1, 16}, {30, 16}, {16, 30}}},
	} {
		img := decodeTrayICO(t, must(t)(trayStateIcon(test.state)))
		for _, p := range test.filled {
			if got := nrgbaAt(img, p.X, p.Y); got != test.symbol {
				t.Fatalf("state %d pixel %v = %v, want symbol %v", test.state, p, got, test.symbol)
			}
		}
		for _, p := range test.empty {
			if got := nrgbaAt(img, p.X, p.Y); got.A != 0 {
				t.Fatalf("state %d pixel %v = %v, want transparent (no circle)", test.state, p, got)
			}
		}
	}
	if _, err := trayStateIcon(trayState(99)); err == nil {
		t.Fatal("unknown state must be rejected")
	}
}

func TestTrayStateOf(t *testing.T) {
	d := tracker.State{}
	if got := trayStateOf(d); got != trayStopped {
		t.Fatalf("idle = %d", got)
	}
	d.PausedProjectID = "a"
	if got := trayStateOf(d); got != trayPaused {
		t.Fatalf("paused = %d", got)
	}
	d.Active = &tracker.Timer{ProjectID: "b"}
	if got := trayStateOf(d); got != trayRunning {
		t.Fatalf("running = %d", got)
	}
}

func TestTrayNotifyIconDataMatchesWin32Layout(t *testing.T) {
	want := uintptr(976) // sizeof(NOTIFYICONDATAW) on 64-bit Windows
	if unsafe.Sizeof(uintptr(0)) == 4 {
		want = 956
	}
	if got := unsafe.Sizeof(trayNotifyIconData{}); got != want {
		t.Fatalf("NOTIFYICONDATAW size = %d, want %d", got, want)
	}
}
