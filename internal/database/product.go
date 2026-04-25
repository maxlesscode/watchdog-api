package database

import (
	"context"

	"github.com/maxlesscode/watchdog/internal/models"
)

type ProductStore interface {
	GetAllProducts(ctx context.Context) ([]models.Product, error)
	GetProductByID(ctx context.Context, id int) (models.Product, error)
	AddProduct(ctx context.Context, p models.Product) (int, error)
	UpdateProduct(ctx context.Context, id int, p models.Product) (models.Product, error)
	DeleteProduct(ctx context.Context, id int) error
	Ping(ctx context.Context) error
}

const selectColumns = "id, name, url, actual_price, target_price"

func (s *PostgresStore) GetAllProducts(ctx context.Context) ([]models.Product, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+selectColumns+" FROM products")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var productList []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.URL, &p.ActualPrice, &p.TargetPrice); err != nil {
			return productList, err
		}
		productList = append(productList, p)
	}

	return productList, rows.Err()
}

func (s *PostgresStore) GetProductByID(ctx context.Context, id int) (models.Product, error) {
	var p models.Product
	err := s.db.QueryRowContext(ctx, "SELECT "+selectColumns+" FROM products WHERE id = $1", id).
		Scan(&p.ID, &p.Name, &p.URL, &p.ActualPrice, &p.TargetPrice)
	return p, err
}

func (s *PostgresStore) AddProduct(ctx context.Context, p models.Product) (int, error) {
	query := "INSERT INTO products(name, url, actual_price, target_price) VALUES ($1, $2, $3, $4) RETURNING id"
	var returnedID int
	err := s.db.QueryRowContext(ctx, query, p.Name, p.URL, p.ActualPrice, p.TargetPrice).Scan(&returnedID)
	if err != nil {
		return -1, err
	}
	return returnedID, nil
}

func (s *PostgresStore) UpdateProduct(ctx context.Context, id int, p models.Product) (models.Product, error) {
	query := "UPDATE products SET name=$1, url=$2, actual_price=$3, target_price=$4 WHERE id=$5 RETURNING " + selectColumns
	var updated models.Product
	err := s.db.QueryRowContext(ctx, query, p.Name, p.URL, p.ActualPrice, p.TargetPrice, id).
		Scan(&updated.ID, &updated.Name, &updated.URL, &updated.ActualPrice, &updated.TargetPrice)
	if err != nil {
		return models.Product{}, err
	}
	return updated, nil
}

func (s *PostgresStore) DeleteProduct(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM products WHERE id = $1", id)
	return err
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
