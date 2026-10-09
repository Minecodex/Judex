// SPDX-License-Identifier: Apache-2.0

// judex CLI entry: thin wrapper over internal/cli (docs/plans/v1/07 §2).
package main

import (
	"os"

	"github.com/kakj-go/Judex/internal/cli"
)

func main() {
	root := cli.Root()
	if err := root.Execute(); err != nil {
		os.Exit(cli.TakeExitCode())
	}
}
