package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// monitorSupportsHDR returns true if the EDID for connector advertises an HDR
// Static Metadata Data Block. False on any read or parse failure (can't
// confirm HDR -> hide HDR options).
func monitorSupportsHDR(connectorName string) bool {
	data := readEDID(connectorName)
	if data == nil {
		return false
	}
	return edidHasHDRStaticMetadata(data)
}

// monitorEDIDPeakLuminance returns the panel's Desired Content Max Luminance
// in cd/m2 from byte 4 of the HDR Static Metadata Data Block, or 0 when
// unavailable.
func monitorEDIDPeakLuminance(connectorName string) int {
	data := readEDID(connectorName)
	if data == nil {
		return 0
	}
	return edidPeakLuminance(data)
}

func readEDID(connectorName string) []byte {
	matches, err := filepath.Glob(fmt.Sprintf("/sys/class/drm/card*-%s/edid", connectorName))
	if err != nil || len(matches) == 0 {
		return nil
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return nil
	}
	return data
}

// edidPeakLuminance decodes byte 4 of the HDR Static Metadata Data Block as
// cd/m2 using CTA-861-G formula: nits = 50 * 2^(raw/32). Returns 0 if no HDR
// block is present, the block is shorter than 4 payload bytes, or raw is 0.
func edidPeakLuminance(edid []byte) int {
	if len(edid) < 128 {
		return 0
	}
	numExt := int(edid[0x7E])
	for ext := 1; ext <= numExt; ext++ {
		offset := ext * 128
		if offset+128 > len(edid) {
			return 0
		}
		block := edid[offset : offset+128]
		if block[0] != 0x02 {
			continue
		}
		dtdOffset := int(block[2])
		if dtdOffset < 4 || dtdOffset > 128 {
			continue
		}
		i := 4
		for i < dtdOffset {
			header := block[i]
			tag := (header >> 5) & 0x07
			length := int(header & 0x1F)
			if i+1+length > dtdOffset {
				break
			}
			if tag == 7 && length >= 4 && block[i+1] == 0x06 {
				raw := block[i+4]
				if raw == 0 {
					return 0
				}
				return int(math.Round(50.0 * math.Pow(2.0, float64(raw)/32.0)))
			}
			i += 1 + length
		}
	}
	return 0
}

// edidHasHDRStaticMetadata scans CTA-861 extension blocks for Extended Tag
// 0x06 (HDR Static Metadata Data Block per CTA-861-G).
func edidHasHDRStaticMetadata(edid []byte) bool {
	if len(edid) < 128 {
		return false
	}
	numExt := int(edid[0x7E])
	for ext := 1; ext <= numExt; ext++ {
		offset := ext * 128
		if offset+128 > len(edid) {
			return false
		}
		block := edid[offset : offset+128]
		if block[0] != 0x02 {
			continue
		}
		dtdOffset := int(block[2])
		if dtdOffset < 4 || dtdOffset > 128 {
			continue
		}
		i := 4
		for i < dtdOffset {
			header := block[i]
			tag := (header >> 5) & 0x07
			length := int(header & 0x1F)
			if i+1+length > dtdOffset {
				break
			}
			if tag == 7 && length >= 2 {
				extTag := block[i+1]
				if extTag == 0x06 {
					return true
				}
			}
			i += 1 + length
		}
	}
	return false
}
