//go:build windows
// +build windows

package main

type uiText struct {
	Title, Printer, Refresh, Address, Port                          string
	AutoStart, WindowsStart, Minimize                               string
	Stopped, Running, Save, Start, Started, Stop                    string
	CheckPort, TestPrint, Exit, Log, Ready, ServerStarted, Printers string
}

type languageOption struct{ code, name string }

var languageOptions = []languageOption{
	{"ru", "Русский"},
	{"en", "English"},
	{"de", "Deutsch"},
	{"es", "Español"},
}

var uiTexts = map[string]uiText{
	"ru": {
		Title: "PrinterOne XP — сервер печати", Printer: "Принтер:", Refresh: "Обновить", Address: "Адрес:", Port: "Порт:",
		AutoStart: "Автостарт", WindowsStart: "С Windows", Minimize: "В трей", Stopped: "Сервер остановлен",
		Running: "Работает", Save: "Сохранить", Start: "Запустить", Started: "Запущено", Stop: "Остановить",
		CheckPort: "Порт", TestPrint: "Тест печати", Exit: "Выход", Log: "Журнал:", Ready: "Готов",
		ServerStarted: "Сервер запущен", Printers: "Принтеров: %d",
	},
	"en": {
		Title: "PrinterOne XP — print server", Printer: "Printer:", Refresh: "Refresh", Address: "Address:", Port: "Port:",
		AutoStart: "Auto start", WindowsStart: "With Windows", Minimize: "To tray", Stopped: "Server stopped",
		Running: "Running", Save: "Save", Start: "Start", Started: "Started", Stop: "Stop",
		CheckPort: "Port", TestPrint: "Test print", Exit: "Exit", Log: "Log:", Ready: "Ready",
		ServerStarted: "Server started", Printers: "Printers: %d",
	},
	"de": {
		Title: "PrinterOne XP — Druckserver", Printer: "Drucker:", Refresh: "Aktualisieren", Address: "Adresse:", Port: "Port:",
		AutoStart: "Autostart", WindowsStart: "Mit Windows", Minimize: "In Tray", Stopped: "Server gestoppt",
		Running: "Aktiv", Save: "Speichern", Start: "Starten", Started: "Gestartet", Stop: "Stoppen",
		CheckPort: "Port", TestPrint: "Testdruck", Exit: "Beenden", Log: "Protokoll:", Ready: "Bereit",
		ServerStarted: "Server gestartet", Printers: "Drucker: %d",
	},
	"es": {
		Title: "PrinterOne XP — servidor", Printer: "Impresora:", Refresh: "Actualizar", Address: "Dirección:", Port: "Puerto:",
		AutoStart: "Auto inicio", WindowsStart: "Con Windows", Minimize: "A bandeja", Stopped: "Servidor detenido",
		Running: "Activo", Save: "Guardar", Start: "Iniciar", Started: "Iniciado", Stop: "Detener",
		CheckPort: "Puerto", TestPrint: "Impresión test", Exit: "Salir", Log: "Registro:", Ready: "Listo",
		ServerStarted: "Servidor iniciado", Printers: "Impresoras: %d",
	},
}

func normalizeLanguage(code string) string {
	if _, ok := uiTexts[code]; ok {
		return code
	}
	return "ru"
}

func (a *application) text() uiText { return uiTexts[a.language] }
