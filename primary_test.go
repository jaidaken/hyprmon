package main

import "testing"

func TestNormalizePositionsAnchorsPrimary(t *testing.T) {
	monitors := []Monitor{
		{Name: "DP-1", X: 1500, Y: 500, Active: true, IsPrimary: true},
		{Name: "HDMI-A-1", X: 0, Y: 200, Active: true},
	}
	normalizePositions(monitors)
	if monitors[0].X != 0 || monitors[0].Y != 0 {
		t.Errorf("primary should be at (0,0), got (%d,%d)", monitors[0].X, monitors[0].Y)
	}
	if monitors[1].X != -1500 || monitors[1].Y != -300 {
		t.Errorf("secondary should shift by (-1500,-300), got (%d,%d)", monitors[1].X, monitors[1].Y)
	}
}

func TestNormalizePositionsPreservesRelativeLayout(t *testing.T) {
	monitors := []Monitor{
		{Name: "DP-1", X: 500, Y: 500, Active: true, IsPrimary: true},
		{Name: "HDMI-A-1", X: -980, Y: 172, Active: true},
		{Name: "DP-2", X: 3060, Y: 500, Active: true},
	}
	dx0 := monitors[1].X - monitors[0].X
	dy0 := monitors[1].Y - monitors[0].Y
	dx2 := monitors[2].X - monitors[0].X
	dy2 := monitors[2].Y - monitors[0].Y

	normalizePositions(monitors)

	if got := monitors[1].X - monitors[0].X; got != dx0 {
		t.Errorf("relative dx HDMI-A-1 changed: was %d, now %d", dx0, got)
	}
	if got := monitors[1].Y - monitors[0].Y; got != dy0 {
		t.Errorf("relative dy HDMI-A-1 changed: was %d, now %d", dy0, got)
	}
	if got := monitors[2].X - monitors[0].X; got != dx2 {
		t.Errorf("relative dx DP-2 changed: was %d, now %d", dx2, got)
	}
	if got := monitors[2].Y - monitors[0].Y; got != dy2 {
		t.Errorf("relative dy DP-2 changed: was %d, now %d", dy2, got)
	}
}

func TestNormalizePositionsNoOpWhenNoPrimary(t *testing.T) {
	monitors := []Monitor{
		{Name: "DP-1", X: 500, Y: 500, Active: true},
		{Name: "HDMI-A-1", X: -980, Y: 172, Active: true},
	}
	normalizePositions(monitors)
	if monitors[0].X != 500 || monitors[0].Y != 500 {
		t.Errorf("DP-1 should be unchanged, got (%d,%d)", monitors[0].X, monitors[0].Y)
	}
	if monitors[1].X != -980 || monitors[1].Y != 172 {
		t.Errorf("HDMI-A-1 should be unchanged, got (%d,%d)", monitors[1].X, monitors[1].Y)
	}
}

func TestNormalizePositionsSkipsInactivePrimary(t *testing.T) {
	monitors := []Monitor{
		{Name: "DP-1", X: 500, Y: 500, Active: false, IsPrimary: true},
		{Name: "HDMI-A-1", X: 0, Y: 0, Active: true},
	}
	normalizePositions(monitors)
	if monitors[0].X != 500 || monitors[1].X != 0 {
		t.Errorf("inactive primary must not trigger shift")
	}
}

func TestNormalizePositionsAlreadyAnchored(t *testing.T) {
	monitors := []Monitor{
		{Name: "DP-1", X: 0, Y: 0, Active: true, IsPrimary: true},
		{Name: "HDMI-A-1", X: -1440, Y: -328, Active: true},
	}
	normalizePositions(monitors)
	if monitors[0].X != 0 || monitors[0].Y != 0 {
		t.Errorf("primary already at origin must stay there")
	}
	if monitors[1].X != -1440 || monitors[1].Y != -328 {
		t.Errorf("secondary must not shift when primary already anchored")
	}
}

func TestNormalizePositionsRealisticUserLayout(t *testing.T) {
	// Mirrors a horizontal-primary + portrait-left-of-it setup.
	monitors := []Monitor{
		{Name: "DP-1", X: 1536, Y: 352, Active: true, IsPrimary: true},
		{Name: "HDMI-A-1", X: 56, Y: 24, Active: true},
	}
	normalizePositions(monitors)
	if monitors[0].X != 0 || monitors[0].Y != 0 {
		t.Errorf("DP-1 not anchored: (%d,%d)", monitors[0].X, monitors[0].Y)
	}
	if monitors[1].X != -1480 || monitors[1].Y != -328 {
		t.Errorf("HDMI-A-1 expected (-1480,-328), got (%d,%d)", monitors[1].X, monitors[1].Y)
	}
}

