// SPDX-License-Identifier: MIT

// Package cli holds what several canopenctl commands share: parsing of
// node-IDs, indices, sub-indices, data types, values and PDO mappings,
// loading EDS/DCF files and traces, and the transmit safety check.
package cli

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/otfabric/go-canbus"
	"github.com/otfabric/go-canbus/capture"
	"github.com/otfabric/go-canbus/socketcan/link"
	"github.com/otfabric/go-canopen"
	"github.com/otfabric/go-canopen/eds"
	"github.com/otfabric/go-canopen/lss"
	"github.com/otfabric/go-canopen/mpdo"
	"github.com/otfabric/go-canopen/od"
	"github.com/otfabric/go-canopen/pdo"
)

// Virtual is the interface name of canopenctl's in-process virtual bus.
const Virtual = "virtual"

// SlcanPrefix starts an interface that is a serial-line CAN adapter:
// "slcan:/dev/ttyACM0", "slcan:COM3" or "slcan:tcp://host:port".
const SlcanPrefix = "slcan:"

// parseNumber parses a decimal or 0x-hexadecimal number of at most bits
// bits.
func parseNumber(s string, bits int) (uint64, error) {
	s = strings.TrimSpace(s)
	if h, ok := strings.CutPrefix(strings.ToLower(s), "0x"); ok {
		return strconv.ParseUint(h, 16, bits)
	}
	return strconv.ParseUint(s, 10, bits)
}

// ParseNode parses a node-ID: 1..127, decimal or 0x hexadecimal. With
// allowAll, "all" and "0" address every node (NMT).
func ParseNode(s string, allowAll bool) (canopen.NodeID, error) {
	if allowAll && strings.EqualFold(strings.TrimSpace(s), "all") {
		return canopen.BroadcastNodeID, nil
	}
	v, err := parseNumber(s, 8)
	if err == nil && v == 0 && allowAll {
		return canopen.BroadcastNodeID, nil
	}
	if err != nil || !canopen.NodeID(v).Valid() {
		if allowAll {
			return 0, errs.Invalid("invalid node %q (1..127, or all)", s)
		}
		return 0, errs.Invalid("invalid node-ID %q (1..127)", s)
	}
	return canopen.NodeID(v), nil
}

// ParseNodeList parses a list of node-IDs and ranges: "1-10,20,0x30".
// Duplicates are removed; the result is in ascending order.
func ParseNodeList(s string) ([]canopen.NodeID, error) {
	var seen [128]bool
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo, hi, isRange := strings.Cut(part, "-")
		a, err := ParseNode(lo, false)
		if err != nil {
			return nil, errs.Invalid("invalid node list %q: %v", s, err)
		}
		b := a
		if isRange {
			if b, err = ParseNode(hi, false); err != nil || b < a {
				return nil, errs.Invalid("invalid node range %q (low-high, 1..127)", part)
			}
		}
		for n := a; n <= b; n++ {
			seen[n] = true
		}
	}
	var out []canopen.NodeID
	for n := canopen.MinNodeID; n <= canopen.MaxNodeID; n++ {
		if seen[n] {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil, errs.Invalid("empty node list %q", s)
	}
	return out, nil
}

// ParseIndex parses an object index: hexadecimal, with or without 0x
// ("1018", "0x1018").
func ParseIndex(s string) (uint16, error) {
	t := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "0x"), "0X")
	v, err := strconv.ParseUint(t, 16, 16)
	if err != nil || t == "" {
		return 0, errs.Invalid("invalid index %q (hexadecimal 0000..FFFF)", s)
	}
	return uint16(v), nil
}

// ParseSub parses a sub-index: decimal, or hexadecimal with 0x ("1",
// "0x01").
func ParseSub(s string) (uint8, error) {
	v, err := parseNumber(s, 8)
	if err != nil {
		return 0, errs.Invalid("invalid sub-index %q (0..255, or 0x00..0xFF)", s)
	}
	return uint8(v), nil
}

