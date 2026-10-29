// SPDX-License-Identifier: MIT

// Package errs holds canopenctl's typed errors, their stable codes and the
// process exit codes they map to (docs/exitcodes.md).
package errs

import (
	"context"
	"errors"
	"fmt"

	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/cia402"
	"github.com/otfabric/go-canopen/master"
	"github.com/otfabric/go-canopen/pdo"
	"github.com/otfabric/go-canopen/timestamp"
)

// Kind is a failure category; it selects the exit code.
type Kind string

// Kinds.
const (
	KindUsage        Kind = "usage"
	KindInvalidInput Kind = "invalid_input"
	KindTransport    Kind = "transport"
	KindTimeout      Kind = "timeout"
	KindProtocol     Kind = "protocol"
	KindUnsupported  Kind = "unsupported"
	KindSafety       Kind = "safety"
	KindPermission   Kind = "permission"
	KindAbort        Kind = "abort"
	KindDevice       Kind = "device"
	KindOutput       Kind = "output"
	KindInternal     Kind = "internal"
)

// Process exit codes.
const (
	ExitOK           = 0
	ExitFindings     = 1 // a result was written and it reports problems
	ExitUsage        = 2
	ExitInvalidInput = 3
	ExitTransport    = 4
	ExitTimeout      = 5
	ExitProtocol     = 6
	ExitPermission   = 7
	ExitUnsupported  = 8
	ExitSafety       = 9
	ExitInternal     = 10
	ExitAbort        = 11 // the device refused an SDO transfer (abort code)
	ExitDevice       = 12 // the device is in a state that prevents the operation
)

// Stable codes for scripts (the JSON error's "code").
const (
	CodeInvalidInput       = "invalid_input"
	CodeInvalidValue       = "invalid_value"
	CodeInvalidEDS         = "invalid_eds"
	CodeInvalidCapture     = "invalid_capture"
	CodeFileOpen           = "file_open_failed"
	CodeInterface          = "interface_unavailable"
	CodeIO                 = "io_failed"
	CodeTimeout            = "timeout"
	CodeSDOTimeout         = "sdo_timeout"
	CodeSDOAbort           = "sdo_abort"
	CodeSDOProtocol        = "sdo_protocol"
	CodeDataType           = "data_type_mismatch"
	CodeNoResponse         = "no_response"
	CodePDOInactive        = "pdo_inactive"
	CodeIdentity           = "identity_mismatch"
	CodeDriveFault         = "drive_fault"
	CodeDriveState         = "drive_state"
	CodeNotProducer        = "not_producer"
	CodeUnsupported        = "unsupported"
	CodeNeedsYes           = "confirmation_required"
	CodePermission         = "permission_denied"
	CodeOutputFailed       = "output_failed"
	CodeCanceled           = "canceled"
	CodeInternalUnexpected = "internal_unexpected"
)

// Abort describes an SDO abort in an error document.
type Abort struct {
	// Code is the abort code as 0x-prefixed hexadecimal ("0x06020000").
	Code string `json:"code"`
	// Text is CiA 301's description of the code; empty for codes it does
	// not define.
	Text string `json:"text,omitempty"`
	// Index and SubIndex are the object the transfer concerned.
	Index    uint16 `json:"index"`
	SubIndex uint8  `json:"subIndex"`
	// Remote is true when the device sent the abort, false when
	// canopenctl aborted the transfer (timeout, protocol violation).
	Remote bool `json:"remote"`
}

// Drive describes the state of a CiA 402 drive in an error document.
type Drive struct {
	Operation  string `json:"operation"`
	State      string `json:"state"`
	Statusword string `json:"statusword"`
	FaultCode  string `json:"faultCode,omitempty"`
}

