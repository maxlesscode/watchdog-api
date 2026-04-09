package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/models"
)

type Env struct {
	Db *sql.DB
}

func (e *Env) ProductsHandler(w http.ResponseWriter, r *http.Request) {
	allProducts, err := database.GetAllProducts(e.Db)
	if err != nil {
		log.Println("Can't get products list: ", err)
		http.Error(w, "Can't get products list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(allProducts)
	if err != nil {
		http.Error(w, "Can't encode json", http.StatusInternalServerError)
	}
}

func (e *Env) CreateProductHandler(w http.ResponseWriter, r *http.Request) {
	var newProduct models.Product
	err := json.NewDecoder(r.Body).Decode(&newProduct)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	newProductID := database.AddProduct(e.Db, newProduct)

	log.Printf("Product created with ID: %d", newProductID)
	w.WriteHeader(http.StatusCreated)
}

func (e *Env) GetProductByID(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Can't convert ID", http.StatusInternalServerError)
	}
	product, err := database.GetProductByID(e.Db, productID)
	if err != nil {
		http.Error(w, "No product with this ID", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(product)
	if err != nil {
		http.Error(w, "Can't encode json", http.StatusInternalServerError)
	}
}

func (e *Env) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	var updatedProduct models.Product
	err := json.NewDecoder(r.Body).Decode(&updatedProduct)
	if err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
	}
	defer r.Body.Close()

	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Can't convert ID", http.StatusInternalServerError)
	}

	if updatedProduct, err = database.UpdateProduct(e.Db, productID, updatedProduct); err != nil {
		http.Error(w, "Can't Update product", http.StatusInternalServerError)
	}
	updatedProduct.ID = productID
	log.Printf("Product at ID %d updated!", productID)
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(updatedProduct)
}

func (e *Env) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	productID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Can't convert ID", http.StatusInternalServerError)
	}

	database.DeleteProduct(e.Db, productID)

	w.WriteHeader(http.StatusNoContent)
}