// typeAliases are short names for --type.
var typeAliases = map[string]canopen.DataType{
	"bool": canopen.Boolean, "boolean": canopen.Boolean,
	"i8": canopen.Integer8, "i16": canopen.Integer16, "i24": canopen.Integer24, "i32": canopen.Integer32,
	"i40": canopen.Integer40, "i48": canopen.Integer48, "i56": canopen.Integer56, "i64": canopen.Integer64,
	"int8": canopen.Integer8, "int16": canopen.Integer16, "int32": canopen.Integer32, "int64": canopen.Integer64,
	"u8": canopen.Unsigned8, "u16": canopen.Unsigned16, "u24": canopen.Unsigned24, "u32": canopen.Unsigned32,
	"u40": canopen.Unsigned40, "u48": canopen.Unsigned48, "u56": canopen.Unsigned56, "u64": canopen.Unsigned64,
	"uint8": canopen.Unsigned8, "uint16": canopen.Unsigned16, "uint32": canopen.Unsigned32, "uint64": canopen.Unsigned64,
	"f32": canopen.Real32, "f64": canopen.Real64, "real32": canopen.Real32, "real64": canopen.Real64,
	"float32": canopen.Real32, "float64": canopen.Real64,
	"str": canopen.VisibleString, "string": canopen.VisibleString, "vs": canopen.VisibleString,
	"os": canopen.OctetString, "octets": canopen.OctetString, "hex": canopen.OctetString, "bytes": canopen.OctetString,
	"us": canopen.UnicodeString, "unicode": canopen.UnicodeString,
	"domain": canopen.Domain, "tod": canopen.TimeOfDay, "td": canopen.TimeDifference,
}

// ParseType parses a --type value: a CiA 301 name (UNSIGNED32), an alias
// (u32, i16, f32, bool, string, octets, domain...) or a type index (0x0007).
// Structured types cannot be transferred as values.
func ParseType(s string) (canopen.DataType, error) {
	if t, ok := typeAliases[strings.ToLower(strings.TrimSpace(s))]; ok {
		return t, nil
	}
	t, err := canopen.ParseDataType(s)
	if err != nil || !t.Known() || t >= canopen.PDOCommParam {
		return 0, errs.Invalid("unknown data type %q (UNSIGNED8..64, INTEGER8..64, REAL32/64, BOOLEAN, VISIBLE_STRING, OCTET_STRING, UNICODE_STRING, DOMAIN; or u8, i32, f32, bool, string, octets...)", s)
	}
	return t, nil
}

// ParseValue parses a value given on the command line as the encoding of
// type t: integers in decimal, 0x hexadecimal or octal with a leading 0
// (negative ones with a sign, $NODEID allowed), floats in decimal (or the
// 0x bit pattern), BOOLEAN as 0/1/true/false/on/off, VISIBLE_STRING and
// UNICODE_STRING as text, OCTET_STRING and DOMAIN as hexadecimal digit
// pairs.
func ParseValue(t canopen.DataType, s string, node canopen.NodeID) ([]byte, error) {
	if t == canopen.Boolean {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "true", "on", "yes":
			s = "1"
		case "false", "off", "no":
			s = "0"
		}
	}
	if t.IsInteger() || t == canopen.Boolean || t.IsFloat() {
		if strings.TrimSpace(s) == "" {
			return nil, errs.New(errs.KindInvalidInput, errs.CodeInvalidValue, "empty %s value", t)
		}
	}
	b, err := canopen.ParseValue(t, s, node)
	if err != nil {
		return nil, &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidValue,
			Message: fmt.Sprintf("invalid %s value %q: %v", t, s, trimPrefix(err)), Cause: err}
	}
	return b, nil
}

// trimPrefix drops go-canopen's "canopen: data type mismatch: " from an
// error message (the caller says what failed).
func trimPrefix(err error) string {
	m := err.Error()
	if i := strings.LastIndex(m, ": "); i >= 0 && errors.Is(err, canopen.ErrDataType) {
		return m[i+2:]
	}
	return m
}

