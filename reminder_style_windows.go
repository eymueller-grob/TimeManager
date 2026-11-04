package main

import (
	"fmt"
	"image"
	"image/color"
	"time"
	"unsafe"

	"TimeManager/internal/tracker"
	"golang.org/x/sys/windows"
)

// Reminder card size and button areas in 96-DPI logical pixels.
const (
	reminderWidth     = 400
	reminderHeight    = 168
	reminderDismissID = 100
	reminderOpenID    = 101
)

var (
	reminderDismissRect = reminderRect{Left: 132, Top: 120, Right: 220, Bottom: 152}
	reminderOpenRect    = reminderRect{Left: 228, Top: 120, Right: 376, Bottom: 152}

	reminderRunningAccent = color.NRGBA{R: 0x16, G: 0xa3, B: 0x4a, A: 0xff}
	reminderIdleAccent    = color.NRGBA{R: 0x64, G: 0x74, B: 0x8b, A: 0xff}
)

// reminderContent is what the reminder card shows.
type reminderContent struct {
	Eyebrow, Title, Body, Elapsed string
	Accent                        color.NRGBA
	Icon                          string
}

func reminderContentFor(d tracker.State, running, preview bool, now time.Time) reminderContent {
	c := reminderContent{Eyebrow: "NO TIMER RUNNING", Title: "No timer is running",
		Body: "Remember to start a project timer if you are working.", Accent: reminderIdleAccent}
	if running {
		c = reminderContent{Eyebrow: "STILL TRACKING", Title: "Your timer is still running",
			Body: "Remember to stop and save your timer when you finish working.", Accent: reminderRunningAccent}
		if d.Active != nil {
			seconds := max(int64(now.Sub(d.Active.Start).Seconds()), 0)
			c.Elapsed = fmt.Sprintf("%02d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
			for _, p := range d.Projects {
				if p.ID == d.Active.ProjectID {
					c.Title = p.Number + " - " + p.Name
					c.Icon = p.Icon
					if accent, err := parseTrayColor(p.Color); err == nil {
						c.Accent = accent
					}
				}
			}
		}
	}
	if preview {
		c.Eyebrow = "PREVIEW · " + c.Eyebrow
	}
	return c
}

// reminderPalette mirrors the app's shadcn neutral theme colors.
type reminderPalette struct {
	Background, Foreground, Muted, Border, Primary, PrimaryHover, PrimaryText, OutlineHover color.NRGBA
}

func reminderPaletteFor(dark bool) reminderPalette {
	rgb := func(r, g, b uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: 0xff} }
	if dark {
		return reminderPalette{Background: rgb(0x17, 0x17, 0x17), Foreground: rgb(0xfa, 0xfa, 0xfa), Muted: rgb(0xa1, 0xa1, 0xa1),
			Border: rgb(0x2e, 0x2e, 0x2e), Primary: rgb(0xe5, 0xe5, 0xe5), PrimaryHover: rgb(0xcc, 0xcc, 0xcc),
			PrimaryText: rgb(0x17, 0x17, 0x17), OutlineHover: rgb(0x26, 0x26, 0x26)}
	}
	return reminderPalette{Background: rgb(0xff, 0xff, 0xff), Foreground: rgb(0x0a, 0x0a, 0x0a), Muted: rgb(0x73, 0x73, 0x73),
		Border: rgb(0xe5, 0xe5, 0xe5), Primary: rgb(0x17, 0x17, 0x17), PrimaryHover: rgb(0x2e, 0x2e, 0x2e),
		PrimaryText: rgb(0xfa, 0xfa, 0xfa), OutlineHover: rgb(0xf5, 0xf5, 0xf5)}
}

func colorRef(c color.NRGBA) uintptr { return uintptr(c.R) | uintptr(c.G)<<8 | uintptr(c.B)<<16 }

// reminderHit returns the button ID at a logical point, or 0.
func reminderHit(x, y int32) int {
	for _, b := range []struct {
		id   int
		rect reminderRect
	}{{reminderDismissID, reminderDismissRect}, {reminderOpenID, reminderOpenRect}} {
		if x >= b.rect.Left && x < b.rect.Right && y >= b.rect.Top && y < b.rect.Bottom {
			return b.id
		}
	}
	return 0
}

type reminderBitmapInfo struct {
	Size                        uint32
	Width, Height               int32
	Planes, BitCount            uint16
	Compression, SizeImage      uint32
	XPerMeter, YPerMeter        int32
	ColorsUsed, ColorsImportant uint32
}

// reminderPainter draws the reminder card with GDI, double-buffered.
type reminderPainter struct {
	dpi                                  uint32
	eyebrow, title, body, elapsed, label uintptr
	iconKey                              string
	icon                                 image.Image
}

func (p *reminderPainter) scale(n int32) int32 { return n * int32(p.dpi) / 96 }

func (p *reminderPainter) release() {
	for _, font := range []*uintptr{&p.eyebrow, &p.title, &p.body, &p.elapsed, &p.label} {
		if *font != 0 {
			reminderGDI.NewProc("DeleteObject").Call(*font)
			*font = 0
		}
	}
}

