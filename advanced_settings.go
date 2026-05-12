package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const liveApplyDebounce = 150 * time.Millisecond

type liveApplyTickMsg struct {
	gen uint64
}

type liveApplyResultMsg struct {
	err error
}

// liveApplyFn is the function the dialog calls to push a monitor's state to
// Hyprland. Indirected through a package var so tests can inject a fake.
var liveApplyFn = applyMonitor

type advancedSettingsModel struct {
	monitor      *Monitor
	focusedField int
	width        int
	height       int
	liveApplyGen uint64
}

// scheduleLiveApply bumps the generation counter and returns a debounced tick.
// When the tick fires, Update compares its captured gen against the current
// counter; if a later edit superseded it, the tick is dropped.
func (m *advancedSettingsModel) scheduleLiveApply() tea.Cmd {
	m.liveApplyGen++
	gen := m.liveApplyGen
	return tea.Tick(liveApplyDebounce, func(time.Time) tea.Msg {
		return liveApplyTickMsg{gen: gen}
	})
}

const (
	fieldBitDepth = iota
	fieldColorMode
	fieldSDRBrightness
	fieldSDRSaturation
	fieldSDRMinLuminance
	fieldSDRMaxLuminance
	fieldVRR
	fieldTransform
	fieldUseDescFormat
	fieldCount
)

const (
	sdrMinLuminanceStep = 0.05
	sdrMinLuminanceMin  = 0.0
	sdrMinLuminanceMax  = 5.0
	sdrMaxLuminanceStep = 5.0
	sdrMaxLuminanceMin  = 0.0
	sdrMaxLuminanceMax  = 10000.0
)

func newAdvancedSettingsModel(monitor *Monitor) advancedSettingsModel {
	return advancedSettingsModel{
		monitor:      monitor,
		focusedField: 0,
	}
}

func (m advancedSettingsModel) Init() tea.Cmd {
	return nil
}

func (m advancedSettingsModel) Update(msg tea.Msg) (advancedSettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case liveApplyTickMsg:
		if msg.gen != m.liveApplyGen {
			// superseded by a later edit
			return m, nil
		}
		monCopy := *m.monitor
		return m, func() tea.Msg {
			return liveApplyResultMsg{err: liveApplyFn(monCopy)}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "shift+tab":
			m.navigateUp()

		case "down", "tab":
			m.navigateDown()

		case "left":
			m.adjustValue(-1)
			return m, m.scheduleLiveApply()

		case "right":
			m.adjustValue(1)
			return m, m.scheduleLiveApply()

		case " ", "space":
			if m.shouldLiveApplyToggle() {
				m.toggleValue()
				return m, m.scheduleLiveApply()
			}
			m.toggleValue()
		}
	}

	return m, nil
}

// shouldLiveApplyToggle returns false for fields whose toggle doesn't change
// runtime monitor state (currently just UseDescFormat, which only affects
// how monitor= lines are persisted to hyprland.conf).
func (m advancedSettingsModel) shouldLiveApplyToggle() bool {
	return m.focusedField != fieldUseDescFormat
}

func isHDRField(field int) bool {
	return field == fieldSDRBrightness ||
		field == fieldSDRSaturation ||
		field == fieldSDRMinLuminance ||
		field == fieldSDRMaxLuminance
}

func (m *advancedSettingsModel) navigateDown() {
	isHDR := strings.Contains(m.monitor.ColorMode, "hdr")
	descDisabled := !canUseDescFormat(*m.monitor)

	for i := 0; i < fieldCount; i++ {
		m.focusedField++
		if m.focusedField >= fieldCount {
			m.focusedField = 0
		}
		if !isHDR && isHDRField(m.focusedField) {
			continue
		}
		if descDisabled && m.focusedField == fieldUseDescFormat {
			continue
		}
		return
	}
}

func (m *advancedSettingsModel) navigateUp() {
	isHDR := strings.Contains(m.monitor.ColorMode, "hdr")
	descDisabled := !canUseDescFormat(*m.monitor)

	for i := 0; i < fieldCount; i++ {
		m.focusedField--
		if m.focusedField < 0 {
			m.focusedField = fieldCount - 1
		}
		if !isHDR && isHDRField(m.focusedField) {
			continue
		}
		if descDisabled && m.focusedField == fieldUseDescFormat {
			continue
		}
		return
	}
}

