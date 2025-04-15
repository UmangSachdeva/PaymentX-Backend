package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/UmangSachdeva/PaymentX/config"
	"github.com/UmangSachdeva/PaymentX/models"
	"github.com/dgrijalva/jwt-go"
	cont "github.com/gorilla/context"
	"github.com/plaid/plaid-go/v32/plaid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetUserFromContext(userContext interface{}) (models.User, error) {
	mongoClient, err := config.ConnectToMongo()
	var userDB models.User

	// userContext := cont.Get(r, "user")

	if userContext == nil {

		return userDB, fmt.Errorf("Unauthorized")
	}

	userClaims, ok := userContext.(jwt.MapClaims)
	if !ok {
		return userDB, fmt.Errorf("Invalid user context")
	}

	userIDStr, ok := userClaims["user_id"].(string)
	if !ok {
		return userDB, fmt.Errorf("Invalid user ID")
	}

	defer mongoClient.Disconnect(context.Background())

	collection := mongoClient.Database("paymentx").Collection("users")

	// Extract the hex ID from the string
	hexID := strings.TrimPrefix(userIDStr, "ObjectID(\"")
	hexID = strings.TrimSuffix(hexID, "\")")

	// Convert to primitive.ObjectID
	objectID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		return userDB, fmt.Errorf("Invalid ObjectID format")
	}

	// var user models.User
	if err := collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&userDB); err != nil {
		return userDB, fmt.Errorf("No user found")
	}

	return userDB, nil
}

func LinkUser(w http.ResponseWriter, r *http.Request) {
	mongoClient, err := config.ConnectToMongo()
	// plaidClient := config.PlaidInit()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userContext := cont.Get(r, "user")

	if userContext == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userClaims, ok := userContext.(jwt.MapClaims)
	if !ok {
		http.Error(w, "Invalid user context", http.StatusInternalServerError)
		return
	}

	userIDStr, ok := userClaims["user_id"].(string)
	if !ok {
		http.Error(w, "Invalid user ID", http.StatusInternalServerError)
		return
	}

	defer mongoClient.Disconnect(context.Background())

	collection := mongoClient.Database("paymentx").Collection("users")

	var userDB models.User

	// Extract the hex ID from the string
	hexID := strings.TrimPrefix(userIDStr, "ObjectID(\"")
	hexID = strings.TrimSuffix(hexID, "\")")

	// Convert to primitive.ObjectID
	objectID, err := primitive.ObjectIDFromHex(hexID)
	if err != nil {
		http.Error(w, "Invalid ObjectID format", http.StatusBadRequest)
		return
	}

	// var user models.User
	if err := collection.FindOne(context.Background(), bson.M{"_id": objectID}).Decode(&userDB); err != nil {
		http.Error(w, "No user found", http.StatusInternalServerError)
		return
	}

	client := config.PlaidInit()

	plaidUser := plaid.LinkTokenCreateRequestUser{
		ClientUserId: userDB.ID.String(),
	}

	// Create a link_token for the given user
	request := plaid.NewLinkTokenCreateRequest("Plaid", "en", []plaid.CountryCode{plaid.COUNTRYCODE_US}, plaidUser)

	request.SetRedirectUri("http://localhost:5173")

	request.SetProducts([]plaid.Products{plaid.PRODUCTS_AUTH})

	resp, _, err := client.PlaidApi.LinkTokenCreate(context.Background()).LinkTokenCreateRequest(*request).Execute()

	if err != nil {
		if plaidErr, ok := err.(plaid.GenericOpenAPIError); ok {
			fmt.Printf("Plaid API error: %s\n", string(plaidErr.Body()))
		}
		http.Error(w, "Failed to create link token: "+err.Error(), http.StatusBadRequest)
		return
	}

	linkToken := resp.GetLinkToken()

	response := struct {
		Status    string `json:"status"`
		LinkToken string `json:"link_token"`
	}{
		Status:    "success",
		LinkToken: linkToken,
	}

	json.NewEncoder(w).Encode(response)
}

func InputTransactionData(w http.ResponseWriter, r *http.Request) {
	userContext := cont.Get(r, "user")

	userDB, err := GetUserFromContext(userContext)

	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var transactionsArr []models.Transaction

	if err := json.NewDecoder(r.Body).Decode(&transactionsArr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var transactionInterface []interface{}

	for i := range transactionsArr {
		transactionsArr[i].UserID = userDB.ID
		transactionInterface = append(transactionInterface, transactionsArr[i])
	}

	client, err := config.ConnectToMongo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	collection := client.Database("paymentx").Collection("transactions")
	_, err = collection.InsertMany(context.Background(), transactionInterface)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Status:  "success",
		Message: "Transaction Added Successfully",
	}

	json.NewEncoder(w).Encode(response)
}
