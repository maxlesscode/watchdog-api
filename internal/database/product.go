package database

import (
	"github.com/maxlesscode/watchdog/internal/models"
)

type ProductStore interface {
	GetAllProducts() ([]models.Product, error)
	GetProductByID(id int) (models.Product, error)
	AddProduct(p models.Product) (int, error)
	UpdateProduct(id int, p models.Product) (models.Product, error)
	DeleteProduct(id int) error
}

func (s *PostgresStore) GetAllProducts() ([]models.Product, error) {
	products, err := s.db.Query("SELECT * FROM products")
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

func (s *PostgresStore) GetProductByID(id int) (models.Product, error) {
	var product models.Product

	err := s.db.QueryRow("SELECT * FROM products WHERE id = $1", id).Scan(&product.ID, &product.Name, &product.URL, &product.ActualPrice, &product.TargetPrice)
	if err != nil {
		return product, err
	}

	return product, nil
}

func (s *PostgresStore) AddProduct(p models.Product) (int, error) {
	var retrunedId int
	query := "INSERT INTO products(name, url, actual_price, target_price) VALUES ($1, $2, $3, $4) RETURNING id"

	err := s.db.QueryRow(query, p.Name, p.URL, p.ActualPrice, p.TargetPrice).Scan(&retrunedId)
	if err != nil {
		return -1, err
	}

	return retrunedId, nil
}

func (s *PostgresStore) UpdateProduct(id int, p models.Product) (models.Product, error) {
	query := "UPDATE products SET (name, url, actual_price, target_price) = ($1, $2, $3, $4) WHERE id = $5"
	var updatedProduct models.Product

	_, err := s.db.Exec(query, p.Name, p.URL, p.ActualPrice, p.TargetPrice, id)
	if err != nil {
		return updatedProduct, err
	}

	return p, nil
}

func (s *PostgresStore) DeleteProduct(id int) error {
	query := "DELETE FROM products WHERE id = $1"

	_, err := s.db.Exec(query, id)
	if err != nil {
		return err
	}

	return nil
}

func ValidateProduct(p models.Product) map[string]string {
	var error map[string]string
	error = make(map[string]string)

	if len(p.Name) == 0 {
		error["name"] = "is required"
	}

	if p.TargetPrice < 0 {
		error["target_price"] = "must be a positive number"
	}

	if len(p.URL) == 0 {
		error["url"] = "is required"
	}

	return error
}
