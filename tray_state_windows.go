package main

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var trayShell = windows.NewLazySystemDLL("shell32.dll")

const (
	trayStateIconID = 101
	// getlantern/systray opens its menu for this callback message (WM_USER+1)
	// regardless of the icon ID, so the status icon shares the same menu.
	traySystrayCallback = 0x0400 + 1
	nimAdd              = 0
	nimModify           = 1
	nimDelete           = 2
	nifMessage          = 0x1
	nifIcon             = 0x2
	nifTip              = 0x4
)

type trayNotifyIconData struct {
	Size                       uint32
	Window                     uintptr
	ID, Flags, CallbackMessage uint32
	Icon                       uintptr
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Timeout                    uint32 // union with uVersion
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GUID                       windows.GUID
	BalloonIcon                uintptr
}

// trayStateIndicator is a second notification icon attached to the systray
// window. It is re-added automatically when Explorer restarts.
type trayStateIndicator struct {
	mu    sync.Mutex
	data  trayNotifyIconData
	added bool
}

func findSystrayWindow() (uintptr, error) {
	class, err := windows.UTF16PtrFromString("SystrayClass")
	if err != nil {
		return 0, err
	}
	pid := uint32(os.Getpid())
	var after uintptr
	for {
		window, _, _ := reminderUser.NewProc("FindWindowExW").Call(0, after, uintptr(unsafe.Pointer(class)), 0)
		if window == 0 {
			return 0, fmt.Errorf("systray window not found")
		}
		var owner uint32
		reminderUser.NewProc("GetWindowThreadProcessId").Call(window, uintptr(unsafe.Pointer(&owner)))
		if owner == pid {
			return window, nil
		}
		after = window
	}
}

func newTrayStateIndicator(window uintptr) *trayStateIndicator {
	s := &trayStateIndicator{}
	s.data.Size = uint32(unsafe.Sizeof(s.data))
	s.data.Window = window
	s.data.ID = trayStateIconID
	s.data.CallbackMessage = traySystrayCallback
	s.data.Flags = nifMessage | nifIcon | nifTip
	return s
}

func (s *trayStateIndicator) notify(operation uintptr) bool {
	ok, _, _ := trayShell.NewProc("Shell_NotifyIconW").Call(operation, uintptr(unsafe.Pointer(&s.data)))
	return ok != 0
}

// update replaces the icon when ico is non-nil and always refreshes the tooltip.
func (s *trayStateIndicator) update(ico []byte, tip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.data.Icon
	if ico != nil {
		if len(ico) <= 22 {
			return fmt.Errorf("invalid status icon")
		}
		payload := ico[22:]
		icon, _, err := reminderUser.NewProc("CreateIconFromResourceEx").Call(
			uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 1, 0x00030000, 32, 32, 0)
		if icon == 0 {
			return fmt.Errorf("create status icon: %w", err)
		}
		s.data.Icon = icon
	}
	if s.data.Icon == 0 {
		return fmt.Errorf("status icon has no image")
	}
	text, err := windows.UTF16FromString(tip)
	if err != nil {
		return err
	}
	s.data.Tip = [128]uint16{}
	copy(s.data.Tip[:len(s.data.Tip)-1], text)
	ok := s.added && s.notify(nimModify)
	if !ok {
		// Adding again also restores the icon after Explorer restarts.
		ok = s.notify(nimAdd)
	}
	// The shell keeps its own copy, so the previous handle can be released.
	if previous != 0 && previous != s.data.Icon {
		reminderUser.NewProc("DestroyIcon").Call(previous)
	}
	s.added = ok
	if !ok {
		return fmt.Errorf("show status tray icon failed")
	}
	return nil
}

func (s *trayStateIndicator) remove() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.added {
		s.notify(nimDelete)
		s.added = false
	}
	if s.data.Icon != 0 {
		reminderUser.NewProc("DestroyIcon").Call(s.data.Icon)
		s.data.Icon = 0
	}
}
