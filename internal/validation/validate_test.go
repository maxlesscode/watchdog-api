package validation_test

import (
	"testing"

	"github.com/maxlesscode/watchdog/internal/models"
	"github.com/maxlesscode/watchdog/internal/validation"
)

func TestValidateProduct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		product     models.Product
		wantClean   bool
		wantErrKeys []string
	}{
		{
			name:      "valid http product",
			product:   models.Product{Name: "Widget", URL: "http://example.com", TargetPrice: 9.99},
			wantClean: true,
		},
		{
			name:      "valid https product with path and query",
			product:   models.Product{Name: "Widget", URL: "https://shop.example.com/item?id=42", TargetPrice: 0.01},
			wantClean: true,
		},
		{
			name:        "missing name",
			product:     models.Product{URL: "https://example.com", TargetPrice: 9.99},
			wantErrKeys: []string{"name"},
		},
		{
			name:        "zero target price",
			product:     models.Product{Name: "Widget", URL: "https://example.com", TargetPrice: 0},
			wantErrKeys: []string{"target_price"},
		},
		{
			name:        "negative target price",
			product:     models.Product{Name: "Widget", URL: "https://example.com", TargetPrice: -1},
			wantErrKeys: []string{"target_price"},
		},
		{
			name:        "missing url",
			product:     models.Product{Name: "Widget", TargetPrice: 9.99},
			wantErrKeys: []string{"url"},
		},
		{
			name:        "ftp scheme rejected",
			product:     models.Product{Name: "Widget", URL: "ftp://example.com/file", TargetPrice: 9.99},
			wantErrKeys: []string{"url"},
		},
		{
			name:        "relative path has no host",
			product:     models.Product{Name: "Widget", URL: "/just/a/path", TargetPrice: 9.99},
			wantErrKeys: []string{"url"},
		},
		{
			name:        "private IP 192.168.x.x rejected",
			product:     models.Product{Name: "Widget", URL: "http://192.168.1.1/product", TargetPrice: 9.99},
			wantErrKeys: []string{"url"},
		},
		{
			name:        "loopback 127.0.0.1 rejected",
			product:     models.Product{Name: "Widget", URL: "http://127.0.0.1/product", TargetPrice: 9.99},
			wantErrKeys: []string{"url"},
		},
		{
			name:        "cloud metadata 169.254.169.254 rejected",
			product:     models.Product{Name: "Widget", URL: "http://169.254.169.254/latest/meta-data/", TargetPrice: 9.99},
			wantErrKeys: []string{"url"},
		},
		{
			name:        "private 10.x.x.x rejected",
			product:     models.Product{Name: "Widget", URL: "http://10.0.0.1/api/price", TargetPrice: 9.99},
			wantErrKeys: []string{"url"},
		},
		{
			name:        "multiple errors: name and url both missing",
			product:     models.Product{TargetPrice: 9.99},
			wantErrKeys: []string{"name", "url"},
		},
		{
			name:        "all three fields invalid",
			product:     models.Product{},
			wantErrKeys: []string{"name", "url", "target_price"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := validation.ValidateProduct(tt.product)

			if tt.wantClean {
				if len(errs) > 0 {
					t.Errorf("expected no errors, got %v", errs)
				}
				return
			}

			for _, key := range tt.wantErrKeys {
				if _, ok := errs[key]; !ok {
					t.Errorf("expected error key %q, got map: %v", key, errs)
				}
			}
		})
	}
}