func (p *reminderPainter) setDPI(dpi uint32) error {
	if dpi == p.dpi && p.title != 0 {
		return nil
	}
	p.release()
	p.dpi = dpi
	face, err := windows.UTF16PtrFromString("Segoe UI")
	if err != nil {
		return err
	}
	for _, font := range []struct {
		target       *uintptr
		size, weight int32
	}{{&p.eyebrow, 10, 700}, {&p.title, 18, 600}, {&p.body, 13, 400}, {&p.elapsed, 28, 600}, {&p.label, 13, 500}} {
		// Negative heights select by character height; 5 is CLEARTYPE_QUALITY.
		h, err := nativeReminderCall(reminderGDI, "CreateFontW", uintptr(-p.scale(font.size)), 0, 0, 0, uintptr(font.weight),
			0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
		if err != nil {
			p.release()
			return err
		}
		*font.target = h
	}
	return nil
}

func (p *reminderPainter) scaled(r reminderRect) reminderRect {
	return reminderRect{Left: p.scale(r.Left), Top: p.scale(r.Top), Right: p.scale(r.Right), Bottom: p.scale(r.Bottom)}
}

func (p *reminderPainter) paint(window uintptr, c reminderContent, pal reminderPalette, hover int) error {
	var paint [72]byte // PAINTSTRUCT
	hdc, err := nativeReminderCall(reminderUser, "BeginPaint", window, uintptr(unsafe.Pointer(&paint[0])))
	if err != nil {
		return err
	}
	defer reminderUser.NewProc("EndPaint").Call(window, uintptr(unsafe.Pointer(&paint[0])))
	var client reminderRect
	if _, err := nativeReminderCall(reminderUser, "GetClientRect", window, uintptr(unsafe.Pointer(&client))); err != nil {
		return err
	}
	mem, err := nativeReminderCall(reminderGDI, "CreateCompatibleDC", hdc)
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("DeleteDC").Call(mem)
	bitmap, err := nativeReminderCall(reminderGDI, "CreateCompatibleBitmap", hdc, uintptr(client.Right), uintptr(client.Bottom))
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("DeleteObject").Call(bitmap)
	oldBitmap, err := nativeReminderCall(reminderGDI, "SelectObject", mem, bitmap)
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("SelectObject").Call(mem, oldBitmap)

	if err := fillReminderRect(mem, client, pal.Background); err != nil {
		return err
	}
	if err := fillReminderRect(mem, reminderRect{Right: p.scale(5), Bottom: client.Bottom}, c.Accent); err != nil {
		return err
	}
	border, err := nativeReminderCall(reminderGDI, "CreateSolidBrush", colorRef(pal.Border))
	if err != nil {
		return err
	}
	reminderUser.NewProc("FrameRect").Call(mem, uintptr(unsafe.Pointer(&client)), border)
	reminderGDI.NewProc("DeleteObject").Call(border)
	reminderGDI.NewProc("SetBkMode").Call(mem, 1)

	const (
		center, right, vcenter, wordBreak, single = 0x1, 0x2, 0x4, 0x10, 0x20
		noPrefix, ellipsis                        = 0x800, 0x8000
	)
	right24 := p.scale(reminderWidth - 24)
	reminderGDI.NewProc("SetTextCharacterExtra").Call(mem, uintptr(p.scale(1)))
	err = drawReminderText(mem, p.eyebrow, pal.Muted, c.Eyebrow, reminderRect{Left: p.scale(24), Top: p.scale(16), Right: right24, Bottom: p.scale(32)}, single|noPrefix|ellipsis)
	reminderGDI.NewProc("SetTextCharacterExtra").Call(mem, 0)
	if err != nil {
		return err
	}
	titleRight := right24
	if c.Elapsed != "" {
		elapsedRect := reminderRect{Left: right24 - p.scale(160), Top: p.scale(30), Right: right24, Bottom: p.scale(64)}
		if err := drawReminderText(mem, p.elapsed, pal.Foreground, c.Elapsed, elapsedRect, right|vcenter|single|noPrefix); err != nil {
			return err
		}
		titleRight = right24 - p.measure(mem, p.elapsed, c.Elapsed) - p.scale(12)
	}
	titleLeft := p.scale(24)
	if c.Icon != "" {
		size := p.scale(22)
		if err := p.drawIcon(mem, c.Icon, titleLeft, p.scale(36), size); err != nil {
			return err
		}
		titleLeft += size + p.scale(8)
	}
	if err := drawReminderText(mem, p.title, pal.Foreground, c.Title, reminderRect{Left: titleLeft, Top: p.scale(32), Right: titleRight, Bottom: p.scale(62)}, vcenter|single|noPrefix|ellipsis); err != nil {
		return err
	}
	if err := drawReminderText(mem, p.body, pal.Muted, c.Body, reminderRect{Left: p.scale(24), Top: p.scale(68), Right: right24, Bottom: p.scale(108)}, wordBreak|noPrefix); err != nil {
		return err
	}
	for _, b := range []struct {
		id                     int
		rect                   reminderRect
		text                   string
		fill, hover, edge, ink color.NRGBA
	}{
		{reminderDismissID, reminderDismissRect, "Dismiss", pal.Background, pal.OutlineHover, pal.Border, pal.Foreground},
		{reminderOpenID, reminderOpenRect, "Open TimeManager", pal.Primary, pal.PrimaryHover, pal.Primary, pal.PrimaryText},
	} {
		fill := b.fill
		if hover == b.id {
			fill = b.hover
		}
		r := p.scaled(b.rect)
		if err := roundReminderRect(mem, r, fill, b.edge, p.scale(16)); err != nil {
			return err
		}
		if err := drawReminderText(mem, p.label, b.ink, b.text, r, center|vcenter|single|noPrefix); err != nil {
			return err
		}
	}
	_, err = nativeReminderCall(reminderGDI, "BitBlt", hdc, 0, 0, uintptr(client.Right), uintptr(client.Bottom), mem, 0, 0, 0x00CC0020)
	return err
}

func (p *reminderPainter) measure(dc, font uintptr, text string) int32 {
	value, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return 0
	}
	old, _, _ := reminderGDI.NewProc("SelectObject").Call(dc, font)
	defer reminderGDI.NewProc("SelectObject").Call(dc, old)
	var r reminderRect
	reminderUser.NewProc("DrawTextW").Call(dc, uintptr(unsafe.Pointer(value)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), 0x20|0x400|0x800)
	return r.Right - r.Left
}

