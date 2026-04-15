package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

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
		errors.SendError(w, http.StatusInternalServerError, errors.APIError{
			Message: "failed to fetch products",
			Code:    errors.CodeDatabaseError,
		})
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
	if err != nil {
		slog.Error("failed to decode request body", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.APIError{
			Message: "invalid request body",
			Code:    errors.CodeBadRequest,
		})
		return
	}

	newProductID, err := database.AddProduct(e.Db, newProduct)
	if err != nil {
		slog.Error("failed to create product", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.APIError{
			Message: "failed to create product",
			Code:    errors.CodeDatabaseError,
		})
		return
	}

	slog.Info("product created", "id", newProductID)

	w.WriteHeader(http.StatusCreated)
}

func (e *Env) GetProductByID(w http.ResponseWriter, r *http.Request) {
	slog.Info("request received", "method", "GET", "path", "/products/{id}")

	defer r.Body.Close()

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.APIError{
			Message: "failed to parse id",
			Code:    errors.CodeInvalidID,
		})
		return
	}
	product, err := database.GetProductByID(e.Db, productID)
	if err != nil {
		slog.Error("failed to get product", "err", err)
		errors.SendError(w, http.StatusNotFound, errors.APIError{
			Message: "wrong id requested",
			Code:    errors.CodeNotFound,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(product)
	if err != nil {
		slog.Error("failed to encode json", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.APIError{
			Message: "failed to encode json",
			Code:    errors.CodeInternalError,
		})
		return
	}
}

func (e *Env) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	slog.Info("request received", "method", "PATCH", "path", "/products/{id}")

	defer r.Body.Close()

	var updatedProduct models.Product
	err := json.NewDecoder(r.Body).Decode(&updatedProduct)
	if err != nil {
		slog.Error("failed to decode json", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.APIError{
			Message: "invalid json body",
			Code:    errors.CodeBadRequest,
		})
		return
	}

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		slog.Error("failed to parse id", "err", err)
		errors.SendError(w, http.StatusBadRequest, errors.APIError{
			Message: "failed to parse id",
			Code:    errors.CodeInvalidID,
		})
		return
	}

	if updatedProduct, err = database.UpdateProduct(e.Db, productID, updatedProduct); err != nil {
		slog.Error("failed to update product", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.APIError{
			Message: "failed to update product",
			Code:    errors.CodeInternalError,
		})
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
		errors.SendError(w, http.StatusBadRequest, errors.APIError{
			Message: "failed to convert id",
			Code:    errors.CodeInvalidID,
		})
		return
	}

	err = database.DeleteProduct(e.Db, productID)
	if err != nil {
		slog.Error("failed to delete product", "err", err)
		errors.SendError(w, http.StatusInternalServerError, errors.APIError{
			Message: "failed to delete product",
			Code:    errors.CodeDatabaseError,
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
