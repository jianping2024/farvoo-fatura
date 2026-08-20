//go:build windows

package main

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/alexbrainman/printer"
	"golang.org/x/sys/windows"
)

var (
	modWinspool          = windows.NewLazySystemDLL("winspool.drv")
	procOpenPrinterW     = modWinspool.NewProc("OpenPrinterW")
	procClosePrinter     = modWinspool.NewProc("ClosePrinter")
	procGetJobW          = modWinspool.NewProc("GetJobW")
	procStartDocPrinterW = modWinspool.NewProc("StartDocPrinterW")
	procEndDocPrinter    = modWinspool.NewProc("EndDocPrinter")
	procStartPagePrinter = modWinspool.NewProc("StartPagePrinter")
	procEndPagePrinter   = modWinspool.NewProc("EndPagePrinter")
	procWritePrinter     = modWinspool.NewProc("WritePrinter")
)

// jobInfo1 — 64-bit layout (JOB_INFO_1W).
type jobInfo1 struct {
	JobID        uint32
	_pad         uint32
	pPrinterName uintptr
	pMachineName uintptr
	pUserName    uintptr
	pDocument    uintptr
	pDataType    uintptr
	pStatus      uintptr
	Status       uint32
}

func winspoolOpenPrinter(printerName string) (windows.Handle, error) {
	namePtr, err := windows.UTF16PtrFromString(printerName)
	if err != nil {
		return 0, err
	}
	var h windows.Handle
	r0, _, e1 := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(&h)),
		0,
	)
	if r0 == 0 {
		return 0, fmt.Errorf("open printer %q: %w", printerName, e1)
	}
	return h, nil
}

func winspoolJobStatus(h windows.Handle, jobID uint32) (uint32, error) {
	var needed uint32
	_, _, _ = procGetJobW.Call(
		uintptr(h),
		uintptr(jobID),
		1,
		0,
		0,
		uintptr(unsafe.Pointer(&needed)),
	)
	if needed < uint32(unsafe.Sizeof(jobInfo1{})) {
		needed = uint32(unsafe.Sizeof(jobInfo1{}))
	}
	buf := make([]byte, needed)
	r0, _, e1 := procGetJobW.Call(
		uintptr(h),
		uintptr(jobID),
		1,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(needed),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r0 == 0 {
		return 0, fmt.Errorf("GetJob(%d): %w", jobID, e1)
	}
	return (*jobInfo1)(unsafe.Pointer(&buf[0])).Status, nil
}

func winspoolVerifyJobOutcome(h windows.Handle, jobID uint32, printerName string) error {
	const (
		polls    = 8
		interval = 400 * time.Millisecond
	)
	var last uint32
	for i := 0; i < polls; i++ {
		time.Sleep(interval)
		st, err := winspoolJobStatus(h, jobID)
		if err != nil {
			break
		}
		last = st
		if winspoolJobStatusIsProblem(st) {
			return fmt.Errorf("print job on %q failed (job status 0x%X)", printerName, st)
		}
		if st&jobStatusComplete != 0 {
			break
		}
	}
	if winspoolJobStatusIsProblem(last) {
		return fmt.Errorf("print job on %q failed (job status 0x%X)", printerName, last)
	}
	return nil
}

type docInfo1 struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}

func winspoolCheckReady(printerName string) error {
	h, err := winspoolOpenPrinter(printerName)
	if err != nil {
		return err
	}
	defer procClosePrinter.Call(uintptr(h))
	return nil
}

func winspoolPrint(printerName string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty print payload")
	}
	h, err := winspoolOpenPrinter(printerName)
	if err != nil {
		return fmt.Errorf("%w: %v", errPrinterNotReady, err)
	}
	defer procClosePrinter.Call(uintptr(h))

	var last error
	for _, dtype := range []string{"RAW", "XPS_PASS"} {
		if err := winspoolPrintRawDoc(h, printerName, dtype, data); err == nil {
			return nil
		} else {
			last = err
			if dtype == "XPS_PASS" {
				return fmt.Errorf("%w: %v", errPrinterNotReady, err)
			}
		}
	}
	if last != nil {
		return fmt.Errorf("%w: %v", errPrinterNotReady, last)
	}
	return fmt.Errorf("%w: raw print to %q failed", errPrinterNotReady, printerName)
}

func winspoolPrintRawDoc(h windows.Handle, printerName, datatype string, data []byte) error {
	docName, _ := windows.UTF16PtrFromString(printAgentName)
	dtypePtr, err := windows.UTF16PtrFromString(datatype)
	if err != nil {
		return err
	}
	di := docInfo1{
		pDocName:    docName,
		pOutputFile: nil,
		pDatatype:   dtypePtr,
	}
	job, _, e1 := procStartDocPrinterW.Call(
		uintptr(h),
		1,
		uintptr(unsafe.Pointer(&di)),
	)
	if job == 0 {
		return fmt.Errorf("StartDocPrinter(%s): %w", datatype, e1)
	}
	jobID := uint32(job)

	r0, _, e2 := procStartPagePrinter.Call(uintptr(h))
	if r0 == 0 {
		procEndDocPrinter.Call(uintptr(h))
		return fmt.Errorf("StartPagePrinter: %w", e2)
	}

	var written uint32
	r0, _, e3 := procWritePrinter.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&written)),
	)
	procEndPagePrinter.Call(uintptr(h))
	if r0 == 0 {
		procEndDocPrinter.Call(uintptr(h))
		return fmt.Errorf("WritePrinter to %q: %w", printerName, e3)
	}
	if int(written) != len(data) {
		procEndDocPrinter.Call(uintptr(h))
		return fmt.Errorf("short write to %q: %d/%d bytes", printerName, written, len(data))
	}
	procEndDocPrinter.Call(uintptr(h))
	return winspoolVerifyJobOutcome(h, jobID, printerName)
}

func listWinspoolPrinterNames() ([]string, error) {
	return printer.ReadNames()
}
