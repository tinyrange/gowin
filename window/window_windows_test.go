//go:build windows

package window

import "testing"

func TestWindowsMessageKeyResolvesGenericModifiers(t *testing.T) {
	tests := []struct {
		name   string
		vk     uint32
		scan   uintptr
		extend bool
		want   Key
	}{
		{name: "left shift", vk: vkShift, scan: 0x2a, want: KeyLeftShift},
		{name: "right shift", vk: vkShift, scan: 0x36, want: KeyRightShift},
		{name: "left control", vk: vkControl, scan: 0x1d, want: KeyLeftControl},
		{name: "right control", vk: vkControl, scan: 0x1d, extend: true, want: KeyRightControl},
		{name: "left alt", vk: vkMenu, scan: 0x38, want: KeyLeftAlt},
		{name: "right alt", vk: vkMenu, scan: 0x38, extend: true, want: KeyRightAlt},
		{name: "left windows", vk: vkLWin, scan: 0x5b, extend: true, want: KeyLeftSuper},
		{name: "right windows", vk: vkRWin, scan: 0x5c, extend: true, want: KeyRightSuper},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lParam := test.scan << 16
			if test.extend {
				lParam |= 1 << 24
			}
			if got := windowsMessageKey(test.vk, lParam); got != test.want {
				t.Fatalf("windowsMessageKey(%#x, %#x) = %v, want %v", test.vk, lParam, got, test.want)
			}
		})
	}
}

func TestLogicalPixelsToPhysicalUsesDisplayDPI(t *testing.T) {
	tests := []struct {
		logical int
		dpi     uint32
		want    int32
	}{
		{logical: 1440, dpi: 96, want: 1440},
		{logical: 1440, dpi: 120, want: 1800},
		{logical: 1440, dpi: 144, want: 2160},
	}
	for _, test := range tests {
		if got := logicalPixelsToPhysical(test.logical, test.dpi); got != test.want {
			t.Errorf("logicalPixelsToPhysical(%d, %d) = %d, want %d", test.logical, test.dpi, got, test.want)
		}
	}
}

func TestReleaseCapturedKeysReleasesEveryHeldKey(t *testing.T) {
	w := &winWindow{
		keyStates: map[Key]KeyState{
			KeyA:           KeyStateDown,
			KeyLeftAlt:     KeyStatePressed,
			KeyLeftControl: KeyStateRepeated,
			KeyLeftSuper:   KeyStateDown,
			KeyB:           KeyStateUp,
		},
	}

	w.releaseCapturedKeys()

	for _, key := range []Key{KeyA, KeyLeftAlt, KeyLeftControl, KeyLeftSuper} {
		if state := w.keyStates[key]; state != KeyStateReleased {
			t.Errorf("key %v state = %v, want released", key, state)
		}
	}
	if state := w.keyStates[KeyB]; state != KeyStateUp {
		t.Errorf("unheld key state = %v, want up", state)
	}
	if len(w.inputEvents) != 4 {
		t.Fatalf("release events = %d, want 4", len(w.inputEvents))
	}
	for _, event := range w.inputEvents {
		if event.Type != InputEventKeyUp {
			t.Errorf("event type = %v, want key up", event.Type)
		}
	}
}

func TestClampWindowSizeToWorkArea(t *testing.T) {
	workArea := rect{left: 10, top: 20, right: 1034, bottom: 788}
	if width, height := clampWindowSize(1800, 1125, workArea); width != 1024 || height != 768 {
		t.Fatalf("clamped window = %dx%d, want 1024x768", width, height)
	}
	if width, height := clampWindowSize(800, 600, workArea); width != 800 || height != 600 {
		t.Fatalf("already fitting window = %dx%d, want 800x600", width, height)
	}
}

func TestMaximizedIntegratedWindowUsesWorkArea(t *testing.T) {
	monitor := monitorInfo{
		monitor: rect{left: 0, top: 0, right: 1920, bottom: 1080},
		work:    rect{left: 0, top: 0, right: 1920, bottom: 1040},
	}
	bounds := minMaxInfo{}
	applyMaximizedWorkArea(&bounds, monitor)
	if bounds.maxPosition != (point{x: 0, y: 0}) || bounds.maxSize != (point{x: 1920, y: 1040}) {
		t.Fatalf("maximized bounds = position %+v size %+v", bounds.maxPosition, bounds.maxSize)
	}
}