func (m *advancedSettingsModel) adjustValue(delta int) {
	switch m.focusedField {
	case fieldSDRBrightness:
		m.monitor.SDRBrightness += float32(delta) * 0.1
		if m.monitor.SDRBrightness < 0.5 {
			m.monitor.SDRBrightness = 0.5
		}
		if m.monitor.SDRBrightness > 2.0 {
			m.monitor.SDRBrightness = 2.0
		}

	case fieldSDRSaturation:
		m.monitor.SDRSaturation += float32(delta) * 0.1
		if m.monitor.SDRSaturation < 0.5 {
			m.monitor.SDRSaturation = 0.5
		}
		if m.monitor.SDRSaturation > 1.5 {
			m.monitor.SDRSaturation = 1.5
		}

	case fieldSDRMinLuminance:
		m.monitor.SDRMinLuminance += float32(delta) * sdrMinLuminanceStep
		if m.monitor.SDRMinLuminance < sdrMinLuminanceMin {
			m.monitor.SDRMinLuminance = sdrMinLuminanceMin
		}
		if m.monitor.SDRMinLuminance > sdrMinLuminanceMax {
			m.monitor.SDRMinLuminance = sdrMinLuminanceMax
		}

	case fieldSDRMaxLuminance:
		m.monitor.SDRMaxLuminance += float32(delta) * sdrMaxLuminanceStep
		if m.monitor.SDRMaxLuminance < sdrMaxLuminanceMin {
			m.monitor.SDRMaxLuminance = sdrMaxLuminanceMin
		}
		if m.monitor.SDRMaxLuminance > sdrMaxLuminanceMax {
			m.monitor.SDRMaxLuminance = sdrMaxLuminanceMax
		}
	}
}

func (m *advancedSettingsModel) toggleValue() {
	switch m.focusedField {
	case fieldBitDepth:
		if m.monitor.BitDepth == 8 {
			m.monitor.BitDepth = 10
		} else {
			m.monitor.BitDepth = 8
		}

	case fieldColorMode:
		modes := []string{"auto", "srgb", "wide", "edid", "hdr", "hdredid"}
		currentIdx := 0
		for i, mode := range modes {
			if m.monitor.ColorMode == mode {
				currentIdx = i
				break
			}
		}
		currentIdx = (currentIdx + 1) % len(modes)
		m.monitor.ColorMode = modes[currentIdx]

		// If we switched away from HDR and were focused on SDR fields, move focus
		if !strings.Contains(m.monitor.ColorMode, "hdr") && isHDRField(m.focusedField) {
			m.focusedField = fieldColorMode
		}

	case fieldVRR:
		m.monitor.VRR = (m.monitor.VRR + 1) % 3

	case fieldTransform:
		m.monitor.Transform = (m.monitor.Transform + 1) % 8

	case fieldUseDescFormat:
		if canUseDescFormat(*m.monitor) {
			m.monitor.UseDescFormat = !m.monitor.UseDescFormat
			// Persist immediately so the setting survives across sessions
			// even when the user never saves a profile.
			if s, err := loadSettings(); err == nil {
				setMonitorPref(s, m.monitor.HardwareID, MonitorPref{
					UseDescFormat: m.monitor.UseDescFormat,
				})
				_ = saveSettings(s) // best-effort; UI flow continues on error
			}
		}
	}
}

