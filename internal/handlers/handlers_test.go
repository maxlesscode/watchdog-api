package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/models"
)

type mockStore struct {
	getAllProductsFn  func(ctx context.Context) ([]models.Product, error)
	getProductByIDFn func(ctx context.Context, id int) (models.Product, error)
	addProductFn     func(ctx context.Context, p models.Product) (int, error)
	updateProductFn  func(ctx context.Context, id int, p models.Product) (models.Product, error)
	deleteProductFn  func(ctx context.Context, id int) error
	pingFn           func(ctx context.Context) error
}

func (m *mockStore) GetAllProducts(ctx context.Context) ([]models.Product, error) {
	return m.getAllProductsFn(ctx)
}

func (m *mockStore) GetProductByID(ctx context.Context, id int) (models.Product, error) {
	return m.getProductByIDFn(ctx, id)
}

func (m *mockStore) AddProduct(ctx context.Context, p models.Product) (int, error) {
	return m.addProductFn(ctx, p)
}

func (m *mockStore) UpdateProduct(ctx context.Context, id int, p models.Product) (models.Product, error) {
	return m.updateProductFn(ctx, id, p)
}

func (m *mockStore) DeleteProduct(ctx context.Context, id int) error {
	return m.deleteProductFn(ctx, id)
}

func (m *mockStore) Ping(ctx context.Context) error {
	return m.pingFn(ctx)
}

func (m *mockStore) UpdateActualPrice(_ context.Context, _ database.UpdateActualPriceInput) error {
	return nil
}

func (m *mockStore) InsertPriceHistory(_ context.Context, _ database.InsertPriceHistoryInput) error {
	return nil
}

func (m *mockStore) UpdateLastAlerted(_ context.Context, _ database.UpdateLastAlertedInput) error {
	return nil
}

