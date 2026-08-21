package printdata

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

// DetectFormat returns a conservative description of a printer-ready stream.
// It never claims that the selected printer supports the detected language.
func DetectFormat(data []byte) string {
	if len(data) == 0 {
		return "empty"
	}
	upper := bytes.ToUpper(data)
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return "PDF"
	case bytes.HasPrefix(data, []byte("%!PS")):
		return "PostScript"
	case bytes.Contains(upper, []byte("ENTER LANGUAGE=UFRII")):
		return "Canon UFR II"
	case bytes.Contains(upper, []byte("ENTER LANGUAGE=PCL")) || bytes.Contains(upper, []byte("@PJL")):
		return "PJL/PCL"
	case bytes.HasPrefix(data, []byte("^XA")):
		return "ZPL"
	case bytes.HasPrefix(upper, []byte("SIZE ")) || bytes.Contains(upper, []byte("\r\nPRINT ")):
		return "TSPL"
	case bytes.HasPrefix(data, []byte{0x1b, '@'}) || bytes.HasPrefix(data, []byte{0x1d}):
		return "ESC/POS"
	case bytes.HasPrefix(data, []byte{0x1b}):
		return "ESC/P or another ESC-based language"
	case bytes.HasPrefix(data, []byte("PK\x03\x04")) && bytes.Contains(data, []byte("[Content_Types].xml")):
		return "XPS/OpenXPS package"
	case mostlyText(data):
		return "plain text"
	default:
		return fmt.Sprintf("binary/unknown (%d bytes)", len(data))
	}
}

func mostlyText(data []byte) bool {
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	text := strings.ToValidUTF8(string(sample), "")
	if text == "" {
		return false
	}
	printable := 0
	for _, r := range text {
		if unicode.IsPrint(r) || r == '\r' || r == '\n' || r == '\t' {
			printable++
		}
	}
	return printable*100/len([]rune(text)) >= 90
}
