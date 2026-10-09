// SPDX-License-Identifier: Apache-2.0
package integrationtest_test

import (
	"context"
	"github.com/google/uuid"
	"testing"

	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/platform/auth"
	"github.com/kakj-go/Judex/internal/platform/errors"
	integration "github.com/kakj-go/Judex/tests/integration"
)

func newDeviceEnv(t *testing.T) *identity.Service {
	t.Helper()
	fixture := integration.StartPG(t)
	limiter := identity.NewRateLimiter(fixture.Pool.Pool, nil)
	return identity.NewService(fixture.Pool, limiter, identity.Options{RegisterPerIP: 1000}, nil)
}

// TestDeviceFlowAndRotation (D01/D02): 设备码轮询 pending→批准→发 token；
// grant 解析带 scope/项目范围；refresh 轮换；重放撤销整族；revoke 立即失效。
func TestDeviceFlowAndRotation(t *testing.T) {
	svc := newDeviceEnv(t)
	ctx := context.Background()
	user, _, err := svc.Register(ctx, "DV", "dv@dv.test", "password-dv-dv-1", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	scopes := []string{auth.ScopeProjectsRead, auth.ScopeMaterialsWrite}
	deviceCode, userCode, _, _, err := svc.StartDeviceAuthorization(ctx, "测试终端", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Before approval: pending / slow_down semantics.
	state, _, err := svc.PollDeviceToken(ctx, deviceCode)
	if err != nil || (state != 0 && state != 1) {
		t.Fatalf("pre-approval poll: state=%d err=%v", state, err)
	}
	// Scope expansion rejected.
	if err := svc.ConfirmDeviceAuthorization(ctx, user.ID, userCode, true, []string{auth.ScopeProjectsRead, "admin:all"}); errors.IsCode(err, errors.Validation) == false {
		t.Fatalf("scope expansion must be rejected, got %v", err)
	}
	// Approve narrowed scopes.
	if err := svc.ConfirmDeviceAuthorization(ctx, user.ID, userCode, true, []string{auth.ScopeProjectsRead}); err != nil {
		t.Fatal(err)
	}
	// Double confirm rejected.
	if err := svc.ConfirmDeviceAuthorization(ctx, user.ID, userCode, true, nil); errors.IsCode(err, errors.InvalidTransition) == false {
		t.Fatalf("double confirm must be invalid, got %v", err)
	}
	state, refresh, err := svc.PollDeviceToken(ctx, deviceCode)
	if err != nil || state != 4 { // PollCompleted
		t.Fatalf("poll after approve: state=%d err=%v", state, err)
	}
	if refresh.AccessToken == "" || refresh.RefreshToken == refresh.AccessToken {
		t.Fatal("completed poll must return a token")
	}
	// Grant resolves with narrowed scope only.
	principal, err := svc.ResolveGrant(ctx, refresh.AccessToken)
	if err != nil || principal.Kind != auth.KindCLI || !principal.HasScope(auth.ScopeProjectsRead) {
		t.Fatalf("grant resolve: %v %+v", err, principal)
	}
	if principal.InProjectScope(uuid.New()) {
		t.Fatal("empty grant scope was interpreted as all projects")
	}
	if principal.HasScope(auth.ScopeMaterialsWrite) {
		t.Fatal("narrowed scope must not include materials:write")
	}
	// Rotation: old token works once, replay revokes family.
	rotated, err := svc.RotateRefreshToken(ctx, refresh.RefreshToken)
	if err != nil || rotated.AccessToken == "" || rotated.RefreshToken == refresh.RefreshToken {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := svc.RotateRefreshToken(ctx, refresh.RefreshToken); errors.IsCode(err, errors.GrantRevoked) == false {
		t.Fatalf("replay must revoke family, got %v", err)
	}
	// The rotated token is dead too (family revoked).
	if _, err := svc.ResolveGrant(ctx, rotated.AccessToken); errors.IsCode(err, errors.GrantRevoked) == false {
		t.Fatalf("family revoke must kill rotated token, got %v", err)
	}
	if _, err := svc.ResolveGrant(ctx, refresh.RefreshToken); !errors.IsCode(err, errors.Unauthenticated) {
		t.Fatalf("refresh accepted as bearer: %v", err)
	}
	if _, _, err := svc.PollDeviceToken(ctx, deviceCode); err == nil {
		t.Fatal("device code minted duplicate credentials")
	}
	dc2, userCode2, _, _, err := svc.StartDeviceAuthorization(ctx, "终端2", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.ConfirmDeviceAuthorization(ctx, user.ID, userCode2, true, nil); err != nil {
		t.Fatal(err)
	}
	_, pair2, err := svc.PollDeviceToken(ctx, dc2)
	if err != nil {
		t.Fatal(err)
	}
	principal2, err := svc.ResolveGrant(ctx, pair2.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.RevokeGrant(ctx, user.ID, principal2.GrantID); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ResolveGrant(ctx, pair2.AccessToken); !errors.IsCode(err, errors.GrantRevoked) {
		t.Fatalf("revocation did not kill access: %v", err)
	}
	// Deny path.
	_, userCode3, _, _, err := svc.StartDeviceAuthorization(ctx, "终端3", scopes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmDeviceAuthorization(ctx, user.ID, userCode3, false, nil); err != nil {
		t.Fatal(err)
	}
	grants, err := svc.ListGrants(ctx, user.ID)
	if err != nil || len(grants) == 0 {
		t.Fatalf("grants list: %v %+v", err, grants)
	}
}