func TestGetAllProducts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		storeFn    func(ctx context.Context) ([]models.Product, error)
		wantStatus int
		wantLen    int
	}{
		{
			name: "returns products",
			storeFn: func(ctx context.Context) ([]models.Product, error) {
				return []models.Product{
					{ID: 1, Name: "Widget", URL: "https://example.com", ActualPrice: 10, TargetPrice: 8},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantLen:    1,
		},
		{
			name: "empty list",
			storeFn: func(ctx context.Context) ([]models.Product, error) {
				return []models.Product{}, nil
			},
			wantStatus: http.StatusOK,
			wantLen:    0,
		},
		{
			name: "db error returns 500",
			storeFn: func(ctx context.Context) ([]models.Product, error) {
				return nil, sql.ErrConnDone
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &Env{DB: &mockStore{getAllProductsFn: tt.storeFn}}
			req := httptest.NewRequest(http.MethodGet, "/products", nil)
			w := httptest.NewRecorder()

			env.GetAllProducts(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				var products []models.Product
				if err := json.NewDecoder(w.Body).Decode(&products); err != nil {
					t.Fatal("decode response:", err)
				}
				if len(products) != tt.wantLen {
					t.Errorf("len = %d, want %d", len(products), tt.wantLen)
				}
			}
		})
	}
}

func TestCreateProduct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		storeFn    func(ctx context.Context, p models.Product) (int, error)
		wantStatus int
	}{
		{
			name: "valid product created",
			body: `{"name":"Widget","url":"https://example.com","actual_price":10,"target_price":8}`,
			storeFn: func(ctx context.Context, p models.Product) (int, error) {
				return 42, nil
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid json",
			body:       `{not json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing name",
			body:       `{"url":"https://example.com","actual_price":10,"target_price":8}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing url",
			body:       `{"name":"Widget","actual_price":10,"target_price":8}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "db error returns 500",
			body: `{"name":"Widget","url":"https://example.com","actual_price":10,"target_price":8}`,
			storeFn: func(ctx context.Context, p models.Product) (int, error) {
				return -1, sql.ErrConnDone
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &Env{DB: &mockStore{addProductFn: tt.storeFn}}
			req := httptest.NewRequest(http.MethodPost, "/products", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			env.CreateProduct(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestGetProductByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pathID     string
		storeFn    func(ctx context.Context, id int) (models.Product, error)
		wantStatus int
	}{
		{
			name:   "found",
			pathID: "1",
			storeFn: func(ctx context.Context, id int) (models.Product, error) {
				return models.Product{ID: 1, Name: "Widget", URL: "https://example.com"}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "not found",
			pathID: "99",
			storeFn: func(ctx context.Context, id int) (models.Product, error) {
				return models.Product{}, sql.ErrNoRows
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid id",
			pathID:     "abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "db error returns 500",
			pathID: "1",
			storeFn: func(ctx context.Context, id int) (models.Product, error) {
				return models.Product{}, sql.ErrConnDone
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &Env{DB: &mockStore{getProductByIDFn: tt.storeFn}}

			mux := http.NewServeMux()
			mux.HandleFunc("GET /products/{id}", env.GetProductByID)

			req := httptest.NewRequest(http.MethodGet, "/products/"+tt.pathID, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestUpdateProduct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pathID     string
		body       string
		storeFn    func(ctx context.Context, id int, p models.Product) (models.Product, error)
		wantStatus int
	}{
		{
			name:   "valid update",
			pathID: "1",
			body:   `{"name":"Updated","url":"https://example.com","target_price":5}`,
			storeFn: func(ctx context.Context, id int, p models.Product) (models.Product, error) {
				return models.Product{ID: id, Name: p.Name, URL: p.URL, TargetPrice: p.TargetPrice}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid json",
			pathID:     "1",
			body:       `{bad`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid id",
			pathID:     "nope",
			body:       `{"name":"Updated","url":"https://example.com","target_price":5}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "validation fail — missing name",
			pathID:     "1",
			body:       `{"url":"https://example.com","target_price":5}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "db error returns 500",
			pathID: "1",
			body:   `{"name":"Updated","url":"https://example.com","target_price":5}`,
			storeFn: func(ctx context.Context, id int, p models.Product) (models.Product, error) {
				return models.Product{}, sql.ErrConnDone
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &Env{DB: &mockStore{updateProductFn: tt.storeFn}}

			mux := http.NewServeMux()
			mux.HandleFunc("PATCH /products/{id}", env.UpdateProduct)

			req := httptest.NewRequest(http.MethodPatch, "/products/"+tt.pathID, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestDeleteProduct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pathID     string
		storeFn    func(ctx context.Context, id int) error
		wantStatus int
	}{
		{
			name:   "success",
			pathID: "1",
			storeFn: func(ctx context.Context, id int) error {
				return nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "invalid id",
			pathID:     "abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "db error returns 500",
			pathID: "1",
			storeFn: func(ctx context.Context, id int) error {
				return sql.ErrConnDone
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &Env{DB: &mockStore{deleteProductFn: tt.storeFn}}

			mux := http.NewServeMux()
			mux.HandleFunc("DELETE /products/{id}", env.DeleteProduct)

			req := httptest.NewRequest(http.MethodDelete, "/products/"+tt.pathID, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pingFn     func(ctx context.Context) error
		wantStatus int
		wantStatus_ string
	}{
		{
			name:        "db reachable",
			pingFn:      func(ctx context.Context) error { return nil },
			wantStatus:  http.StatusOK,
			wantStatus_: "up",
		},
		{
			name:        "db unreachable",
			pingFn:      func(ctx context.Context) error { return sql.ErrConnDone },
			wantStatus:  http.StatusServiceUnavailable,
			wantStatus_: "down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := &Env{DB: &mockStore{pingFn: tt.pingFn}}
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()

			env.HealthCheck(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			var body map[string]string
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatal("decode:", err)
			}
			if body["status"] != tt.wantStatus_ {
				t.Errorf("status field = %q, want %q", body["status"], tt.wantStatus_)
			}
		})
	}
}

func TestAdminScrape(t *testing.T) {
	t.Parallel()

	t.Run("nil trigger returns 503", func(t *testing.T) {
		t.Parallel()
		env := &Env{DB: &mockStore{}, TriggerScrape: nil}
		req := httptest.NewRequest(http.MethodPost, "/admin/scrape", nil)
		w := httptest.NewRecorder()

		env.AdminScrape(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", w.Code)
		}
	})

	t.Run("wired trigger returns 202 and fires", func(t *testing.T) {
		t.Parallel()
		var called atomic.Bool
		done := make(chan struct{})

		env := &Env{
			DB: &mockStore{},
			TriggerScrape: func(ctx context.Context) {
				called.Store(true)
				close(done)
			},
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/scrape", nil)
		w := httptest.NewRecorder()

		env.AdminScrape(w, req)

		if w.Code != http.StatusAccepted {
			t.Errorf("status = %d, want 202", w.Code)
		}
		<-done
		if !called.Load() {
			t.Error("TriggerScrape not called")
		}
	})
}
