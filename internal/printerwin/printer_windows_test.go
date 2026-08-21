//go:build windows

package printerwin

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexbrainman/printer"
)

type fakeSpoolPrinter struct {
	driver       printer.DriverInfo
	writes       []int
	written      int
	endDocErr    error
	closeErr     error
	aborted      bool
	startedPage  bool
	endedPage    bool
	documentType string
}

func (f *fakeSpoolPrinter) DriverInfo() (*printer.DriverInfo, error) { return &f.driver, nil }
func (f *fakeSpoolPrinter) StartDocument(_ string, datatype string) (uint32, error) {
	f.documentType = datatype
	return 42, nil
}
func (f *fakeSpoolPrinter) StartPage() error { f.startedPage = true; return nil }
func (f *fakeSpoolPrinter) Write(data []byte) (int, error) {
	n := len(data)
	if len(f.writes) > 0 {
		n, f.writes = f.writes[0], f.writes[1:]
		if n > len(data) {
			n = len(data)
		}
	}
	f.written += n
	return n, nil
}
func (f *fakeSpoolPrinter) EndPage() error     { f.endedPage = true; return nil }
func (f *fakeSpoolPrinter) EndDocument() error { return f.endDocErr }
func (f *fakeSpoolPrinter) Abort() error       { f.aborted = true; return nil }
func (f *fakeSpoolPrinter) Close() error       { return f.closeErr }

func TestSubmitRawWritesAllBytesAndFinalizes(t *testing.T) {
	fake := &fakeSpoolPrinter{driver: printer.DriverInfo{Name: "Canon UFR II"}, writes: []int{2, 3}}
	jobID, err := submitRaw(fake, "Canon MF230", []byte("12345"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if jobID != 42 || fake.written != 5 {
		t.Fatalf("jobID=%d written=%d", jobID, fake.written)
	}
	if !fake.startedPage || !fake.endedPage || fake.documentType != "RAW" || fake.aborted {
		t.Fatalf("unexpected state: %#v", fake)
	}
}

func TestSubmitRawReturnsEndDocumentErrorAndAborts(t *testing.T) {
	want := errors.New("end document failed")
	fake := &fakeSpoolPrinter{driver: printer.DriverInfo{Name: "Canon UFR II"}, endDocErr: want}
	_, err := submitRaw(fake, "Canon MF230", []byte("data"), nil)
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if !fake.aborted {
		t.Fatal("failed job was not aborted")
	}
}

func TestSubmitRawReturnsCloseError(t *testing.T) {
	want := errors.New("close failed")
	fake := &fakeSpoolPrinter{driver: printer.DriverInfo{Name: "Canon UFR II"}, closeErr: want}
	_, err := submitRaw(fake, "Canon MF230", []byte("data"), nil)
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}

func TestXPSDriverUsesPassThroughWithoutPageCalls(t *testing.T) {
	fake := &fakeSpoolPrinter{driver: printer.DriverInfo{Name: "XPS driver", Attributes: printer.PRINTER_DRIVER_XPS}}
	_, err := submitRaw(fake, "physical XPSDrv printer", []byte("data"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if fake.documentType != "XPS_PASS" || fake.startedPage || fake.endedPage {
		t.Fatalf("unexpected XPS state: %#v", fake)
	}
}

func TestVirtualDocumentPrintersAreRejected(t *testing.T) {
	for _, name := range []string{"Microsoft Print to PDF", "Microsoft XPS Document Writer"} {
		if !isUnsupportedVirtualPrinter(name) {
			t.Fatalf("%q was not rejected", name)
		}
	}
	if isUnsupportedVirtualPrinter("Canon MF230") {
		t.Fatal("physical printer was rejected")
	}
}

func TestDescribeJobStatus(t *testing.T) {
	got := describeJobStatus(printer.JOB_STATUS_PRINTING | printer.JOB_STATUS_PAPEROUT)
	if !strings.Contains(got, "printing") || !strings.Contains(got, "out of paper") {
		t.Fatalf("status=%q", got)
	}
	if !isTerminalFailure(printer.JOB_STATUS_PAPEROUT) {
		t.Fatal("paper-out status must be a terminal failure")
	}
}
