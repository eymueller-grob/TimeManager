package main

import (
	"fmt"
	"sort"
	"strings"

	"TimeManager/internal/tracker"
)

type projectSearchResult struct {
	id, label string
	running   bool
}

func projectSearchResults(d tracker.State, query string) []projectSearchResult {
	projects := make(map[string]tracker.Project, len(d.Projects))
	for _, p := range d.Projects {
		projects[p.ID] = p
	}
	terms := strings.Fields(strings.ToLower(query))
	results := make([]projectSearchResult, 0)
	for _, p := range d.Projects {
		if p.Archived {
			continue
		}
		path := p.Name
		seen := map[string]bool{p.ID: true}
		for id := p.ParentID; id != "" && !seen[id]; {
			parent, ok := projects[id]
			if !ok {
				break
			}
			seen[id] = true
			path = parent.Name + " / " + path
			id = parent.ParentID
		}
		label := fmt.Sprintf("%s - %s (%s)", p.Number, path, p.Customer)
		text := label
		for _, def := range d.PropertyDefinitions {
			if value, present := p.Properties[def.ID]; present {
				text += " " + def.Name + " " + value
				if def.Type == "checkbox" {
					if value == "true" {
						text += " yes"
					} else if value == "false" {
						text += " no"
					}
				}
			}
		}
		text = strings.ToLower(text)
		match := true
		for _, term := range terms {
			if !strings.Contains(text, term) {
				match = false
				break
			}
		}
		if match {
			results = append(results, projectSearchResult{id: p.ID, label: label, running: d.Active != nil && d.Active.ProjectID == p.ID})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].label) < strings.ToLower(results[j].label)
	})
	return results
}
