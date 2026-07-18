package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "standard prefix", input: "0001_init.sql", want: 1},
		{name: "multi digit", input: "0012_add_index.sql", want: 12},
		{name: "no underscore", input: "init.sql", wantErr: true},
		{name: "non-numeric prefix", input: "abc_init.sql", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := migrationVersion(tt.input)

			// Assert
			if tt.wantErr {
				if err == nil {
					t.Fatalf("migrationVersion(%q) expected error, got %d", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("migrationVersion(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("migrationVersion(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadMigrationsSortedAndNonEmpty(t *testing.T) {
	// Act
	migrations, err := loadMigrations()

	// Assert
	if err != nil {
		t.Fatalf("loadMigrations() error: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least one embedded migration")
	}
	for i := 1; i < len(migrations); i++ {
		if migrations[i-1].version >= migrations[i].version {
			t.Errorf("migrations not strictly ascending: %d before %d",
				migrations[i-1].version, migrations[i].version)
		}
	}
	if migrations[0].version != 1 {
		t.Errorf("first migration version = %d, want 1", migrations[0].version)
	}
}

// TestStoreIntegration exercises the real store against Postgres. It is
// skipped unless KUBESPACES_TEST_DB_DSN is set, e.g.
// "host=localhost port=5432 dbname=kubespaces_test user=postgres password=postgres".
func TestStoreIntegration(t *testing.T) {
	dsn := os.Getenv("KUBESPACES_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("KUBESPACES_TEST_DB_DSN not set; skipping DB integration test")
	}

	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := New(pool)
	name := "it-" + time.Now().UTC().Format("20060102150405")
	t.Cleanup(func() { _ = s.HardDeleteTenant(context.Background(), name) })

	// Act + Assert: create, duplicate, get, soft-delete.
	record := TenantRecord{Name: name, Owner: "it@example.com"}
	if err := s.CreateTenant(ctx, record); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.CreateTenant(ctx, record); err != ErrConflict {
		t.Errorf("duplicate create error = %v, want ErrConflict", err)
	}
	got, err := s.GetTenant(ctx, name)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Owner != "it@example.com" {
		t.Errorf("owner = %q, want it@example.com", got.Owner)
	}
	if err := s.SoftDeleteTenant(ctx, name); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := s.GetTenant(ctx, name); err != ErrNotFound {
		t.Errorf("get after soft delete error = %v, want ErrNotFound", err)
	}
}
