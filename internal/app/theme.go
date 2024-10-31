package app

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type guiTheme struct {
	variant fyne.ThemeVariant
}

func (t *guiTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x00, G: 0x9D, B: 0xC4, A: 0xFF} // Professional blue

	case theme.ColorNameBackground:
		if t.variant == theme.VariantDark {
			return color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF} // Darker background
		}
		return color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF} // Lighter background

	case theme.ColorNameButton:
		if t.variant == theme.VariantDark {
			return color.NRGBA{R: 0x2D, G: 0x2D, B: 0x2D, A: 0xFF}
		}
		return color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF}

	case theme.ColorNameDisabled:
		if t.variant == theme.VariantDark {
			return color.NRGBA{R: 0x35, G: 0x35, B: 0x35, A: 0xFF}
		}
		return color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}

	case theme.ColorNameInputBackground:
		if t.variant == theme.VariantDark {
			return color.NRGBA{R: 0x25, G: 0x25, B: 0x25, A: 0xFF}
		}
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	case theme.ColorNamePlaceHolder:
		if t.variant == theme.VariantDark {
			return color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF}
		}
		return color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF}

	case theme.ColorNameForeground:
		if t.variant == theme.VariantDark {
			return color.NRGBA{R: 0xF0, G: 0xF0, B: 0xF0, A: 0xFF}
		}
		return color.NRGBA{R: 0x21, G: 0x21, B: 0x21, A: 0xFF}
	}
	return theme.DefaultTheme().Color(n, t.variant)
}

func (*guiTheme) Size(s fyne.ThemeSizeName) float32 {
	switch s {
	case theme.SizeNamePadding:
		return 12
	case theme.SizeNameInlineIcon:
		return 24
	case theme.SizeNameText:
		return 16
	case theme.SizeNameHeadingText:
		return 28
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNameSeparatorThickness:
		return 2
	}
	return theme.DefaultTheme().Size(s)
}

func (*guiTheme) Font(s fyne.TextStyle) fyne.Resource {
	if s.Monospace {
		return theme.DefaultTheme().Font(s)
	}

	if s.Bold {
		if s.Italic {
			return theme.DefaultTheme().Font(s)
		}
		return theme.DefaultTheme().Font(s)
	}

	if s.Italic {
		return theme.DefaultTheme().Font(s)
	}
	return theme.DefaultTheme().Font(s)
}

func (*guiTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (t *guiTheme) Type() fyne.ThemeVariant {
	return t.variant
}
