//go:build windows
// +build windows

package spooler

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/ashtray01/printerone/legacy/xp/printdata"
)

type LogFunc func(string)

const (
	printerEnumLocal        = 0x00000002
	printerEnumConnections  = 0x00000004
	errorInsufficientBuffer = syscall.Errno(122)
	errorInvalidParameter   = syscall.Errno(87)
	jobControlDelete        = 5
	printerAccessAdminister = 0x00000004
	printerOneDocumentName  = "PrinterOne RAW job"
	allQueuedJobs           = 0xffffffff

	jobStatusPaused           = 0x00000001
	jobStatusError            = 0x00000002
	jobStatusDeleting         = 0x00000004
	jobStatusSpooling         = 0x00000008
	jobStatusPrinting         = 0x00000010
	jobStatusOffline          = 0x00000020
	jobStatusPaperOut         = 0x00000040
	jobStatusPrinted          = 0x00000080
	jobStatusDeleted          = 0x00000100
	jobStatusBlockedDevQ      = 0x00000200
	jobStatusUserIntervention = 0x00000400
)

var (
	winspool             = syscall.NewLazyDLL("winspool.drv")
	procEnumPrinters     = winspool.NewProc("EnumPrintersW")
	procOpenPrinter      = winspool.NewProc("OpenPrinterW")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
	procStartDocPrinter  = winspool.NewProc("StartDocPrinterW")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procAbortPrinter     = winspool.NewProc("AbortPrinter")
	procGetJob           = winspool.NewProc("GetJobW")
	procEnumJobs         = winspool.NewProc("EnumJobsW")
	procSetJob           = winspool.NewProc("SetJobW")
)

type printerInfo4 struct {
	PrinterName *uint16
	ServerName  *uint16
	Attributes  uint32
}

type docInfo1 struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

type printerDefaults struct {
	Datatype      *uint16
	DevMode       uintptr
	DesiredAccess uint32
}

type systemTime struct {
	Year, Month, DayOfWeek, Day, Hour, Minute, Second, Milliseconds uint16
}

type jobInfo1 struct {
	JobID        uint32
	PrinterName  *uint16
	MachineName  *uint16
	UserName     *uint16
	Document     *uint16
	Datatype     *uint16
	StatusText   *uint16
	Status       uint32
	Priority     uint32
	Position     uint32
	TotalPages   uint32
	PagesPrinted uint32
	Submitted    systemTime
}

func List() ([]string, error) {
	var needed, returned uint32
	_, _, firstErr := procEnumPrinters.Call(
		uintptr(printerEnumLocal|printerEnumConnections), 0, 4, 0, 0,
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)),
	)
	if needed == 0 {
		if errno(firstErr) != 0 && errno(firstErr) != errorInsufficientBuffer {
			return nil, firstErr
		}
		return []string{}, nil
	}
	buf := make([]byte, needed)
	ok, _, callErr := procEnumPrinters.Call(
		uintptr(printerEnumLocal|printerEnumConnections), 0, 4,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)),
	)
	if ok == 0 {
		return nil, winError(callErr)
	}
	result := make([]string, 0, int(returned))
	itemSize := unsafe.Sizeof(printerInfo4{})
	for i := uint32(0); i < returned; i++ {
		item := (*printerInfo4)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(i)*itemSize))
		name := utf16String(item.PrinterName)
		if name != "" {
			result = append(result, name)
		}
	}
	return result, nil
}