// drawIcon alpha-blends the project icon, scaled to size×size, at (x, y).
func (p *reminderPainter) drawIcon(dc uintptr, dataURL string, x, y, size int32) error {
	if p.iconKey != dataURL || p.icon == nil {
		icon, err := tracker.DecodeProjectIcon(dataURL)
		if err != nil {
			return fmt.Errorf("decode reminder project icon: %w", err)
		}
		p.iconKey, p.icon = dataURL, icon
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, int(size), int(size)))
	drawScaled(scaled, p.icon)
	info := reminderBitmapInfo{Size: uint32(unsafe.Sizeof(reminderBitmapInfo{})), Width: size, Height: -size, Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	dib, err := nativeReminderCall(reminderGDI, "CreateDIBSection", dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("DeleteObject").Call(dib)
	pixels := unsafe.Slice((*byte)(bits), len(scaled.Pix))
	for i := 0; i < len(scaled.Pix); i += 4 {
		a := uint32(scaled.Pix[i+3])
		// GDI expects premultiplied BGRA.
		pixels[i] = uint8(uint32(scaled.Pix[i+2]) * a / 255)
		pixels[i+1] = uint8(uint32(scaled.Pix[i+1]) * a / 255)
		pixels[i+2] = uint8(uint32(scaled.Pix[i]) * a / 255)
		pixels[i+3] = uint8(a)
	}
	source, err := nativeReminderCall(reminderGDI, "CreateCompatibleDC", dc)
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("DeleteDC").Call(source)
	old, err := nativeReminderCall(reminderGDI, "SelectObject", source, dib)
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("SelectObject").Call(source, old)
	// BLENDFUNCTION{AC_SRC_OVER, 0, 255, AC_SRC_ALPHA}
	_, err = nativeReminderCall(reminderGDI, "GdiAlphaBlend", dc, uintptr(x), uintptr(y), uintptr(size), uintptr(size), source, 0, 0, uintptr(size), uintptr(size), 0x01FF0000)
	return err
}

func fillReminderRect(dc uintptr, r reminderRect, c color.NRGBA) error {
	brush, err := nativeReminderCall(reminderGDI, "CreateSolidBrush", colorRef(c))
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("DeleteObject").Call(brush)
	_, err = nativeReminderCall(reminderUser, "FillRect", dc, uintptr(unsafe.Pointer(&r)), brush)
	return err
}

func roundReminderRect(dc uintptr, r reminderRect, fill, edge color.NRGBA, radius int32) error {
	brush, err := nativeReminderCall(reminderGDI, "CreateSolidBrush", colorRef(fill))
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("DeleteObject").Call(brush)
	pen, err := nativeReminderCall(reminderGDI, "CreatePen", 0, 1, colorRef(edge))
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("DeleteObject").Call(pen)
	oldBrush, _, _ := reminderGDI.NewProc("SelectObject").Call(dc, brush)
	defer reminderGDI.NewProc("SelectObject").Call(dc, oldBrush)
	oldPen, _, _ := reminderGDI.NewProc("SelectObject").Call(dc, pen)
	defer reminderGDI.NewProc("SelectObject").Call(dc, oldPen)
	_, err = nativeReminderCall(reminderGDI, "RoundRect", dc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	return err
}

func drawReminderText(dc, font uintptr, ink color.NRGBA, text string, r reminderRect, flags uintptr) error {
	value, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return err
	}
	old, err := nativeReminderCall(reminderGDI, "SelectObject", dc, font)
	if err != nil {
		return err
	}
	defer reminderGDI.NewProc("SelectObject").Call(dc, old)
	reminderGDI.NewProc("SetTextColor").Call(dc, colorRef(ink))
	_, err = nativeReminderCall(reminderUser, "DrawTextW", dc, uintptr(unsafe.Pointer(value)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
	return err
}
