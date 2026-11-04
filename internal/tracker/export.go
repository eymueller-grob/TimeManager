package tracker

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func DateRange(from, to string, loc *time.Location) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation("2006-01-02", from, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("export start date must be YYYY-MM-DD")
	}
	end, err := time.ParseInLocation("2006-01-02", to, loc)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("export end date must be YYYY-MM-DD")
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, errors.New("export end date must be on or after the start date")
	}
	return start, end.AddDate(0, 0, 1), nil
}

func ProjectPath(d State, id string) string {
	names := []string{}
	for id != "" {
		p, ok := project(d, id)
		if !ok {
			break
		}
		names = append([]string{p.Name}, names...)
		id = p.ParentID
	}
	return strings.Join(names, " / ")
}

// Export clips cross-midnight bookings to the inclusive local-date filter.
func Export(d State, from, to string, loc *time.Location) ([]byte, error) {
	start, end, err := DateRange(from, to, loc)
	if err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Bookings"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	headers := []interface{}{"Customer", "Project number", "Project / subproject", "Start (local)", "End (local)", "Hours", "Notes", "Project color"}
	for _, def := range d.PropertyDefinitions {
		headers = append(headers, "Property: "+def.Name)
	}
	headers = append(headers, "Start CW", "End CW")
	lastColumn, err := excelize.ColumnNumberToName(len(headers))
	if err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return nil, err
	}
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: strptr("yyyy-mm-dd hh:mm:ss")})
	if err != nil {
		return nil, err
	}
	hourStyle, err := f.NewStyle(&excelize.Style{NumFmt: 2})
	if err != nil {
		return nil, err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"25344A"}}})
	if err != nil {
		return nil, err
	}
	bookings := append([]Booking{}, d.Bookings...)
	sort.Slice(bookings, func(i, j int) bool { return bookings[i].Start.Before(bookings[j].Start) })
	row := 2
	var total float64
	daily := make(map[dailyProjectKey]time.Duration)
	for _, b := range bookings {
		if !b.Start.Before(end) || !b.End.After(start) {
			continue
		}
		a, z := b.Start, b.End
		if a.Before(start) {
			a = start
		}
		if z.After(end) {
			z = end
		}
		p, ok := project(d, b.ProjectID)
		if !ok {
			return nil, errors.New("export booking references a missing project")
		}
		hours := z.Sub(a).Hours()
		total += hours
		for part := a.In(loc); part.Before(z); {
			day := time.Date(part.Year(), part.Month(), part.Day(), 0, 0, 0, 0, loc)
			next := day.AddDate(0, 0, 1)
			if next.After(z) {
				next = z
			}
			daily[dailyProjectKey{date: day.Format("2006-01-02"), projectID: p.ID}] += next.Sub(part)
			part = next.In(loc)
		}
		values := []interface{}{p.Customer, p.Number, ProjectPath(d, p.ID), a.In(loc), z.In(loc), hours, b.Notes, p.Color}
		properties, err := exportPropertyValues(d.PropertyDefinitions, p)
		if err != nil {
			return nil, err
		}
		values = append(values, properties...)
		values = append(values, isoWeekLabel(a.In(loc)), isoWeekLabel(z.In(loc)))
		if err := f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &values); err != nil {
			return nil, err
		}
		fill, err := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{strings.TrimPrefix(p.Color, "#")}}})
		if err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(sheet, fmt.Sprintf("H%d", row), fmt.Sprintf("H%d", row), fill); err != nil {
			return nil, err
		}
		row++
	}
	if row > 2 {
		if err := f.SetCellStyle(sheet, "D2", fmt.Sprintf("E%d", row-1), dateStyle); err != nil {
			return nil, err
		}
	}
	if err := f.SetCellStr(sheet, fmt.Sprintf("E%d", row), "Total hours"); err != nil {
		return nil, err
	}
	if err := f.SetCellFloat(sheet, fmt.Sprintf("F%d", row), total, -1, 64); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "F2", fmt.Sprintf("F%d", row), hourStyle); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", lastColumn+"1", headerStyle); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "A", "C", 28); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "D", "E", 23); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "F", "F", 14); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "G", "G", 40); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "H", "H", 18); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "I", lastColumn, 24); err != nil {
		return nil, err
	}
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}
	if row > 2 {
		if err := f.AutoFilter(sheet, fmt.Sprintf("A1:%s%d", lastColumn, row-1), nil); err != nil {
			return nil, err
		}
	}
	if err := writeDailySummary(f, d, daily, headerStyle, hourStyle); err != nil {
		return nil, fmt.Errorf("create daily summary sheet: %w", err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("create Excel workbook: %w", err)
	}
	return buf.Bytes(), nil
}

