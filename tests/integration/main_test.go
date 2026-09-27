// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"os"
	"testing"

	integration "github.com/kakj-go/Judex/tests/integration"
)

// TestMain tears down the shared postgres container once the whole suite is
// done; each test still owns an isolated database (see fixture.go).
func TestMain(m *testing.M) {
	code := m.Run()
	integration.ShutdownContainer()
	os.Exit(code)
}
