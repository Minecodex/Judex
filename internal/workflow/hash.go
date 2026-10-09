// SPDX-License-Identifier: Apache-2.0

package workflow

import "crypto/sha256"

func __sha256(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