type dailyProjectKey struct {
	date, projectID string
}

func writeDailySummary(f *excelize.File, d State, daily map[dailyProjectKey]time.Duration, headerStyle, hourStyle int) error {
	const sheet = "Daily project totals"
	if _, err := f.NewSheet(sheet); err != nil {
		return err
	}
	headers := []interface{}{"Date (local)", "Customer", "Project number", "Project / subproject", "Hours"}
	for _, def := range d.PropertyDefinitions {
		headers = append(headers, "Property: "+def.Name)
	}
	headers = append(headers, "CW")
	lastColumn, err := excelize.ColumnNumberToName(len(headers))
	if err != nil {
		return err
	}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return err
	}
	keys := make([]dailyProjectKey, 0, len(daily))
	projects := make(map[string]Project, len(d.Projects))
	paths := make(map[string]string, len(d.Projects))
	for _, p := range d.Projects {
		projects[p.ID] = p
		paths[p.ID] = ProjectPath(d, p.ID)
	}
	for key := range daily {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].date != keys[j].date {
			return keys[i].date < keys[j].date
		}
		if paths[keys[i].projectID] != paths[keys[j].projectID] {
			return paths[keys[i].projectID] < paths[keys[j].projectID]
		}
		return keys[i].projectID < keys[j].projectID
	})
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: strptr("yyyy-mm-dd")})
	if err != nil {
		return err
	}
	var total time.Duration
	for i, key := range keys {
		date, err := time.Parse("2006-01-02", key.date)
		if err != nil {
			return err
		}
		p := projects[key.projectID]
		duration := daily[key]
		total += duration
		values := []interface{}{date, p.Customer, p.Number, paths[p.ID], duration.Hours()}
		properties, err := exportPropertyValues(d.PropertyDefinitions, p)
		if err != nil {
			return err
		}
		values = append(values, properties...)
		values = append(values, isoWeekLabel(date))
		if err := f.SetSheetRow(sheet, fmt.Sprintf("A%d", i+2), &values); err != nil {
			return err
		}
	}
	row := len(keys) + 2
	if err := f.SetCellStr(sheet, fmt.Sprintf("D%d", row), "Total hours"); err != nil {
		return err
	}
	if err := f.SetCellFloat(sheet, fmt.Sprintf("E%d", row), total.Hours(), -1, 64); err != nil {
		return err
	}
	if len(keys) > 0 {
		if err := f.SetCellStyle(sheet, "A2", fmt.Sprintf("A%d", row-1), dateStyle); err != nil {
			return err
		}
		if err := f.AutoFilter(sheet, fmt.Sprintf("A1:%s%d", lastColumn, row-1), nil); err != nil {
			return err
		}
	}
	if err := f.SetCellStyle(sheet, "E2", fmt.Sprintf("E%d", row), hourStyle); err != nil {
		return err
	}
	if err := f.SetCellStyle(sheet, "A1", lastColumn+"1", headerStyle); err != nil {
		return err
	}
	for _, col := range []struct {
		first, last string
		width       float64
	}{{"A", "A", 16}, {"B", "C", 28}, {"D", "D", 40}, {"E", "E", 14}} {
		if err := f.SetColWidth(sheet, col.first, col.last, col.width); err != nil {
			return err
		}
	}
	if err := f.SetColWidth(sheet, "F", lastColumn, 24); err != nil {
		return err
	}
	return f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

func isoWeekLabel(date time.Time) string {
	_, week := date.ISOWeek()
	return fmt.Sprintf("CW%02d", week)
}

func exportPropertyValues(definitions []PropertyDefinition, p Project) ([]interface{}, error) {
	values := make([]interface{}, 0, len(definitions))
	for _, def := range definitions {
		value, present := p.Properties[def.ID]
		if !present {
			values = append(values, nil)
			continue
		}
		if err := validatePropertyValue(def, value); err != nil {
			return nil, fmt.Errorf("export property %q: %w", def.Name, err)
		}
		switch def.Type {
		case "number":
			n, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("export property %q: %w", def.Name, err)
			}
			values = append(values, n)
		case "checkbox":
			values = append(values, value == "true")
		default:
			values = append(values, value)
		}
	}
	return values, nil
}

func strptr(s string) *string { return &s }
