package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"TimeManager/internal/tracker"
	"github.com/getlantern/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type trayState int

const (
	trayStopped trayState = iota
	trayPaused
	trayRunning
)

func trayStateOf(d tracker.State) trayState {
	if d.Active != nil {
		return trayRunning
	}
	if d.PausedProjectID != "" {
		return trayPaused
	}
	return trayStopped
}

func parseTrayColor(hex string) (color.NRGBA, error) {
	if !colorPattern.MatchString(hex) {
		return color.NRGBA{}, fmt.Errorf("tray color must be a six-digit hex color")
	}
	n, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return color.NRGBA{}, err
	}
	return color.NRGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}, nil
}

// trayProjectMark draws the project's icon, or a full-size circle in the
// project color, filling the whole tray slot.
func trayProjectMark(hex, projectIcon string) ([]byte, error) {
	c, err := parseTrayColor(hex)
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	if projectIcon != "" {
		source, err := tracker.DecodeProjectIcon(projectIcon)
		if err != nil {
			return nil, err
		}
		drawScaled(img, source)
		return encodeICO(img)
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if dx, dy := 2*x-31, 2*y-31; dx*dx+dy*dy <= 31*31 {
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return encodeICO(img)
}

var trayStateColors = map[trayState]color.NRGBA{
	trayRunning: {R: 0x16, G: 0xa3, B: 0x4a, A: 0xff},
	trayPaused:  {R: 0xd9, G: 0x77, B: 0x06, A: 0xff},
	trayStopped: {R: 0x64, G: 0x74, B: 0x8b, A: 0xff},
}

// trayBlankIcon is a fully transparent icon that keeps the project slot in the
// tray while no project is running or paused.
func trayBlankIcon() ([]byte, error) {
	return encodeICO(image.NewNRGBA(image.Rect(0, 0, 32, 32)))
}

// trayStateIcon draws a large status symbol without background: green play,
// amber pause, or gray stop.
func trayStateIcon(state trayState) ([]byte, error) {
	c, ok := trayStateColors[state]
	if !ok {
		return nil, fmt.Errorf("unknown tray state %d", state)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			var symbol bool
			switch state {
			case trayRunning:
				symbol = x >= 3 && x <= 29 && 26*abs(2*y-31) <= 29*(29-x)
			case trayPaused:
				symbol = y >= 2 && y <= 29 && ((x >= 4 && x <= 12) || (x >= 19 && x <= 27))
			default:
				symbol = x >= 3 && x <= 28 && y >= 3 && y <= 28
			}
			if symbol {
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return encodeICO(img)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// drawScaled fits source into dst, preserving its aspect ratio, by averaging
// premultiplied source pixels for each destination pixel.
func drawScaled(dst *image.NRGBA, source image.Image) {
	sb := source.Bounds()
	size := dst.Bounds().Dx()
	scale := max(float64(sb.Dx()), float64(sb.Dy())) / float64(size)
	width, height := int(float64(sb.Dx())/scale+0.5), int(float64(sb.Dy())/scale+0.5)
	offsetX, offsetY := (size-width)/2, (size-height)/2
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			x0, x1 := sb.Min.X+int(float64(x)*scale), sb.Min.X+max(int(float64(x+1)*scale), int(float64(x)*scale)+1)
			y0, y1 := sb.Min.Y+int(float64(y)*scale), sb.Min.Y+max(int(float64(y+1)*scale), int(float64(y)*scale)+1)
			var r, g, b, a, count uint64
			for sy := y0; sy < min(y1, sb.Max.Y); sy++ {
				for sx := x0; sx < min(x1, sb.Max.X); sx++ {
					pr, pg, pb, pa := source.At(sx, sy).RGBA()
					r, g, b, a, count = r+uint64(pr), g+uint64(pg), b+uint64(pb), a+uint64(pa), count+1
				}
			}
			if count == 0 || a == 0 {
				continue
			}
			dst.SetNRGBA(offsetX+x, offsetY+y, color.NRGBA{
				R: uint8(r * 0xff / a), G: uint8(g * 0xff / a), B: uint8(b * 0xff / a), A: uint8(a / count >> 8),
			})
		}
	}
}

func encodeICO(img image.Image) ([]byte, error) {
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
	if d.PausedProjectID != "" {
		for _, p := range d.Projects {
			if p.ID == d.PausedProjectID {
				return p.Color, fmt.Sprintf("%s | Paused | %s", p.Name, p.Number), false
			}
		}
	}
	return "#64748b", "TimeManager - Stopped (no timer running)", false
}

// trayStatusIcon returns the icon of the running or paused project, if any.
func trayStatusIcon(d tracker.State) string {
	id := d.PausedProjectID
	if d.Active != nil {
		id = d.Active.ProjectID
	}
	if id == "" {
		return ""
	}
	for _, p := range d.Projects {
		if p.ID == id {
			return p.Icon
		}
	}
	return ""
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
	var stateTray struct {
		sync.Mutex
		indicator *trayStateIndicator
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			if window, err := findSystrayWindow(); err != nil {
				a.showError(fmt.Errorf("create status tray icon: %w", err))
			} else {
				stateTray.Lock()
				stateTray.indicator = newTrayStateIndicator(window)
				stateTray.Unlock()
			}
			restore := systray.AddMenuItem("Open TimeManager", "Restore the window")
			stop := systray.AddMenuItem("Stop and save timer", "Save the active booking")
			pause := systray.AddMenuItem("Pause timer", "Save the current segment and pause tracking")
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
			// Never a valid color, so the first update always sets an icon.
			lastColor := "unset"
			lastProjectIcon := ""
			lastState := trayState(-1)
			stateFailed := false
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
				pause.SetTitle("Pause timer")
				pause.Disable()
				if running {
					pause.Enable()
				} else if d.PausedProjectID != "" {
					pause.SetTitle("Resume paused timer")
					pause.Enable()
				}
				projectIcon := trayStatusIcon(d)
				if trayStateOf(d) == trayStopped {
					hex, projectIcon = "", ""
				}
				if hex != lastColor || projectIcon != lastProjectIcon {
					var icon []byte
					var err error
					if hex == "" {
						icon, err = trayBlankIcon()
					} else {
						icon, err = trayProjectMark(hex, projectIcon)
					}
					if err != nil {
						a.showError(fmt.Errorf("create tray icon: %w", err))
						return
					}
					systray.SetIcon(icon)
					lastColor = hex
					lastProjectIcon = projectIcon
				}
				stateTray.Lock()
				if indicator := stateTray.indicator; indicator != nil {
					state := trayStateOf(d)
					var icon []byte
					var err error
					if state != lastState {
						icon, err = trayStateIcon(state)
					}
					if err == nil {
						err = indicator.update(icon, tooltip(text))
					}
					if err == nil {
						lastState = state
						stateFailed = false
					} else if !stateFailed {
						// Report once; Explorer may be restarting and the icon is retried every second.
						stateFailed = true
						a.showError(fmt.Errorf("update status tray icon: %w", err))
					}
				}
				stateTray.Unlock()
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
					{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(pause.ClickedCh)},
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
					if a.store.Snapshot().Active != nil {
						if err := a.PauseTimer(); err != nil {
							a.showError(err)
						}
					} else if err := a.ResumeTimer(); err != nil {
						a.showError(err)
					}
					update()
				case 5:
					a.requestQuitConfirmation()
				case 6:
					if err := a.openProjectSearch(); err != nil {
						a.showError(fmt.Errorf("open project search: %w", err))
					}
				case 7:
					if err := a.StartTemporaryTimer(); err != nil {
						a.showError(fmt.Errorf("start temporary timer: %w", err))
					}
					update()
				default:
					id := entries[selected-8].id
					if err := a.timerAction("favorite", id); err != nil {
						a.showError(fmt.Errorf("change tray project: %w", err))
					}
					update()
				}
			}
		}, func() {})
	}()
	return func() {
		close(done)
		stateTray.Lock()
		stateTray.indicator.remove()
		stateTray.indicator = nil
		stateTray.Unlock()
		systray.Quit()
	}
}