func (m advancedSettingsModel) View() string {
	if m.width == 0 || m.height == 0 {
		m.width = 60
		m.height = 20
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("42")).
		Padding(1, 2).
		Width(56).
		Height(19)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Width(16).
		Foreground(lipgloss.Color("244"))

	focusedLabelStyle := labelStyle.
		Foreground(lipgloss.Color("214")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	focusedValueStyle := valueStyle.
		Foreground(lipgloss.Color("214")).
		Bold(true)

	var content strings.Builder

	title := fmt.Sprintf("Advanced Display Settings [%s]", m.monitor.Name)
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// Bit Depth
	label := "Color Depth:"
	value := m.renderBitDepth()
	if m.focusedField == fieldBitDepth {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")

	// Color Mode
	label = "Color Mode:"
	value = m.renderColorMode()
	if m.focusedField == fieldColorMode {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")

	// SDR Brightness (only show if HDR mode)
	if strings.Contains(m.monitor.ColorMode, "hdr") {
		label = "SDR Brightness:"
		value = m.renderSDRBrightness()
		if m.focusedField == fieldSDRBrightness {
			content.WriteString(focusedLabelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(focusedValueStyle.Render(value))
		} else {
			content.WriteString(labelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(valueStyle.Render(value))
		}
		content.WriteString("\n")

		// SDR Saturation
		label = "SDR Saturation:"
		value = m.renderSDRSaturation()
		if m.focusedField == fieldSDRSaturation {
			content.WriteString(focusedLabelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(focusedValueStyle.Render(value))
		} else {
			content.WriteString(labelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(valueStyle.Render(value))
		}
		content.WriteString("\n")

		// SDR Min Luminance (cd/m2)
		label = "SDR Min Lum:"
		value = m.renderSDRMinLuminance()
		if m.focusedField == fieldSDRMinLuminance {
			content.WriteString(focusedLabelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(focusedValueStyle.Render(value))
		} else {
			content.WriteString(labelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(valueStyle.Render(value))
		}
		content.WriteString("\n")

		// SDR Max Luminance (cd/m2)
		label = "SDR Max Lum:"
		value = m.renderSDRMaxLuminance()
		if m.focusedField == fieldSDRMaxLuminance {
			content.WriteString(focusedLabelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(focusedValueStyle.Render(value))
		} else {
			content.WriteString(labelStyle.Render(label))
			content.WriteString("  ")
			content.WriteString(valueStyle.Render(value))
		}
		content.WriteString("\n")
	}

	// VRR
	label = "VRR Mode:"
	value = m.renderVRR()
	if m.focusedField == fieldVRR {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")

	// Transform
	label = "Transform:"
	value = m.renderTransform()
	if m.focusedField == fieldTransform {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")

	// Write as desc:
	label = "Write as desc:"
	value, disabled := m.renderUseDescFormat()
	if disabled {
		dimLabelStyle := labelStyle.Foreground(lipgloss.Color("238"))
		dimValueStyle := valueStyle.Foreground(lipgloss.Color("238"))
		content.WriteString(dimLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(dimValueStyle.Render(value))
	} else if m.focusedField == fieldUseDescFormat {
		content.WriteString(focusedLabelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(focusedValueStyle.Render(value))
	} else {
		content.WriteString(labelStyle.Render(label))
		content.WriteString("  ")
		content.WriteString(valueStyle.Render(value))
	}
	content.WriteString("\n")

	content.WriteString("\n")

	// Controls
	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	controls := "[Tab/↑↓] Navigate  [Space] Toggle  [←→] Adjust\n[Enter] Apply  [Esc] Cancel"
	content.WriteString(controlsStyle.Render(controls))

	dialog := dialogStyle.Render(content.String())

	// Center the dialog
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog)
}

func (m advancedSettingsModel) renderBitDepth() string {
	if m.monitor.BitDepth == 10 {
		return "○ 8-bit  ● 10-bit"
	}
	return "● 8-bit  ○ 10-bit"
}

func (m advancedSettingsModel) renderColorMode() string {
	modes := map[string]string{
		"auto":    "Auto",
		"srgb":    "sRGB",
		"wide":    "Wide",
		"edid":    "EDID",
		"hdr":     "HDR",
		"hdredid": "HDR-EDID",
	}

	var parts []string
	for _, key := range []string{"auto", "srgb", "wide", "edid", "hdr", "hdredid"} {
		if m.monitor.ColorMode == key {
			parts = append(parts, "● "+modes[key])
		} else {
			parts = append(parts, "○ "+modes[key])
		}
	}
	return strings.Join(parts, "  ")
}

func (m advancedSettingsModel) renderSDRBrightness() string {
	// Create a slider visualization
	width := 20
	value := m.monitor.SDRBrightness
	if value == 0 {
		value = 1.0 // Default
	}
	pos := int((value - 0.5) / 1.5 * float32(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}

	slider := make([]rune, width)
	for i := range slider {
		if i == pos {
			slider[i] = '●'
		} else {
			slider[i] = '─'
		}
	}
	return fmt.Sprintf("[%s] %.1f", string(slider), value)
}

func (m advancedSettingsModel) renderSDRSaturation() string {
	// Create a slider visualization
	width := 20
	value := m.monitor.SDRSaturation
	if value == 0 {
		value = 1.0 // Default
	}
	pos := int((value - 0.5) / 1.0 * float32(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}

	slider := make([]rune, width)
	for i := range slider {
		if i == pos {
			slider[i] = '●'
		} else {
			slider[i] = '─'
		}
	}
	return fmt.Sprintf("[%s] %.1f", string(slider), value)
}

func (m advancedSettingsModel) renderSDRMinLuminance() string {
	width := 20
	value := m.monitor.SDRMinLuminance
	if value <= 0 {
		return fmt.Sprintf("[%s] (unset)", strings.Repeat("─", width))
	}
	pos := int((value - sdrMinLuminanceMin) / (sdrMinLuminanceMax - sdrMinLuminanceMin) * float32(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}
	slider := make([]rune, width)
	for i := range slider {
		if i == pos {
			slider[i] = '●'
		} else {
			slider[i] = '─'
		}
	}
	return fmt.Sprintf("[%s] %.2f cd/m²", string(slider), value)
}

func (m advancedSettingsModel) renderSDRMaxLuminance() string {
	width := 20
	value := m.monitor.SDRMaxLuminance
	if value <= 0 {
		return fmt.Sprintf("[%s] (unset)", strings.Repeat("─", width))
	}
	pos := int((value - sdrMaxLuminanceMin) / (sdrMaxLuminanceMax - sdrMaxLuminanceMin) * float32(width))
	if pos < 0 {
		pos = 0
	}
	if pos >= width {
		pos = width - 1
	}
	slider := make([]rune, width)
	for i := range slider {
		if i == pos {
			slider[i] = '●'
		} else {
			slider[i] = '─'
		}
	}
	return fmt.Sprintf("[%s] %.0f cd/m²", string(slider), value)
}

func (m advancedSettingsModel) renderVRR() string {
	switch m.monitor.VRR {
	case 1:
		return "○ Off  ● On  ○ Fullscreen"
	case 2:
		return "○ Off  ○ On  ● Fullscreen"
	default:
		return "● Off  ○ On  ○ Fullscreen"
	}
}

func (m advancedSettingsModel) renderTransform() string {
	transforms := []string{"Normal", "90°", "180°", "270°", "Flipped", "Flip+90°", "Flip+180°", "Flip+270°"}
	if m.monitor.Transform >= 0 && m.monitor.Transform < len(transforms) {
		// Show current and next option
		current := transforms[m.monitor.Transform]
		if m.monitor.Transform <= 3 {
			// Show rotation options
			var parts []string
			for i := 0; i <= 3; i++ {
				if i == m.monitor.Transform {
					parts = append(parts, "● "+transforms[i])
				} else {
					parts = append(parts, "○ "+transforms[i])
				}
			}
			return strings.Join(parts, "  ")
		} else {
			// Show flipped options
			return "Flipped: " + current
		}
	}
	return "Normal"
}

// renderUseDescFormat returns (value, disabled). When disabled is true the
// toggle cannot be flipped (the row is rendered dimmed by the caller) and
// `value` explains why.
func (m advancedSettingsModel) renderUseDescFormat() (string, bool) {
	if m.monitor.EDIDName == "" {
		return "(unavailable — no EDID description)", true
	}
	if m.monitor.HardwareID == "" {
		return "(unavailable — no EDID description)", true
	}
	if strings.Contains(m.monitor.HardwareID, "/#") {
		return "(unavailable — description not unique)", true
	}
	if sanitizeDesc(m.monitor.EDIDName) == "" {
		return "(unavailable — description contains unsupported characters)", true
	}
	if m.monitor.UseDescFormat {
		return "● On  ○ Off", false
	}
	return "○ On  ● Off", false
}
