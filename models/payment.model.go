package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TransactionType string

const (
	Debit  TransactionType = "DEBIT"
	Credit TransactionType = "CREDIT"
)

// Transaction represents a unique transaction in the database.
// To ensure uniqueness, you should create a unique index in MongoDB on relevant fields.
// For example, you can create a unique index on (UserID, TransactionDate, Amount, Details, Type).
type Transaction struct {
	ID              primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID          primitive.ObjectID `json:"user_id,omitempty" bson:"user_id,omitempty"`
	Amount          float64            `json:"amount,omitempty"`
	TransactionDate primitive.DateTime `json:"transaction_date"`
	TransactionTime string             `json:"transaction_time"`
	ValueDate       string             `json:"value_date"`
	Details         string             `json:"details"`
	Type            TransactionType    `json:"type"`
	Balance         float64            `json:"balance"`
	TransactionID   string				`json:"transaction_id"`
}
