package validation

import "github.com/maxlesscode/watchdog/internal/models"

func ValidateProduct(p models.Product) map[string]string {
	error := make(map[string]string)

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