// ParseMapping parses PDO mapping entries "IIII:SS:BITS" (or IIII:SS/BITS),
// comma-separated: index and sub-index hexadecimal (0x optional), the
// length in bits decimal.
func ParseMapping(s string) (pdo.Mapping, error) {
	var m pdo.Mapping
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f := strings.FieldsFunc(part, func(r rune) bool { return r == ':' || r == '/' })
		if len(f) != 3 {
			return nil, errs.Invalid("invalid mapping entry %q (IIII:SS:BITS, e.g. 0x6041:00:16)", part)
		}
		idx, err := ParseIndex(f[0])
		if err != nil {
			return nil, errs.Invalid("invalid mapping entry %q: %v", part, err)
		}
		sub, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(f[1], "0x"), "0X"), 16, 8)
		if err != nil {
			return nil, errs.Invalid("invalid mapping entry %q: sub-index %q (hexadecimal 00..FF)", part, f[1])
		}
		bits, err := strconv.ParseUint(f[2], 10, 8)
		if err != nil || bits == 0 || bits > pdo.MaxBits {
			return nil, errs.Invalid("invalid mapping entry %q: length %q (1..64 bits)", part, f[2])
		}
		m = append(m, pdo.Entry{Index: idx, SubIndex: uint8(sub), Bits: uint8(bits)})
	}
	if len(m) > pdo.MaxEntries {
		return nil, errs.Invalid("%d mapping entries (at most %d)", len(m), pdo.MaxEntries)
	}
	if m.Bits() > pdo.MaxBits {
		return nil, errs.Invalid("mapping of %d bits (a PDO carries at most %d)", m.Bits(), pdo.MaxBits)
	}
	return m, nil
}

// ParseCOBID parses a COB-ID: a CAN identifier in hexadecimal (0x
// optional), 11-bit unless it does not fit.
func ParseCOBID(s string) (canopen.COBID, error) {
	t := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "0x"), "0X")
	v, err := strconv.ParseUint(t, 16, 32)
	if err != nil || canbus.ID(v) > canbus.MaxExtendedID {
		return 0, errs.Invalid("invalid COB-ID %q (hexadecimal CAN identifier)", s)
	}
	return canopen.NewCOBID(canbus.ID(v)), nil
}

// GuessType chooses a data type for a value read without one: an unsigned
// integer of its width for 1..8 bytes, VISIBLE_STRING for longer printable
// text, OCTET_STRING otherwise.
func GuessType(b []byte) canopen.DataType {
	switch len(b) {
	case 1:
		return canopen.Unsigned8
	case 2:
		return canopen.Unsigned16
	case 3:
		return canopen.Unsigned24
	case 4:
		return canopen.Unsigned32
	case 5:
		return canopen.Unsigned40
	case 6:
		return canopen.Unsigned48
	case 7:
		return canopen.Unsigned56
	case 8:
		return canopen.Unsigned64
	}
	if len(b) > 0 && utf8.Valid(b) {
		printable := true
		for _, c := range string(b) {
			if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
				printable = false
				break
			}
		}
		if printable {
			return canopen.VisibleString
		}
	}
	return canopen.OctetString
}

// JSONValue returns the JSON form of an encoded value of type t: a number
// for integers and finite floats ("NaN", "+Inf", "-Inf" otherwise),
// a boolean, a string for strings, hexadecimal digits for everything else.
// ok is false when the data do not decode as t (the hex digits are
// returned).
func JSONValue(t canopen.DataType, b []byte) (v any, ok bool) {
	d, err := canopen.Decode(t, b)
	if err != nil {
		return fmt.Sprintf("%x", b), false
	}
	switch x := d.(type) {
	case float32:
		return floatJSON(float64(x)), true
	case float64:
		return floatJSON(x), true
	case []byte:
		return fmt.Sprintf("%x", x), true
	}
	return d, true
}

func floatJSON(f float64) any {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return f
}

// ValueText formats an encoded value of type t for people: decimal for
// signed integers and floats, unsigned integers in decimal and
// hexadecimal, text for strings, hexadecimal bytes otherwise.
func ValueText(t canopen.DataType, b []byte) string {
	if _, err := canopen.Decode(t, b); err != nil {
		return fmt.Sprintf("% x (not a %s)", b, t)
	}
	switch {
	case t.IsInteger() && !t.IsSigned():
		u := canopen.DecodeUint(b)
		return fmt.Sprintf("%d (0x%0*X)", u, len(b)*2, u)
	case t == canopen.Boolean:
		if b[0] != 0 {
			return "true"
		}
		return "false"
	}
	return canopen.FormatValue(t, b)
}

// LoadEDS reads an EDS or DCF file. Lines it cannot read are an
// invalid_eds error (exit 3).
func LoadEDS(path string) (*eds.File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, errs.New(errs.KindInvalidInput, errs.CodeFileOpen, "cannot read EDS: %w", err)
	}
	f, err := eds.ParseBytes(b)
	if err != nil {
		return nil, &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidEDS, Message: fmt.Sprintf("%s: %v", path, err), Cause: err}
	}
	return f, nil
}

