package main

import (
	"fmt"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/handlers"
)

func main() {
	database := database.StartDB()
	defer database.Close()

	env := &handlers.Env{Db: database}

	http.HandleFunc("GET /products", env.ProductsHandler)
	http.HandleFunc("GET /products/{id}", env.GetProductByID)
	http.HandleFunc("POST /products", env.CreateProductHandler)
	http.HandleFunc("PATCH /products/{id}", env.UpdateProduct)
	http.HandleFunc("DELETE /products/{id}", env.DeleteProduct)

	fmt.Println("HTTP Server up at 'localhost:9999'")
	http.ListenAndServe(":9999", nil)
}
