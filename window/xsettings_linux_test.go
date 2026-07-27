//go:build linux

package window

import (
	"encoding/binary"
	"testing"
)

func TestParseXSettingsScaleUsesFractionalDPI(t *testing.T) {
	data := xSettingsIntFixture("Xft/DPI", 144*1024)
	if got := parseXSettingsScale(data); got != 1.5 {
		t.Fatalf("scale = %v, want 1.5", got)
	}
}

func TestParseXSettingsScaleRejectsTruncatedProperty(t *testing.T) {
	data := xSettingsIntFixture("Xft/DPI", 192*1024)
	if got := parseXSettingsScale(data[:len(data)-1]); got != 0 {
		t.Fatalf("scale = %v, want 0 for truncated property", got)
	}
}

func xSettingsIntFixture(name string, value int32) []byte {
	nameBytes := []byte(name)
	namePadded := (len(nameBytes) + 3) &^ 3
	data := make([]byte, 12+4+namePadded+4+4)
	data[0] = 'l'
	binary.LittleEndian.PutUint32(data[8:12], 1)
	data[12] = 0
	binary.LittleEndian.PutUint16(data[14:16], uint16(len(nameBytes)))
	copy(data[16:16+len(nameBytes)], nameBytes)
	offset := 16 + namePadded
	binary.LittleEndian.PutUint32(data[offset:offset+4], 1)
	binary.LittleEndian.PutUint32(data[offset+4:offset+8], uint32(value))
	return data
}
