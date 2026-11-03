package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"TimeManager/internal/tracker"
	"github.com/getlantern/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func trayIcon(hex string, running bool) ([]byte, error) {
	if len(hex) != 7 || hex[0] != '#' {
		return nil, fmt.Errorf("tray color must be a six-digit hex color")
	}
	n, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	c := color.NRGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}
	symbol := color.NRGBA{255, 255, 255, 255}
	if 299*int(c.R)+587*int(c.G)+114*int(c.B) > 160000 {
		symbol = color.NRGBA{0, 0, 0, 255}
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			dx, dy := x-16, y-16
			if dx*dx+dy*dy <= 225 {
				img.SetNRGBA(x, y, c)
			}
			play := x >= 11 && x <= 23 && y >= 16-(23-x)*2/3 && y <= 16+(23-x)*2/3
			stop := x >= 10 && x <= 22 && y >= 10 && y <= 22
			if (running && play) || (!running && stop) {
				img.SetNRGBA(x, y, symbol)
			}
		}
	}
	var payload bytes.Buffer
	if err := png.Encode(&payload, img); err != nil {
		return nil, err
	}
	// Windows accepts a PNG image inside a single-entry ICO container.
	header := make([]byte, 22)
	binary.LittleEndian.PutUint16(header[2:], 1)
	binary.LittleEndian.PutUint16(header[4:], 1)
	header[6], header[7] = 32, 32
	binary.LittleEndian.PutUint16(header[10:], 1)
	binary.LittleEndian.PutUint16(header[12:], 32)
	binary.LittleEndian.PutUint32(header[14:], uint32(payload.Len()))
	binary.LittleEndian.PutUint32(header[18:], 22)
	return append(header, payload.Bytes()...), nil
}

func trayStatus(d tracker.State, now time.Time) (string, string, bool) {
	if d.Active != nil {
		for _, p := range d.Projects {
			if p.ID == d.Active.ProjectID {
				seconds := max(int64(now.Sub(d.Active.Start).Seconds()), 0)
				return p.Color, fmt.Sprintf("%s | Running %02d:%02d:%02d | %s", p.Name, seconds/3600, seconds/60%60, seconds%60, p.Number), true
			}
		}
	}
	return "#64748b", "TimeManager - Stopped (no timer running)", false
}

func tooltip(s string) string {
	units := utf16.Encode([]rune(s))
	if len(units) > 120 {
		units = units[:120]
		if units[len(units)-1] >= 0xD800 && units[len(units)-1] <= 0xDBFF {
			units = units[:len(units)-1]
		}
	}
	return string(utf16.Decode(units))
}

type trayProjectOption struct {
	id, title string
	running   bool
}

type trayProjectGroupKey struct {
	kind, value string
}

type trayProjectGroup struct {
	key      trayProjectGroupKey
	title    string
	projects []trayProjectOption
}

func trayProjectGroups(d tracker.State) []trayProjectGroup {
	all := trayProjectOptions(d)
	groups := []trayProjectGroup{{key: trayProjectGroupKey{kind: "all"}, title: "All favorites", projects: all}}
	projects := make(map[string]tracker.Project, len(d.Projects))
	for _, p := range d.Projects {
		projects[p.ID] = p
	}
	indexes := make(map[trayProjectGroupKey]int)
	add := func(key trayProjectGroupKey, title string, option trayProjectOption) {
		index, ok := indexes[key]
		if !ok {
			index = len(groups)
			indexes[key] = index
			groups = append(groups, trayProjectGroup{key: key, title: title})
		}
		groups[index].projects = append(groups[index].projects, option)
	}
	for _, option := range all {
		p := projects[option.id]
		add(trayProjectGroupKey{"customer", p.Customer}, p.Customer, option)
		root := p
		seen := make(map[string]bool)
		for root.ParentID != "" && !seen[root.ID] {
			seen[root.ID] = true
			parent, ok := projects[root.ParentID]
			if !ok {
				break
			}
			root = parent
		}
		add(trayProjectGroupKey{"project", root.ID}, root.Number+" - "+root.Name, option)
	}
	sort.Slice(groups[1:], func(i, j int) bool {
		a, b := groups[i+1], groups[j+1]
		if a.key.kind != b.key.kind {
			return a.key.kind < b.key.kind
		}
		return strings.ToLower(a.title) < strings.ToLower(b.title)
	})
	return groups
}

func trayProjectOptions(d tracker.State) []trayProjectOption {
	options := make([]trayProjectOption, 0)
	for _, p := range d.Projects {
		if p.Favorite && !p.Archived {
			options = append(options, trayProjectOption{
				id: p.ID, title: strings.ReplaceAll(fmt.Sprintf("%s - %s (%s)", p.Number, p.Name, p.Customer), "&", "&&"),
				running: d.Active != nil && d.Active.ProjectID == p.ID,
			})
		}
	}
	sort.Slice(options, func(i, j int) bool {
		return strings.ToLower(options[i].title) < strings.ToLower(options[j].title)
	})
	return options
}

