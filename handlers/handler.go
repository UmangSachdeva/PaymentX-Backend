package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/UmangSachdeva/PaymentX/config"
	"github.com/UmangSachdeva/PaymentX/helpers"
	"github.com/UmangSachdeva/PaymentX/models"
	"github.com/UmangSachdeva/PaymentX/utils"
	cont "github.com/gorilla/context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Login(w http.ResponseWriter, r *http.Request) {
	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	collection := client.Database("paymentx").Collection("users")

	var userDB models.User

	var user models.User

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "No user found", http.StatusBadRequest)
		return
	}

	if err := collection.FindOne(context.Background(), bson.M{"email": user.Email}).Decode(&userDB); err != nil {
		http.Error(w, "No user found", http.StatusInternalServerError)
		return
	}

	password, err := helpers.Encrypt(user.Password)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if userDB.Password != password {
		http.Error(w, "Password does not match", http.StatusUnauthorized)
		return
	}

	userId := fmt.Sprintf("%v", userDB.ID)

	token, err := utils.GenerateToken(userId)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		User  interface{} `json:"user"`
		Token string      `json:"token"`
	}{
		User:  userDB,
		Token: token,
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(&response)

}

func RegisterUser(w http.ResponseWriter, r *http.Request) {
	var user models.User

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	password := user.Password

	encryptedPassword, err := helpers.Encrypt(password)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user.Password = encryptedPassword

	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	collection := client.Database("paymentx").Collection("users")

	result, err := collection.InsertOne(context.Background(), user)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userId := fmt.Sprintf("%v", result.InsertedID)

	token, err := utils.GenerateToken(userId)

	response := struct {
		InsertedID interface{} `json:"insertedId"`
		Token      string      `json:"token"`
	}{
		InsertedID: result.InsertedID,
		Token:      token,
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(&response)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	collection := client.Database("paymentx").Collection("users")
	result, err := collection.InsertOne(context.Background(), user)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(result)
}

func GetAllUsers(w http.ResponseWriter, r *http.Request) {
	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	userContext := cont.Get(r, "user")

	if userContext == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userClaims, ok := userContext.(map[string]interface{})
	if !ok {
		http.Error(w, "Invalid user context", http.StatusInternalServerError)
		return
	}

	userID, ok := userClaims["user_id"].(string)
	if !ok {
		http.Error(w, "Invalid user ID", http.StatusInternalServerError)
		return
	}

	fmt.Println("User ID:", userID)

	collection := client.Database("paymentx").Collection("users")

	cursor, err := collection.Find(context.Background(), bson.D{})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer cursor.Close(context.Background())

	var users []models.User

	for cursor.Next(context.Background()) {
		var user models.User
		if err := cursor.Decode(&user); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		users = append(users, user)
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(users)
}

func GetUserById(w http.ResponseWriter, r *http.Request) {
	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	id, err := primitive.ObjectIDFromHex(r.URL.Query().Get("id"))

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var user models.User

	collection := client.Database("paymentx").Collection("users")

	if err := collection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(user)
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {

	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	id, err := primitive.ObjectIDFromHex(r.URL.Query().Get("id"))

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	collection := client.Database("paymentx").Collection("users")

	var updatedUser models.User

	if err := json.NewDecoder(r.Body).Decode(&updatedUser); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	update := bson.M{"$set": updatedUser}

	result, err := collection.UpdateOne(context.Background(), bson.M{"_id": id}, update)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(result)
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	id, err := primitive.ObjectIDFromHex(r.URL.Query().Get("id"))

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	collection := client.Database("paymentx").Collection("users")
	result, err := collection.DeleteOne(context.Background(), bson.M{"_id": id})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	json.NewEncoder(w).Encode(result)
}

func GetUserDetails(w http.ResponseWriter, r *http.Request){
	client, err := config.ConnectToMongo();

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError);
		return;
	}

	defer client.Disconnect(context.Background());

	var userContext = cont.Get(r, "user");

	if userContext == nil {
		http.Error(w, "User Not authenticated", http.StatusUnauthorized)
		return;
	}

	userDB, err := GetUserFromContext(userContext);

	if err != nil {
		http.Error(w, "No User Found", http.StatusNotFound)
	}

	type Response struct {
		Status   string `json:"status"`
		Name string `json:"username"`
		Email    string `json:"email"`
	}

	resp := Response{
		Status: "success",
		Name: userDB.Name,
		Email: userDB.Email,
	}

	w.Header().Add("Content-type", "application/json")
	json.NewEncoder(w).Encode(resp)
}