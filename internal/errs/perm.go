// SPDX-License-Identifier: MIT

package errs

import (
	"errors"
	"io/fs"
	"strings"
)

// isPermission recognises a missing privilege (EPERM, EACCES; the netlink
// package says "CAP_NET_ADMIN needed").
func isPermission(err error) bool {
	return errors.Is(err, fs.ErrPermission) || strings.Contains(err.Error(), "CAP_NET_ADMIN") ||
		strings.Contains(err.Error(), "operation not permitted")
}
