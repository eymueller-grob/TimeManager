package tracker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func pngIcon(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: uint8(x), B: uint8(y), A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return projectIconPrefix + base64.StdEncoding.EncodeToString(b.Bytes())
}

func TestValidateProjectIcon(t *testing.T) {
	valid := pngIcon(t, 64, 64)
	encoded := strings.TrimPrefix(valid, projectIconPrefix)
	for _, test := range []struct {
		name, icon, err string
	}{
		{name: "empty", icon: ""},
		{name: "png", icon: valid},
		{name: "largest", icon: pngIcon(t, MaxProjectIconSize, MaxProjectIconSize)},
		{name: "smallest", icon: pngIcon(t, 1, 1)},
		{name: "too wide", icon: pngIcon(t, MaxProjectIconSize+1, 1), err: "pixels"},
		{name: "svg data URL", icon: "data:image/svg+xml;base64," + encoded, err: "PNG data URL"},
		{name: "emoji text", icon: "\U0001F680", err: "PNG data URL"},
		{name: "invalid base64", icon: projectIconPrefix + "%%%", err: "base64"},
		{name: "not png", icon: projectIconPrefix + base64.StdEncoding.EncodeToString([]byte("<svg/>")), err: "valid PNG"},
		{name: "truncated png", icon: projectIconPrefix + encoded[:len(encoded)/8*4], err: "valid PNG"},
		{name: "too long", icon: projectIconPrefix + strings.Repeat("A", MaxProjectIconLength), err: "too large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateProjectIcon(test.icon)
			if test.err == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if test.err != "" && (err == nil || !strings.Contains(err.Error(), test.err)) {
				t.Fatalf("error = %v, want containing %q", err, test.err)
			}
		})
	}
}

func TestDecodeProjectIcon(t *testing.T) {
	img, err := DecodeProjectIcon(pngIcon(t, 3, 2))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 3 || img.Bounds().Dy() != 2 {
		t.Fatalf("bounds = %v", img.Bounds())
	}
	if _, err := DecodeProjectIcon(""); err == nil {
		t.Fatal("empty icon decoded")
	}
}

func TestProjectIconPersistsAndRejectsInvalidIcons(t *testing.T) {
	s := newStore(t)
	p := addProject(t, s, "100", "")
	p.Icon = pngIcon(t, 64, 64)
	if err := s.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Snapshot().Projects[0].Icon; got != p.Icon {
		t.Fatal("icon was not persisted")
	}
	before := reopened.Snapshot()
	p.Icon = "data:image/svg+xml;base64,PHN2Zy8+"
	if err := reopened.SaveProject(p); err == nil {
		t.Fatal("invalid icon was saved")
	}
	if !reflect.DeepEqual(before, reopened.Snapshot()) {
		t.Fatal("rejected icon changed state")
	}
	p.Icon = ""
	if err := reopened.SaveProject(p); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"icon"`) {
		t.Fatal("removed icon is still stored")
	}
}

func TestMigrationToVersion8KeepsProjectsWithoutIcons(t *testing.T) {
	s := newStore(t)
	addProject(t, s, "100", "")
	legacy := s.Snapshot()
	legacy.Version = 7
	legacy.Settings.RequireTemporaryProject = true
	b, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.Snapshot()
	legacy.Version = CurrentVersion
	if !reflect.DeepEqual(got, legacy) || got.Projects[0].Icon != "" {
		t.Fatal("version 7 migration changed existing data or settings")
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted State
	if err := json.Unmarshal(saved, &persisted); err != nil || persisted.Version != CurrentVersion {
		t.Fatalf("persisted version = %d, %v", persisted.Version, err)
	}
}

func TestOpenRejectsNewerSchemaAndInvalidStoredIcon(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*State)
	}{
		{"newer schema", func(d *State) { d.Version = CurrentVersion + 1 }},
		{"invalid icon", func(d *State) { d.Projects[0].Icon = "not an icon" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newStore(t)
			addProject(t, s, "100", "")
			d := s.Snapshot()
			test.mutate(&d)
			b, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.path, b, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(s.path); err == nil {
				t.Fatal("invalid saved data opened")
			}
			after, err := os.ReadFile(s.path)
			if err != nil || !bytes.Equal(after, b) {
				t.Fatal("invalid saved data was rewritten")
			}
		})
	}
}
