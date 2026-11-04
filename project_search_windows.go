package main

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

type projectSearchReady struct {
	window uintptr
	err    error
}

func (a *App) startProjectSearch() (func(), func() error, error) {
	ready := make(chan projectSearchReady, 1)
	stopped := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(stopped)
		window, err := a.runProjectSearch(ready)
		if window == 0 {
			ready <- projectSearchReady{err: err}
		} else if err != nil {
			go a.showError(fmt.Errorf("project search: %w", err))
		}
	}()
	result := <-ready
	if result.err != nil {
		return nil, nil, result.err
	}
	post := func(message uintptr) error {
		_, err := nativeReminderCall(reminderUser, "PostMessageW", result.window, message, 0, 0)
		return err
	}
	var once sync.Once
	closeSearch := func() {
		once.Do(func() {
			if err := post(0x8002); err != nil {
				wailsruntime.LogError(a.ctx, fmt.Sprintf("Close project search: %v", err))
			}
			<-stopped
		})
	}
	return closeSearch, func() error { return post(0x8001) }, nil
}

func (a *App) runProjectSearch(ready chan<- projectSearchReady) (uintptr, error) {
	instance, err := nativeReminderCall(reminderKernel, "GetModuleHandleW", 0)
	if err != nil {
		return 0, err
	}
	className, err := windows.UTF16PtrFromString("TimeManagerProjectSearch")
	if err != nil {
		return 0, err
	}
	var hwnd, edit, list, status, choose uintptr
	var results []projectSearchResult
	var nativeErr error
	var refresh func() error
	setText := func(control uintptr, text string) error {
		value, err := windows.UTF16PtrFromString(text)
		if err != nil {
			return err
		}
		_, err = nativeReminderCall(reminderUser, "SetWindowTextW", control, uintptr(unsafe.Pointer(value)))
		return err
	}
	hide := func() { reminderUser.NewProc("ShowWindow").Call(hwnd, 0) }
	selectProject := func() error {
		index, _, _ := reminderUser.NewProc("SendMessageW").Call(list, 0x0188, 0, 0) // LB_GETCURSEL
		if int32(index) < 0 || int(index) >= len(results) {
			return nil
		}
		id := results[index].id
		if err := a.timerAction("start", id); err != nil {
			return setText(status, "Cannot change project: "+err.Error())
		}
		hide()
		return nil
	}
	refresh = func() error {
		text := make([]uint16, 4097)
		reminderUser.NewProc("GetWindowTextW").Call(edit, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
		next := projectSearchResults(a.store.Snapshot(), windows.UTF16ToString(text))
		if reflect.DeepEqual(results, next) {
			return nil
		}
		selectedID := ""
		index, _, _ := reminderUser.NewProc("SendMessageW").Call(list, 0x0188, 0, 0)
		if int32(index) >= 0 && int(index) < len(results) {
			selectedID = results[index].id
		}
		reminderUser.NewProc("SendMessageW").Call(list, 0x0184, 0, 0) // LB_RESETCONTENT
		selected := uintptr(0)
		for i, result := range next {
			label := result.label
			if result.running {
				label = "[RUNNING] " + label
			}
			value, err := windows.UTF16PtrFromString(label)
			if err != nil {
				return err
			}
			added, _, _ := reminderUser.NewProc("SendMessageW").Call(list, 0x0180, 0, uintptr(unsafe.Pointer(value)))
			if int32(added) < 0 {
				return fmt.Errorf("add project search result failed (%d)", int32(added))
			}
			if result.id == selectedID {
				selected = uintptr(i)
			}
		}
		results = next
		enabled := uintptr(0)
		if len(results) != 0 {
			enabled = 1
		}
		reminderUser.NewProc("EnableWindow").Call(choose, enabled)
		if len(results) == 0 {
			return setText(status, "No matching projects. Try a number, name, customer, parent, or property.")
		}
		reminderUser.NewProc("SendMessageW").Call(list, 0x0186, selected, 0) // LB_SETCURSEL
		return setText(status, fmt.Sprintf("%d projects. Arrow keys to select, Enter to switch, Escape to close.", len(results)))
	}
	proc := windows.NewCallback(func(window uintptr, message uint32, wparam, lparam uintptr) uintptr {
		switch message {
		case 0x0010: // WM_CLOSE dismisses; the dedicated thread is retained.
			hide()
			return 0
		case 0x8002:
			reminderUser.NewProc("DestroyWindow").Call(window)
			return 0
		case 0x0002:
			reminderUser.NewProc("PostQuitMessage").Call(0)
			return 0
		case 0x8001:
			results = nil
			nativeErr = setText(edit, "")
			if nativeErr == nil {
				nativeErr = refresh()
			}
			reminderUser.NewProc("ShowWindow").Call(window, 9)
			reminderUser.NewProc("SetForegroundWindow").Call(window)
			reminderUser.NewProc("SetFocus").Call(edit)
		case 0x0111:
			switch wparam & 0xffff {
			case 201:
				if wparam>>16 == 0x0300 && list != 0 { // EN_CHANGE
					nativeErr = refresh()
				}
			case 202:
				if wparam>>16 == 2 { // LBN_DBLCLK
					nativeErr = selectProject()
				}
			case 203:
				nativeErr = selectProject()
			case 204:
				hide()
			}
		case 0x0113:
			visible, _, _ := reminderUser.NewProc("IsWindowVisible").Call(window)
			if visible != 0 {
				nativeErr = refresh()
			}
		default:
			result, _, _ := reminderUser.NewProc("DefWindowProcW").Call(window, uintptr(message), wparam, lparam)
			return result
		}
		if nativeErr != nil {
			reminderUser.NewProc("DestroyWindow").Call(window)
		}
		return 0
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
	create := func(class, caption string, style, parent, id uintptr) (uintptr, error) {
		name, err := windows.UTF16PtrFromString(class)
		if err != nil {
			return 0, err
		}
		text, err := windows.UTF16PtrFromString(caption)
		if err != nil {
			return 0, err
		}
		ex := uintptr(0)
		if parent == 0 {
			ex = 0x00010080
		} // CONTROLPARENT + TOOLWINDOW
		return nativeReminderCall(reminderUser, "CreateWindowExW", ex, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(text)), style, 0, 0, 680, 430, parent, id, instance, 0)
	}
	hwnd, err = create("TimeManagerProjectSearch", "Change project - Search", 0x80C80000, 0, 0)
	if err != nil {
		return 0, err
	}
	defer reminderUser.NewProc("DestroyWindow").Call(hwnd)
	font, err := nativeReminderCall(reminderGDI, "GetStockObject", 17)
	if err != nil {
		return 0, err
	}
	dpi, err := nativeReminderCall(reminderUser, "GetDpiForWindow", hwnd)
	if err != nil {
		return 0, err
	}
	scale := func(n int32) uintptr { return uintptr(n * int32(dpi) / 96) }
	var label, cancel uintptr
	for _, c := range []struct {
		class, text string
		style, id   uintptr
		x, y, w, h  int32
		target      *uintptr
	}{
		{"STATIC", "Search by number, name, customer, parent, or custom properties:", 0x50000000, 200, 16, 12, 640, 22, &label},
		{"EDIT", "", 0x50810080, 201, 16, 38, 640, 28, &edit},
		{"LISTBOX", "", 0x50B10101, 202, 16, 78, 640, 232, &list},
		{"STATIC", "", 0x50000000, 205, 16, 318, 640, 36, &status},
		{"BUTTON", "Switch project", 0x50010001, 203, 406, 362, 130, 30, &choose},
		{"BUTTON", "Cancel", 0x50010000, 204, 546, 362, 110, 30, &cancel},
	} {
		h, err := create(c.class, c.text, c.style, hwnd, c.id)
		if err != nil {
			return 0, err
		}
		*c.target = h
		reminderUser.NewProc("SendMessageW").Call(h, 0x0030, font, 1)
		if _, err := nativeReminderCall(reminderUser, "MoveWindow", h, scale(c.x), scale(c.y), scale(c.w), scale(c.h), 1); err != nil {
			return 0, err
		}
	}
	reminderUser.NewProc("SendMessageW").Call(edit, 0x00C5, 4096, 0)        // EM_LIMITTEXT
	reminderUser.NewProc("SendMessageW").Call(list, 0x0194, scale(2400), 0) // LB_SETHORIZONTALEXTENT
	monitor, err := nativeReminderCall(reminderUser, "MonitorFromPoint", 0, 1)
	if err != nil {
		return 0, err
	}
	info := reminderMonitor{Size: uint32(unsafe.Sizeof(reminderMonitor{}))}
	if _, err := nativeReminderCall(reminderUser, "GetMonitorInfoW", monitor, uintptr(unsafe.Pointer(&info))); err != nil {
		return 0, err
	}
	// Adjust the outer frame so all controls fit at the window's DPI.
	rect := reminderRect{Right: int32(scale(680)), Bottom: int32(scale(410))}
	if _, err := nativeReminderCall(reminderUser, "AdjustWindowRectEx", uintptr(unsafe.Pointer(&rect)), 0x80C80000, 0, 0x00010080); err != nil {
		return 0, err
	}
	width, height := rect.Right-rect.Left, rect.Bottom-rect.Top
	x, y := info.Work.Left+(info.Work.Right-info.Work.Left-width)/2, info.Work.Top+(info.Work.Bottom-info.Work.Top-height)/2
	if _, err := nativeReminderCall(reminderUser, "SetWindowPos", hwnd, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x0014); err != nil {
		return 0, err
	}
	if _, err := nativeReminderCall(reminderUser, "SetTimer", hwnd, 1, 1000, 0); err != nil {
		return 0, err
	}
	ready <- projectSearchReady{window: hwnd}
	var message reminderMessage
	for {
		result, _, callErr := reminderUser.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) == -1 {
			return hwnd, fmt.Errorf("project search message loop: %w", callErr)
		}
		if result == 0 {
			return hwnd, nativeErr
		}
		if message.Message == 0x0100 { // WM_KEYDOWN
			switch message.WParam {
			case 0x1b:
				hide()
				continue
			case 0x0d:
				if message.Window == cancel {
					hide()
				} else {
					if err := selectProject(); err != nil {
						return hwnd, err
					}
				}
				continue
			case 0x26, 0x28:
				if message.Window == edit {
					index, _, _ := reminderUser.NewProc("SendMessageW").Call(list, 0x0188, 0, 0)
					next := int(int32(index))
					if message.WParam == 0x28 {
						next++
					} else {
						next--
					}
					if next >= 0 && next < len(results) {
						reminderUser.NewProc("SendMessageW").Call(list, 0x0186, uintptr(next), 0)
					}
					continue
				}
			}
		}
		handled, _, _ := reminderUser.NewProc("IsDialogMessageW").Call(hwnd, uintptr(unsafe.Pointer(&message)))
		if handled == 0 {
			reminderUser.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
			reminderUser.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
		}
	}
}
