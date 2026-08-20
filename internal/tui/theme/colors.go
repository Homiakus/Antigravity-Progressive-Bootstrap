package theme

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Palette defines the semantic colors for the TUI with rich modern tones.
type Palette struct {
	Background    lipgloss.Color
	Surface       lipgloss.Color
	SurfaceActive lipgloss.Color
	SidebarBg     lipgloss.Color
	ConsoleBg     lipgloss.Color

	Border        lipgloss.Color
	BorderActive  lipgloss.Color
	BorderSidebar lipgloss.Color
	BorderConsole lipgloss.Color

	TextPrimary   lipgloss.Color
	TextSecondary lipgloss.Color
	TextMuted     lipgloss.Color

	Accent        lipgloss.Color
	AccentSoft    lipgloss.Color
	AccentGlow    lipgloss.Color
	Purple        lipgloss.Color
	Cyan          lipgloss.Color

	Success       lipgloss.Color
	Warning       lipgloss.Color
	Error         lipgloss.Color
	Info          lipgloss.Color
}

// DarkPalette represents a vibrant, modern developer dark theme (Raycast / Linear / TokyoNight style).
var DarkPalette = Palette{
	Background:    lipgloss.Color("#0B0E14"),
	Surface:       lipgloss.Color("#151B26"),
	SurfaceActive: lipgloss.Color("#1F293D"),
	SidebarBg:     lipgloss.Color("#11151F"),
	ConsoleBg:     lipgloss.Color("#080B10"),

	Border:        lipgloss.Color("#2D3748"),
	BorderActive:  lipgloss.Color("#38BDF8"), // Bright Sky/Cyan focus glow
	BorderSidebar: lipgloss.Color("#1E293B"),
	BorderConsole: lipgloss.Color("#1A202C"),

	TextPrimary:   lipgloss.Color("#F8FAFC"),
	TextSecondary: lipgloss.Color("#94A3B8"),
	TextMuted:     lipgloss.Color("#64748B"),

	Accent:        lipgloss.Color("#38BDF8"), // Electric Sky Blue
	AccentSoft:    lipgloss.Color("#0284C7"),
	AccentGlow:    lipgloss.Color("#7DD3FC"),
	Purple:        lipgloss.Color("#A855F7"), // Linear Purple
	Cyan:          lipgloss.Color("#06B6D4"),

	Success:       lipgloss.Color("#22C55E"), // Vivid Emerald
	Warning:       lipgloss.Color("#F59E0B"), // Vivid Amber
	Error:         lipgloss.Color("#EF4444"), // Vivid Coral
	Info:          lipgloss.Color("#38BDF8"),
}

// LightPalette represents a high-contrast clean light theme.
var LightPalette = Palette{
	Background:    lipgloss.Color("#F8FAFC"),
	Surface:       lipgloss.Color("#FFFFFF"),
	SurfaceActive: lipgloss.Color("#E2E8F0"),
	SidebarBg:     lipgloss.Color("#F1F5F9"),
	ConsoleBg:     lipgloss.Color("#0F172A"), // Dark console even in light mode for readability

	Border:        lipgloss.Color("#CBD5E1"),
	BorderActive:  lipgloss.Color("#0284C7"),
	BorderSidebar: lipgloss.Color("#E2E8F0"),
	BorderConsole: lipgloss.Color("#334155"),

	TextPrimary:   lipgloss.Color("#0F172A"),
	TextSecondary: lipgloss.Color("#475569"),
	TextMuted:     lipgloss.Color("#94A3B8"),

	Accent:        lipgloss.Color("#0284C7"),
	AccentSoft:    lipgloss.Color("#0369A1"),
	AccentGlow:    lipgloss.Color("#38BDF8"),
	Purple:        lipgloss.Color("#7C3AED"),
	Cyan:          lipgloss.Color("#0891B2"),

	Success:       lipgloss.Color("#16A34A"),
	Warning:       lipgloss.Color("#D97706"),
	Error:         lipgloss.Color("#DC2626"),
	Info:          lipgloss.Color("#0284C7"),
}

// Symbols represents iconography with graceful ASCII fallbacks.
type Symbols struct {
	Bullet      string
	ArrowRight  string
	ArrowUp     string
	ArrowDown   string
	Checkmark   string
	Cross       string
	Warning     string
	Spinner     []string
	ActiveBadge string
	IdleBadge   string
	SidebarIcon string
	ConsoleIcon string
}

var UnicodeSymbols = Symbols{
	Bullet:      "•",
	ArrowRight:  "›",
	ArrowUp:     "↑",
	ArrowDown:   "↓",
	Checkmark:   "✓",
	Cross:       "✕",
	Warning:     "!",
	Spinner:     []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	ActiveBadge: "●",
	IdleBadge:   "○",
	SidebarIcon: "◈",
	ConsoleIcon: "",
}

var ASCIISymbols = Symbols{
	Bullet:      "*",
	ArrowRight:  ">",
	ArrowUp:     "^",
	ArrowDown:   "v",
	Checkmark:   "[OK]",
	Cross:       "[X]",
	Warning:     "[!]",
	Spinner:     []string{"-", "\\", "|", "/"},
	ActiveBadge: "[*]",
	IdleBadge:   "[ ]",
	SidebarIcon: "#",
	ConsoleIcon: ">",
}

// IsNoColor returns true if NO_COLOR environment variable is set.
func IsNoColor() bool {
	return os.Getenv("NO_COLOR") != ""
}
