package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// hyprctlTimeout is the timeout for hyprctl commands
	hyprctlTimeout = 5 * time.Second

	// File permissions
	configFileMode  = 0600 // rw------- (user-only access for config files)
	backupFileMode  = 0600 // rw------- (user-only access for backups)
	profileDirMode  = 0700 // rwx------ (user-only access for profile directory)
	profileFileMode = 0600 // rw------- (user-only access for profile files)

	// Default world dimensions
	defaultWorldWidth  = 3840
	defaultWorldHeight = 2160
	defaultWorldScale  = 1.0

	// World bounds padding
	worldPaddingPx = 500

	// UI layout constants
	desktopBorderMargin = 3  // Border (2) + margin (1)
	desktopFooterHeight = 10 // Height reserved for footer
)

// isValidMonitorName validates that a monitor name is safe to use in commands
func isValidMonitorName(name string) bool {
	if name == "" {
		return false
	}
	// Monitor names should only contain alphanumeric, dash, underscore, and dot
	for _, r := range name {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		isSpecial := r == '-' || r == '_' || r == '.'
		if !isLower && !isUpper && !isDigit && !isSpecial {
			return false
		}
	}
	return true
}

// isValidColorMode validates that a color mode is from the allowed set
func isValidColorMode(mode string) bool {
	validModes := map[string]bool{
		"auto":    true,
		"srgb":    true,
		"wide":    true,
		"edid":    true,
		"hdr":     true,
		"hdredid": true,
		"":        true, // empty is valid (default)
	}
	return validModes[mode]
}

// sanitizeDesc validates and trims an EDID description for use in a
// monitor=desc:... line. Returns "" when the string is unsafe for the
// Hyprland parser or for shell-quoted inclusion elsewhere.
func sanitizeDesc(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, r := range s {
		if r == ',' || r == '"' || r == '\n' || r < 0x20 {
			return ""
		}
	}
	return s
}

// canUseDescFormat reports whether a monitor can safely be written as
// monitor=desc:<description>,... A monitor qualifies when it has a
// non-empty HardwareID that is NOT disambiguated (no /# suffix), and its
// EDIDName survives sanitizeDesc unchanged (non-empty after sanitization).
func canUseDescFormat(m Monitor) bool {
	if m.HardwareID == "" {
		return false
	}
	if strings.Contains(m.HardwareID, "/#") {
		return false
	}
	return sanitizeDesc(m.EDIDName) != ""
}

// applyMonitorPrefs merges per-monitor preferences from the hyprmon
// settings file into the given monitor slice. Monitors without a
// HardwareID are skipped (nothing to key on).
func applyMonitorPrefs(monitors []Monitor, s *Settings) {
	if s == nil {
		return
	}
	for i := range monitors {
		if monitors[i].HardwareID == "" {
			continue
		}
		pref := getMonitorPref(s, monitors[i].HardwareID)
		monitors[i].UseDescFormat = pref.UseDescFormat
		monitors[i].IsPrimary = pref.Primary
	}
}

// normalizePositions shifts every monitor so the primary sits at (0, 0).
// No-op when no monitor is marked primary, or when the primary monitor is
// inactive. Operates in place.
func normalizePositions(monitors []Monitor) {
	var primary *Monitor
	for i := range monitors {
		if monitors[i].IsPrimary && monitors[i].Active {
			primary = &monitors[i]
			break
		}
	}
	if primary == nil {
		return
	}
	dx, dy := primary.X, primary.Y
	if dx == 0 && dy == 0 {
		return
	}
	for i := range monitors {
		monitors[i].X -= dx
		monitors[i].Y -= dy
	}
}

// execHyprctl executes a hyprctl command with the given arguments and returns the output
func execHyprctl(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "hyprctl", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute hyprctl %v: %w", args, err)
	}
	return output, nil
}

// execHyprctlJSON executes a hyprctl command and unmarshals the JSON output into the provided result
func execHyprctlJSON(result interface{}, args ...string) error {
	output, err := execHyprctl(args...)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(output, result); err != nil {
		return fmt.Errorf("failed to parse JSON from hyprctl %v: %w", args, err)
	}
	return nil
}

