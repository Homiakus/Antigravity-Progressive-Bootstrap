package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme encapsulates colors, symbols, and pre-computed styles.
type Theme struct {
	Palette Palette
	Symbols Symbols
	NoColor bool

	// Typography & Elements
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	MicroLabel    lipgloss.Style
	Body          lipgloss.Style
	Muted         lipgloss.Style
	Bold          lipgloss.Style
	Code          lipgloss.Style

	// List & Table Items
	ItemNormal    lipgloss.Style
	ItemActive    lipgloss.Style
	ItemMuted     lipgloss.Style
	TableHeader   lipgloss.Style
	TableRow      lipgloss.Style
	TableRowAlt   lipgloss.Style

	// Sidebar Styles
	SidebarBox          lipgloss.Style
	SidebarBoxActive    lipgloss.Style
	SidebarTitle        lipgloss.Style
	SidebarItemNormal   lipgloss.Style
	SidebarItemActive   lipgloss.Style
	SidebarItemNumber   lipgloss.Style

	// Live Console Styles
	ConsoleBox          lipgloss.Style
	ConsoleBoxActive    lipgloss.Style
	ConsoleTitle        lipgloss.Style
	ConsoleTimestamp    lipgloss.Style
	ConsoleInfo         lipgloss.Style
	ConsoleSuccess      lipgloss.Style
	ConsoleWarn         lipgloss.Style
	ConsoleErr          lipgloss.Style

	// Badges
	BadgeSuccess  lipgloss.Style
	BadgeWarning  lipgloss.Style
	BadgeError    lipgloss.Style
	BadgeInfo     lipgloss.Style
	BadgeNeutral  lipgloss.Style
	BadgePurple   lipgloss.Style
	BadgeCyan     lipgloss.Style

	// Containers & Boxes
	Box           lipgloss.Style
	BoxActive     lipgloss.Style
	BoxMuted      lipgloss.Style
	HeaderBox     lipgloss.Style
	FooterBox     lipgloss.Style
	ModalBox      lipgloss.Style
	Card          lipgloss.Style

	// Key Hints
	Key           lipgloss.Style
	KeyDesc       lipgloss.Style
}

