// SPDX-License-Identifier: Apache-2.0

// Package clock provides an injectable time source so timeout/expiry logic
// is testable without sleeping (docs/plans/v1/01 §6).
package clock

import "time"

// Clock returns the authoritative server time. Business deadlines are always
// computed from this, never from client-supplied timestamps.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

// System is the production clock.
var System Clock = systemClock{}

// Fixed returns a frozen clock for tests.
type Fixed struct{ T time.Time }

func (f Fixed) Now() time.Time { return f.T }
