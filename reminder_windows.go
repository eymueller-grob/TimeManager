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
	reminderDWM    = windows.NewLazySystemDLL("dwmapi.dll")
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
	var hwnd uintptr
	var painter reminderPainter
	var content reminderContent
	var palette reminderPalette
	var hover int
	var visible, shownRunning, shownPreview bool
	var clock tracker.ReminderClock
	var nativeErr error
	invalidate := func() { reminderUser.NewProc("InvalidateRect").Call(hwnd, 0, 0) }
	hide := func() {
		reminderUser.NewProc("KillTimer").Call(hwnd, 2)
		reminderUser.NewProc("ShowWindow").Call(hwnd, 0)
		visible, hover = false, 0
	}
	show := func(running, preview bool) error {
		d := a.store.Snapshot()
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
		if d.Settings.Theme == "win95" {
			palette = reminderPaletteWin95()
		} else {
			palette = reminderPaletteFor(dark)
		}
		content = reminderContentFor(d, running, preview, time.Now())
		shownRunning, shownPreview, hover = running, preview, 0
		// Windows 11 border color; unsupported on older Windows, where the painted frame is used instead.
		border := uint32(colorRef(palette.Border))
		reminderDWM.NewProc("DwmSetWindowAttribute").Call(hwnd, 34, uintptr(unsafe.Pointer(&border)), 4)
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
		if err := painter.setDPI(uint32(dpi)); err != nil {
			return err
		}
		width, height, margin := painter.scale(reminderWidth), painter.scale(reminderHeight), painter.scale(16)
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
		visible = true
		invalidate()
		_, err = nativeReminderCall(reminderUser, "SetTimer", hwnd, 2, 15000, 0)
		return err
	}
	logicalPoint := func(lparam uintptr) (int32, int32) {
		x, y := int32(int16(lparam&0xffff)), int32(int16(lparam>>16&0xffff))
		if painter.dpi == 0 {
			return x, y
		}
		return x * 96 / int32(painter.dpi), y * 96 / int32(painter.dpi)
	}
	var handCursor uintptr
	proc := windows.NewCallback(func(window uintptr, message uint32, wparam, lparam uintptr) uintptr {
		switch message {
		case 0x0010: // WM_CLOSE
			reminderUser.NewProc("DestroyWindow").Call(window)
			return 0
		case 0x0002: // WM_DESTROY
			reminderUser.NewProc("PostQuitMessage").Call(0)
			return 0
		case 0x0014: // WM_ERASEBKGND: WM_PAINT covers the whole card.
			return 1
		case 0x000F: // WM_PAINT
			if err := painter.paint(window, content, palette, hover); err != nil && nativeErr == nil {
				nativeErr = fmt.Errorf("draw reminder: %w", err)
				reminderUser.NewProc("DestroyWindow").Call(window)
			}
			return 0
		case 0x0021: // WM_MOUSEACTIVATE: clicks must not take focus from other apps.
			return 3
		case 0x0020: // WM_SETCURSOR
			if hover != 0 && handCursor != 0 {
				reminderUser.NewProc("SetCursor").Call(handCursor)
				return 1
			}
		case 0x0200: // WM_MOUSEMOVE
			if hit := reminderHit(logicalPoint(lparam)); hit != hover {
				hover = hit
				invalidate()
			}
			track := struct {
				Size, Flags uint32
				Window      uintptr
				HoverTime   uint32
			}{Flags: 2, Window: window}
			track.Size = uint32(unsafe.Sizeof(track))
			reminderUser.NewProc("TrackMouseEvent").Call(uintptr(unsafe.Pointer(&track)))
			return 0
		case 0x02A3: // WM_MOUSELEAVE
			if hover != 0 {
				hover = 0
				invalidate()
			}
			return 0
		case 0x0202: // WM_LBUTTONUP
			switch reminderHit(logicalPoint(lparam)) {
			case reminderDismissID:
				hide()
			case reminderOpenID:
				hide()
				go a.restoreWindow()
			}
			return 0
		case 0x0100: // WM_KEYDOWN
			if wparam == 0x1b {
				hide()
				return 0
			}
		case 0x0113: // WM_TIMER
			if wparam == 2 {
				hide()
				return 0
			}
			now := time.Now()
			d := a.store.Snapshot()
			due, reset := clock.Poll(now, d)
			if reset {
				hide()
			}
			if due {
				nativeErr = show(d.Active != nil, false)
			} else if visible && shownRunning {
				content = reminderContentFor(d, true, shownPreview, now)
				invalidate()
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
	handCursor, err = nativeReminderCall(reminderUser, "LoadCursorW", 0, 32649)
	if err != nil {
		return 0, err
	}
	// CS_DROPSHADOW gives the borderless card a soft shadow.
	class := reminderClass{Size: uint32(unsafe.Sizeof(reminderClass{})), Style: 0x00020000, Proc: proc, Instance: instance, Cursor: cursor, Name: className}
	if _, err := nativeReminderCall(reminderUser, "RegisterClassExW", uintptr(unsafe.Pointer(&class))); err != nil {
		return 0, err
	}
	defer reminderUser.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(className)), instance)
	caption, err := windows.UTF16PtrFromString("TimeManager reminder")
	if err != nil {
		return 0, err
	}
	// WS_POPUP; WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE.
	hwnd, err = nativeReminderCall(reminderUser, "CreateWindowExW", 0x08000088, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(caption)),
		0x80000000, 0, 0, reminderWidth, reminderHeight, 0, 0, instance, 0)
	if err != nil {
		return 0, err
	}
	defer reminderUser.NewProc("DestroyWindow").Call(hwnd)
	defer painter.release()
	// Windows 11 rounded corners (DWMWCP_ROUND); older Windows keeps square corners.
	corner := uint32(2)
	reminderDWM.NewProc("DwmSetWindowAttribute").Call(hwnd, 33, uintptr(unsafe.Pointer(&corner)), 4)
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
