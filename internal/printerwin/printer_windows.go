//go:build windows

package printerwin

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/alexbrainman/printer"
	"github.com/ashtray01/printerone/internal/printdata"
	"golang.org/x/sys/windows"
)

type LogFunc func(string)

type spoolPrinter interface {
	DriverInfo() (*printer.DriverInfo, error)
	StartDocument(name, datatype string) (uint32, error)
	StartPage() error
	Write([]byte) (int, error)
	EndPage() error
	EndDocument() error
	Abort() error
	Close() error
}

type nativePrinter struct {
	h syscall.Handle
}

var (
	winspool            = windows.NewLazySystemDLL("winspool.drv")
	procStartDocPrinter = winspool.NewProc("StartDocPrinterW")
	procAbortPrinter    = winspool.NewProc("AbortPrinter")
)

func List() ([]string, error) { return printer.ReadNames() }

// PrintRaw submits an already-rendered printer language stream to the selected
// Windows queue. It confirms every Winspool stage and reports only spooler
// acceptance; physical device completion is monitored separately when exposed.
func PrintRaw(printerName string, data []byte, logger ...LogFunc) error {
	var log LogFunc
	if len(logger) > 0 {
		log = logger[0]
	}
	if len(data) == 0 {
		return errors.New("print job is empty")
	}
	format := printdata.DetectFormat(data)
	logf(log, "[FORMAT] Detected data format: %s", format)
	if isUnsupportedVirtualPrinter(printerName) {
		return fmt.Errorf("%s requires a rendered Windows/XPS print pipeline and cannot accept PrinterOne RAW jobs (%s)", printerName, format)
	}

	p, err := openNativePrinter(printerName)
	if err != nil {
		return fmt.Errorf("open printer %q: %w", printerName, err)
	}
	jobID, err := submitRaw(p, printerName, data, log)
	if err != nil {
		return err
	}
	if log != nil {
		go monitorJob(printerName, jobID, log)
	}
	return nil
}

func openNativePrinter(name string) (*nativePrinter, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	var handle syscall.Handle
	if err := printer.OpenPrinter(namePtr, &handle, 0); err != nil {
		return nil, err
	}
	return &nativePrinter{h: handle}, nil
}

func (p *nativePrinter) DriverInfo() (*printer.DriverInfo, error) {
	var needed uint32
	buf := make([]byte, 1)
	for {
		err := printer.GetPrinterDriver(p.h, nil, 8, &buf[0], uint32(len(buf)), &needed)
		if err == nil {
			break
		}
		if err != syscall.ERROR_INSUFFICIENT_BUFFER || needed <= uint32(len(buf)) {
			return nil, err
		}
		buf = make([]byte, needed)
	}
	driver := (*printer.DRIVER_INFO_8)(unsafe.Pointer(&buf[0]))
	return &printer.DriverInfo{
		Name:        windows.UTF16PtrToString(driver.Name),
		Environment: windows.UTF16PtrToString(driver.Environment),
		DriverPath:  windows.UTF16PtrToString(driver.DriverPath),
		Attributes:  driver.PrinterDriverAttributes,
	}, nil
}

func (p *nativePrinter) StartDocument(name, datatype string) (uint32, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	typePtr, err := windows.UTF16PtrFromString(datatype)
	if err != nil {
		return 0, err
	}
	info := printer.DOC_INFO_1{DocName: namePtr, Datatype: typePtr}
	jobID, _, callErr := procStartDocPrinter.Call(uintptr(p.h), 1, uintptr(unsafe.Pointer(&info)))
	if jobID == 0 {
		return 0, win32CallError(callErr)
	}
	return uint32(jobID), nil
}

func (p *nativePrinter) StartPage() error { return printer.StartPagePrinter(p.h) }
func (p *nativePrinter) EndPage() error   { return printer.EndPagePrinter(p.h) }
func (p *nativePrinter) EndDocument() error {
	return printer.EndDocPrinter(p.h)
}
func (p *nativePrinter) Write(data []byte) (int, error) {
	var written uint32
	if err := printer.WritePrinter(p.h, &data[0], uint32(len(data)), &written); err != nil {
		return int(written), err
	}
	return int(written), nil
}
func (p *nativePrinter) Abort() error {
	result, _, callErr := procAbortPrinter.Call(uintptr(p.h))
	if result == 0 {
		return win32CallError(callErr)
	}
	return nil
}
func (p *nativePrinter) Close() error { return printer.ClosePrinter(p.h) }