func (a *App) startTray() func() {
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			restore := systray.AddMenuItem("Open TimeManager", "Restore the window")
			stop := systray.AddMenuItem("Stop and save timer", "Save the active booking")
			status := systray.AddMenuItem("No timer running", "Current timer")
			status.Disable()
			systray.AddSeparator()
			changeProject := systray.AddMenuItem("Change project", "Find and start a project")
			search := changeProject.AddSubMenuItem("Search projects...", "Search all non-archived projects")
			allMenu := changeProject.AddSubMenuItem("All favorites", "Select any favorite project")
			customerMenu := changeProject.AddSubMenuItem("By customer", "Find a favorite by its customer")
			parentMenu := changeProject.AddSubMenuItem("By top-level project", "Find favorite projects and descendants under a top-level project")
			empty := changeProject.AddSubMenuItem("No favorite projects - add favorites in Projects", "")
			empty.Disable()
			temporary := systray.AddMenuItem("Start temporary timer", "Track unassigned time and assign its bookings to a project later")
			quit := systray.AddMenuItem("Quit", "Exit TimeManager")
			groupMenus := make(map[trayProjectGroupKey]*systray.MenuItem)
			type entryKey struct {
				group trayProjectGroupKey
				id    string
			}
			projectMenus := make(map[entryKey]*systray.MenuItem)
			var groups []trayProjectGroup
			var entries []entryKey
			tick := time.NewTicker(time.Second)
			defer tick.Stop()
			lastColor := ""
			lastRunning := false
			update := func() {
				d := a.store.Snapshot()
				nextGroups := trayProjectGroups(d)
				if !reflect.DeepEqual(groups, nextGroups) {
					for _, item := range projectMenus {
						item.Hide()
					}
					for _, item := range groupMenus {
						item.Hide()
					}
					entries = nil
					for _, group := range nextGroups {
						parent := allMenu
						if group.key.kind != "all" {
							parent = groupMenus[group.key]
							if parent == nil {
								route := customerMenu
								if group.key.kind == "project" {
									route = parentMenu
								}
								parent = route.AddSubMenuItem("", "Select a project to switch timers")
								groupMenus[group.key] = parent
							}
							parent.SetTitle(strings.ReplaceAll(group.title, "&", "&&"))
							parent.Show()
						}
						for _, option := range group.projects {
							key := entryKey{group: group.key, id: option.id}
							item := projectMenus[key]
							if item == nil {
								item = parent.AddSubMenuItem(option.title, "Save the current timer and switch to this project")
								projectMenus[key] = item
							}
							item.SetTitle(option.title)
							if option.running {
								item.Check()
								item.Disable()
							} else {
								item.Uncheck()
								item.Enable()
							}
							item.Show()
							entries = append(entries, key)
						}
					}
					for _, route := range []*systray.MenuItem{allMenu, customerMenu, parentMenu} {
						if len(entries) == 0 {
							route.Disable()
						} else {
							route.Enable()
						}
					}
					if len(entries) == 0 {
						empty.Show()
					} else {
						empty.Hide()
					}
					groups = nextGroups
				}
				hex, text, running := trayStatus(d, time.Now())
				if running {
					stop.Enable()
				} else {
					stop.Disable()
				}
				if hex != lastColor || running != lastRunning {
					icon, err := trayIcon(hex, running)
					if err != nil {
						a.showError(fmt.Errorf("create tray icon: %w", err))
						return
					}
					systray.SetIcon(icon)
					lastColor = hex
					lastRunning = running
				}
				systray.SetTooltip(tooltip(text))
				status.SetTitle(strings.ReplaceAll(tooltip(text), "&", "&&"))
				if wailsruntime.WindowIsMinimised(a.ctx) {
					wailsruntime.WindowHide(a.ctx)
				}
			}
			update()
			for {
				cases := []reflect.SelectCase{
					{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(done)},
					{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(tick.C)},
					{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(restore.ClickedCh)},
					{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(stop.ClickedCh)},
					{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(quit.ClickedCh)},
					{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(search.ClickedCh)},
					{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(temporary.ClickedCh)},
				}
				for _, key := range entries {
					cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(projectMenus[key].ClickedCh)})
				}
				selected, _, _ := reflect.Select(cases)
				switch selected {
				case 0:
					return
				case 1:
					update()
				case 2:
					a.restoreWindow()
				case 3:
					if err := a.StopTimer(); err != nil {
						a.showError(err)
					}
					update()
				case 4:
					a.restoreWindow()
					if err := a.QuitApplication(); err != nil {
						a.showError(err)
					}
				case 5:
					if err := a.openProjectSearch(); err != nil {
						a.showError(fmt.Errorf("open project search: %w", err))
					}
				case 6:
					if err := a.StartTemporaryTimer(); err != nil {
						a.showError(fmt.Errorf("start temporary timer: %w", err))
					}
					update()
				default:
					id := entries[selected-7].id
					if _, err := a.timerAction("favorite", id); err != nil {
						a.showError(fmt.Errorf("change tray project: %w", err))
					}
					update()
				}
			}
		}, func() {})
	}()
	return func() {
		close(done)
		systray.Quit()
	}
}
