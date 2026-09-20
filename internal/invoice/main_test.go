package invoice_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/pasipiya/invoice-service-quest/internal/store"
)

// db is the shared connection for tests that need real data. It is nil when no
// database is reachable, in which case those tests skip rather than fail: the
// unit tests still run, and `go test ./...` stays useful without Docker.
var db *store.Store

func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5433/invoices?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if st, err := store.Open(ctx, dsn); err == nil {
		db = st
	}
	cancel()

	code := m.Run()

	if db != nil {
		db.Close()
	}
	os.Exit(code)
}

// requireDB skips a test when no seeded database is available.
func requireDB(t *testing.T) *store.Store {
	t.Helper()
	if db == nil {
		t.Skip("no database reachable; run `make up && make seed`")
	}
	return db
}