// Dictionary builds the object dictionary an EDS or DCF describes for a
// node (lenient: objects it cannot hold are skipped, reported in the
// diagnostics).
func Dictionary(f *eds.File, path string, node canopen.NodeID, lenient bool) (*od.Dictionary, []eds.Diagnostic, error) {
	d, diags, err := f.BuildDictionary(node, eds.BuildOptions{Lenient: lenient})
	if err != nil {
		return nil, diags, &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidEDS,
			Message: fmt.Sprintf("%s: %v (eds validate lists the problems)", path, err), Cause: err}
	}
	return d, diags, nil
}

// OpenTrace opens a capture file ("-" for stdin) of any supported format.
func OpenTrace(path string, stdin io.Reader) (capture.Reader, func(), error) {
	r := stdin
	closer := func() {}
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, errs.New(errs.KindInvalidInput, errs.CodeFileOpen, "cannot open trace: %w", err)
		}
		r, closer = f, func() { _ = f.Close() }
	}
	cr, _, err := capture.NewReader(r)
	if err != nil {
		closer()
		return nil, nil, &errs.Error{Kind: errs.KindInvalidInput, Code: errs.CodeInvalidCapture, Message: fmt.Sprintf("%s: %v", path, err), Cause: err}
	}
	return cr, closer, nil
}

// TransmitCheck enforces the transmit safety rule: a command that changes
// the state of a device or the network (writes, NMT, PDO mapping, drive
// motion, SYNC/TIME production, a simulated node) needs --yes on an
// interface that is not virtual (canopenctl's in-process bus, vcan or
// vxcan).
func TransmitCheck(iface string, yes bool, what string) error {
	if iface == Virtual || yes {
		return nil
	}
	if strings.HasPrefix(iface, SlcanPrefix) {
		return errs.New(errs.KindSafety, errs.CodeNeedsYes,
			"%s would transmit on %s, a serial-line CAN adapter: CANopen devices may move actuators. Check the network, then repeat with --yes", what, iface)
	}
	it, err := link.Get(iface)
	if err == nil && it.Virtual() {
		return nil
	}
	kind := "a physical or unknown interface"
	if err == nil {
		kind = "a " + it.Kind + " interface"
	}
	if errors.Is(err, canbus.ErrInterfaceDown) {
		return errs.New(errs.KindTransport, errs.CodeInterface, "%s: %v", iface, err)
	}
	if errors.Is(err, canbus.ErrUnsupported) && !strings.Contains(err.Error(), "not a CAN interface") {
		return errs.New(errs.KindUnsupported, errs.CodeUnsupported, "%s: %v", iface, err)
	}
	return errs.New(errs.KindSafety, errs.CodeNeedsYes,
		"%s would transmit on %s, %s: CANopen devices may move actuators. Check the network, then repeat with --yes", what, iface, kind)
}

// Hex formats a uint as 0x-prefixed hexadecimal with digits digits.
func Hex(v uint64, digits int) string { return fmt.Sprintf("0x%0*X", digits, v) }

// Key formats an object address as "0x1018:01".
func Key(index uint16, sub uint8) string { return fmt.Sprintf("0x%04X:%02X", index, sub) }

// ParseUint32 parses an UNSIGNED32: decimal or 0x hexadecimal.
func ParseUint32(s string) (uint32, error) {
	v, err := parseNumber(s, 32)
	if err != nil {
		return 0, errs.Invalid("invalid value %q (decimal or 0x hexadecimal UNSIGNED32)", s)
	}
	return uint32(v), nil
}

// ParseLSSAddress parses an LSS address as vendor:product:revision:serial,
// each part hexadecimal (0x optional), as lss.Address.String writes it.
func ParseLSSAddress(s string) (lss.Address, error) {
	var a lss.Address
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 4 {
		return a, errs.Invalid("invalid LSS address %q (vendor:product:revision:serial, hexadecimal)", s)
	}
	for i, p := range parts {
		t := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(p), "0x"), "0X")
		v, err := strconv.ParseUint(t, 16, 32)
		if err != nil {
			return a, errs.Invalid("invalid LSS address %q (vendor:product:revision:serial, hexadecimal)", s)
		}
		a.SetPart(lss.Part(i), uint32(v))
	}
	return a, nil
}

