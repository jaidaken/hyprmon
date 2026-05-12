package main

import (
	"sync"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// stubApply replaces the package-level liveApplyFn for the duration of the
// test. It records every Monitor pushed to it and returns nil.
type stubApply struct {
	mu       sync.Mutex
	captured []Monitor
	count    atomic.Int64
}

func (s *stubApply) fn(m Monitor) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.captured = append(s.captured, m)
	s.count.Add(1)
	return nil
}

func withStubApply(t *testing.T) *stubApply {
	t.Helper()
	stub := &stubApply{}
	orig := liveApplyFn
	liveApplyFn = stub.fn
	t.Cleanup(func() { liveApplyFn = orig })
	return stub
}

func TestScheduleLiveApplyIncrementsGen(t *testing.T) {
	mon := &Monitor{Name: "DP-1", PxW: 2560, PxH: 1440, Hz: 60, Scale: 1, Active: true}
	dlg := newAdvancedSettingsModel(mon)

	if dlg.liveApplyGen != 0 {
		t.Fatalf("expected initial gen 0, got %d", dlg.liveApplyGen)
	}

	_ = dlg.scheduleLiveApply(liveApplyDebounce)
	if dlg.liveApplyGen != 1 {
		t.Errorf("expected gen 1 after first schedule, got %d", dlg.liveApplyGen)
	}

	_ = dlg.scheduleLiveApply(liveApplyDebounce)
	_ = dlg.scheduleLiveApply(liveApplyDebounceToggle)
	if dlg.liveApplyGen != 3 {
		t.Errorf("expected gen 3 after three schedules, got %d", dlg.liveApplyGen)
	}
}

func TestLiveApplyTickStaleIsDropped(t *testing.T) {
	stub := withStubApply(t)
	mon := &Monitor{Name: "DP-1", PxW: 2560, PxH: 1440, Hz: 60, Scale: 1, Active: true}
	dlg := newAdvancedSettingsModel(mon)
	dlg.liveApplyGen = 5

	updated, cmd := dlg.Update(liveApplyTickMsg{gen: 3})
	if cmd != nil {
		t.Errorf("stale tick should not return a cmd, got %T", cmd)
	}
	if stub.count.Load() != 0 {
		t.Errorf("stale tick should not invoke liveApplyFn, got %d calls", stub.count.Load())
	}
	if updated.liveApplyGen != 5 {
		t.Errorf("gen must not change on stale tick, got %d", updated.liveApplyGen)
	}
}

func TestLiveApplyTickCurrentInvokesApply(t *testing.T) {
	stub := withStubApply(t)
	mon := &Monitor{
		Name:            "DP-1",
		PxW:             2560,
		PxH:             1440,
		Hz:              74.93,
		Scale:           1,
		Active:          true,
		BitDepth:        10,
		ColorMode:       "hdredid",
		SDRBrightness:   2.0,
		SDRMinLuminance: 0.1,
		SDRMaxLuminance: 20,
	}
	dlg := newAdvancedSettingsModel(mon)
	dlg.liveApplyGen = 7

	_, cmd := dlg.Update(liveApplyTickMsg{gen: 7})
	if cmd == nil {
		t.Fatal("matching tick should return a cmd")
	}
	msg := cmd()
	res, ok := msg.(liveApplyResultMsg)
	if !ok {
		t.Fatalf("expected liveApplyResultMsg, got %T", msg)
	}
	if res.err != nil {
		t.Errorf("stub apply should not error: %v", res.err)
	}
	if got := stub.count.Load(); got != 1 {
		t.Fatalf("expected exactly one apply, got %d", got)
	}
	captured := stub.captured[0]
	if captured.SDRMaxLuminance != 20 {
		t.Errorf("captured SDRMaxLuminance = %v, want 20", captured.SDRMaxLuminance)
	}
	if captured.ColorMode != "hdredid" {
		t.Errorf("captured ColorMode = %q, want hdredid", captured.ColorMode)
	}
}

func TestAdjustValueSchedulesApply(t *testing.T) {
	withStubApply(t)
	mon := &Monitor{Name: "DP-1", PxW: 2560, PxH: 1440, Hz: 60, Scale: 1, Active: true, ColorMode: "hdr", SDRBrightness: 1.0}
	dlg := newAdvancedSettingsModel(mon)
	dlg.focusedField = fieldSDRBrightness

	updated, cmd := dlg.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd == nil {
		t.Fatal("right arrow on a slider must return a cmd")
	}
	if updated.liveApplyGen != 1 {
		t.Errorf("expected gen 1, got %d", updated.liveApplyGen)
	}
	if mon.SDRBrightness <= 1.0 {
		t.Errorf("right arrow should have increased SDRBrightness from 1.0, got %v", mon.SDRBrightness)
	}
}

func TestUseDescFormatToggleDoesNotApply(t *testing.T) {
	withStubApply(t)
	mon := &Monitor{
		Name:       "DP-1",
		HardwareID: "Dell Inc./DELL U3419W/5HJB6T2",
		EDIDName:   "Dell Inc. DELL U3419W 5HJB6T2",
		PxW:        2560, PxH: 1440, Hz: 60, Scale: 1, Active: true,
	}
	dlg := newAdvancedSettingsModel(mon)
	dlg.focusedField = fieldUseDescFormat

	updated, cmd := dlg.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd != nil {
		t.Errorf("UseDescFormat toggle should NOT schedule live-apply, got cmd %T", cmd)
	}
	if updated.liveApplyGen != 0 {
		t.Errorf("UseDescFormat toggle must not bump gen, got %d", updated.liveApplyGen)
	}
}

func TestColorModeToggleSchedulesApply(t *testing.T) {
	withStubApply(t)
	mon := &Monitor{Name: "DP-1", PxW: 2560, PxH: 1440, Hz: 60, Scale: 1, Active: true, ColorMode: "srgb"}
	dlg := newAdvancedSettingsModel(mon)
	dlg.focusedField = fieldColorMode

	updated, cmd := dlg.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd == nil {
		t.Fatal("color mode toggle must schedule live-apply")
	}
	if updated.liveApplyGen != 1 {
		t.Errorf("expected gen 1 after toggle, got %d", updated.liveApplyGen)
	}
}
