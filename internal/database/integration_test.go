//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/models"
)

var (
	testDB    *sql.DB
	testStore *database.PostgresStore
)

func TestMain(m *testing.M) {
	testURL := os.Getenv("TEST_DB_URL")
	if testURL == "" {
		fmt.Fprintln(os.Stderr, "skipping integration tests: TEST_DB_URL not set")
		os.Exit(0)
	}

	var err error
	testDB, err = sql.Open("postgres", testURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	if err = testDB.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ping db: %v\n", err)
		os.Exit(1)
	}
	if err = database.MigrateDB(testDB); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}

	testStore = database.NewPostgresStore(testDB)
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func truncate(t *testing.T) {
	t.Helper()
	if _, err := testDB.Exec("TRUNCATE products, price_history RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestAddAndGetProduct(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { truncate(t) })

	ctx := context.Background()
	p := models.Product{Name: "Keyboard", URL: "https://shop.example.com/kb", TargetPrice: 80}

	id, err := testStore.AddProduct(ctx, p)
	if err != nil {
		t.Fatalf("AddProduct: %v", err)
	}
	if id <= 0 {
		t.Fatalf("got id %d, want > 0", id)
	}

	got, err := testStore.GetProductByID(ctx, id)
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	if got.Name != p.Name || got.URL != p.URL || got.TargetPrice != p.TargetPrice {
		t.Errorf("got %+v, want name=%s url=%s target=%v", got, p.Name, p.URL, p.TargetPrice)
	}
}

func TestGetAllProducts_Integration(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { truncate(t) })

	ctx := context.Background()
	for _, name := range []string{"A", "B", "C"} {
		if _, err := testStore.AddProduct(ctx, models.Product{Name: name, URL: "https://example.com/" + name, TargetPrice: 10}); err != nil {
			t.Fatalf("AddProduct %s: %v", name, err)
		}
	}

	products, err := testStore.GetAllProducts(ctx)
	if err != nil {
		t.Fatalf("GetAllProducts: %v", err)
	}
	if len(products) != 3 {
		t.Errorf("len = %d, want 3", len(products))
	}
}

func TestUpdateProduct_Integration(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { truncate(t) })

	ctx := context.Background()
	id, _ := testStore.AddProduct(ctx, models.Product{Name: "Old", URL: "https://example.com", TargetPrice: 50})

	updated, err := testStore.UpdateProduct(ctx, id, models.Product{Name: "New", URL: "https://example.com/new", TargetPrice: 40})
	if err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}
	if updated.Name != "New" || updated.TargetPrice != 40 {
		t.Errorf("updated = %+v, want name=New target=40", updated)
	}
}

func TestDeleteProduct_Integration(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { truncate(t) })

	ctx := context.Background()
	id, _ := testStore.AddProduct(ctx, models.Product{Name: "ToDelete", URL: "https://example.com", TargetPrice: 10})

	if err := testStore.DeleteProduct(ctx, id); err != nil {
		t.Fatalf("DeleteProduct: %v", err)
	}
	if _, err := testStore.GetProductByID(ctx, id); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestUpdateActualPrice_Integration(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { truncate(t) })

	ctx := context.Background()
	id, _ := testStore.AddProduct(ctx, models.Product{Name: "P", URL: "https://example.com", TargetPrice: 100})

	now := time.Now().UTC().Truncate(time.Second)
	if err := testStore.UpdateActualPrice(ctx, database.UpdateActualPriceInput{ID: id, Price: 79.99, CheckedAt: now}); err != nil {
		t.Fatalf("UpdateActualPrice: %v", err)
	}

	got, _ := testStore.GetProductByID(ctx, id)
	if got.ActualPrice != 79.99 {
		t.Errorf("actual_price = %v, want 79.99", got.ActualPrice)
	}
	if got.LastCheckedAt == nil {
		t.Error("last_checked_at is nil")
	}
}

func TestInsertPriceHistory_Integration(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { truncate(t) })

	ctx := context.Background()
	id, _ := testStore.AddProduct(ctx, models.Product{Name: "P", URL: "https://example.com", TargetPrice: 100})

	err := testStore.InsertPriceHistory(ctx, database.InsertPriceHistoryInput{
		ProductID: id, Price: 55.0, CheckedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("InsertPriceHistory: %v", err)
	}
}

func TestUpdateLastAlerted_Integration(t *testing.T) {
	truncate(t)
	t.Cleanup(func() { truncate(t) })

	ctx := context.Background()
	id, _ := testStore.AddProduct(ctx, models.Product{Name: "P", URL: "https://example.com", TargetPrice: 100})

	now := time.Now().UTC().Truncate(time.Second)
	if err := testStore.UpdateLastAlerted(ctx, database.UpdateLastAlertedInput{ID: id, AlertedAt: now}); err != nil {
		t.Fatalf("UpdateLastAlerted: %v", err)
	}

	got, _ := testStore.GetProductByID(ctx, id)
	if got.LastAlertedAt == nil {
		t.Error("last_alerted_at is nil after update")
	}
}
