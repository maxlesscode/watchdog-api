package database

import (
	"database/sql"

	"github.com/maxlesscode/watchdog/internal/models"
)

func GetAllProducts(db *sql.DB) ([]models.Product, error) {
	products, err := db.Query("SELECT * FROM products")
	if err != nil {
		return nil, err
	}
	defer products.Close()

	var products_list []models.Product

	for products.Next() {
		var product models.Product
		if err := products.Scan(&product.ID, &product.Name, &product.URL, &product.ActualPrice, &product.TargetPrice); err != nil {
			return products_list, err
		}
		products_list = append(products_list, product)
	}

	return products_list, nil
}

func GetProductByID(db *sql.DB, id int) (models.Product, error) {
	var product models.Product

	err := db.QueryRow("SELECT * FROM products WHERE id = $1", id).Scan(&product.ID, &product.Name, &product.URL, &product.ActualPrice, &product.TargetPrice)
	if err != nil {
		return product, err
	}

	return product, nil
}

func AddProduct(db *sql.DB, p models.Product) int {
	var retrunedId int
	query := "INSERT INTO products(name, url, actual_price, target_price) VALUES ($1, $2, $3, $4) RETURNING id"

	_ = db.QueryRow(query, p.Name, p.URL, p.ActualPrice, p.TargetPrice).Scan(&retrunedId)

	return retrunedId
}

func UpdateProduct(db *sql.DB, id int, p models.Product) (models.Product, error) {
	query := "UPDATE products SET (name, url, actual_price, target_price) = ($1, $2, $3, $4) WHERE id = $5"
	var updatedProduct models.Product

	_, err := db.Exec(query, p.Name, p.URL, p.ActualPrice, p.TargetPrice, id)
	if err != nil {
		return updatedProduct, err
	}

	return p, nil
}

func DeleteProduct(db *sql.DB, id int) error {
	query := "DELETE FROM products WHERE id = $1"

	_, err := db.Exec(query, id)
	if err != nil {
		return err
	}

	return nil
}
