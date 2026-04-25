package notifier

import (
	"context"

	"github.com/maxlesscode/watchdog/internal/models"
)

// Notifier sends an alert when a product's price reaches its target.
type Notifier interface {
	Notify(ctx context.Context, p models.Product) error
}
