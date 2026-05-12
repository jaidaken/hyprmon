package main

import "testing"

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

func TestGenerateMonitorLineDescFormat(t *testing.T) {
	base := Monitor{
		Name:       "DP-9",
		HardwareID: "Dell Inc./DELL U3419W/5HJB6T2",
		EDIDName:   "Dell Inc. DELL U3419W 5HJB6T2",
		PxW:        3440,
		PxH:        1440,
		Hz:         60,
		X:          0,
		Y:          0,
		Scale:      1.0,
		Active:     true,
	}

	t.Run("desc off writes connector name", func(t *testing.T) {
		m := base
		m.UseDescFormat = false
		got := generateMonitorLine(m)
		want := "monitor=DP-9,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on writes description line", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		got := generateMonitorLine(m)
		want := "monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on with disabled monitor", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.Active = false
		got := generateMonitorLine(m)
		want := "monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,disable"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on with mirror keeps source connector", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.IsMirrored = true
		m.MirrorSource = "DP-1"
		got := generateMonitorLine(m)
		want := "monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,3440x1440@60.00,0x0,1.00,mirror,DP-1"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on but ambiguous falls back to connector name", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.HardwareID = "Dell Inc./DELL U3419W/#1"
		got := generateMonitorLine(m)
		want := "monitor=DP-9,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on but empty description falls back", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.EDIDName = ""
		got := generateMonitorLine(m)
		want := "monitor=DP-9,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on but description has comma falls back", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.EDIDName = "Apple Computer Inc., Studio Display"
		got := generateMonitorLine(m)
		want := "monitor=DP-9,3440x1440@60.00,0x0,1.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("desc on with advanced options", func(t *testing.T) {
		m := base
		m.UseDescFormat = true
		m.BitDepth = 10
		m.VRR = 1
		got := generateMonitorLine(m)
		want := "monitor=desc:Dell Inc. DELL U3419W 5HJB6T2,3440x1440@60.00,0x0,1.00,bitdepth,10,vrr,1"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})
}

func TestGenerateMonitorLineLuminance(t *testing.T) {
	base := Monitor{
		Name:      "DP-1",
		PxW:       2560,
		PxH:       1440,
		Hz:        74.93,
		X:         0,
		Y:         0,
		Scale:     1.0,
		Active:    true,
		BitDepth:  10,
		ColorMode: "hdredid",
	}

	t.Run("both luminance values emit", func(t *testing.T) {
		m := base
		m.SDRBrightness = 2.0
		m.SDRSaturation = 1.2
		m.SDRMinLuminance = 0.1
		m.SDRMaxLuminance = 20
		got := generateMonitorLine(m)
		want := "monitor=DP-1,2560x1440@74.93,0x0,1.00,bitdepth,10,cm,hdredid,sdrbrightness,2.00,sdrsaturation,1.20,sdr_min_luminance,0.10,sdr_max_luminance,20.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("zero luminance is omitted", func(t *testing.T) {
		m := base
		got := generateMonitorLine(m)
		want := "monitor=DP-1,2560x1440@74.93,0x0,1.00,bitdepth,10,cm,hdredid"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("luminance ignored outside HDR modes", func(t *testing.T) {
		m := base
		m.ColorMode = "srgb"
		m.SDRMinLuminance = 0.1
		m.SDRMaxLuminance = 20
		got := generateMonitorLine(m)
		want := "monitor=DP-1,2560x1440@74.93,0x0,1.00,bitdepth,10"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})

	t.Run("only max set", func(t *testing.T) {
		m := base
		m.SDRMaxLuminance = 400
		got := generateMonitorLine(m)
		want := "monitor=DP-1,2560x1440@74.93,0x0,1.00,bitdepth,10,cm,hdredid,sdr_max_luminance,400.00"
		if got != want {
			t.Errorf("generateMonitorLine() = %q, want %q", got, want)
		}
	})
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
