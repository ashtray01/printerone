package printdata

import "bytes"

func DetectFormat(data []byte) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "empty"
	}
	if bytes.HasPrefix(trimmed, []byte("%!PS")) {
		return "PostScript"
	}
	if bytes.HasPrefix(trimmed, []byte("%PDF-")) {
		return "PDF"
	}
	if bytes.HasPrefix(trimmed, []byte("\x1b%-12345X")) || bytes.Contains(trimmed, []byte("@PJL")) {
		return "PJL/PCL"
	}
	if bytes.HasPrefix(trimmed, []byte("\x1bE")) || bytes.Contains(trimmed, []byte("\x1b&")) {
		return "PCL"
	}
	if bytes.HasPrefix(trimmed, []byte("^XA")) {
		return "ZPL"
	}
	if mostlyText(trimmed) {
		return "plain text"
	}
	return "binary/unknown"
}

func mostlyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	printable, limit := 0, len(data)
	if limit > 4096 {
		limit = 4096
	}
	for _, b := range data[:limit] {
		if b == '\r' || b == '\n' || b == '\t' || b == '\f' || (b >= 32 && b < 127) || b >= 0x80 {
			printable++
		}
	}
	return printable*100/limit >= 90
}
