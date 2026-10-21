package accountimport

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kakj-go/Judex/internal/identity"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestImportPreservesPasswordSessionAndRecoveryWithoutBusinessData(t *testing.T) {
	dsn := os.Getenv("JUDEX_V2_TEST_DSN")
	if dsn == "" {
		t.Skip("set JUDEX_V2_TEST_DSN for account import acceptance")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(ctx)
	create := func(label string) (*postgres.Pool, string) {
		name := "judex_v2_" + label + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, e = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); e != nil {
			t.Fatal(e)
		}
		u, _ := url.Parse(dsn)
		u.Path = "/" + name
		pool, e := postgres.Open(ctx, postgres.Options{URL: u.String()}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if e = pool.Migrate(ctx); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() {
			pool.Close()
			_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		})
		return pool, u.String()
	}
	source, sourceURL := create("source")
	target, targetURL := create("target")
	svc := identity.NewService(source, nil, identity.Options{}, nil)
	user, session, e := svc.Register(ctx, "Existing user", "existing@test.local", "Original-password-123", "127.0.0.1")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.IssueRecoveryCode(ctx, user.Email, "test", "account cutover"); e != nil {
		t.Fatal(e)
	}
	dc, code, _, _, e := svc.StartDeviceAuthorization(ctx, "old-cli", []string{"projects:read"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.ConfirmDeviceAuthorization(ctx, user.ID, code, true, nil); e != nil {
		t.Fatal(e)
	}
	_, oldGrant, e := svc.PollDeviceToken(ctx, dc)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = source.Exec(ctx, `INSERT INTO projects(id,name,owner_id) VALUES($1,'old business',$2)`, uuid.New(), user.ID); e != nil {
		t.Fatal(e)
	}
	// Simulate a legacy account store: migration must refuse to mutate it.
	if _, e = source.Exec(ctx, `DROP TABLE judex_schema`); e != nil {
		t.Fatal(e)
	}
	if e = source.Migrate(ctx); e == nil {
		t.Fatal("legacy database was accepted for in-place migration")
	}
	report, e := Import(ctx, sourceURL, targetURL)
	if e != nil {
		t.Fatal(e)
	}
	if len(report.Tables) != 4 {
		t.Fatal("import omitted a required account table")
	}
	restored := identity.NewService(target, nil, identity.Options{}, nil)
	resolved, e := restored.ResolveSession(ctx, session.Secret)
	if e != nil {
		t.Fatal(e)
	}
	if resolved.User.ID != user.ID || resolved.SessionID != session.SessionID || resolved.CSRFToken != session.CSRFToken {
		t.Fatal("valid web session changed")
	}
	if _, _, e = restored.Login(ctx, user.Email, "Original-password-123", "127.0.0.2"); e != nil {
		t.Fatal("password credential changed:", e)
	}
	if _, e = restored.ResolveGrant(ctx, oldGrant.AccessToken); e == nil {
		t.Fatal("old CLI grant survived cutover")
	}
	var count int
	if e = target.QueryRow(ctx, `SELECT count(*) FROM projects`).Scan(&count); e != nil || count != 0 {
		t.Fatal("business data migrated")
	}
	if _, e = Import(ctx, sourceURL, targetURL); e == nil {
		t.Fatal("nonempty target overwritten")
	}
}
