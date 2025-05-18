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
	restricted.HandleFunc("/transactions", handlers.GetUserTransaction).Methods("GET", "OPTIONS")
	restricted.HandleFunc("/transactions/analysis", handlers.GetTransactionAnalysis).Methods("GET", "OPTIONS")
	restricted.HandleFunc("/transactions/monthly", handlers.GetMonthlyTransactions).Methods("OPTIONS", "GET")
	restricted.HandleFunc("/transactions/average", handlers.GetUserAverageMonthlySpend).Methods("OPTIONS", "GET")
	restricted.HandleFunc("/transactions/pattern", handlers.MonthlyWeeklyPattern).Methods("OPTIONS", "GET")
	restricted.HandleFunc("/transactions/time", handlers.GetSpendingTimeAnalysis).Methods("OPTIONS", "GET")
	restricted.HandleFunc("/transactions/debitvscredit", handlers.GetDebitVsCredit).Methods("OPTIONS", "GET")
	return r
}
