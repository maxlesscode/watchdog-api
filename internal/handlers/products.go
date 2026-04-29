package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/maxlesscode/watchdog/internal/database"
	apierrors "github.com/maxlesscode/watchdog/internal/errors"
	"github.com/maxlesscode/watchdog/internal/models"
	"github.com/maxlesscode/watchdog/internal/validation"
)

const maxBodyBytes = 1 << 20 // 1 MB

type Env struct {
	DB            database.ProductStore
	TriggerScrape func(ctx context.Context)
}

func (e *Env) GetAllProducts(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	allProducts, err := e.DB.GetAllProducts(r.Context())
	if err != nil {
		slog.Error("failed to fetch products", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusInternalServerError, Tech: apierrors.CodeDatabaseError, Message: "failed to fetch products"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(allProducts); err != nil {
		slog.Error("failed to encode json", "err", err)
	}
}

func (e *Env) CreateProduct(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer func() { _ = r.Body.Close() }()

	var newProduct models.Product
	if err := json.NewDecoder(r.Body).Decode(&newProduct); err != nil {
		slog.Error("failed to decode request body", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeBadRequest, Message: "invalid request body"})
		return
	}

	if details := validation.ValidateProduct(newProduct); len(details) > 0 {
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeValidationFailed, Message: "validation failed", Details: details})
		return
	}

	newProductID, err := e.DB.AddProduct(r.Context(), newProduct)
	if err != nil {
		slog.Error("failed to create product", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusInternalServerError, Tech: apierrors.CodeDatabaseError, Message: "failed to create product"})
		return
	}

	slog.Info("product created", "id", newProductID)
	newProduct.ID = newProductID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err = json.NewEncoder(w).Encode(newProduct); err != nil {
		slog.Warn("failed to send json response", "err", err)
	}
}

func (e *Env) GetProductByID(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeInvalidID, Message: "failed to parse id"})
		return
	}

	product, err := e.DB.GetProductByID(r.Context(), productID)
	if errors.Is(err, database.ErrNotFound) {
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusNotFound, Tech: apierrors.CodeNotFound, Message: "no product with id"})
		return
	} else if err != nil {
		slog.Error("failed to get product", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusInternalServerError, Tech: apierrors.CodeInternalError, Message: "failed to get product"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(product); err != nil {
		slog.Error("failed to encode json", "err", err)
	}
}

// UpdateProduct replaces all fields of a product. All fields are required (PUT semantics).
func (e *Env) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer func() { _ = r.Body.Close() }()

	var updatedProduct models.Product
	if err := json.NewDecoder(r.Body).Decode(&updatedProduct); err != nil {
		slog.Error("failed to decode json", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeBadRequest, Message: "invalid json body"})
		return
	}

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeInvalidID, Message: "failed to parse id"})
		return
	}

	if details := validation.ValidateProduct(updatedProduct); len(details) > 0 {
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeBadRequest, Message: "invalid json body", Details: details})
		return
	}

	updatedProduct, err = e.DB.UpdateProduct(r.Context(), productID, updatedProduct)
	if errors.Is(err, database.ErrNotFound) {
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusNotFound, Tech: apierrors.CodeNotFound, Message: "no product with id"})
		return
	} else if err != nil {
		slog.Error("failed to update product", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusInternalServerError, Tech: apierrors.CodeInternalError, Message: "failed to update product"})
		return
	}

	slog.Info("product updated", "id", productID)
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(updatedProduct); err != nil {
		slog.Error("failed to send updated product json", "err", err)
	}
}

func (e *Env) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeBadRequest, Message: "failed to convert id"})
		return
	}

	if err = e.DB.DeleteProduct(r.Context(), productID); errors.Is(err, database.ErrNotFound) {
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusNotFound, Tech: apierrors.CodeNotFound, Message: "no product with id"})
		return
	} else if err != nil {
		slog.Error("failed to delete product", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusInternalServerError, Tech: apierrors.CodeDatabaseError, Message: "failed to delete product"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetPriceHistory returns the scrape price history for a product, newest first.
func (e *Env) GetPriceHistory(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusBadRequest, Tech: apierrors.CodeInvalidID, Message: "failed to parse id"})
		return
	}

	history, err := e.DB.GetPriceHistory(r.Context(), productID)
	if errors.Is(err, database.ErrNotFound) {
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusNotFound, Tech: apierrors.CodeNotFound, Message: "no product with id"})
		return
	} else if err != nil {
		slog.Error("failed to fetch price history", "err", err)
		apierrors.SendError(r.Context(), w, apierrors.ErrorInput{Code: http.StatusInternalServerError, Tech: apierrors.CodeDatabaseError, Message: "failed to fetch price history"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(history); err != nil {
		slog.Error("failed to encode json", "err", err)
	}
}

func (e *Env) HealthCheck(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()

	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	response := map[string]string{
		"status": "up",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := e.DB.Ping(ctx); err != nil {
		response["status"] = "down"
		response["database"] = "unreachable"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Warn("failed to encode json", "err", err)
	}
}
