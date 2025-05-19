package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/UmangSachdeva/PaymentX/config"
	"github.com/UmangSachdeva/PaymentX/helpers"
	"github.com/UmangSachdeva/PaymentX/models"
	"github.com/dgrijalva/jwt-go"
	cont "github.com/gorilla/context"
	"github.com/plaid/plaid-go/v32/plaid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func generateHash(txn models.Transaction) string {
	key := fmt.Sprintf("%s|%s|%s|%s|%s",
		txn.TransactionDate.Time().Format(time.RFC3339),
		fmt.Sprintf("%v", txn.Amount),
		txn.Details,
		txn.TransactionTime,
		txn.UserID,
	)
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}


func convertStringToDateTime(dateStr string) (primitive.DateTime, error) {
	if dateStr == "" {
		todaysDate := time.Now()
		return primitive.NewDateTimeFromTime(todaysDate), nil
	}

	// Parse the date string into a time.Time object
	date, err := time.Parse("2006-01-02", dateStr)

	if err != nil {
		todaysDate := time.Now()
		return primitive.NewDateTimeFromTime(todaysDate), err
	}

	// Convert the time.Time object to a MongoDB DateTime
	return primitive.NewDateTimeFromTime(date), nil
}

func absInt(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func GetUserFromContext(userContext interface{}) (models.User, error) {
	mongoClient, _ := config.ConnectToMongo()

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
		transactionsArr[i].TransactionID = generateHash(transactionsArr[i])
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
	// Insert many, skip duplicates based on TransactionID (unique index recommended on TransactionID)
	opts := options.InsertMany().SetOrdered(false)
	_, err = collection.InsertMany(context.Background(), transactionInterface, opts)
	if err != nil {
		// If error is due to duplicate key, ignore it (continue)
		if we, ok := err.(mongo.BulkWriteException); ok {
			// Check if all errors are duplicate key errors
			allDup := true
			for _, writeErr := range we.WriteErrors {
				if writeErr.Code != 11000 {
					allDup = false
					break
				}
			}
			if allDup {
				err = nil // ignore duplicate key errors
			}
		}
	}

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

func GetUserTransaction(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Getting User Transactions.....")
	userContext := cont.Get(r, "user")

	userDB, err := GetUserFromContext(userContext)

	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	client, err := config.ConnectToMongo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	page := r.URL.Query().Get("page")
	if page == "" {
		page = "0"
	}
	pageInt, err := strconv.Atoi(page)

	if err != nil {
		http.Error(w, "Invalid page number", http.StatusBadRequest)
		return
	}

	limit := r.URL.Query().Get("limit")

	if limit == "" {
		limit = "10"
	}

	limitInt, err := strconv.Atoi(limit)
	fmt.Println(limitInt)
	fmt.Println(pageInt)

	if err != nil {
		http.Error(w, "Invalid limit number", http.StatusBadRequest)
		return
	}

	sort := map[string]interface{}{
		"transactiondate": -1,
	}

	collection := client.Database("paymentx").Collection("transactions")
	cursor, err := collection.Find(context.Background(), bson.M{"user_id": userDB.ID}, helpers.NewMongoPaginate(absInt(int64(limitInt)), absInt(int64(pageInt)), sort).GetPaginatedOpts().SortQuery(sort).BuildFindOptions())

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get total count of transactions
	count, err := collection.CountDocuments(context.Background(), bson.M{"user_id": userDB.ID})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var transactions []models.Transaction
	if err = cursor.All(context.Background(), &transactions); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Data  []models.Transaction `json:"data"`
		Total int                  `json:"total"`
		Page  int                  `json:"page"`
		Limit int                  `json:"limit"`
	}{
		Data:  transactions,
		Total: int(count),
		Page:  pageInt,
		Limit: limitInt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetTransactionAnalysis(w http.ResponseWriter, r *http.Request) {
	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	}

	startDate, err := convertStringToDateTime(r.URL.Query().Get("start_date"))

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	endDate, err := convertStringToDateTime(r.URL.Query().Get("end_date"))

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer client.Disconnect(context.Background())

	userContext := cont.Get(r, "user")
	userDB, err := GetUserFromContext(userContext)

	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Build Filter Options
	opts := options.Find().SetSort(bson.M{"transactiondate": 1})

	// Set the date range filter
	filter := bson.M{}
	daterange := bson.M{}

	if r.URL.Query().Get("start_date") != "" {
		daterange["$gte"] = startDate
		filter["transactiondate"] = daterange
	}

	if r.URL.Query().Get("end_date") != "" {
		daterange["$lte"] = endDate
		filter["transactiondate"] = daterange
	}

	if r.URL.Query().Get("type") != "" {
		filter["type"] = r.URL.Query().Get("type")
	}

	filter["user_id"] = userDB.ID

	fmt.Println(bson.M{"filter": filter})	

	collection := client.Database("paymentx").Collection("transactions")
	cursor, err := collection.Find(context.Background(), filter, opts)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer cursor.Close(context.Background())
	var transactions []models.Transaction
	if err = cursor.All(context.Background(), &transactions); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Total int                  `json:"total"`
		Data  []models.Transaction `json:"data"`
	}{
		Total: len(transactions),
		Data:  transactions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func GetMonthlyTransactions(w http.ResponseWriter, r *http.Request){
	client, err := config.ConnectToMongo();

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return;
	}

	defer client.Disconnect(context.Background())

	userContext := cont.Get(r, "user")
	userDB, err := GetUserFromContext(userContext)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	collection := client.Database("paymentx").Collection("transactions")

	tp := r.URL.Query().Get("type")
	if tp == "" {
		tp = "DEBIT"
	}

	
	// Group by year and month extracted from transactiondate (which is a date/time field)
	pipeline := bson.A{
		bson.M{"$match": bson.M{"user_id": userDB.ID, "type": tp}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$transactiondate"},
				"month": bson.M{"$month": "$transactiondate"},
			},
			"total_spend": bson.M{"$sum": "$amount"},
		}},
		bson.M{"$sort": bson.M{"_id.year": 1, "_id.month": 1}},
	}

	cursor, err := collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	type MonthlySpend struct {
		Year       int     `json:"year"`
		Month      int     `json:"month"`
		TotalSpend float64 `json:"total_spend"`
	}

	var results []MonthlySpend
	for cursor.Next(context.Background()) {
		var doc struct {
			ID struct {
				Year  int `bson:"year"`
				Month int `bson:"month"`
			} `bson:"_id"`
			TotalSpend float64 `bson:"total_spend"`
		}
		if err := cursor.Decode(&doc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, MonthlySpend{
			Year:       doc.ID.Year,
			Month:      doc.ID.Month,
			TotalSpend: doc.TotalSpend,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func GetUserAverageMonthlySpend(w http.ResponseWriter, r *http.Request){
	client, err := config.ConnectToMongo()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	defer client.Disconnect(context.Background())

	userContext := cont.Get(r, "user")
	userDB, err := GetUserFromContext(userContext)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	collection := client.Database("paymentx").Collection("transactions")

	// Parse year and month from query params, default to current if not provided
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	if yearStr == "" || monthStr == "" {
		now := time.Now()
		if yearStr == "" {
			yearStr = fmt.Sprintf("%d", now.Year())
		}
		if monthStr == "" {
			monthStr = fmt.Sprintf("%d", int(now.Month()))
		}
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "invalid year", http.StatusBadRequest)
		return
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		http.Error(w, "invalid month", http.StatusBadRequest)
		return
	}

	tp := r.URL.Query().Get("type")
	if tp == "" {
		tp = "DEBIT"
	}

	// Pipeline for current month
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": userDB.ID,
			"type":    tp,
			"$expr": bson.M{
				"$and": bson.A{
					bson.M{"$eq": bson.A{bson.M{"$year": "$transactiondate"}, year}},
					bson.M{"$eq": bson.A{bson.M{"$month": "$transactiondate"}, month}},
				},
			},
		}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$transactiondate"},
				"month": bson.M{"$month": "$transactiondate"},
				"day":   bson.M{"$dayOfMonth": "$transactiondate"},
			},
			"daily_spend": bson.M{"$sum": "$amount"},
		}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"year":  "$_id.year",
				"month": "$_id.month",
			},
			"average_daily_spend": bson.M{"$avg": "$daily_spend"},
		}},
	}

	cursor, err := collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	var currentMonthAvg float64
	for cursor.Next(context.Background()) {
		var doc struct {
			ID struct {
				Year  int `bson:"year"`
				Month int `bson:"month"`
			} `bson:"_id"`
			AverageDailySpend float64 `bson:"average_daily_spend"`
		}
		if err := cursor.Decode(&doc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		currentMonthAvg = doc.AverageDailySpend
	}

	// Pipeline for previous month
	prevYear := year
	prevMonth := month - 1
	if prevMonth == 0 {
		prevMonth = 12
		prevYear = year - 1
	}
	prevPipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": userDB.ID,
			"type":    tp,
			"$expr": bson.M{
				"$and": bson.A{
					bson.M{"$eq": bson.A{bson.M{"$year": "$transactiondate"}, prevYear}},
					bson.M{"$eq": bson.A{bson.M{"$month": "$transactiondate"}, prevMonth}},
				},
			},
		}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": "$transactiondate"},
				"month": bson.M{"$month": "$transactiondate"},
				"day":   bson.M{"$dayOfMonth": "$transactiondate"},
			},
			"daily_spend": bson.M{"$sum": "$amount"},
		}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"year":  "$_id.year",
				"month": "$_id.month",
			},
			"average_daily_spend": bson.M{"$avg": "$daily_spend"},
		}},
	}

	prevCursor, err := collection.Aggregate(context.Background(), prevPipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer prevCursor.Close(context.Background())

	var prevMonthAvg float64
	for prevCursor.Next(context.Background()) {
		var doc struct {
			ID struct {
				Year  int `bson:"year"`
				Month int `bson:"month"`
			} `bson:"_id"`
			AverageDailySpend float64 `bson:"average_daily_spend"`
		}
		if err := prevCursor.Decode(&doc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		prevMonthAvg = doc.AverageDailySpend
	}

	percentageChange := 0.0
	if prevMonthAvg != 0 {
		percentageChange = ((currentMonthAvg - prevMonthAvg) / prevMonthAvg) * 100
	}

	type Result struct {
		Year              int     `json:"year"`
		Month             int     `json:"month"`
		AverageDailySpend float64 `json:"average_daily_spend"`
		PercentageChange  float64 `json:"percentage_change"`
	}

	result := Result{
		Year:              year,
		Month:             month,
		AverageDailySpend: currentMonthAvg,
		PercentageChange:  percentageChange,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func MonthlyWeeklyPattern(w http.ResponseWriter, r *http.Request){
	client, err := config.ConnectToMongo();

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return;
	}

	defer client.Disconnect(context.Background())

	userContext := cont.Get(r, "user")
	userDB, err := GetUserFromContext(userContext)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	collection := client.Database("paymentx").Collection("transactions")

	// Parse year and month from query params, default to current if not provided
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	now := time.Now()
	if yearStr == "" {
		yearStr = fmt.Sprintf("%d", now.Year())
	}
	if monthStr == "" {
		monthStr = fmt.Sprintf("%d", int(now.Month()))
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "invalid year", http.StatusBadRequest)
		return
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		http.Error(w, "invalid month", http.StatusBadRequest)
		return
	}

	tp := r.URL.Query().Get("type")
	if tp == "" {
		tp = "DEBIT"
	}

	// Group by year, month, week extracted from transactiondate, filter by year and month
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": userDB.ID,
			"type":    tp,
			"$expr": bson.M{
				"$and": bson.A{
					bson.M{"$eq": bson.A{bson.M{"$year": "$transactiondate"}, year}},
					bson.M{"$eq": bson.A{bson.M{"$month": "$transactiondate"}, month}},
				},
			},
		}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"year":      bson.M{"$year": "$transactiondate"},
            "month":     bson.M{"$month": "$transactiondate"},
				"dayOfWeek": bson.M{"$dayOfWeek": "$transactiondate"},
			},
			"total_spend": bson.M{"$sum": "$amount"},
		}},
		bson.M{"$sort": bson.M{"_id.dayOfWeek": 1}},
	}

	cursor, err := collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	type WeeklySpend struct {
		Year         int               `json:"year"`
		Month        int               `json:"month"`
		DayOfWeek    int               `json:"day_of_week"`
		TotalSpend   float64           `json:"total_spend"`
	}

	var results []WeeklySpend
	for cursor.Next(context.Background()) {
		var doc struct {
			ID struct {
				Year      int `bson:"year"`
				Month     int `bson:"month"`
				DayOfWeek int `bson:"dayOfWeek"`
			} `bson:"_id"`
			TotalSpend   float64           `bson:"total_spend"`
	
		}
		if err := cursor.Decode(&doc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, WeeklySpend{
			Year:         doc.ID.Year,
			Month:        doc.ID.Month,
			DayOfWeek:    doc.ID.DayOfWeek,
			TotalSpend:   doc.TotalSpend,

		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)

}

func GetSpendingTimeAnalysis(w http.ResponseWriter, r *http.Request) {
	client, err := config.ConnectToMongo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Disconnect(context.Background())

	userContext := cont.Get(r, "user")
	userDB, err := GetUserFromContext(userContext)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	collection := client.Database("paymentx").Collection("transactions")

	tp := r.URL.Query().Get("type")
	if tp == "" {
		tp = "DEBIT"
	}

	// Group by hour of the day
	// Pipeline to group by hour and get each transaction's amount for scatter plot
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": userDB.ID,
			"type":    tp,
		}},
		bson.M{"$addFields": bson.M{
			"hour24": bson.M{
				"$let": bson.M{
					"vars": bson.M{
						"timeParts": bson.M{"$split": bson.A{"$transactiontime", " "}},
						"hourMinute": bson.M{"$split": bson.A{
							bson.M{"$arrayElemAt": bson.A{
								bson.M{"$split": bson.A{"$transactiontime", " "}}, 0,
							}},
							":",
						}},
						"ampm": bson.M{"$arrayElemAt": bson.A{
							bson.M{"$split": bson.A{"$transactiontime", " "}}, 1,
						}},
					},
					"in": bson.M{
						"$let": bson.M{
							"vars": bson.M{
								"hour": bson.M{"$toInt": bson.M{"$arrayElemAt": bson.A{"$$hourMinute", 0}}},
							},
							"in": bson.M{
								"$cond": bson.A{
									bson.M{"$eq": bson.A{"$$ampm", "AM"}},
									bson.M{
										"$cond": bson.A{
											bson.M{"$eq": bson.A{"$$hour", 12}},
											0,
											"$$hour",
										},
									},
									bson.M{
										"$cond": bson.A{
											bson.M{"$eq": bson.A{"$$hour", 12}},
											12,
											bson.M{"$add": bson.A{"$$hour", 12}},
										},
									},
								},
							},
						},
					},
				},
			},
		}},
		bson.M{"$project": bson.M{
			"hour":   "$hour24",
			"amount": 1,
			"_id":    0,
		}},
		bson.M{"$sort": bson.M{"hour": 1}},
	}

	cursor, err := collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	type TimeAnalysis struct {
		Hour   int     `json:"hour"`
		Amount float64 `json:"amount"`
	}

	var results []TimeAnalysis
	for cursor.Next(context.Background()) {
		var doc struct {
			Hour   int     `bson:"hour"`
			Amount float64 `bson:"amount"`
		}
		if err := cursor.Decode(&doc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, TimeAnalysis{
			Hour:   doc.Hour,
			Amount: doc.Amount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func GetDebitVsCredit(w http.ResponseWriter, r *http.Request) {
	client, err := config.ConnectToMongo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Disconnect(context.Background())

	userContext := cont.Get(r, "user")
	userDB, err := GetUserFromContext(userContext)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	collection := client.Database("paymentx").Collection("transactions")

	// Parse year from query params, default to current year if not provided
	yearStr := r.URL.Query().Get("year")
	now := time.Now()
	if yearStr == "" {
		yearStr = fmt.Sprintf("%d", now.Year())
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "invalid year", http.StatusBadRequest)
		return
	}

	// Group by month and type for the given year
	pipeline := bson.A{
		bson.M{"$match": bson.M{
			"user_id": userDB.ID,
			"$expr": bson.M{
				"$eq": bson.A{bson.M{"$year": "$transactiondate"}, year},
			},
		}},
		bson.M{"$group": bson.M{
			"_id": bson.M{
				"month": bson.M{"$month": "$transactiondate"},
				"type":  "$type",
			},
			"total": bson.M{"$sum": "$amount"},
		}},
		bson.M{"$sort": bson.M{"_id.month": 1}},
	}

	cursor, err := collection.Aggregate(context.Background(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.Background())

	type MonthResult struct {
		Month  int     `json:"month"`
		Debit  float64 `json:"debit"`
		Credit float64 `json:"credit"`
	}
	// Map of month to MonthResult
	monthMap := make(map[int]*MonthResult)

	for cursor.Next(context.Background()) {
		var doc struct {
			ID struct {
				Month int    `bson:"month"`
				Type  string `bson:"type"`
			} `bson:"_id"`
			Total float64 `bson:"total"`
		}
		if err := cursor.Decode(&doc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		m, ok := monthMap[doc.ID.Month]
		if !ok {
			m = &MonthResult{Month: doc.ID.Month}
			monthMap[doc.ID.Month] = m
		}
		if doc.ID.Type == "DEBIT" {
			m.Debit = doc.Total
		} else if doc.ID.Type == "CREDIT" {
			m.Credit = doc.Total
		}
	}

	// Prepare results for all 12 months (fill missing months with zero)
	var results []MonthResult
	for i := 1; i <= 12; i++ {
		if m, ok := monthMap[i]; ok {
			results = append(results, *m)
		} else {
			results = append(results, MonthResult{Month: i, Debit: 0, Credit: 0})
		}
	}

	type Response struct {
		Year    int           `json:"year"`
		Results []MonthResult `json:"results"`
	}

	resp := Response{
		Year:    year,
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}