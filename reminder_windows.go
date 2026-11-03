package main

import (
	"fmt"
	goruntime "runtime"
	"sync"
	"time"
	"unsafe"

	"TimeManager/internal/tracker"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	reminderUser   = windows.NewLazySystemDLL("user32.dll")
	reminderGDI    = windows.NewLazySystemDLL("gdi32.dll")
	reminderKernel = windows.NewLazySystemDLL("kernel32.dll")
)

type reminderRect struct{ Left, Top, Right, Bottom int32 }
type reminderMonitor struct {
	Size          uint32
	Monitor, Work reminderRect
	Flags         uint32
}
type reminderClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type reminderMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

func nativeReminderCall(dll *windows.LazyDLL, name string, args ...uintptr) (uintptr, error) {
	result, _, err := dll.NewProc(name).Call(args...)
	if result == 0 {
		return 0, fmt.Errorf("%s failed: %w", name, err)
	}
	return result, nil
}

func (a *App) startReminders() (func(), func(bool) error, error) {
	ready := make(chan struct {
		window uintptr
		err    error
	}, 1)
	stopped := make(chan struct{})
	go func() {
		goruntime.LockOSThread()
		defer goruntime.UnlockOSThread()
		defer close(stopped)
		window, err := a.runReminderWindow(ready)
		if err != nil {
			if window == 0 {
				ready <- struct {
					window uintptr
					err    error
				}{err: err}
			} else {
				go a.showError(err)
			}
		}
	}()
	result := <-ready
	if result.err != nil {
		return nil, nil, result.err
	}
	post := func(message, value uintptr) error {
		_, err := nativeReminderCall(reminderUser, "PostMessageW", result.window, message, value, 0)
		return err
	}
	var once sync.Once
	closeWindow := func() {
		once.Do(func() {
			if err := post(0x0010, 0); err != nil {
				// Shutdown must not reopen the main window.
				wailsruntime.LogError(a.ctx, fmt.Sprintf("Close reminder window: %v", err))
			}
			<-stopped
		})
	}
	preview := func(running bool) error {
		value := uintptr(0)
		if running {
			value = 1
		}
		return post(0x8001, value)
	}
	return closeWindow, preview, nil
}