func TestSetPrimaryPrefMutualExclusion(t *testing.T) {
	s := &Settings{MonitorPrefs: map[string]MonitorPref{
		"A": {Primary: true},
		"B": {Primary: false, UseDescFormat: true},
		"C": {Primary: true},
	}}
	setPrimaryPref(s, "B")
	if s.MonitorPrefs["A"].Primary {
		t.Errorf("A should be cleared")
	}
	if !s.MonitorPrefs["B"].Primary {
		t.Errorf("B should be primary")
	}
	if !s.MonitorPrefs["B"].UseDescFormat {
		t.Errorf("B's other fields must be preserved")
	}
	if s.MonitorPrefs["C"].Primary {
		t.Errorf("C should be cleared")
	}
}

func TestSetPrimaryPrefCreatesEntry(t *testing.T) {
	s := &Settings{}
	setPrimaryPref(s, "new-hwid")
	if !s.MonitorPrefs["new-hwid"].Primary {
		t.Errorf("new entry should be created with Primary=true")
	}
}

func TestClearPrimaryPref(t *testing.T) {
	s := &Settings{MonitorPrefs: map[string]MonitorPref{
		"A": {Primary: true, UseDescFormat: true},
	}}
	clearPrimaryPref(s, "A")
	if s.MonitorPrefs["A"].Primary {
		t.Errorf("Primary should be false")
	}
	if !s.MonitorPrefs["A"].UseDescFormat {
		t.Errorf("UseDescFormat must remain")
	}
}

func TestUpdateWorldHandlesNegativeCoords(t *testing.T) {
	m := model{Monitors: []Monitor{
		{Name: "DP-1", X: 0, Y: 0, PxW: 2560, PxH: 1440, Scale: 1, Active: true, IsPrimary: true},
		{Name: "HDMI-A-1", X: -1480, Y: -328, PxW: 1440, PxH: 2560, Scale: 1, Active: true},
	}}
	m.updateWorld()

	if m.World.OffsetX != -1480-worldPaddingPx {
		t.Errorf("OffsetX = %d, want %d", m.World.OffsetX, -1480-worldPaddingPx)
	}
	if m.World.OffsetY != -328-worldPaddingPx {
		t.Errorf("OffsetY = %d, want %d", m.World.OffsetY, -328-worldPaddingPx)
	}
	if m.World.Width != 2560-(-1480)+2*worldPaddingPx {
		t.Errorf("Width = %d, want %d", m.World.Width, 2560-(-1480)+2*worldPaddingPx)
	}
	if m.World.Height != 2232-(-328)+2*worldPaddingPx {
		t.Errorf("Height = %d, want %d", m.World.Height, 2232-(-328)+2*worldPaddingPx)
	}
}

func TestUpdateWorldPositiveOnlyUnchanged(t *testing.T) {
	m := model{Monitors: []Monitor{
		{Name: "DP-1", X: 0, Y: 0, PxW: 2560, PxH: 1440, Scale: 1, Active: true},
		{Name: "HDMI-A-1", X: 2560, Y: 0, PxW: 1920, PxH: 1080, Scale: 1, Active: true},
	}}
	m.updateWorld()

	if m.World.OffsetX != 0-worldPaddingPx {
		t.Errorf("OffsetX = %d, want %d", m.World.OffsetX, -worldPaddingPx)
	}
	if m.World.Width != 4480+2*worldPaddingPx {
		t.Errorf("Width = %d, want %d", m.World.Width, 4480+2*worldPaddingPx)
	}
}

func TestApplyMonitorPrefsLoadsPrimary(t *testing.T) {
	monitors := []Monitor{
		{Name: "DP-1", HardwareID: "make/model/serial-A"},
		{Name: "HDMI-A-1", HardwareID: "make/model/serial-B"},
	}
	s := &Settings{MonitorPrefs: map[string]MonitorPref{
		"make/model/serial-A": {Primary: true, UseDescFormat: true},
	}}
	applyMonitorPrefs(monitors, s)
	if !monitors[0].IsPrimary {
		t.Errorf("DP-1 should be primary from settings")
	}
	if !monitors[0].UseDescFormat {
		t.Errorf("DP-1 UseDescFormat should be loaded")
	}
	if monitors[1].IsPrimary {
		t.Errorf("HDMI-A-1 should not be primary")
	}
}
