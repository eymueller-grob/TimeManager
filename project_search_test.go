package main

import (
	"reflect"
	"testing"

	"TimeManager/internal/tracker"
)

func TestProjectSearchResults(t *testing.T) {
	d := tracker.State{Projects: []tracker.Project{
		{ID: "root", Number: "1", Name: "Umbrella", Customer: "Parent customer"},
		{ID: "child", Number: "100", Name: "Alpha", Customer: "Acme", ParentID: "root", Favorite: true},
		{ID: "grandchild", Number: "100", Name: "Beta", Customer: "Globex", ParentID: "child"},
		{ID: "archived", Number: "200", Name: "Old", Customer: "Acme", Archived: true},
	}, Active: &tracker.Timer{ProjectID: "grandchild"}}
	for _, test := range []struct {
		query string
		want  []string
	}{
		{"", []string{"root", "child", "grandchild"}},
		{"  ", []string{"root", "child", "grandchild"}},
		{"100", []string{"child", "grandchild"}},
		{" beTA ", []string{"grandchild"}},
		{"GLOBEX", []string{"grandchild"}},
		{"umbrella globex", []string{"grandchild"}},
		{"alpha", []string{"child", "grandchild"}},
		{"acme", []string{"child"}},
		{"old", []string{}},
		{"missing", []string{}},
	} {
		t.Run(test.query, func(t *testing.T) {
			got := projectSearchResults(d, test.query)
			ids := make([]string, 0)
			for _, result := range got {
				ids = append(ids, result.id)
				if result.running != (result.id == "grandchild") {
					t.Fatal("incorrect running marker")
				}
			}
			if !reflect.DeepEqual(ids, test.want) {
				t.Fatalf("IDs = %v, want %v", ids, test.want)
			}
		})
	}
	got := projectSearchResults(d, "beta")
	if got[0].label != "100 - Umbrella / Alpha / Beta (Globex)" {
		t.Fatalf("label = %q", got[0].label)
	}
	d.Projects[2].Archived = true
	if got := projectSearchResults(d, "beta"); len(got) != 0 {
		t.Fatal("archived search result retained")
	}
	if got := projectSearchResults(tracker.State{}, ""); len(got) != 0 {
		t.Fatal("empty state has search results")
	}
}