func (a *App) runReminderWindow(ready chan<- struct {
	window uintptr
	err    error
}) (uintptr, error) {
	instance, err := nativeReminderCall(reminderKernel, "GetModuleHandleW", 0)
	if err != nil {
		return 0, err
	}
	className, err := windows.UTF16PtrFromString("TimeManagerReminderWindow")
	if err != nil {
		return 0, err
	}
	var hwnd, title, body, closeButton, openButton, brush uintptr
	var background, foreground uintptr
	var clock tracker.ReminderClock
	var nativeErr error
	setText := func(control uintptr, text string) error {
		value, err := windows.UTF16PtrFromString(text)
		if err != nil {
			return err
		}
		_, err = nativeReminderCall(reminderUser, "SetWindowTextW", control, uintptr(unsafe.Pointer(value)))
		return err
	}
	hide := func() {
		reminderUser.NewProc("KillTimer").Call(hwnd, 2)
		reminderUser.NewProc("ShowWindow").Call(hwnd, 0)
	}
	show := func(running, preview bool) error {
		d := a.store.Snapshot()
		heading, text := "No timer is running", "Remember to start a project timer if you are working."
		if running {
			heading, text = "Your timer is still running", "Remember to stop and save your timer when you finish working."
			if d.Active != nil {
				seconds := int64(time.Since(d.Active.Start).Seconds())
				if seconds < 0 {
					seconds = 0
				}
				for _, p := range d.Projects {
					if p.ID == d.Active.ProjectID {
						text = fmt.Sprintf("%s - %s\nRunning for %02d:%02d:%02d. Stop and save when finished.", p.Number, p.Name, seconds/3600, seconds/60%60, seconds%60)
					}
				}
			}
		}
		if preview {
			heading = "Preview: " + heading
		}
		dark := d.Settings.Theme == "dark"
		if d.Settings.Theme == "system" {
			key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
			if err != nil {
				return fmt.Errorf("read Windows appearance for reminder: %w", err)
			}
			value, _, readErr := key.GetIntegerValue("AppsUseLightTheme")
			key.Close()
			if readErr != nil {
				return fmt.Errorf("read Windows color preference: %w", readErr)
			}
			dark = value == 0
		}
		background, foreground = 0xFFFFFF, 0x4A3425
		if dark {
			background, foreground = 0x322319, 0xF4E9E3
		}
		nextBrush, err := nativeReminderCall(reminderGDI, "CreateSolidBrush", background)
		if err != nil {
			return err
		}
		old := brush
		brush = nextBrush
		if old != 0 {
			reminderGDI.NewProc("DeleteObject").Call(old)
		}
		if err := setText(title, heading); err != nil {
			return err
		}
		if err := setText(body, text); err != nil {
			return err
		}
		monitor, err := nativeReminderCall(reminderUser, "MonitorFromPoint", 0, 1)
		if err != nil {
			return err
		}
		info := reminderMonitor{Size: uint32(unsafe.Sizeof(reminderMonitor{}))}
		if _, err := nativeReminderCall(reminderUser, "GetMonitorInfoW", monitor, uintptr(unsafe.Pointer(&info))); err != nil {
			return err
		}
		dpi, err := nativeReminderCall(reminderUser, "GetDpiForWindow", hwnd)
		if err != nil {
			return err
		}
		scale := func(n int32) int32 { return n * int32(dpi) / 96 }
		width, height, margin := scale(390), scale(180), scale(16)
		x, y := info.Work.Right-width-margin, info.Work.Bottom-height-margin
		if x < info.Work.Left {
			x = info.Work.Left
		}
		if y < info.Work.Top {
			y = info.Work.Top
		}
		// TOPMOST + NOACTIVATE: visible over other apps without stealing typing focus.
		if _, err := nativeReminderCall(reminderUser, "SetWindowPos", hwnd, ^uintptr(0), uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x0050); err != nil {
			return err
		}
		for _, control := range []struct {
			h            uintptr
			x, y, w, hgt int32
		}{
			{title, 16, 14, 354, 28}, {body, 16, 48, 354, 64}, {openButton, 16, 126, 168, 32}, {closeButton, 278, 126, 92, 32},
		} {
			if _, err := nativeReminderCall(reminderUser, "MoveWindow", control.h, uintptr(scale(control.x)), uintptr(scale(control.y)), uintptr(scale(control.w)), uintptr(scale(control.hgt)), 1); err != nil {
				return err
			}
		}
		reminderUser.NewProc("InvalidateRect").Call(hwnd, 0, 1)
		_, err = nativeReminderCall(reminderUser, "SetTimer", hwnd, 2, 15000, 0)
		return err
	}
	proc := windows.NewCallback(func(window uintptr, message uint32, wparam, lparam uintptr) uintptr {
		switch message {
		case 0x0010: // WM_CLOSE
			reminderUser.NewProc("DestroyWindow").Call(window)
			return 0
		case 0x0002:
			reminderUser.NewProc("PostQuitMessage").Call(0)
			return 0
		case 0x0014: // WM_ERASEBKGND
			if brush != 0 {
				var rect reminderRect
				reminderUser.NewProc("GetClientRect").Call(window, uintptr(unsafe.Pointer(&rect)))
				reminderUser.NewProc("FillRect").Call(wparam, uintptr(unsafe.Pointer(&rect)), brush)
				return 1
			}
		case 0x0138: // WM_CTLCOLORSTATIC
			if brush != 0 {
				reminderGDI.NewProc("SetTextColor").Call(wparam, foreground)
				reminderGDI.NewProc("SetBkColor").Call(wparam, background)
				return brush
			}
		case 0x0111:
			switch wparam & 0xffff {
			case 100:
				hide()
			case 101:
				hide()
				go a.restoreWindow()
			}
			return 0
		case 0x0100:
			if wparam == 0x1b {
				hide()
				return 0
			}
		case 0x0113:
			if wparam == 2 {
				hide()
				return 0
			}
			d := a.store.Snapshot()
			due, reset := clock.Poll(time.Now(), d)
			if reset {
				hide()
			}
			if due {
				nativeErr = show(d.Active != nil, false)
			}
			if nativeErr != nil {
				reminderUser.NewProc("DestroyWindow").Call(window)
			}
			return 0
		case 0x8001:
			nativeErr = show(wparam != 0, true)
			if nativeErr != nil {
				reminderUser.NewProc("DestroyWindow").Call(window)
			}
			return 0
		}
		result, _, _ := reminderUser.NewProc("DefWindowProcW").Call(window, uintptr(message), wparam, lparam)
		return result
	})
	cursor, err := nativeReminderCall(reminderUser, "LoadCursorW", 0, 32512)
	if err != nil {
		return 0, err
	}
	class := reminderClass{Size: uint32(unsafe.Sizeof(reminderClass{})), Proc: proc, Instance: instance, Cursor: cursor, Background: 6, Name: className}
	if _, err := nativeReminderCall(reminderUser, "RegisterClassExW", uintptr(unsafe.Pointer(&class))); err != nil {
		return 0, err
	}
	defer reminderUser.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(className)), instance)
	create := func(className, text string, style, parent, id uintptr) (uintptr, error) {
		name, err := windows.UTF16PtrFromString(className)
		if err != nil {
			return 0, err
		}
		caption, err := windows.UTF16PtrFromString(text)
		if err != nil {
			return 0, err
		}
		exStyle := uintptr(0)
		if parent == 0 {
			exStyle = 0x08000088
		}
		return nativeReminderCall(reminderUser, "CreateWindowExW", exStyle, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(caption)), style, 0, 0, 390, 180, parent, id, instance, 0)
	}
	hwnd, err = create("TimeManagerReminderWindow", "TimeManager reminder", 0x80800000, 0, 0)
	if err != nil {
		return 0, err
	}
	defer reminderUser.NewProc("DestroyWindow").Call(hwnd)
	defer func() {
		if brush != 0 {
			reminderGDI.NewProc("DeleteObject").Call(brush)
		}
	}()
	font, err := nativeReminderCall(reminderGDI, "GetStockObject", 17)
	if err != nil {
		return 0, err
	}
	for _, control := range []struct {
		class, text string
		style, id   uintptr
		target      *uintptr
	}{
		{"STATIC", "", 0x50000000, 102, &title},
		{"STATIC", "", 0x50000000, 103, &body},
		{"BUTTON", "Close", 0x50010000, 100, &closeButton},
		{"BUTTON", "Open TimeManager", 0x50010000, 101, &openButton},
	} {
		h, err := create(control.class, control.text, control.style, hwnd, control.id)
		if err != nil {
			return 0, err
		}
		*control.target = h
		reminderUser.NewProc("SendMessageW").Call(h, 0x0030, font, 1)
	}
	clock.Poll(time.Now(), a.store.Snapshot())
	if _, err := nativeReminderCall(reminderUser, "SetTimer", hwnd, 1, 1000, 0); err != nil {
		return 0, err
	}
	ready <- struct {
		window uintptr
		err    error
	}{window: hwnd}
	var message reminderMessage
	for {
		result, _, callErr := reminderUser.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			return hwnd, fmt.Errorf("reminder message loop: %w", callErr)
		}
		if result == 0 {
			return hwnd, nativeErr
		}
		reminderUser.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
		reminderUser.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
	}
}
