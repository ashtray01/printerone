package printdata

import "testing"

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"pdf", []byte("%PDF-1.7"), "PDF"},
		{"postscript", []byte("%!PS-Adobe"), "PostScript"},
		{"ufrii", []byte("\x1b%-12345X@PJL ENTER LANGUAGE=UFRII\r\n"), "Canon UFR II"},
		{"pcl", []byte("\x1b%-12345X@PJL ENTER LANGUAGE=PCLXL\r\n"), "PJL/PCL"},
		{"zpl", []byte("^XA^FO20,20^FDTest^FS^XZ"), "ZPL"},
		{"tspl", []byte("SIZE 40 mm,30 mm\r\nCLS\r\nPRINT 1\r\n"), "TSPL"},
		{"escpos", []byte("\x1b@PrinterOne\n\x1dV\x00"), "ESC/POS"},
		{"text", []byte("PrinterOne test\r\n"), "plain text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectFormat(tt.data); got != tt.want {
				t.Fatalf("DetectFormat()=%q want %q", got, tt.want)
			}
		})
	}
}
