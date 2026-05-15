package window

import "testing"

func TestInputEventTypeStringIncludesGenericEditorEvents(t *testing.T) {
	tests := map[InputEventType]string{
		InputEventMouseMove: "MouseMove",
		InputEventScroll:    "Scroll",
		InputEventPinch:     "Pinch",
	}
	for typ, want := range tests {
		if got := typ.String(); got != want {
			t.Fatalf("%v.String() = %q, want %q", uint8(typ), got, want)
		}
	}
}

func TestInputEventCanRepresentGenericDragScrollAndPinch(t *testing.T) {
	drag := InputEvent{
		Type:        InputEventMouseMove,
		Button:      ButtonMiddle,
		ButtonValid: true,
		MouseX:      12,
		MouseY:      34,
		Mods:        ModShift,
	}
	if drag.Type != InputEventMouseMove || !drag.ButtonValid || drag.Button != ButtonMiddle {
		t.Fatalf("drag event not represented: %#v", drag)
	}

	scroll := InputEvent{
		Type:          InputEventScroll,
		ScrollY:       0.25,
		RawScrollY:    2.5,
		PreciseScroll: true,
	}
	if scroll.Type != InputEventScroll || !scroll.PreciseScroll || scroll.RawScrollY == 0 {
		t.Fatalf("precise scroll event not represented: %#v", scroll)
	}

	pinch := InputEvent{
		Type:       InputEventPinch,
		PinchDelta: 0.1,
		PinchScale: 1.1,
	}
	if pinch.Type != InputEventPinch || pinch.PinchScale <= 1 {
		t.Fatalf("pinch event not represented: %#v", pinch)
	}
}
