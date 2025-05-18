package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/UmangSachdeva/PaymentX/middleware"
	"github.com/UmangSachdeva/PaymentX/router"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("Hello, World!")

	// Load Env file
	godotenv.Load(".env")

	r := router.Router()
	paymentRouter := router.PaymentRouter()

	r.Use(middleware.CORSMiddleware)

	r.PathPrefix("/").Handler(http.StripPrefix("/api/v1/payments", paymentRouter))

	log.Fatal(http.ListenAndServe(":5001", r))

	fmt.Println("Listening on port 5001")
}
