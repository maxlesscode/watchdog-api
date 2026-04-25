package database

import (
	"context"
	"time"

	"github.com/maxlesscode/watchdog/internal/models"
)

type ProductStore interface {
	GetAllProducts(ctx context.Context) ([]models.Product, error)
	GetProductByID(ctx context.Context, id int) (models.Product, error)
	AddProduct(ctx context.Context, p models.Product) (int, error)
	UpdateProduct(ctx context.Context, id int, p models.Product) (models.Product, error)
	DeleteProduct(ctx context.Context, id int) error
	Ping(ctx context.Context) error
	UpdateActualPrice(ctx context.Context, in UpdateActualPriceInput) error
	InsertPriceHistory(ctx context.Context, in InsertPriceHistoryInput) error
	UpdateLastAlerted(ctx context.Context, in UpdateLastAlertedInput) error
}

const selectColumns = "id, name, url, COALESCE(actual_price, 0), target_price, COALESCE(price_selector, ''), last_checked_at, last_alerted_at"

type UpdateActualPriceInput struct {
	ID        int
	Price     float64
	CheckedAt time.Time
}

type InsertPriceHistoryInput struct {
	ProductID int
	Price     float64
	CheckedAt time.Time
}

type UpdateLastAlertedInput struct {
	ID        int
	AlertedAt time.Time
}

func (s *PostgresStore) GetAllProducts(ctx context.Context) ([]models.Product, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+selectColumns+" FROM products")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var productList []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.URL, &p.ActualPrice, &p.TargetPrice, &p.PriceSelector, &p.LastCheckedAt, &p.LastAlertedAt); err != nil {
			return productList, err
		}
		productList = append(productList, p)
	}

	return productList, rows.Err()
}

func (s *PostgresStore) GetProductByID(ctx context.Context, id int) (models.Product, error) {
	var p models.Product
	err := s.db.QueryRowContext(ctx, "SELECT "+selectColumns+" FROM products WHERE id = $1", id).
		Scan(&p.ID, &p.Name, &p.URL, &p.ActualPrice, &p.TargetPrice, &p.PriceSelector, &p.LastCheckedAt, &p.LastAlertedAt)
	return p, err
}

func (s *PostgresStore) AddProduct(ctx context.Context, p models.Product) (int, error) {
	query := "INSERT INTO products(name, url, actual_price, target_price, price_selector) VALUES ($1, $2, $3, $4, $5) RETURNING id"
	var returnedID int
	err := s.db.QueryRowContext(ctx, query, p.Name, p.URL, p.ActualPrice, p.TargetPrice, p.PriceSelector).Scan(&returnedID)
	if err != nil {
		return -1, err
	}
	return returnedID, nil
}

func (s *PostgresStore) UpdateProduct(ctx context.Context, id int, p models.Product) (models.Product, error) {
	query := "UPDATE products SET name=$1, url=$2, actual_price=$3, target_price=$4, price_selector=$5 WHERE id=$6 RETURNING " + selectColumns
	var updated models.Product
	err := s.db.QueryRowContext(ctx, query, p.Name, p.URL, p.ActualPrice, p.TargetPrice, p.PriceSelector, id).
		Scan(&updated.ID, &updated.Name, &updated.URL, &updated.ActualPrice, &updated.TargetPrice, &updated.PriceSelector, &updated.LastCheckedAt, &updated.LastAlertedAt)
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

func (s *PostgresStore) UpdateActualPrice(ctx context.Context, in UpdateActualPriceInput) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE products SET actual_price = $1, last_checked_at = $2 WHERE id = $3",
		in.Price, in.CheckedAt, in.ID,
	)
	return err
}

func (s *PostgresStore) InsertPriceHistory(ctx context.Context, in InsertPriceHistoryInput) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO price_history (product_id, price, checked_at) VALUES ($1, $2, $3)",
		in.ProductID, in.Price, in.CheckedAt,
	)
	return err
}

func (s *PostgresStore) UpdateLastAlerted(ctx context.Context, in UpdateLastAlertedInput) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE products SET last_alerted_at = $1 WHERE id = $2",
		in.AlertedAt, in.ID,
	)
	return err
}
