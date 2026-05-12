package main

import "github.com/charmbracelet/lipgloss"

// Centralised colour palette. Every dialog and view should pull from here so a
// future theming pass only needs to edit one file. Names are semantic, not
// chromatic.
const (
	colorTitle     = "12"  // Picker dialog titles
	colorPrimary   = "214" // Selected / focused / highlighted accent
	colorAccent    = "42"  // Active state, success, default border
	colorMuted     = "244" // Regular labels
	colorListItem  = "243" // Inert list rows
	colorPreview   = "245" // Italic preview text
	colorDim       = "241" // Help footer / muted controls
	colorBorder    = "240" // Plain inactive border
	colorDisabled  = "238" // Greyed-out disabled rows
	colorError     = "196" // Errors
	colorErrorAlt  = "9"   // Legacy 16-colour red, used in older dialogs
	colorWarning   = "208" // Warnings
	colorRecommend = "33"  // Recommended-config hints
)

// Pre-baked colour values to avoid repeating lipgloss.Color() at call sites.
var (
	fgTitle     = lipgloss.Color(colorTitle)
	fgPrimary   = lipgloss.Color(colorPrimary)
	fgAccent    = lipgloss.Color(colorAccent)
	fgMuted     = lipgloss.Color(colorMuted)
	fgListItem  = lipgloss.Color(colorListItem)
	fgPreview   = lipgloss.Color(colorPreview)
	fgDim       = lipgloss.Color(colorDim)
	fgBorder    = lipgloss.Color(colorBorder)
	fgDisabled  = lipgloss.Color(colorDisabled)
	fgError     = lipgloss.Color(colorError)
	fgErrorAlt  = lipgloss.Color(colorErrorAlt)
	fgWarning   = lipgloss.Color(colorWarning)
	fgRecommend = lipgloss.Color(colorRecommend)
)
