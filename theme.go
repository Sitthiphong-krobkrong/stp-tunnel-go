package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// stpTheme is a dark theme tuned to match the app's design: near-black
// surfaces, restrained borders, one blue accent. Fyne's default theme is
// deliberately generic, so everything visual is set here rather than
// scattered through the widget code.
type stpTheme struct{}

var (
	colBackground = color.NRGBA{R: 0x18, G: 0x19, B: 0x1b, A: 0xff}
	colSurface    = color.NRGBA{R: 0x1f, G: 0x20, B: 0x23, A: 0xff}
	colForeground = color.NRGBA{R: 0xe8, G: 0xe9, B: 0xea, A: 0xff}
	colMuted      = color.NRGBA{R: 0x8a, G: 0x8b, B: 0x8f, A: 0xff}
	colBorder     = color.NRGBA{R: 0x33, G: 0x34, B: 0x38, A: 0xff}
	colAccent     = color.NRGBA{R: 0x3d, G: 0x7f, B: 0xd6, A: 0xff}
	colSuccess    = color.NRGBA{R: 0x3a, G: 0xa6, B: 0x4c, A: 0xff}
	colDanger     = color.NRGBA{R: 0xd9, G: 0x56, B: 0x4c, A: 0xff}
	colPill       = color.NRGBA{R: 0x2c, G: 0x2d, B: 0x31, A: 0xff}
)

func (stpTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNameForeground:
		return colForeground
	case theme.ColorNameForegroundOnPrimary:
		return color.White
	case theme.ColorNameDisabled, theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNameInputBackground, theme.ColorNameOverlayBackground, theme.ColorNameMenuBackground:
		return colSurface
	case theme.ColorNameInputBorder, theme.ColorNameSeparator, theme.ColorNameShadow:
		return colBorder
	case theme.ColorNamePrimary, theme.ColorNameHyperlink, theme.ColorNameFocus:
		return colAccent
	case theme.ColorNameSuccess:
		return colSuccess
	case theme.ColorNameError:
		return colDanger
	case theme.ColorNameButton:
		return colSurface
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x2a, G: 0x2b, B: 0x2f, A: 0xff}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x2c, G: 0x51, B: 0x82, A: 0xff}
	}
	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (stpTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (stpTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (stpTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameInnerPadding:
		return 7
	case theme.SizeNameText:
		return 13
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameInputBorder:
		return 1
	}
	return theme.DefaultTheme().Size(n)
}
