//go:build windows

package printerwin

import (
	"fmt"

	"github.com/alexbrainman/printer"
)

func List() ([]string, error) { return printer.ReadNames() }

func PrintRaw(printerName string, data []byte) error {
	p, err := printer.Open(printerName)
	if err != nil {
		return fmt.Errorf("open printer: %w", err)
	}
	defer p.Close()
	if err := p.StartRawDocument("PrinterOne RAW job"); err != nil {
		return fmt.Errorf("start job: %w", err)
	}
	defer p.EndDocument()
	if _, err := p.Write(data); err != nil {
		return fmt.Errorf("write job: %w", err)
	}
	return nil
}
