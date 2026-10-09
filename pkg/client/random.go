// SPDX-License-Identifier: Apache-2.0

package client

import "crypto/rand"

func readRandom(b []byte) (int, error) { return rand.Read(b) }
