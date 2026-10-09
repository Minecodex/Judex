// SPDX-License-Identifier: Apache-2.0

package httptransport

import (
	"github.com/kakj-go/Judex/internal/platform/errors"
)

// Sentinel transport errors kept separate from the router wiring for clarity.
var (
	errInternalPanic    = errors.New(errors.Internal, "request failed")
	errDraining         = errors.Newf(errors.DependencyDown, "server is shutting down").WithRetryable(true)
	errMethodNotAllowed = errors.New(errors.MethodNotAllowed, "method not allowed")
	errAPINotFound      = errors.New(errors.NotFound, "API endpoint not found")
	errAssetNotFound    = errors.New(errors.NotFound, "asset not found")
	errWebBuildMissing  = errors.New(errors.NotFound, "WEB_BUILD_MISSING: build web/ or use its Vite development server")
)
