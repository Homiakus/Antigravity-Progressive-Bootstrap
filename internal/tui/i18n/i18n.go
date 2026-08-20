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
	Theme    string   `json:"theme"`   // "dark", "light"
	Accent   string   `json:"accent"`  // "cyan", "purple", "green"
	Density  string   `json:"density"` // "comfortable", "compact"
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

func GetSettings() Settings {
	mu.RLock()
	defer mu.RUnlock()
	return currentSettings
}

func SetLanguage(lang Language) {
	mu.Lock()
	defer mu.Unlock()
	currentSettings.Language = lang
}

func SetTheme(theme string) {
	mu.Lock()
	defer mu.Unlock()
	currentSettings.Theme = theme
}

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
	"status_busy":     {LangRU: "ЗАНЯТ", LangEN: "BUSY"},
	"mode_normal":     {LangRU: "СТАНДАРТ", LangEN: "NORMAL"},
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
	"hint_sidebar":    {LangRU: "меню", LangEN: "sidebar"},
	"hint_workspace":  {LangRU: "рабочая зона", LangEN: "workspace"},
	"hint_console":    {LangRU: "консоль", LangEN: "console"},
	"hint_jump":       {LangRU: "1..6 Перейти • Tab Окно", LangEN: "1..6 Jump • Tab Move"},

	// Dashboard View
	"dash_metrics":    {LangRU: "ОБЗОР СИСТЕМЫ И МЕТРИКИ", LangEN: "SYSTEM OVERVIEW & METRICS"},
	"dash_actions":    {LangRU: "БЫСТРЫЕ ДЕЙСТВИЯ", LangEN: "QUICK ACTIONS"},
	"dash_router":     {LangRU: "Адаптивный роутер:", LangEN: "Adaptive Router:"},
	"dash_loop":       {LangRU: "Автономный цикл:", LangEN: "Autonomous Loop:"},
	"dash_health":     {LangRU: "Состояние окружения:", LangEN: "Environment Health:"},
	"dash_inventory":  {LangRU: "Ресурсы:", LangEN: "Inventory:"},
	"dash_act_rec":    {LangRU: "Рекомендованная установка", LangEN: "Recommended Install"},
	"dash_act_rec_d":  {LangRU: "Установка ядра, хуков и базовых meta-skills", LangEN: "Install core binary, hooks and meta-skills"},
	"dash_act_full":   {LangRU: "Полная стабильная установка", LangEN: "Full Stable Setup"},
	"dash_act_full_d": {LangRU: "Установка всех пакетов скиллов и проверка MCP", LangEN: "Install all skill packs and verify MCP"},
	"dash_act_doc":    {LangRU: "Диагностика Doctor", LangEN: "Run Doctor Diagnostics"},
	"dash_act_doc_d":  {LangRU: "Полный аудит окружения и инструментов", LangEN: "Complete health check and audit"},
	"dash_act_probe":  {LangRU: "Live Probe MCP серверов", LangEN: "Live Probe MCP Servers"},
	"dash_act_probe_d":{LangRU: "Измерение реального пинга и доступных инструментов", LangEN: "Measure latency and active tools"},
	"dash_act_sync":   {LangRU: "Синхронизация скиллов", LangEN: "Sync Recommended Skills"},
	"dash_act_sync_d": {LangRU: "Обновление пакетов Superpowers и Gemini", LangEN: "Update Superpowers and Gemini packs"},
	"dash_footer":     {LangRU: "Enter — выполнить • Tab — перейти в другое окно", LangEN: "Enter execute • Tab panel switch"},

	// Setup View
	"setup_title":     {LangRU: "УСТАНОВКА, ЗАВИСИМОСТИ И ДИАГНОСТИКА DOCTOR", LangEN: "SETUP, PREREQUISITES & DOCTOR DIAGNOSTICS"},
	"setup_rec":       {LangRU: "Рекомендованная установка", LangEN: "Recommended Install"},
	"setup_rec_d":     {LangRU: "Установка встроенных скиллов, хуков и бинарника", LangEN: "Install embedded skills, hooks & core binary"},
	"setup_full":      {LangRU: "Полная стабильная установка", LangEN: "Full Stable Setup"},
	"setup_full_d":     {LangRU: "Установка всех пакетов, сайдкаров и опрос MCP", LangEN: "Install all packs, sidecars and probe all MCP"},
	"setup_doc":       {LangRU: "Диагностика Doctor", LangEN: "Doctor Diagnostics"},
	"setup_doc_d":     {LangRU: "Комплексная проверка окружения и настроек", LangEN: "Run comprehensive environment check"},
	"setup_prereq":    {LangRU: "Проверка зависимостей", LangEN: "Check Prerequisites"},
	"setup_prereq_d":  {LangRU: "Проверка наличия Git, Go, Node.js, Ripgrep", LangEN: "Verify Git, Go, Node.js, Ripgrep"},
	"setup_self":      {LangRU: "Обновить бинарник agctl и хуки", LangEN: "Install/Update agctl Binary & Hooks"},
	"setup_self_d":    {LangRU: "Скомпилировать в bin и зарегистрировать хуки", LangEN: "Compile self into bin and register hooks"},
	"setup_footer":    {LangRU: "Enter — запустить • Полный лог выводится в Живую Консоль справа", LangEN: "Press Enter to execute • Results stream to Live Console"},

	// Capabilities View
	"caps_title":      {LangRU: "ВОЗМОЖНОСТИ И РАСШИРЕНИЯ СРЕДЫ", LangEN: "CAPABILITIES & RUNTIME EXTENSIONS"},
	"caps_tab_skills": {LangRU: "1. Навыки (Skills)", LangEN: "1. Skills"},
	"caps_tab_mcp":    {LangRU: "2. MCP Серверы", LangEN: "2. MCP Servers"},
	"caps_tab_packs":  {LangRU: "3. Пакеты скиллов", LangEN: "3. Skill Packs"},
	"caps_details":    {LangRU: "ПОДРОБНОСТИ: ", LangEN: "DETAILS: "},
	"caps_probe_hint": {LangRU: "Нажмите 'r' для замера задержки и доступности инструментов", LangEN: "Press 'r' to probe server latency & tools"},
	"caps_footer":     {LangRU: "← / → — переключить вкладку • ↑ / ↓ — выбор элемента", LangEN: "← / → switch tabs • ↑ / ↓ select"},

	// Autonomy View
	"auto_title":      {LangRU: "ДВИЖОК АВТОНОМИИ И ОРКЕСТРАЦИЯ", LangEN: "AUTONOMY ENGINE & ORCHESTRATION"},
	"auto_router":     {LangRU: "Адаптивный роутер", LangEN: "Adaptive Router"},
	"auto_router_d":   {LangRU: "Перехватывает запросы и выбирает минимальный набор инструментов", LangEN: "Intercepts requests & routes to smallest capability set"},
	"auto_rmode":      {LangRU: "Режим роутера", LangEN: "Router Mode"},
	"auto_rmode_d":    {LangRU: "transparent (прозрачный) / balanced / maximum", LangEN: "transparent vs balanced vs maximum"},
	"auto_loop":       {LangRU: "Автономный цикл (Loop)", LangEN: "Autonomous Loop"},
	"auto_loop_d":     {LangRU: "Исполняет задачи до подтверждения критерия готовности", LangEN: "Evaluates goals until verification passes"},
	"auto_lperm":      {LangRU: "Политика цикла", LangEN: "Loop Permission"},
	"auto_lperm_d":    {LangRU: "guarded (защищенная) / unrestricted (полная)", LangEN: "guarded vs unrestricted"},
	"auto_queue":      {LangRU: "ФОНОВАЯ ОЧЕРЕДЬ ЗАДАЧ: ", LangEN: "HEADLESS TASK QUEUE: "},
	"auto_no_tasks":   {LangRU: "Нет активных задач в очереди.", LangEN: "No queued tasks running."},
	"auto_footer":     {LangRU: "Пробел / Enter — изменить параметр • Tab — другое окно", LangEN: "Space/Enter toggle parameter • Tab switch panel"},

	// Governance View
	"gov_title":       {LangRU: "УПРАВЛЕНИЕ, БЕЗОПАСНОСТЬ И ВОССТАНОВЛЕНИЕ", LangEN: "GOVERNANCE, SECURITY & RECOVERY"},
	"gov_tab_perm":    {LangRU: "1. Разрешения", LangEN: "1. Permissions"},
	"gov_tab_sec":     {LangRU: "2. Безопасность", LangEN: "2. Security"},
	"gov_tab_bkp":     {LangRU: "3. Бэкапы", LangEN: "3. Backups"},
	"gov_policy_title":{LangRU: "Политики выполнения инструментов", LangEN: "Permission Execution Policies"},
	"gov_policy_tool": {LangRU: "Политика инструментов:", LangEN: "Tool Policy:"},
	"gov_policy_rev":  {LangRU: "Политика артефактов:", LangEN: "Review Policy:"},
	"gov_sec_score":   {LangRU: "Рейтинг безопасности Control Plane:", LangEN: "Control Plane Security Score:"},
	"gov_sec_ok":      {LangRU: "● Уязвимостей и утечек разрешений не обнаружено.", LangEN: "● No security vulnerabilities or unsafe permission leaks detected."},
	"gov_bkp_title":   {LangRU: "Снимки конфигурации:", LangEN: "Configuration Snapshots:"},
	"gov_bkp_none":    {LangRU: "Резервные копии еще не созданы.", LangEN: "No backups created yet."},
	"gov_footer":      {LangRU: "← / → — переключить вкладку • Пробел — сменить режим", LangEN: "← / → switch tabs • Space toggle mode"},

	// Settings View
	"set_title":       {LangRU: "НАСТРОЙКИ ПРИЛОЖЕНИЯ AGCTL", LangEN: "AGCTL APPLICATION SETTINGS"},
	"set_lang":        {LangRU: "Язык интерфейса (Language)", LangEN: "Interface Language"},
	"set_theme":       {LangRU: "Тема оформления (Theme)", LangEN: "Visual Theme"},
	"set_accent":      {LangRU: "Цветовой акцент (Accent Color)", LangEN: "Accent Color"},
	"set_density":     {LangRU: "Плотность элементов (Density)", LangEN: "Interface Density"},
	"set_saved":       {LangRU: "Настройки сохранены в файл конфигурации", LangEN: "Settings saved to configuration file"},
	"set_hint":        {LangRU: "Пробел / Enter — изменить значение • Tab — переход между панелями", LangEN: "Space / Enter to toggle • Tab to switch panels"},
}

func T(key string) string {
	mu.RLock()
	lang := currentSettings.Language
	mu.RUnlock()

	if m, ok := dict[key]; ok {
		if val, exists := m[lang]; exists {
			return val
		}
		if val, exists := m[LangEN]; exists {
			return val
		}
	}
	return key
}

func CurrentLanguage() Language {
	mu.RLock()
	defer mu.RUnlock()
	return currentSettings.Language
}
