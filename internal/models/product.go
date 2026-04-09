package models

type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	ActualPrice float64 `json:"actual_price"`
	TargetPrice float64 `json:"target_price"`
}
