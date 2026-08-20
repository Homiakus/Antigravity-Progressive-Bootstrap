package theme

var DefaultTheme = NewTheme(true)

// Current returns the default active theme.
func Current() *Theme {
	return DefaultTheme
}
