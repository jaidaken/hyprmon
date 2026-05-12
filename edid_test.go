package main

import "testing"

// makeEDIDWithExt returns a 256-byte EDID: 128-byte base block declaring one
// extension, then a 128-byte CTA-861 extension block containing the supplied
// data-block payload starting at byte 4.
func makeEDIDWithExt(dataBlocks []byte) []byte {
	edid := make([]byte, 256)
	edid[0x7E] = 1
	edid[128] = 0x02
	edid[128+1] = 0x03
	dtdOffset := 4 + len(dataBlocks)
	if dtdOffset > 128 {
		dtdOffset = 128
	}
	edid[128+2] = byte(dtdOffset)
	copy(edid[128+4:], dataBlocks)
	return edid
}

func TestEDIDHasHDRStaticMetadataPositive(t *testing.T) {
	hdrBlock := []byte{
		(7 << 5) | 3,
		0x06,
		0x03,
		0x00,
	}
	edid := makeEDIDWithExt(hdrBlock)
	if !edidHasHDRStaticMetadata(edid) {
		t.Errorf("EDID with HDR Static Metadata block should be detected")
	}
}

func TestEDIDHasHDRStaticMetadataNegative(t *testing.T) {
	otherExt := []byte{
		(7 << 5) | 2,
		0x05,
		0x00,
	}
	edid := makeEDIDWithExt(otherExt)
	if edidHasHDRStaticMetadata(edid) {
		t.Errorf("EDID without HDR block must not be detected as HDR")
	}
}

func TestEDIDHasHDRStaticMetadataNoExt(t *testing.T) {
	edid := make([]byte, 128)
	if edidHasHDRStaticMetadata(edid) {
		t.Errorf("EDID with zero extensions must report no HDR")
	}
}

func TestEDIDHasHDRStaticMetadataShort(t *testing.T) {
	short := make([]byte, 32)
	if edidHasHDRStaticMetadata(short) {
		t.Errorf("malformed/short EDID must report no HDR")
	}
}

func TestEDIDHasHDRStaticMetadataMultipleBlocks(t *testing.T) {
	mixed := []byte{
		(1 << 5) | 1,
		0x09,
		(7 << 5) | 2,
		0x05,
		0x00,
		(7 << 5) | 3,
		0x06,
		0x03,
		0x00,
	}
	edid := makeEDIDWithExt(mixed)
	if !edidHasHDRStaticMetadata(edid) {
		t.Errorf("HDR block among other blocks must be detected")
	}
}

func TestEDIDPeakLuminance(t *testing.T) {
	// Raw 0xC0 (192) -> 50 * 2^(192/32) = 50 * 64 = 3200 cd/m2
	hdrBlock := []byte{
		(7 << 5) | 5,
		0x06,
		0x03,
		0x00,
		0xC0,
		0xC0,
	}
	edid := makeEDIDWithExt(hdrBlock)
	got := edidPeakLuminance(edid)
	want := 3200
	if got < want-5 || got > want+5 {
		t.Errorf("expected ~%d cd/m2 for raw 0xC0, got %d", want, got)
	}
}

func TestEDIDPeakLuminanceTypicalHDR400(t *testing.T) {
	// Raw 0x80 (128) -> 50 * 2^4 = 800 cd/m2 (HDR1000 panel)
	hdrBlock := []byte{
		(7 << 5) | 4,
		0x06,
		0x03,
		0x00,
		0x80,
	}
	edid := makeEDIDWithExt(hdrBlock)
	got := edidPeakLuminance(edid)
	if got != 800 {
		t.Errorf("expected 800 cd/m2 for raw 0x80, got %d", got)
	}
}

func TestEDIDPeakLuminanceZeroRaw(t *testing.T) {
	hdrBlock := []byte{
		(7 << 5) | 4,
		0x06,
		0x03,
		0x00,
		0x00,
	}
	edid := makeEDIDWithExt(hdrBlock)
	if got := edidPeakLuminance(edid); got != 0 {
		t.Errorf("raw 0 should return 0, got %d", got)
	}
}

func TestEDIDPeakLuminanceNoBlock(t *testing.T) {
	if got := edidPeakLuminance(make([]byte, 128)); got != 0 {
		t.Errorf("EDID with no HDR block should return 0, got %d", got)
	}
}

func TestColorModesForFiltering(t *testing.T) {
	with := &Monitor{SupportsHDR: true}
	without := &Monitor{SupportsHDR: false}
	withModes := colorModesFor(with)
	withoutModes := colorModesFor(without)

	containsHDR := func(modes []string) bool {
		for _, m := range modes {
			if m == "hdr" || m == "hdredid" {
				return true
			}
		}
		return false
	}

	if !containsHDR(withModes) {
		t.Errorf("HDR-capable monitor must expose hdr/hdredid options")
	}
	if containsHDR(withoutModes) {
		t.Errorf("non-HDR monitor must hide hdr/hdredid options")
	}
}
