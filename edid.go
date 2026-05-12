package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// monitorSupportsHDR returns true if the EDID for connector advertises an HDR
// Static Metadata Data Block. False on any read or parse failure (can't
// confirm HDR -> hide HDR options).
func monitorSupportsHDR(connectorName string) bool {
	matches, err := filepath.Glob(fmt.Sprintf("/sys/class/drm/card*-%s/edid", connectorName))
	if err != nil || len(matches) == 0 {
		return false
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return false
	}
	return edidHasHDRStaticMetadata(data)
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