// NewTheme creates and initializes a Theme with styles computed for the palette.
func NewTheme(isDark bool) *Theme {
	noColor := IsNoColor()
	var pal Palette
	if isDark {
		pal = DarkPalette
	} else {
		pal = LightPalette
	}

	sym := UnicodeSymbols

	t := &Theme{
		Palette: pal,
		Symbols: sym,
		NoColor: noColor,
	}

	if noColor {
		t.Title = lipgloss.NewStyle().Bold(true)
		t.Subtitle = lipgloss.NewStyle()
		t.MicroLabel = lipgloss.NewStyle()
		t.Body = lipgloss.NewStyle()
		t.Muted = lipgloss.NewStyle()
		t.Bold = lipgloss.NewStyle().Bold(true)
		t.Code = lipgloss.NewStyle()

		t.ItemNormal = lipgloss.NewStyle()
		t.ItemActive = lipgloss.NewStyle().Bold(true)
		t.ItemMuted = lipgloss.NewStyle()
		t.TableHeader = lipgloss.NewStyle().Bold(true)
		t.TableRow = lipgloss.NewStyle()
		t.TableRowAlt = lipgloss.NewStyle()

		t.SidebarBox = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
		t.SidebarBoxActive = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).Bold(true)
		t.SidebarTitle = lipgloss.NewStyle().Bold(true)
		t.SidebarItemNormal = lipgloss.NewStyle()
		t.SidebarItemActive = lipgloss.NewStyle().Bold(true)
		t.SidebarItemNumber = lipgloss.NewStyle()

		t.ConsoleBox = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
		t.ConsoleBoxActive = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).Bold(true)
		t.ConsoleTitle = lipgloss.NewStyle().Bold(true)
		t.ConsoleTimestamp = lipgloss.NewStyle()
		t.ConsoleInfo = lipgloss.NewStyle()
		t.ConsoleSuccess = lipgloss.NewStyle()
		t.ConsoleWarn = lipgloss.NewStyle()
		t.ConsoleErr = lipgloss.NewStyle()

		t.BadgeSuccess = lipgloss.NewStyle().Bold(true)
		t.BadgeWarning = lipgloss.NewStyle().Bold(true)
		t.BadgeError = lipgloss.NewStyle().Bold(true)
		t.BadgeInfo = lipgloss.NewStyle().Bold(true)
		t.BadgeNeutral = lipgloss.NewStyle()
		t.BadgePurple = lipgloss.NewStyle().Bold(true)
		t.BadgeCyan = lipgloss.NewStyle().Bold(true)

		t.Box = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
		t.BoxActive = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).Bold(true)
		t.BoxMuted = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
		t.HeaderBox = lipgloss.NewStyle()
		t.FooterBox = lipgloss.NewStyle()
		t.ModalBox = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
		t.Card = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())

		t.Key = lipgloss.NewStyle().Bold(true)
		t.KeyDesc = lipgloss.NewStyle()
		return t
	}

	// Rich Color Styles
	t.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(pal.TextPrimary)

	t.Subtitle = lipgloss.NewStyle().
		Foreground(pal.TextSecondary)

	t.MicroLabel = lipgloss.NewStyle().
		Foreground(pal.TextMuted).
		Bold(true)

	t.Body = lipgloss.NewStyle().
		Foreground(pal.TextPrimary)

	t.Muted = lipgloss.NewStyle().
		Foreground(pal.TextMuted)

	t.Bold = lipgloss.NewStyle().
		Bold(true).
		Foreground(pal.TextPrimary)

	t.Code = lipgloss.NewStyle().
		Foreground(pal.Cyan).
		Background(pal.SurfaceActive).
		Padding(0, 1)

	// List & Table Items
	t.ItemNormal = lipgloss.NewStyle().
		Foreground(pal.TextPrimary)

	t.ItemActive = lipgloss.NewStyle().
		Foreground(pal.AccentGlow).
		Background(pal.SurfaceActive).
		Bold(true)

	t.ItemMuted = lipgloss.NewStyle().
		Foreground(pal.TextMuted)

	t.TableHeader = lipgloss.NewStyle().
		Foreground(pal.TextSecondary).
		Bold(true).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(pal.Border)

	t.TableRow = lipgloss.NewStyle().
		Foreground(pal.TextPrimary)

	t.TableRowAlt = lipgloss.NewStyle().
		Foreground(pal.TextPrimary).
		Background(pal.Surface)

	// Sidebar Styles
	t.SidebarBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(pal.BorderSidebar).
		Background(pal.SidebarBg).
		Padding(0, 1)

	t.SidebarBoxActive = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(pal.Accent).
		Background(pal.SidebarBg).
		Padding(0, 1)

	t.SidebarTitle = lipgloss.NewStyle().
		Foreground(pal.Purple).
		Bold(true)

	t.SidebarItemNormal = lipgloss.NewStyle().
		Foreground(pal.TextSecondary)

	t.SidebarItemActive = lipgloss.NewStyle().
		Foreground(pal.TextPrimary).
		Background(pal.SurfaceActive).
		Bold(true)

	t.SidebarItemNumber = lipgloss.NewStyle().
		Foreground(pal.Accent).
		Bold(true)

	// Console Styles
	t.ConsoleBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(pal.BorderConsole).
		Background(pal.ConsoleBg).
		Padding(0, 1)

	t.ConsoleBoxActive = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(pal.Accent).
		Background(pal.ConsoleBg).
		Padding(0, 1)

	t.ConsoleTitle = lipgloss.NewStyle().
		Foreground(pal.Cyan).
		Bold(true)

	t.ConsoleTimestamp = lipgloss.NewStyle().
		Foreground(pal.TextMuted)

	t.ConsoleInfo = lipgloss.NewStyle().
		Foreground(pal.Info)

	t.ConsoleSuccess = lipgloss.NewStyle().
		Foreground(pal.Success).
		Bold(true)

	t.ConsoleWarn = lipgloss.NewStyle().
		Foreground(pal.Warning).
		Bold(true)

	t.ConsoleErr = lipgloss.NewStyle().
		Foreground(pal.Error).
		Bold(true)

	// Badges
	t.BadgeSuccess = lipgloss.NewStyle().
		Foreground(pal.Success).
		Bold(true)

	t.BadgeWarning = lipgloss.NewStyle().
		Foreground(pal.Warning).
		Bold(true)

	t.BadgeError = lipgloss.NewStyle().
		Foreground(pal.Error).
		Bold(true)

	t.BadgeInfo = lipgloss.NewStyle().
		Foreground(pal.Info).
		Bold(true)

	t.BadgeNeutral = lipgloss.NewStyle().
		Foreground(pal.TextSecondary)

	t.BadgePurple = lipgloss.NewStyle().
		Foreground(pal.Purple).
		Bold(true)

	t.BadgeCyan = lipgloss.NewStyle().
		Foreground(pal.Cyan).
		Bold(true)

	// Containers
	t.Box = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(pal.Border).
		Background(pal.Surface).
		Padding(0, 1)

	t.BoxActive = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(pal.BorderActive).
		Background(pal.Surface).
		Padding(0, 1)

	t.BoxMuted = lipgloss.NewStyle().
		BorderStyle(lipgloss.HiddenBorder())

	t.HeaderBox = lipgloss.NewStyle().
		Foreground(pal.TextPrimary).
		Background(pal.Surface).
		Padding(0, 1)

	t.FooterBox = lipgloss.NewStyle().
		Foreground(pal.TextSecondary).
		Background(pal.Surface).
		Padding(0, 1)

	t.ModalBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(pal.Accent).
		Background(pal.SurfaceActive).
		Padding(1, 2)

	t.Card = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(pal.Border).
		Background(pal.Surface).
		Padding(0, 1)

	// Key Hints
	t.Key = lipgloss.NewStyle().
		Foreground(pal.AccentGlow).
		Background(pal.SurfaceActive).
		Padding(0, 1).
		Bold(true)

	t.KeyDesc = lipgloss.NewStyle().
		Foreground(pal.TextSecondary)

	return t
}
