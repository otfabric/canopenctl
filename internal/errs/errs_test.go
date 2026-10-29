// SPDX-License-Identifier: MIT

package errs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/cia402"
	"github.com/otfabric/go-canopen/lss"
	"github.com/otfabric/go-canopen/master"
	"github.com/otfabric/go-canopen/mpdo"
	"github.com/otfabric/go-canopen/pdo"
	"github.com/otfabric/go-canopen/program"
	"github.com/otfabric/go-canopen/timestamp"
	"github.com/otfabric/go-canopen/usdo"
)

func TestClassify(t *testing.T) {
	remote := &canopen.AbortError{Code: canopen.AbortNoObject, Index: 0x5FFF, Remote: true}
	local := &canopen.AbortError{Code: canopen.AbortTimeout, Index: 0x1000}
	cases := []struct {
		err  error
		kind Kind
		code string
	}{
		{context.Canceled, KindInternal, CodeCanceled},
		{fmt.Errorf("x: %w", context.DeadlineExceeded), KindTimeout, CodeTimeout},
		{canbus.ErrTimeout, KindTimeout, CodeTimeout},
		{fmt.Errorf("sdo: %w", remote), KindAbort, CodeSDOAbort},
		{fmt.Errorf("%w: %w", canopen.ErrTimeout, local), KindTimeout, CodeSDOTimeout},
		{canopen.ErrTimeout, KindTimeout, CodeSDOTimeout},
		{fmt.Errorf("%w: %w", canopen.ErrProtocol, &canopen.AbortError{Code: canopen.AbortToggle}), KindProtocol, CodeSDOProtocol},
		{canopen.ErrProtocol, KindProtocol, CodeSDOProtocol},
		{&canopen.AbortError{Code: canopen.AbortGeneral}, KindProtocol, CodeSDOProtocol},
		{canopen.ErrDataType, KindProtocol, CodeDataType},
		{canopen.ErrInvalidNodeID, KindInvalidInput, CodeInvalidInput},
		{pdo.ErrInactive, KindDevice, CodePDOInactive},
		{master.ErrIdentity, KindDevice, CodeIdentity},
		{timestamp.ErrNotProducer, KindDevice, CodeNotProducer},
		{canbus.ErrUnsupported, KindUnsupported, CodeUnsupported},
		{canbus.ErrInterfaceDown, KindTransport, CodeInterface},
		{canbus.ErrInvalidFrame, KindInvalidInput, CodeInvalidInput},
		{fs.ErrPermission, KindPermission, CodePermission},
		{errors.New("socket: operation not permitted"), KindPermission, CodePermission},
		{canbus.ErrClosed, KindTransport, CodeIO},
		{canopen.ErrClosed, KindTransport, CodeIO},
		{errors.New("boom"), KindInternal, CodeInternalUnexpected},
		{New(KindSafety, CodeNeedsYes, "x"), KindSafety, CodeNeedsYes},
		{&cia402.Error{Op: "enable", State: cia402.Fault, FaultCode: 0x2310, Err: cia402.ErrFault}, KindDevice, CodeDriveFault},
		{&cia402.Error{Op: "move", Err: cia402.ErrNotEnabled}, KindDevice, CodeDriveState},
		{&cia402.Error{Op: "enable", Err: fmt.Errorf("x: %w", remote)}, KindAbort, CodeSDOAbort},
		{&cia402.Error{Op: "enable", Err: fmt.Errorf("%w: %w", canopen.ErrTimeout, local)}, KindTimeout, CodeSDOTimeout},
		{&cia402.Error{Op: "enable", Err: canopen.ErrTimeout}, KindTimeout, CodeTimeout},
		{&cia402.Error{Op: "enable", Err: context.Canceled}, KindInternal, CodeCanceled},
		{&cia402.Error{Op: "enable", Err: canbus.ErrClosed}, KindTransport, CodeIO},
		{&usdo.AbortError{Code: usdo.FromSDO(canopen.AbortNoObject), Node: 5, Index: 0x5FFF, Remote: true}, KindAbort, CodeSDOAbort},
		{&usdo.AbortError{Code: usdo.FromSDO(canopen.AbortGeneral), Node: 5}, KindProtocol, CodeSDOProtocol},
		{&master.BootError{Node: 5, Status: master.BootVendorID}, KindDevice, CodeBootFailed},
		{fmt.Errorf("x: %w", &lss.ConfigError{Code: lss.CodeOutOfRange}), KindDevice, CodeLSSRefused},
		{program.FlashWriteError, KindDevice, CodeProgramFailed},
		{master.ErrProgram, KindDevice, CodeProgramFailed},
		{lss.ErrConflict, KindDevice, CodeLSSConflict},
		{lss.ErrNoNodeID, KindDevice, CodeNoNodeID},
		{mpdo.ErrInactive, KindDevice, CodeMPDO},
	}
	for _, c := range cases {
		e := Classify(c.err)
		if e.Kind != c.kind || e.Code != c.code || e.Error() == "" {
			t.Errorf("%v: %s/%s, want %s/%s", c.err, e.Kind, e.Code, c.kind, c.code)
		}
	}
	e := Classify(fmt.Errorf("sdo: %w", remote))
	if e.Abort == nil || e.Abort.Code != "0x06020000" || e.Abort.Index != 0x5FFF || !e.Abort.Remote || e.Abort.Text == "" {
		t.Fatalf("%+v", e.Abort)
	}
	u := Classify(&usdo.AbortError{Code: usdo.FromSDO(canopen.AbortNoObject), Index: 0x5FFF, Remote: true})
	if u.Abort == nil || u.Abort.Code != "0x06020000" || u.Abort.USDOCode == "" || !u.Abort.Remote {
		t.Fatalf("%+v", u.Abort)
	}
	d := Classify(&cia402.Error{Op: "enable", State: cia402.Fault, Statusword: 0x0218, FaultCode: 0x2310, Err: fmt.Errorf("%w: %w", cia402.ErrFault, remote)})
	if d.Drive == nil || d.Drive.FaultCode != "0x2310" || d.Drive.Statusword != "0x0218" || d.Abort == nil {
		t.Fatalf("%+v", d)
	}
	for k, code := range map[Kind]int{KindUsage: 2, KindInvalidInput: 3, KindTransport: 4, KindTimeout: 5, KindProtocol: 6,
		KindPermission: 7, KindUnsupported: 8, KindSafety: 9, KindOutput: 10, KindInternal: 10, KindAbort: 11, KindDevice: 12} {
		if ExitCode(k) != code {
			t.Errorf("%s: %d", k, ExitCode(k))
		}
	}
	w := New(KindTransport, CodeIO, "read: %w", fs.ErrNotExist)
	if !errors.Is(w, fs.ErrNotExist) || Invalid("x %d", 1).Message != "x 1" {
		t.Fatal("wrapping")
	}
}
