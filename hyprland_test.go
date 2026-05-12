package main

import (
	"strings"
	"testing"
)

func TestSanitizeDesc(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain description", "Dell Inc. DELL U3419W 5HJB6T2", "Dell Inc. DELL U3419W 5HJB6T2"},
		{"trims surrounding whitespace", "  Dell Inc. DELL U3419W  ", "Dell Inc. DELL U3419W"},
		{"rejects embedded comma", "Apple Computer Inc., Apple Studio Display", ""},
		{"rejects embedded double quote", `Dell "pro" U3419W`, ""},
		{"rejects newline", "Dell\nU3419W", ""},
		{"rejects control character", "Dell\x01U3419W", ""},
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeDesc(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeDesc(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCanUseDescFormat(t *testing.T) {
	tests := []struct {
		name    string
		monitor Monitor
		want    bool
	}{
		{
			name: "valid description with serial",
			monitor: Monitor{
				Name:       "DP-3",
				HardwareID: "Dell Inc./DELL U3419W/5HJB6T2",
				EDIDName:   "Dell Inc. DELL U3419W 5HJB6T2",
			},
			want: true,
		},
		{
			name: "empty EDIDName",
			monitor: Monitor{
				Name:       "DP-3",
				HardwareID: "Dell Inc./DELL U3419W/5HJB6T2",
				EDIDName:   "",
			},
			want: false,
		},
		{
			name: "ambiguous: disambiguated HardwareID",
			monitor: Monitor{
				Name:       "DP-9",
				HardwareID: "Dell Inc./DELL U3419W/#1",
				EDIDName:   "Dell Inc. DELL U3419W",
			},
			want: false,
		},
		{
			name: "description contains comma",
			monitor: Monitor{
				Name:       "DP-3",
				HardwareID: "Apple Inc./Studio Display/ABC",
				EDIDName:   "Apple Computer Inc., Apple Studio Display ABC",
			},
			want: false,
		},
		{
			name: "empty HardwareID (no EDID make/model)",
			monitor: Monitor{
				Name:       "DP-3",
				HardwareID: "",
				EDIDName:   "Some Description",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canUseDescFormat(tt.monitor)
			if got != tt.want {
				t.Errorf("canUseDescFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateMonitorV2BlockBasic(t *testing.T) {
	m := Monitor{
		Name:   "DP-9",
		PxW:    3440,
		PxH:    1440,
		Hz:     60,
		X:      0,
		Y:      0,
		Scale:  1.0,
		Active: true,
	}
	got := generateMonitorV2Block(m)
	for _, want := range []string{
		"monitorv2 {",
		"output = DP-9",
		"mode = 3440x1440@60.00",
		"position = 0x0",
		"scale = 1.00",
		"}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("block missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestGenerateMonitorV2BlockDescFormat(t *testing.T) {
	m := Monitor{
		Name:          "DP-9",
		HardwareID:    "Dell Inc./DELL U3419W/5HJB6T2",
		EDIDName:      "Dell Inc. DELL U3419W 5HJB6T2",
		UseDescFormat: true,
		PxW:           3440, PxH: 1440, Hz: 60, Scale: 1.0, Active: true,
	}
	got := generateMonitorV2Block(m)
	if !strings.Contains(got, "output = desc:Dell Inc. DELL U3419W 5HJB6T2") {
		t.Errorf("desc-format identifier missing in block:\n%s", got)
	}
}

func TestGenerateMonitorV2BlockDisabled(t *testing.T) {
	m := Monitor{Name: "DP-9", Active: false}
	got := generateMonitorV2Block(m)
	if !strings.Contains(got, "disabled = 1") {
		t.Errorf("disabled block must contain disabled = 1:\n%s", got)
	}
	if strings.Contains(got, "mode =") {
		t.Errorf("disabled block must NOT contain mode/position/scale:\n%s", got)
	}
}

func TestGenerateMonitorV2BlockMirror(t *testing.T) {
	m := Monitor{
		Name: "DP-9", PxW: 3440, PxH: 1440, Hz: 60, Scale: 1.0, Active: true,
		IsMirrored: true, MirrorSource: "DP-1",
	}
	got := generateMonitorV2Block(m)
	if !strings.Contains(got, "mirror = DP-1") {
		t.Errorf("mirror line missing:\n%s", got)
	}
}

func TestGenerateMonitorV2BlockHDRWithLuminance(t *testing.T) {
	m := Monitor{
		Name:            "DP-1",
		PxW:             2560,
		PxH:             1440,
		Hz:              74.93,
		Scale:           1.0,
		Active:          true,
		BitDepth:        10,
		ColorMode:       "hdredid",
		SDRBrightness:   2.0,
		SDRSaturation:   1.2,
		SDRMinLuminance: 0.1,
		SDRMaxLuminance: 200,
	}
	got := generateMonitorV2Block(m)
	for _, want := range []string{
		"bitdepth = 10",
		"cm = hdredid",
		"sdrbrightness = 2.00",
		"sdrsaturation = 1.20",
		"sdr_min_luminance = 0.10",
		"sdr_max_luminance = 200",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("HDR block missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestGenerateMonitorV2BlockLuminanceOmittedOutsideHDR(t *testing.T) {
	m := Monitor{
		Name: "DP-1", PxW: 2560, PxH: 1440, Hz: 60, Scale: 1.0, Active: true,
		ColorMode: "srgb", SDRMinLuminance: 0.1, SDRMaxLuminance: 200,
	}
	got := generateMonitorV2Block(m)
	if strings.Contains(got, "sdr_min_luminance") || strings.Contains(got, "sdr_max_luminance") {
		t.Errorf("luminance must not appear outside HDR:\n%s", got)
	}
}

func TestGenerateMonitorV2BlockTransformAndVRR(t *testing.T) {
	m := Monitor{
		Name: "HDMI-A-1", PxW: 2560, PxH: 1440, Hz: 60, Scale: 1.0, Active: true,
		Transform: 1, VRR: 2,
	}
	got := generateMonitorV2Block(m)
	if !strings.Contains(got, "transform = 1") {
		t.Errorf("transform line missing:\n%s", got)
	}
	if !strings.Contains(got, "vrr = 2") {
		t.Errorf("vrr line missing:\n%s", got)
	}
}

func TestStripAllMonitorDecls(t *testing.T) {
	input := `# user comment
exec-once = hypridle

monitor = DP-1, 2560x1440@60, 0x0, 1
monitor=HDMI-A-1, preferred, auto, 1

monitorv2 {
  output = DP-1
  mode = 2560x1440@60.00
  position = 0x0
  scale = 1.00
  bitdepth = 10
}

input {
  kb_layout = us
}
`
	got := stripAllMonitorDecls(input)
	for _, banned := range []string{"monitor =", "monitor=", "monitorv2"} {
		if strings.Contains(got, banned) {
			t.Errorf("stripped output still contains %q:\n%s", banned, got)
		}
	}
	for _, kept := range []string{"exec-once = hypridle", "kb_layout = us", "# user comment"} {
		if !strings.Contains(got, kept) {
			t.Errorf("stripped output dropped non-monitor content %q:\n%s", kept, got)
		}
	}
}

func TestRebuildWithoutOutputDropsMatching(t *testing.T) {
	input := `monitorv2 {
  output = DP-1
  mode = 2560x1440@60.00
  position = 0x0
  scale = 1.00
}

monitorv2 {
  output = HDMI-A-1
  mode = 1920x1080@60.00
  position = 2560x0
  scale = 1.00
}
`
	got := rebuildWithoutOutput(input, "DP-1")
	if strings.Contains(got, "output = DP-1") {
		t.Errorf("DP-1 block should be removed:\n%s", got)
	}
	if !strings.Contains(got, "output = HDMI-A-1") {
		t.Errorf("HDMI-A-1 block should be preserved:\n%s", got)
	}
}

func TestRebuildWithoutOutputDropsLegacyV1Line(t *testing.T) {
	input := `monitor=DP-1, 2560x1440@60, 0x0, 1
monitor=HDMI-A-1, preferred, auto, 1
`
	got := rebuildWithoutOutput(input, "DP-1")
	if strings.Contains(got, "monitor=DP-1") {
		t.Errorf("v1 DP-1 line should be removed:\n%s", got)
	}
	if !strings.Contains(got, "monitor=HDMI-A-1") {
		t.Errorf("v1 HDMI-A-1 line should be preserved:\n%s", got)
	}
}

func TestApplyMonitorPrefs(t *testing.T) {
	monitors := []Monitor{
		{Name: "DP-9", HardwareID: "Dell Inc./DELL U3419W/5HJB6T2"},
		{Name: "DP-10", HardwareID: "Dell Inc./DELL U3419W/OTHER"},
		{Name: "eDP-1", HardwareID: ""},
	}

	s := &Settings{MonitorPrefs: map[string]MonitorPref{
		"Dell Inc./DELL U3419W/5HJB6T2": {UseDescFormat: true},
	}}

	applyMonitorPrefs(monitors, s)

	if !monitors[0].UseDescFormat {
		t.Errorf("monitors[0] (matching hwid) should have UseDescFormat=true")
	}
	if monitors[1].UseDescFormat {
		t.Errorf("monitors[1] (non-matching hwid) should be unchanged (false)")
	}
	if monitors[2].UseDescFormat {
		t.Errorf("monitors[2] (empty hwid) should be unchanged (false)")
	}
}
