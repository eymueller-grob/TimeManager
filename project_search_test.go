package main

import (
	"reflect"
	"testing"

	"TimeManager/internal/tracker"
)

func TestProjectSearchProperties(t *testing.T) {
	d := tracker.State{
		PropertyDefinitions: []tracker.PropertyDefinition{
			{ID: "text", Name: "Contact", Type: "text"},
			{ID: "number", Name: "Rate", Type: "number"},
			{ID: "check", Name: "Billable", Type: "checkbox"},
			{ID: "drop", Name: "Stage", Type: "dropdown"},
			{ID: "unset", Name: "Unset field", Type: "text"},
		},
		Projects: []tracker.Project{
			{ID: "p", Name: "Alpha", Properties: map[string]string{"text": "Jane Smith", "number": "125.5", "check": "false", "drop": "Closed", "deleted": "orphan"}},
			{ID: "child", Name: "Child", ParentID: "p"},
			{ID: "old", Name: "Old", Archived: true, Properties: map[string]string{"text": "Jane"}},
		},
	}
	for _, query := range []string{"CONTACT", "jAnE", "125.5", "rate", "billable", "false", "no", "stage", "closed", "alpha jane closed"} {
		if got := projectSearchResults(d, query); len(got) != 1 || got[0].id != "p" {
			t.Errorf("%q results = %#v", query, got)
		}
	}
	for _, query := range []string{"unset field", "orphan", "true", "yes"} {
		if got := projectSearchResults(d, query); len(got) != 0 {
			t.Errorf("%q unexpectedly matched %#v", query, got)
		}
	}
	d.Projects[0].Properties["check"] = "true"
	if got := projectSearchResults(d, "yes"); len(got) != 1 {
		t.Fatal("updated checkbox not searchable")
	}
	delete(d.Projects[0].Properties, "text")
	if got := projectSearchResults(d, "jane"); len(got) != 0 {
		t.Fatal("removed property retained in search")
	}
}

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
