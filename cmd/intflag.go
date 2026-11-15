// SPDX-License-Identifier: MIT

package cmd

import (
	"math"
	"strconv"

	"github.com/otfabric/canopenctl/internal/errs"
	"github.com/spf13/pflag"
)

// intFlag is the value of every integer flag. pflag's own int flag parses
// 64 bits and keeps the low 32 of them where int has 32 bits (the
// linux/arm/v7 release), so that 4294967301 became 5 and passed the range
// check of a node-ID. intFlag refuses a value outside the range of a 32-bit
// int on every platform, as invalid input (exit 3), before the command's
// own range check. A value that is not a number, or does not fit 64 bits,
// is pflag's invalid argument (exit 2), as before.
type intFlag struct {
	p    *int
	name string
}

func (f *intFlag) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64) // as pflag's int flag
	if err != nil {
		return err
	}
	if v < math.MinInt32 || v > math.MaxInt32 {
		return errs.Invalid("--%s %s: out of range (%d..%d)", f.name, s, math.MinInt32, math.MaxInt32)
	}
	*f.p = int(v)
	return nil
}

func (f *intFlag) Type() string   { return "int" }
func (f *intFlag) String() string { return strconv.Itoa(*f.p) }

// intVarP defines an integer flag, as pflag's IntVarP, of type intFlag.
func intVarP(fs *pflag.FlagSet, p *int, name, shorthand string, value int, usage string) {
	*p = value
	fs.VarP(&intFlag{p: p, name: name}, name, shorthand, usage)
}

// intVar defines an integer flag, as pflag's IntVar, of type intFlag.
func intVar(fs *pflag.FlagSet, p *int, name string, value int, usage string) {
	intVarP(fs, p, name, "", value, usage)
}