func PrintRaw(printerName string, data []byte, logger LogFunc) (jobID uint32, retErr error) {
	if len(data) == 0 {
		return 0, errors.New("print job is empty")
	}
	if unsupportedVirtualPrinter(printerName) {
		return 0, fmt.Errorf("%s cannot accept PrinterOne RAW jobs", printerName)
	}
	logf(logger, "[FORMAT] Detected data format: %s", printdata.DetectFormat(data))
	name, err := syscall.UTF16PtrFromString(printerName)
	if err != nil {
		return 0, err
	}
	var handle syscall.Handle
	ok, _, callErr := procOpenPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&handle)), 0)
	if ok == 0 {
		return 0, fmt.Errorf("open printer %q: %v", printerName, winError(callErr))
	}
	defer func() {
		ok, _, closeErr := procClosePrinter.Call(uintptr(handle))
		if ok == 0 && retErr == nil {
			retErr = fmt.Errorf("close printer %q: %v", printerName, winError(closeErr))
		}
	}()

	docName, _ := syscall.UTF16PtrFromString(printerOneDocumentName)
	datatype, _ := syscall.UTF16PtrFromString("RAW")
	info := docInfo1{DocName: docName, Datatype: datatype}
	job, _, callErr := procStartDocPrinter.Call(uintptr(handle), 1, uintptr(unsafe.Pointer(&info)))
	if job == 0 {
		return 0, fmt.Errorf("start spool job on %q: %v", printerName, winError(callErr))
	}
	jobID = uint32(job)
	started := true
	defer func() {
		if retErr != nil && started {
			_, _, _ = procAbortPrinter.Call(uintptr(handle))
		}
	}()
	logf(logger, "[SPOOL] Windows job %d created", jobID)
	if ok, _, callErr = procStartPagePrinter.Call(uintptr(handle)); ok == 0 {
		return jobID, fmt.Errorf("start page for job %d: %v", jobID, winError(callErr))
	}
	written := 0
	for written < len(data) {
		var n uint32
		ok, _, callErr = procWritePrinter.Call(uintptr(handle), uintptr(unsafe.Pointer(&data[written])), uintptr(len(data)-written), uintptr(unsafe.Pointer(&n)))
		written += int(n)
		if ok == 0 {
			return jobID, fmt.Errorf("write job %d after %d bytes: %v", jobID, written, winError(callErr))
		}
		if n == 0 {
			return jobID, fmt.Errorf("write job %d: no progress", jobID)
		}
	}
	if ok, _, callErr = procEndPagePrinter.Call(uintptr(handle)); ok == 0 {
		return jobID, fmt.Errorf("end page for job %d: %v", jobID, winError(callErr))
	}
	if ok, _, callErr = procEndDocPrinter.Call(uintptr(handle)); ok == 0 {
		return jobID, fmt.Errorf("finalize job %d: %v", jobID, winError(callErr))
	}
	started = false
	logf(logger, "[SPOOL] Job %d accepted by Windows spooler (%d bytes)", jobID, written)
	go monitorJob(printerName, jobID, logger)
	return jobID, nil
}

// ClearPendingPrinterOneJobs removes only jobs created by this application.
// This prevents an unfinished RAW job from being resumed by the Windows
// spooler after a reboot; jobs created by other applications are untouched.
func ClearPendingPrinterOneJobs(printerName string, logger LogFunc) error {
	if strings.TrimSpace(printerName) == "" {
		return nil
	}
	name, err := syscall.UTF16PtrFromString(printerName)
	if err != nil {
		return err
	}
	var handle syscall.Handle
	defaults := printerDefaults{DesiredAccess: printerAccessAdminister}
	ok, _, callErr := procOpenPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&handle)), uintptr(unsafe.Pointer(&defaults)))
	if ok == 0 {
		// The job owner may still be allowed to remove its own document even
		// when the account cannot open the queue with administrator access.
		ok, _, callErr = procOpenPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&handle)), 0)
	}
	if ok == 0 {
		return fmt.Errorf("open printer %q to clear old jobs: %v", printerName, winError(callErr))
	}
	defer procClosePrinter.Call(uintptr(handle))

	var needed, returned uint32
	_, _, firstErr := procEnumJobs.Call(uintptr(handle), 0, allQueuedJobs, 1, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 {
		if errno(firstErr) != 0 && errno(firstErr) != errorInsufficientBuffer {
			return winError(firstErr)
		}
		return nil
	}
	buf := make([]byte, needed)
	ok, _, callErr = procEnumJobs.Call(uintptr(handle), 0, allQueuedJobs, 1, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if ok == 0 {
		return fmt.Errorf("enumerate jobs on %q: %v", printerName, winError(callErr))
	}
	itemSize := unsafe.Sizeof(jobInfo1{})
	for i := uint32(0); i < returned; i++ {
		item := (*jobInfo1)(unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + uintptr(i)*itemSize))
		if utf16String(item.Document) != printerOneDocumentName {
			continue
		}
		ok, _, callErr = procSetJob.Call(uintptr(handle), uintptr(item.JobID), 0, 0, jobControlDelete)
		if ok == 0 {
			return fmt.Errorf("delete stale PrinterOne job %d: %v", item.JobID, winError(callErr))
		}
		logf(logger, "[SPOOL] Removed stale PrinterOne job %d", item.JobID)
	}
	return nil
}