// Error is a typed failure.
type Error struct {
	Kind    Kind   `json:"kind"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Abort   *Abort `json:"abort,omitempty"`
	Drive   *Drive `json:"drive,omitempty"`
	Cause   error  `json:"-"`
}

func (e *Error) Error() string { return e.Message }

// Unwrap returns the cause.
func (e *Error) Unwrap() error { return e.Cause }

// New returns a typed error.
func New(kind Kind, code, format string, args ...any) *Error {
	err := fmt.Errorf(format, args...)
	return &Error{Kind: kind, Code: code, Message: err.Error(), Cause: errors.Unwrap(err)}
}

// Invalid is an invalid input error.
func Invalid(format string, args ...any) *Error {
	return New(KindInvalidInput, CodeInvalidInput, format, args...)
}

// ExitCode maps a kind to its exit code.
func ExitCode(k Kind) int {
	switch k {
	case KindUsage:
		return ExitUsage
	case KindInvalidInput:
		return ExitInvalidInput
	case KindTransport:
		return ExitTransport
	case KindTimeout:
		return ExitTimeout
	case KindProtocol:
		return ExitProtocol
	case KindPermission:
		return ExitPermission
	case KindUnsupported:
		return ExitUnsupported
	case KindSafety:
		return ExitSafety
	case KindAbort:
		return ExitAbort
	case KindDevice:
		return ExitDevice
	default:
		return ExitInternal
	}
}

// AbortOf describes an SDO abort for an error document.
func AbortOf(ae *canopen.AbortError) *Abort {
	return &Abort{Code: fmt.Sprintf("0x%08X", uint32(ae.Code)), Text: ae.Code.Text(), Index: ae.Index, SubIndex: ae.SubIndex, Remote: ae.Remote}
}

// Classify turns any error into a typed one, recognising go-canbus and
// go-canopen sentinels.
func Classify(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	mk := func(k Kind, code string) *Error { return &Error{Kind: k, Code: code, Message: err.Error(), Cause: err} }
	var ae *canopen.AbortError
	hasAbort := errors.As(err, &ae)
	var de *cia402.Error
	if errors.As(err, &de) {
		r := classifyDrive(err, de)
		if hasAbort {
			r.Abort = AbortOf(ae)
		}
		return r
	}
	switch {
	case errors.Is(err, context.Canceled):
		return mk(KindInternal, CodeCanceled)
	case hasAbort && ae.Remote:
		r := mk(KindAbort, CodeSDOAbort)
		r.Abort = AbortOf(ae)
		return r
	case errors.Is(err, canopen.ErrTimeout):
		r := mk(KindTimeout, CodeSDOTimeout)
		if hasAbort {
			r.Abort = AbortOf(ae)
		}
		return r
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, canbus.ErrTimeout):
		return mk(KindTimeout, CodeTimeout)
	case errors.Is(err, canopen.ErrProtocol):
		r := mk(KindProtocol, CodeSDOProtocol)
		if hasAbort {
			r.Abort = AbortOf(ae)
		}
		return r
	case hasAbort:
		r := mk(KindProtocol, CodeSDOProtocol)
		r.Abort = AbortOf(ae)
		return r
	case errors.Is(err, canopen.ErrDataType):
		return mk(KindProtocol, CodeDataType)
	case errors.Is(err, canopen.ErrInvalidNodeID):
		return mk(KindInvalidInput, CodeInvalidInput)
	case errors.Is(err, pdo.ErrInactive), errors.Is(err, pdo.ErrNumber):
		return mk(KindDevice, CodePDOInactive)
	case errors.Is(err, master.ErrIdentity):
		return mk(KindDevice, CodeIdentity)
	case errors.Is(err, timestamp.ErrNotProducer):
		return mk(KindDevice, CodeNotProducer)
	case errors.Is(err, canbus.ErrUnsupported):
		return mk(KindUnsupported, CodeUnsupported)
	case errors.Is(err, canbus.ErrInterfaceDown):
		return mk(KindTransport, CodeInterface)
	case errors.Is(err, canbus.ErrInvalidFrame), errors.Is(err, canbus.ErrInvalidOption):
		return mk(KindInvalidInput, CodeInvalidInput)
	case isPermission(err):
		return mk(KindPermission, CodePermission)
	case errors.Is(err, canbus.ErrClosed), errors.Is(err, canbus.ErrBufferOverflow), errors.Is(err, canopen.ErrClosed):
		return mk(KindTransport, CodeIO)
	}
	return mk(KindInternal, CodeInternalUnexpected)
}

// classifyDrive classifies the failure of a CiA 402 controller operation.
func classifyDrive(err error, de *cia402.Error) *Error {
	r := &Error{Message: err.Error(), Cause: err,
		Drive: &Drive{Operation: de.Op, State: de.State.String(), Statusword: fmt.Sprintf("0x%04X", de.Statusword)}}
	if de.FaultCode != 0 {
		r.Drive.FaultCode = fmt.Sprintf("0x%04X", de.FaultCode)
	}
	var ae *canopen.AbortError
	switch {
	case errors.Is(err, context.Canceled):
		r.Kind, r.Code = KindInternal, CodeCanceled
	case errors.As(err, &ae) && ae.Remote:
		r.Kind, r.Code = KindAbort, CodeSDOAbort
	case errors.Is(err, canopen.ErrTimeout) && errors.As(err, &ae):
		r.Kind, r.Code = KindTimeout, CodeSDOTimeout
	case errors.Is(err, cia402.ErrFault):
		r.Kind, r.Code = KindDevice, CodeDriveFault
	case errors.Is(err, cia402.ErrNotEnabled), errors.Is(err, cia402.ErrUnreachable), errors.Is(err, cia402.ErrHoming),
		errors.Is(err, cia402.ErrStatusword):
		r.Kind, r.Code = KindDevice, CodeDriveState
	case errors.Is(err, canopen.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		r.Kind, r.Code = KindTimeout, CodeTimeout
	default:
		inner := Classify(de.Err)
		r.Kind, r.Code = inner.Kind, inner.Code
	}
	return r
}

// ErrFindings ends a command whose result was written and reports
// problems: exit code 1, no error document.
var ErrFindings = errors.New("findings reported")
