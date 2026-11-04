package tracker

import (
	"reflect"
	"slices"
	"testing"
)

func TestRelatedProjectIDs(t *testing.T) {
	projects := []Project{
		{ID: "root"}, {ID: "child", ParentID: "root"}, {ID: "grandchild", ParentID: "child"},
		{ID: "sibling", ParentID: "root", Archived: true}, {ID: "other"}, {ID: "other-child", ParentID: "other"},
	}
	for _, test := range []struct {
		name, id string
		want     []string
	}{
		{name: "root", id: "root", want: []string{"child", "grandchild", "sibling"}},
		{name: "middle", id: "child", want: []string{"grandchild", "root", "sibling"}},
		{name: "leaf includes archived relatives", id: "grandchild", want: []string{"child", "root", "sibling"}},
		{name: "separate tree", id: "other-child", want: []string{"other"}},
		{name: "unrelated single project", id: "lonely", want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := RelatedProjectIDs(append(projects, Project{ID: "lonely"}), test.id)
			slices.Sort(got)
			if test.id == "lonely" && len(got) != 0 {
				t.Fatalf("related = %v, want none", got)
			}
			if test.id != "lonely" && !reflect.DeepEqual(got, test.want) {
				t.Fatalf("related = %v, want %v", got, test.want)
			}
		})
	}
	if got := RelatedProjectIDs(projects, "missing"); got != nil {
		t.Fatalf("missing project related = %v", got)
	}
	cycle := []Project{{ID: "a", ParentID: "b"}, {ID: "b", ParentID: "a"}, {ID: "c", ParentID: "b"}}
	got := RelatedProjectIDs(cycle, "c")
	slices.Sort(got)
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("cycle related = %v", got)
	}
}

func TestSaveProjectWithRelatedSharesColorAndIcon(t *testing.T) {
	s := newStore(t)
	root := addProject(t, s, "100", "")
	child := addProject(t, s, "101", root.ID)
	grandchild := addProject(t, s, "102", child.ID)
	sibling := addProject(t, s, "103", root.ID)
	other := addProject(t, s, "200", "")
	icon := pngIcon(t, 64, 64)

	grandchild.Color, grandchild.Icon, grandchild.Name = "#ff0000", icon, "Renamed"
	if err := s.SaveProjectWithRelated(grandchild); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range reopened.Snapshot().Projects {
		related := p.ID != other.ID
		if related && (p.Color != "#ff0000" || p.Icon != icon) {
			t.Fatalf("%s color/icon = %s/%t, want shared", p.Name, p.Color, p.Icon == icon)
		}
		if !related && (p.Color != other.Color || p.Icon != "") {
			t.Fatalf("unrelated project changed: %+v", p)
		}
		if p.ID == root.ID && (p.Name != root.Name || p.Number != root.Number || !p.Favorite) {
			t.Fatalf("related project fields other than color/icon changed: %+v", p)
		}
		if p.ID == grandchild.ID && p.Name != "Renamed" {
			t.Fatalf("edited project was not saved: %+v", p)
		}
	}

	// Removing the icon also removes it from relatives; a plain save leaves them alone.
	sibling.Icon = ""
	sibling.Color = "#00ff00"
	if err := s.SaveProject(sibling); err != nil {
		t.Fatal(err)
	}
	if p, _ := project(s.Snapshot(), root.ID); p.Color != "#ff0000" || p.Icon != icon {
		t.Fatalf("plain SaveProject changed a relative: %+v", p)
	}
	if err := s.SaveProjectWithRelated(sibling); err != nil {
		t.Fatal(err)
	}
	for _, p := range s.Snapshot().Projects {
		if p.ID != other.ID && (p.Color != "#00ff00" || p.Icon != "") {
			t.Fatalf("%s was not updated after removing the icon: %+v", p.Name, p)
		}
	}

	// A new subproject and a moved project join their new tree.
	if err := s.SaveProjectWithRelated(Project{Number: "104", Name: "New child", Customer: "Customer", Color: "#0000ff", ParentID: child.ID}); err != nil {
		t.Fatal(err)
	}
	other.ParentID = root.ID
	other.Color = "#abcdef"
	if err := s.SaveProjectWithRelated(other); err != nil {
		t.Fatal(err)
	}
	for _, p := range s.Snapshot().Projects {
		if p.Color != "#abcdef" {
			t.Fatalf("%s color = %s after moving into the tree", p.Name, p.Color)
		}
	}
}

func TestSaveProjectWithRelatedRejectsInvalidChangesAtomically(t *testing.T) {
	s := newStore(t)
	root := addProject(t, s, "100", "")
	child := addProject(t, s, "101", root.ID)
	before := s.Snapshot()
	for _, test := range []struct {
		name string
		edit func(*Project)
	}{
		{"parent cycle", func(p *Project) { p.ParentID = child.ID; p.Color = "#ff0000" }},
		{"invalid icon", func(p *Project) { p.Icon = "data:image/svg+xml;base64,PHN2Zy8+" }},
		{"invalid color", func(p *Project) { p.Color = "red" }},
		{"missing project", func(p *Project) { p.ID = "missing" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := root
			test.edit(&p)
			if err := s.SaveProjectWithRelated(p); err == nil {
				t.Fatal("invalid change accepted")
			}
			if !reflect.DeepEqual(before, s.Snapshot()) {
				t.Fatal("rejected change modified state")
			}
		})
	}
}
