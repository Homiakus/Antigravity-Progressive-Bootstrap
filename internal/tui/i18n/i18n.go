package i18n

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Language string

const (
	LangRU Language = "ru"
	LangEN Language = "en"
)

type Settings struct {
	Language Language `json:"language"`
	Theme    string   `json:"theme"`    // "dark", "light"
	Accent   string   `json:"accent"`   // "cyan", "blue", "purple", "green"
	Density  string   `json:"density"`  // "comfortable", "compact"
}

var (
	currentSettings = Settings{
		Language: LangRU, // Default is Russian
		Theme:    "dark",
		Accent:   "cyan",
		Density:  "comfortable",
	}
	mu sync.RWMutex
)

// GetSettings returns a copy of current active settings.
func GetSettings() Settings {
	mu.RLock()
	defer mu.RUnlock()
	return currentSettings
}

// SetLanguage changes language and notifies listeners.
func SetLanguage(lang Language) {
	mu.Lock()
	defer mu.Unlock()
	currentSettings.Language = lang
}

// SetTheme changes active theme name.
func SetTheme(theme string) {
	mu.Lock()
	defer mu.Unlock()
	currentSettings.Theme = theme
}

// LoadSettings loads settings from app storage or applies default (Russian).
func LoadSettings(appDir string) Settings {
	mu.Lock()
	defer mu.Unlock()

	p := filepath.Join(appDir, "tui_settings.json")
	b, err := os.ReadFile(p)
	if err == nil {
		var s Settings
		if json.Unmarshal(b, &s) == nil {
			if s.Language == "" {
				s.Language = LangRU
			}
			currentSettings = s
			return currentSettings
		}
	}
	return currentSettings
}

// SaveSettings persists settings to disk.
func SaveSettings(appDir string, s Settings) error {
	mu.Lock()
	defer mu.Unlock()

	currentSettings = s
	_ = os.MkdirAll(appDir, 0o755)
	p := filepath.Join(appDir, "tui_settings.json")
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

// Translations dictionary
var dict = map[string]map[Language]string{
	// Sections / Navigation
	"nav_title":       {LangRU: "НАВИГАЦИЯ", LangEN: "NAVIGATION"},
	"sec_dashboard":   {LangRU: "Панель управления", LangEN: "Dashboard"},
	"sec_setup":       {LangRU: "Установка и Doctor", LangEN: "Setup & Doctor"},
	"sec_caps":        {LangRU: "Возможности и MCP", LangEN: "Capabilities & MCP"},
	"sec_autonomy":    {LangRU: "Автономия и Loop", LangEN: "Autonomy & Loop"},
	"sec_governance":  {LangRU: "Безопасность и Ops", LangEN: "Governance & Ops"},
	"sec_settings":    {LangRU: "Настройки программы", LangEN: "Settings"},

	// Header / Status
	"status_ready":    {LangRU: "ГОТОВ", LangEN: "READY"},
	"status_healthy":  {LangRU: "ЗДОРОВ", LangEN: "HEALTHY"},
	"status_errors":   {LangRU: "ОШИБКИ", LangEN: "ERRORS"},
	"status_running":  {LangRU: "ВЫПОЛНЕНИЕ", LangEN: "RUNNING"},
	"live_console":    {LangRU: "ЖИВАЯ КОНСОЛЬ И ЛОГИ", LangEN: "LIVE CONSOLE & LOGS"},

	// Key hints
	"hint_move":       {LangRU: "навигация", LangEN: "move"},
	"hint_select":     {LangRU: "выбрать", LangEN: "select"},
	"hint_execute":    {LangRU: "выполнить", LangEN: "execute"},
	"hint_panel":      {LangRU: "окно", LangEN: "panel"},
	"hint_commands":   {LangRU: "команды", LangEN: "commands"},
	"hint_help":       {LangRU: "справка", LangEN: "help"},
	"hint_quit":       {LangRU: "выход", LangEN: "quit"},
	"hint_clear":      {LangRU: "очистить логи", LangEN: "clear logs"},

	// Dashboard
	"dash_metrics":    {LangRU: "СИСТЕМНЫЕ МЕТРИКИ", LangEN: "SYSTEM METRICS"},
	"dash_actions":    {LangRU: "БЫСТРЫЕ ДЕЙСТВИЯ", LangEN: "QUICK ACTIONS"},
	"dash_router":     {LangRU: "Адаптивный роутер:", LangEN: "Adaptive Router:"},
	"dash_loop":       {LangRU: "Автономный цикл:", LangEN: "Autonomous Loop:"},
	"dash_health":     {LangRU: "Состояние окружения:", LangEN: "Environment Health:"},
	"dash_inventory":  {LangRU: "Ресурсы:", LangEN: "Inventory:"},

	// Settings View
	"set_title":       {LangRU: "НАСТРОЙКИ ПРИЛОЖЕНИЯ AGCTL", LangEN: "AGCTL APPLICATION SETTINGS"},
	"set_lang":        {LangRU: "Язык интерфейса (Language)", LangEN: "Interface Language"},
	"set_theme":       {LangRU: "Тема оформления (Theme)", LangEN: "Visual Theme"},
	"set_accent":      {LangRU: "Цветовой акцент (Accent Color)", LangEN: "Accent Color"},
	"set_density":     {LangRU: "Плотность элементов (Density)", LangEN: "Interface Density"},
	"set_saved":       {LangRU: "Настройки сохранены в файл конфигурации", LangEN: "Settings saved to configuration file"},
	"set_hint":        {LangRU: "Пробел / Enter — изменить значение • Tab — переход между панелями", LangEN: "Space / Enter to toggle • Tab to switch panels"},
}

// T returns the translated string for current language.
func T(key string) string {
	mu.RLock()
	lang := currentSettings.Language
	mu.RUnlock()

	if m, ok := dict[key]; ok {
		if val, exists := m[lang]; exists {
			return val
		}
		// Fallback to English
		if val, exists := m[LangEN]; exists {
			return val
		}
	}
	return key
}

// CurrentLanguage returns the active language.
func CurrentLanguage() Language {
	mu.RLock()
	defer mu.RUnlock()
	return currentSettings.Language
}