func submitRaw(p spoolPrinter, printerName string, data []byte, log LogFunc) (jobID uint32, retErr error) {
	jobStarted := false
	defer func() {
		if retErr != nil && jobStarted {
			if err := p.Abort(); err != nil {
				logf(log, "[WARN] Unable to abort failed spool job: %v", err)
			}
		}
		if err := p.Close(); retErr == nil && err != nil {
			retErr = fmt.Errorf("close printer %q: %w", printerName, err)
		}
	}()

	logf(log, "[SPOOL] Opening printer: %s", printerName)
	driver, err := p.DriverInfo()
	if err != nil {
		return 0, fmt.Errorf("read driver information for %q: %w", printerName, err)
	}
	datatype := "RAW"
	usePages := true
	if driver.Attributes&printer.PRINTER_DRIVER_XPS != 0 {
		datatype = "XPS_PASS"
		usePages = false
	}
	logf(log, "[SPOOL] Driver: %s; datatype: %s", driver.Name, datatype)

	jobID, err = p.StartDocument("PrinterOne RAW job", datatype)
	if err != nil {
		return 0, fmt.Errorf("start spool job on %q: %w", printerName, err)
	}
	jobStarted = true
	logf(log, "[SPOOL] Windows job %d created", jobID)

	if usePages {
		if err := p.StartPage(); err != nil {
			return jobID, fmt.Errorf("start page for job %d: %w", jobID, err)
		}
	}
	written := 0
	for written < len(data) {
		n, err := p.Write(data[written:])
		written += n
		if err != nil {
			return jobID, fmt.Errorf("write job %d after %d of %d bytes: %w", jobID, written, len(data), err)
		}
		if n == 0 {
			return jobID, fmt.Errorf("write job %d after %d of %d bytes: no progress", jobID, written, len(data))
		}
	}
	logf(log, "[SPOOL] Job %d: wrote %d of %d bytes", jobID, written, len(data))
	if usePages {
		if err := p.EndPage(); err != nil {
			return jobID, fmt.Errorf("end page for job %d: %w", jobID, err)
		}
	}
	if err := p.EndDocument(); err != nil {
		return jobID, fmt.Errorf("finalize job %d: %w", jobID, err)
	}
	jobStarted = false
	logf(log, "[SPOOL] Job %d accepted by Windows spooler", jobID)
	return jobID, nil
}

func monitorJob(printerName string, jobID uint32, log LogFunc) {
	time.Sleep(250 * time.Millisecond)
	p, err := printer.Open(printerName)
	if err != nil {
		logf(log, "[WARN] Job %d status unavailable: %v", jobID, err)
		return
	}
	defer p.Close()

	lastStatus := ""
	foundOnce := false
	for attempt := 0; attempt < 20; attempt++ {
		jobs, err := p.Jobs()
		if err != nil {
			logf(log, "[WARN] Job %d status query failed: %v", jobID, err)
			return
		}
		found := false
		for _, job := range jobs {
			if job.JobID != jobID {
				continue
			}
			found, foundOnce = true, true
			status := describeJobStatus(job.StatusCode)
			if status != lastStatus {
				logf(log, "[SPOOL] Job %d status: %s", jobID, status)
				lastStatus = status
			}
			if isTerminalFailure(job.StatusCode) {
				logf(log, "[ERROR] Job %d reported a Windows spooler failure: %s", jobID, status)
				return
			}
			if job.StatusCode&(printer.JOB_STATUS_PRINTED|printer.JOB_STATUS_COMPLETE) != 0 {
				logf(log, "[SPOOL] Job %d was delivered by Windows: %s", jobID, status)
				return
			}
		}
		if !found {
			if foundOnce {
				logf(log, "[SPOOL] Job %d left the Windows queue; physical completion is not confirmed", jobID)
			} else {
				logf(log, "[SPOOL] Job %d is no longer exposed by Windows; physical completion is not confirmed", jobID)
			}
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	logf(log, "[WARN] Job %d remained in the Windows queue with status: %s", jobID, lastStatus)
}

func describeJobStatus(code uint32) string {
	if code == 0 {
		return "queued/ready"
	}
	states := []struct {
		flag uint32
		name string
	}{
		{printer.JOB_STATUS_PAUSED, "paused"},
		{printer.JOB_STATUS_ERROR, "error"},
		{printer.JOB_STATUS_DELETING, "deleting"},
		{printer.JOB_STATUS_SPOOLING, "spooling"},
		{printer.JOB_STATUS_PRINTING, "printing"},
		{printer.JOB_STATUS_OFFLINE, "printer offline"},
		{printer.JOB_STATUS_PAPEROUT, "out of paper"},
		{printer.JOB_STATUS_PRINTED, "printed"},
		{printer.JOB_STATUS_DELETED, "deleted"},
		{printer.JOB_STATUS_BLOCKED_DEVQ, "driver/device blocked"},
		{printer.JOB_STATUS_USER_INTERVENTION, "user intervention required"},
		{printer.JOB_STATUS_COMPLETE, "sent to printer"},
		{printer.JOB_STATUS_RETAINED, "retained"},
	}
	var result []string
	for _, state := range states {
		if code&state.flag != 0 {
			result = append(result, state.name)
		}
	}
	if len(result) == 0 {
		return fmt.Sprintf("unknown (0x%08x)", code)
	}
	return strings.Join(result, ", ")
}

func isTerminalFailure(code uint32) bool {
	const failureMask = printer.JOB_STATUS_ERROR | printer.JOB_STATUS_DELETED |
		printer.JOB_STATUS_BLOCKED_DEVQ | printer.JOB_STATUS_OFFLINE |
		printer.JOB_STATUS_PAPEROUT | printer.JOB_STATUS_USER_INTERVENTION
	return code&failureMask != 0
}

func isUnsupportedVirtualPrinter(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "microsoft print to pdf") ||
		strings.Contains(name, "microsoft xps document writer")
}

func logf(log LogFunc, format string, args ...any) {
	if log != nil {
		log(fmt.Sprintf(format, args...))
	}
}

func win32CallError(err error) error {
	if err != nil && !errors.Is(err, windows.ERROR_SUCCESS) {
		return err
	}
	return syscall.EINVAL
}