// parseObjectBlock parses "IIII:SS[+n]": an object, and a block size
// (0 when not given).
func parseObjectBlock(s string) (uint16, uint8, uint8, error) {
	obj, n, hasN := strings.Cut(strings.TrimSpace(s), "+")
	idx, sub, ok := strings.Cut(obj, ":")
	if !ok {
		return 0, 0, 0, errs.Invalid("invalid object %q (IIII:SS, hexadecimal)", s)
	}
	i, err := ParseIndex(idx)
	if err != nil {
		return 0, 0, 0, err
	}
	sv, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(sub)), "0x"), 16, 8)
	if err != nil {
		return 0, 0, 0, errs.Invalid("invalid sub-index in %q (IIII:SS, hexadecimal)", s)
	}
	var size uint64
	if hasN {
		if size, err = strconv.ParseUint(strings.TrimSpace(n), 10, 8); err != nil || size == 0 {
			return 0, 0, 0, errs.Invalid("invalid block size in %q (+1..+255)", s)
		}
	}
	return i, uint8(sv), uint8(size), nil
}

// ParseScannerEntry parses an MPDO object scanner list entry:
// "IIII:SS[+n]" (index and sub-index hexadecimal, n sub-indices from SS;
// default 1).
func ParseScannerEntry(s string) (mpdo.ScannerEntry, error) {
	i, sub, n, err := parseObjectBlock(s)
	if err != nil {
		return mpdo.ScannerEntry{}, err
	}
	e := mpdo.ScannerEntry{Index: i, SubIndex: sub, BlockSize: n}
	if i == 0 || e.Validate() != nil {
		return e, errs.Invalid("invalid scanner entry %q: the index must not be 0 and the block must end at sub-index FF", s)
	}
	return e, nil
}

// ParseDispatcherEntry parses an MPDO object dispatcher list entry:
// "NODE:IIII:SS[+n]=IIII:SS", the producer's node-ID and object (and a
// block of n sub-indices) written to the local object.
func ParseDispatcherEntry(s string) (mpdo.DispatcherEntry, error) {
	var e mpdo.DispatcherEntry
	src, dst, ok := strings.Cut(strings.TrimSpace(s), "=")
	nodeS, obj, ok2 := strings.Cut(src, ":")
	if !ok || !ok2 {
		return e, errs.Invalid("invalid dispatcher entry %q (NODE:IIII:SS[+n]=IIII:SS)", s)
	}
	node, err := ParseNode(nodeS, false)
	if err != nil {
		return e, err
	}
	pi, ps, n, err := parseObjectBlock(obj)
	if err != nil {
		return e, err
	}
	li, ls, ln, err := parseObjectBlock(dst)
	if err != nil {
		return e, err
	}
	if ln != 0 {
		return e, errs.Invalid("invalid dispatcher entry %q: the block size goes with the producer's object", s)
	}
	e = mpdo.DispatcherEntry{Producer: node, ProducerIndex: pi, ProducerSubIndex: ps, Index: li, SubIndex: ls, BlockSize: n}
	if pi == 0 || li == 0 || e.Validate() != nil {
		return e, errs.Invalid("invalid dispatcher entry %q: indices must not be 0 and blocks must end at sub-index FF", s)
	}
	return e, nil
}

// storeGroups are the sub-indices of 0x1010 and 0x1011 by name.
var storeGroups = map[string]uint8{"all": 1, "communication": 2, "comm": 2, "application": 3, "app": 3, "manufacturer": 4}

// ParseStoreGroup parses a parameter group of 0x1010/0x1011: all,
// communication, application, manufacturer, or the sub-index 1..127.
func ParseStoreGroup(s string) (uint8, error) {
	if g, ok := storeGroups[strings.ToLower(strings.TrimSpace(s))]; ok {
		return g, nil
	}
	v, err := parseNumber(s, 8)
	if err != nil || v < 1 || v > 127 {
		return 0, errs.Invalid("invalid group %q (all, communication, application, manufacturer or a sub-index 1..127)", s)
	}
	return uint8(v), nil
}

// ParseNodeValue parses "NODE=VALUE" (a node-ID and a file, an address...).
func ParseNodeValue(s string) (canopen.NodeID, string, error) {
	n, v, ok := strings.Cut(s, "=")
	if !ok || strings.TrimSpace(v) == "" {
		return 0, "", errs.Invalid("invalid %q (NODE=VALUE)", s)
	}
	node, err := ParseNode(n, false)
	if err != nil {
		return 0, "", err
	}
	return node, strings.TrimSpace(v), nil
}