type hyprMonitor struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Make            string  `json:"make"`
	Model           string  `json:"model"`
	Serial          string  `json:"serial"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	RefreshRate     float64 `json:"refreshRate"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	ActiveWorkspace struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"activeWorkspace"`
	Reserved        []int    `json:"reserved"`
	Scale           float64  `json:"scale"`
	Transform       int      `json:"transform"`
	Focused         bool     `json:"focused"`
	DpmsStatus      bool     `json:"dpmsStatus"`
	VRR             bool     `json:"vrr"`
	ActivelyTearing        bool     `json:"activelyTearing"`
	Disabled               bool     `json:"disabled"`
	CurrentFormat          string   `json:"currentFormat"`
	MirrorOf               string   `json:"mirrorOf"`
	AvailableModes         []string `json:"availableModes"`
	ColorManagementPreset  string   `json:"colorManagementPreset"`
	SDRBrightness          float64  `json:"sdrBrightness"`
	SDRSaturation          float64  `json:"sdrSaturation"`
	SDRMinLuminance        float64  `json:"sdrMinLuminance"`
	SDRMaxLuminance        float64  `json:"sdrMaxLuminance"`
}

type hyprWorkspace struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Monitor    string `json:"monitor"`
	MonitorID  int    `json:"monitorID"`
	Windows    int    `json:"windows"`
	Persistent bool   `json:"ispersistent"`
}

func readMonitors() ([]Monitor, error) {
	var hyprMonitors []hyprMonitor
	if err := execHyprctlJSON(&hyprMonitors, "monitors", "all", "-j"); err != nil {
		return nil, err
	}

	monitors := make([]Monitor, 0, len(hyprMonitors))

	// First pass: create monitors without mirror relationships
	for _, hm := range hyprMonitors {
		modes := make([]Mode, 0, len(hm.AvailableModes))
		for _, modeStr := range hm.AvailableModes {
			if mode := parseMode(modeStr); mode != nil {
				modes = append(modes, *mode)
			}
		}

		monitor := Monitor{
			Name:       hm.Name,
			Make:       hm.Make,
			Model:      hm.Model,
			Serial:     hm.Serial,
			HardwareID: buildHardwareID(hm.Make, hm.Model, hm.Serial),
			PxW:        uint32(hm.Width),
			PxH:        uint32(hm.Height),
			Hz:         float32(hm.RefreshRate),
			Scale:      float32(hm.Scale),
			X:          int32(hm.X),
			Y:          int32(hm.Y),
			Active:     !hm.Disabled,
			EDIDName:   hm.Description,
			Modes:      modes,

			// Advanced display settings
			Transform: int(hm.Transform),
			VRR: func() int {
				if hm.VRR {
					return 1
				}
				return 0
			}(),
			BitDepth: func() uint8 {
				if strings.Contains(hm.CurrentFormat, "2101010") || strings.Contains(hm.CurrentFormat, "16161616") {
					return 10
				}
				return 8
			}(),
			ColorMode:       hm.ColorManagementPreset,
			SDRBrightness:   float32(hm.SDRBrightness),
			SDRSaturation:   float32(hm.SDRSaturation),
			SDRMinLuminance: float32(hm.SDRMinLuminance),
			SDRMaxLuminance: float32(hm.SDRMaxLuminance),
			SupportsHDR:       monitorSupportsHDR(hm.Name),
			EDIDPeakLuminance: monitorEDIDPeakLuminance(hm.Name),

			// Mirror settings
			IsMirrored: hm.MirrorOf != "" && hm.MirrorOf != "none",
			MirrorSource: func() string {
				if hm.MirrorOf != "" && hm.MirrorOf != "none" {
					return hm.MirrorOf
				}
				return ""
			}(),
			MirrorTargets: []string{}, // Will be populated in second pass
		}
		monitors = append(monitors, monitor)
	}

	// Second pass: build mirror targets lists
	for i := range monitors {
		if monitors[i].IsMirrored && monitors[i].MirrorSource != "" {
			// Find the source monitor and add this monitor to its targets
			for j := range monitors {
				if monitors[j].Name == monitors[i].MirrorSource {
					monitors[j].MirrorTargets = append(monitors[j].MirrorTargets, monitors[i].Name)
					break
				}
			}
		}
	}

	// Disambiguate monitors with identical HardwareIDs
	disambiguateHardwareIDs(monitors)

	// Merge per-monitor preferences from hyprmon settings file. Best-effort:
	// on read errors we log and continue with defaults.
	if s, err := loadSettings(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load hyprmon settings: %v\n", err)
	} else {
		applyMonitorPrefs(monitors, s)
	}

	return monitors, nil
}

// getAvailableModes returns the available modes for a specific monitor
func getAvailableModes(monitorName string) ([]string, error) {
	var hyprMonitors []hyprMonitor
	if err := execHyprctlJSON(&hyprMonitors, "monitors", "all", "-j"); err != nil {
		return nil, err
	}

	for _, hm := range hyprMonitors {
		if hm.Name == monitorName {
			return hm.AvailableModes, nil
		}
	}

	return nil, fmt.Errorf("monitor %s not found", monitorName)
}

func parseMode(modeStr string) *Mode {
	parts := strings.Split(modeStr, "@")
	if len(parts) != 2 {
		return nil
	}

	resParts := strings.Split(parts[0], "x")
	if len(resParts) != 2 {
		return nil
	}

	w, err := strconv.ParseUint(resParts[0], 10, 32)
	if err != nil {
		return nil
	}

	h, err := strconv.ParseUint(resParts[1], 10, 32)
	if err != nil {
		return nil
	}

	hzStr := strings.TrimSuffix(parts[1], "Hz")
	hz, err := strconv.ParseFloat(hzStr, 32)
	if err != nil {
		return nil
	}

	return &Mode{
		W:  uint32(w),
		H:  uint32(h),
		Hz: float32(hz),
	}
}

// buildApplyMonitorCmd describes the action applyMonitor performs (write v2
// block + hyprctl reload). Shown in the dialog's diagnostic strip.
func buildApplyMonitorCmd(m Monitor) (string, error) {
	if !isValidMonitorName(m.Name) {
		return "", fmt.Errorf("invalid monitor name: %s", m.Name)
	}
	if !isValidColorMode(m.ColorMode) {
		return "", fmt.Errorf("invalid color mode: %s", m.ColorMode)
	}
	path := getConfigPath()
	return fmt.Sprintf("write monitorv2{output=%s ...} -> %s ; hyprctl reload", m.Name, path), nil
}

// applyMonitor writes/replaces this monitor's v2 block then triggers reload.
// Other monitors' blocks in the file are preserved.
func applyMonitor(m Monitor) error {
	if !isValidMonitorName(m.Name) {
		return fmt.Errorf("invalid monitor name: %s", m.Name)
	}
	if !isValidColorMode(m.ColorMode) {
		return fmt.Errorf("invalid color mode: %s", m.ColorMode)
	}
	if err := writeOrReplaceMonitorV2(m); err != nil {
		return err
	}
	return reloadConfig()
}

func applyMonitors(monitors []Monitor) error {
	normalizePositions(monitors)
	if err := writeConfig(monitors); err != nil {
		return err
	}
	return reloadConfig()
}

func getConfigPath() string {
	if envPath := os.Getenv("HYPRLAND_CONFIG"); envPath != "" {
		// Validate that the path is absolute and clean to prevent path traversal
		cleanPath := filepath.Clean(envPath)
		if !filepath.IsAbs(cleanPath) {
			return ""
		}
		// Ensure the path doesn't contain directory traversal attempts
		if strings.Contains(envPath, "..") {
			return ""
		}
		return cleanPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "hypr", "hyprland.conf")
}

// generateMonitorV2Block returns a Hyprland monitorv2 { ... } block for a
// monitor. v2 is the only Hyprland monitor syntax that supports the full set
// of fields, including sdr_min_luminance and sdr_max_luminance.
func generateMonitorV2Block(m Monitor) string {
	if !isValidMonitorName(m.Name) {
		return fmt.Sprintf("# Invalid monitor name: %s\n", m.Name)
	}

	identifier := m.Name
	if m.UseDescFormat && canUseDescFormat(m) {
		if desc := sanitizeDesc(m.EDIDName); desc != "" {
			identifier = "desc:" + desc
		}
	}

	var sb strings.Builder
	sb.WriteString("monitorv2 {\n")
	sb.WriteString(fmt.Sprintf("  output = %s\n", identifier))

	if !m.Active {
		sb.WriteString("  disabled = 1\n")
		sb.WriteString("}\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("  mode = %dx%d@%.2f\n", m.PxW, m.PxH, m.Hz))
	sb.WriteString(fmt.Sprintf("  position = %dx%d\n", m.X, m.Y))
	sb.WriteString(fmt.Sprintf("  scale = %.2f\n", m.Scale))

	if m.IsMirrored && m.MirrorSource != "" {
		if isValidMonitorName(m.MirrorSource) {
			sb.WriteString(fmt.Sprintf("  mirror = %s\n", m.MirrorSource))
		}
	}

	if m.BitDepth == 10 {
		sb.WriteString("  bitdepth = 10\n")
	}
	if m.ColorMode != "" && m.ColorMode != "srgb" && isValidColorMode(m.ColorMode) {
		sb.WriteString(fmt.Sprintf("  cm = %s\n", m.ColorMode))
	}
	if m.ColorMode == "hdr" || m.ColorMode == "hdredid" {
		if m.SDREOTF != "" && m.SDREOTF != "default" {
			sb.WriteString(fmt.Sprintf("  sdr_eotf = %s\n", m.SDREOTF))
		}
		if m.SDRBrightness != 0 && m.SDRBrightness != 1.0 {
			sb.WriteString(fmt.Sprintf("  sdrbrightness = %.2f\n", m.SDRBrightness))
		}
		if m.SDRSaturation != 0 && m.SDRSaturation != 1.0 {
			sb.WriteString(fmt.Sprintf("  sdrsaturation = %.2f\n", m.SDRSaturation))
		}
		if m.SDRMinLuminance > 0 {
			sb.WriteString(fmt.Sprintf("  sdr_min_luminance = %.2f\n", m.SDRMinLuminance))
		}
		if m.SDRMaxLuminance > 0 {
			sb.WriteString(fmt.Sprintf("  sdr_max_luminance = %d\n", int(m.SDRMaxLuminance)))
		}
	}
	if m.VRR > 0 {
		sb.WriteString(fmt.Sprintf("  vrr = %d\n", m.VRR))
	}
	if m.Transform > 0 {
		sb.WriteString(fmt.Sprintf("  transform = %d\n", m.Transform))
	}

	sb.WriteString("}\n")
	return sb.String()
}

// stripMonitorDecls returns content with any top-level monitorv2 { ... } blocks
// and any v1 monitor= / monitor = lines removed. Preserves all other content
// including comments. Brace-balanced parser handles nested braces if any.
func stripMonitorDecls(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	depth := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if strings.HasPrefix(trimmed, "monitorv2") && strings.Contains(trimmed, "{") {
				inBlock = true
				depth = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
				if depth <= 0 {
					inBlock = false
				}
				continue
			}
			if strings.HasPrefix(trimmed, "monitor=") || strings.HasPrefix(trimmed, "monitor ") {
				continue
			}
			out = append(out, line)
			continue
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth <= 0 {
			inBlock = false
		}
	}
	return strings.Join(out, "\n")
}

// writeOrReplaceMonitorV2 writes/replaces a single monitor's v2 block in the
// config file, preserving other monitors' blocks and non-monitor content.
func writeOrReplaceMonitorV2(m Monitor) error {
	configPath := getConfigPath()
	if configPath == "" {
		return fmt.Errorf("could not determine config path")
	}

	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config: %w", err)
	}

	stripped := rebuildWithoutOutput(string(existing), m.Name)
	newBlock := generateMonitorV2Block(m)
	combined := strings.TrimRight(stripped, "\n") + "\n\n" + newBlock

	return os.WriteFile(configPath, []byte(combined), configFileMode)
}

// rebuildWithoutOutput returns content with the monitorv2 block whose output
// field matches name removed, plus any legacy v1 monitor= line for the same
// connector. Brace-balanced parser.
func rebuildWithoutOutput(content, name string) string {
	lines := strings.Split(content, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "monitorv2") && strings.Contains(trimmed, "{") {
			// Gather block.
			block := []string{line}
			depth := strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			j := i + 1
			for j < len(lines) && depth > 0 {
				block = append(block, lines[j])
				depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
				j++
			}
			matchesName := false
			for _, bl := range block {
				bt := strings.TrimSpace(bl)
				if strings.HasPrefix(bt, "output") && strings.Contains(bt, "=") {
					val := strings.TrimSpace(strings.SplitN(bt, "=", 2)[1])
					if val == name {
						matchesName = true
						break
					}
				}
			}
			if !matchesName {
				out = append(out, block...)
			}
			i = j
			continue
		}

		if strings.HasPrefix(trimmed, "monitor=") || strings.HasPrefix(trimmed, "monitor ") {
			rest := strings.TrimPrefix(trimmed, "monitor=")
			rest = strings.TrimPrefix(rest, "monitor ")
			rest = strings.TrimSpace(rest)
			if strings.HasPrefix(rest, name+",") || strings.HasPrefix(rest, name+" ") || rest == name {
				i++
				continue
			}
		}

		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n")
}

func writeConfig(monitors []Monitor) error {
	normalizePositions(monitors)
	configPath := getConfigPath()
	if configPath == "" {
		return fmt.Errorf("could not determine config path")
	}

	input, _ := os.ReadFile(configPath)
	if len(input) > 0 {
		backupPath := fmt.Sprintf("%s.bak.%d", configPath, time.Now().Unix())
		if err := os.WriteFile(backupPath, input, backupFileMode); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	stripped := stripAllMonitorDecls(string(input))

	var sb strings.Builder
	sb.WriteString(strings.TrimRight(stripped, "\n"))
	if sb.Len() > 0 {
		sb.WriteString("\n\n")
	}
	for _, m := range monitors {
		sb.WriteString(generateMonitorV2Block(m))
		sb.WriteString("\n")
	}

	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, configFileMode)
	if err != nil {
		return fmt.Errorf("failed to open config for writing: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close config: %w", closeErr)
		}
	}()

	if _, err = file.Write([]byte(sb.String())); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err = file.Sync(); err != nil {
		return fmt.Errorf("failed to sync config: %w", err)
	}
	return nil
}

// stripAllMonitorDecls removes every monitorv2 { ... } block and every
// legacy v1 monitor= / monitor = line. Preserves non-monitor content.
func stripAllMonitorDecls(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "monitorv2") && strings.Contains(trimmed, "{") {
			depth := strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
			j := i + 1
			for j < len(lines) && depth > 0 {
				depth += strings.Count(lines[j], "{") - strings.Count(lines[j], "}")
				j++
			}
			i = j
			continue
		}
		if strings.HasPrefix(trimmed, "monitor=") || strings.HasPrefix(trimmed, "monitor ") {
			i++
			continue
		}
		out = append(out, line)
		i++
	}
	return strings.Join(out, "\n")
}

func reloadConfig() error {
	ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
	defer cancel()

	return exec.CommandContext(ctx, "hyprctl", "reload").Run()
}

var previousMonitors []Monitor

func saveRollback(monitors []Monitor) {
	previousMonitors = make([]Monitor, len(monitors))
	copy(previousMonitors, monitors)
}

func rollback() error {
	if previousMonitors == nil {
		return fmt.Errorf("no previous state to rollback to")
	}
	return applyMonitors(previousMonitors)
}

func readWorkspaces() ([]hyprWorkspace, error) {
	var workspaces []hyprWorkspace
	if err := execHyprctlJSON(&workspaces, "workspaces", "-j"); err != nil {
		return nil, err
	}
	return workspaces, nil
}

func getCurrentMonitorNames() ([]string, error) {
	var hyprMonitors []hyprMonitor
	if err := execHyprctlJSON(&hyprMonitors, "monitors", "-j"); err != nil {
		return nil, err
	}

	var names []string
	for _, m := range hyprMonitors {
		if !m.Disabled {
			names = append(names, m.Name)
		}
	}
	return names, nil
}

func migrateOrphanedWorkspaces(previousMonitorNames, currentMonitorNames []string) error {
	// Check if we're switching to a single monitor setup
	if len(currentMonitorNames) == 1 {
		// Move all workspaces to the single active monitor
		workspaces, err := readWorkspaces()
		if err != nil {
			return fmt.Errorf("failed to read workspaces: %w", err)
		}

		targetMonitor := currentMonitorNames[0]
		for _, workspace := range workspaces {
			// Move workspace if it's not already on the target monitor
			if workspace.Monitor != targetMonitor {
				cmd := fmt.Sprintf("hyprctl dispatch moveworkspacetomonitor %d %s", workspace.ID, targetMonitor)
				ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
				defer cancel()
				if err := exec.CommandContext(ctx, "sh", "-c", cmd).Run(); err != nil {
					return fmt.Errorf("failed to migrate workspace %d to monitor %s: %w", workspace.ID, targetMonitor, err)
				}
			}
		}
		return nil
	}

	// Original logic for multiple monitors - only migrate from removed monitors
	removedMonitors := findRemovedMonitors(previousMonitorNames, currentMonitorNames)
	if len(removedMonitors) == 0 {
		return nil
	}

	workspaces, err := readWorkspaces()
	if err != nil {
		return fmt.Errorf("failed to read workspaces: %w", err)
	}

	for _, workspace := range workspaces {
		for _, removedMonitor := range removedMonitors {
			if workspace.Monitor == removedMonitor {
				cmd := fmt.Sprintf("hyprctl dispatch moveworkspacetomonitor %d current", workspace.ID)
				ctx, cancel := context.WithTimeout(context.Background(), hyprctlTimeout)
				defer cancel()
				if err := exec.CommandContext(ctx, "sh", "-c", cmd).Run(); err != nil {
					return fmt.Errorf("failed to migrate workspace %d: %w", workspace.ID, err)
				}
			}
		}
	}

	return nil
}

func findRemovedMonitors(previous, current []string) []string {
	currentSet := make(map[string]bool)
	for _, name := range current {
		currentSet[name] = true
	}

	var removed []string
	for _, name := range previous {
		if !currentSet[name] {
			removed = append(removed, name)
		}
	}
	return removed
}
