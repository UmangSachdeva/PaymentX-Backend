package router

import (
	"github.com/UmangSachdeva/PaymentX/handlers"
	"github.com/UmangSachdeva/PaymentX/middleware"
	"github.com/gorilla/mux"
)

func Router() *mux.Router {

	r := mux.NewRouter()

	r.HandleFunc("/api/v1/auth/signup", handlers.RegisterUser).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/v1/auth/login", handlers.Login).Methods("POST", "OPTIONS")

	restricted := r.PathPrefix("/").Subrouter()

	restricted.HandleFunc("/api/v1/auth/user/{id}", handlers.UpdateUser).Methods("PATCH", "OPTIONS")
	restricted.HandleFunc("/api/v1/auth/users", handlers.GetAllUsers).Methods("GET", "OPTIONS")
	restricted.HandleFunc("/api/v1/auth", handlers.GetUserDetails).Methods("GET", "OPTIONS")
	restricted.Use(middleware.AuthenticationMiddleware)

	return r
}