func monitorJob(printerName string, jobID uint32, logger LogFunc) {
	time.Sleep(250 * time.Millisecond)
	name, err := syscall.UTF16PtrFromString(printerName)
	if err != nil {
		return
	}
	var handle syscall.Handle
	ok, _, _ := procOpenPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&handle)), 0)
	if ok == 0 {
		return
	}
	defer procClosePrinter.Call(uintptr(handle))
	found := false
	last := ""
	for attempt := 0; attempt < 20; attempt++ {
		status, present, err := getJobStatus(handle, jobID)
		if err != nil {
			logf(logger, "[WARN] Job %d status unavailable: %v", jobID, err)
			return
		}
		if !present {
			if found {
				logf(logger, "[SPOOL] Job %d left the Windows queue", jobID)
			}
			return
		}
		found = true
		description := describeStatus(status)
		if description != last {
			logf(logger, "[SPOOL] Job %d status: %s", jobID, description)
			last = description
		}
		if status&jobStatusPrinted != 0 {
			return
		}
		if status&(jobStatusError|jobStatusDeleted|jobStatusBlockedDevQ|jobStatusOffline|jobStatusPaperOut|jobStatusUserIntervention) != 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func getJobStatus(handle syscall.Handle, jobID uint32) (uint32, bool, error) {
	var needed uint32
	_, _, firstErr := procGetJob.Call(uintptr(handle), uintptr(jobID), 1, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		if errno(firstErr) == errorInvalidParameter {
			return 0, false, nil
		}
		return 0, false, winError(firstErr)
	}
	buf := make([]byte, needed)
	ok, _, callErr := procGetJob.Call(uintptr(handle), uintptr(jobID), 1, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 {
		if errno(callErr) == errorInvalidParameter {
			return 0, false, nil
		}
		return 0, false, winError(callErr)
	}
	return (*jobInfo1)(unsafe.Pointer(&buf[0])).Status, true, nil
}

func describeStatus(code uint32) string {
	if code == 0 {
		return "queued/ready"
	}
	values := []struct {
		flag uint32
		name string
	}{
		{jobStatusPaused, "paused"}, {jobStatusError, "error"}, {jobStatusDeleting, "deleting"},
		{jobStatusSpooling, "spooling"}, {jobStatusPrinting, "printing"}, {jobStatusOffline, "printer offline"},
		{jobStatusPaperOut, "out of paper"}, {jobStatusPrinted, "printed"}, {jobStatusDeleted, "deleted"},
		{jobStatusBlockedDevQ, "driver/device blocked"}, {jobStatusUserIntervention, "user intervention required"},
	}
	var result []string
	for _, value := range values {
		if code&value.flag != 0 {
			result = append(result, value.name)
		}
	}
	if len(result) == 0 {
		return fmt.Sprintf("unknown (0x%08x)", code)
	}
	return strings.Join(result, ", ")
}

func unsupportedVirtualPrinter(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "microsoft print to pdf") || strings.Contains(name, "microsoft xps document writer")
}

func logf(log LogFunc, format string, args ...interface{}) {
	if log != nil {
		log(fmt.Sprintf(format, args...))
	}
}

func errno(err error) syscall.Errno {
	if value, ok := err.(syscall.Errno); ok {
		return value
	}
	return 0
}
func winError(err error) error {
	if errno(err) != 0 {
		return err
	}
	return syscall.EINVAL
}

func utf16String(value *uint16) string {
	if value == nil {
		return ""
	}
	data := (*[1 << 20]uint16)(unsafe.Pointer(value))[:]
	return syscall.UTF16ToString(data)
}
