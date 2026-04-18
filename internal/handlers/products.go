package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/errors"
	"github.com/maxlesscode/watchdog/internal/models"
)

type Env struct {
	Db *sql.DB
}

func (e *Env) GetAllProducts(w http.ResponseWriter, r *http.Request) {
	slog.Info("request received", "method", "GET", "path", "/products")

	defer r.Body.Close()

	allProducts, err := database.GetAllProducts(e.Db)
	if err != nil {
		slog.Error("failed to fetch products", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.CodeDatabaseError, "failed to fetch products")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(allProducts)
	if err != nil {
		slog.Error("failed to encode json", "err", err)
	}
}

func (e *Env) CreateProduct(w http.ResponseWriter, r *http.Request) {
	slog.Info("request received", "method", "POST", "path", "/products")

	defer r.Body.Close()

	var newProduct models.Product
	err := json.NewDecoder(r.Body).Decode(&newProduct)
	details := database.ValidateProduct(newProduct)
	if err != nil {
		slog.Error("failed to decode request body", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.CodeBadRequest, "invalid request body", details)
		return
	}

	newProductID, err := database.AddProduct(e.Db, newProduct)
	if err != nil {
		slog.Error("failed to create product", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.CodeDatabaseError, "failed to create product", details)
		return
	}

	slog.Info("product created", "id", newProductID)

	newProduct.ID = newProductID
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newProduct)
}

func (e *Env) GetProductByID(w http.ResponseWriter, r *http.Request) {
	slog.Info("request received", "method", "GET", "path", "/products/{id}")

	defer r.Body.Close()

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.CodeInvalidID, "failed to parse id")
		return
	}
	product, err := database.GetProductByID(e.Db, productID)
	if err == sql.ErrNoRows {
		slog.Error("no product with id", "err", err)
		errors.SendError(w, http.StatusNotFound, errors.CodeNotFound, "no product with id")
		return
	} else if err != nil {
		slog.Error("failed to get product", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.CodeInternalError, "failed to get product")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(product)
	if err != nil {
		slog.Error("failed to encode json", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.CodeInternalError, "failed to encode json")
		return
	}
}

func (e *Env) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	slog.Info("request received", "method", "PATCH", "path", "/products/{id}")

	defer r.Body.Close()

	var updatedProduct models.Product
	err := json.NewDecoder(r.Body).Decode(&updatedProduct)
	details := database.ValidateProduct(updatedProduct)

	if err != nil {
		slog.Error("failed to decode json", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.CodeBadRequest, "invalid json body", details)
		return
	}

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.CodeInvalidID, "failed to parse id")
		return
	}

	if updatedProduct, err = database.UpdateProduct(e.Db, productID, updatedProduct); err != nil {
		slog.Error("failed to update product", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.CodeInternalError, "failed to update product", details)
		return
	}

	updatedProduct.ID = productID

	slog.Info("product updated", "id", productID)
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(updatedProduct)
	if err != nil {
		slog.Error("failed to send updated product json", "err", err)
		return
	}
}

func (e *Env) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	slog.Info("request received", "method", "DELETE", "path", "/products/{id}")

	defer r.Body.Close()

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.CodeBadRequest, "failed to convert id")
		return
	}

	err = database.DeleteProduct(e.Db, productID)
	if err != nil {
		slog.Error("failed to delete product", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.CodeDatabaseError, "failed to delete product")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (e *Env) HealthCheck(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	err := e.Db.PingContext(ctx)

	response := map[string]string{
		"status": "up",
		"time":   time.Now().Format(time.RFC3339),
	}

	if err != nil {
		response["status"] = "down"
		response["database"] = "unreachable"

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
