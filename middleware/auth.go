package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/context"

	"github.com/UmangSachdeva/PaymentX/utils"
)

// Context key for user data
type contextKey string

const UserContextKey contextKey = "user"

func AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			http.Error(w, "Missing Authorization header", http.StatusBadRequest)
			return
		}

		tokenParts := strings.Split(tokenString, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header", http.StatusBadRequest)
			return
		}

		tokenString = tokenParts[1]

		claims, err := utils.VerifyToken(tokenString)

		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		fmt.Println(claims)

		// add user to the context
		context.Set(r, "user", claims)
		// ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r)
	})
}
