package utils

// ANSI color codes for terminal output.
const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
)

func Color(c, s string) string { return c + s + Reset }
func BoldText(s string) string  { return Bold + s + Reset }
func DimText(s string) string   { return Dim + s + Reset }
func RedText(s string) string   { return Red + s + Reset }
func GreenText(s string) string { return Green + s + Reset }
func CyanText(s string) string  { return Cyan + s + Reset }
func BlueText(s string) string  { return Blue + s + Reset }
