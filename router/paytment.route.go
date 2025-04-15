package router

import (
	"github.com/UmangSachdeva/PaymentX/handlers"
	"github.com/UmangSachdeva/PaymentX/middleware"
	"github.com/gorilla/mux"
)

func PaymentRouter() *mux.Router {
	r := mux.NewRouter()

	r.Use(middleware.AuthenticationMiddleware)
	r.HandleFunc("/link", handlers.LinkUser).Methods("POST", "OPTIONS")

	restricted := r.PathPrefix("/").Subrouter()
	restricted.Use(middleware.AuthenticationMiddleware)
	restricted.HandleFunc("/transactions", handlers.InputTransactionData).Methods("POST", "OPTIONS")
	return r
}
