package config

import (
	"context"
	"fmt"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectToMongo() (*mongo.Client, error) {
	// godotenv.Load(".env")

	uri := os.Getenv("MONGO_URI")

	fmt.Println(uri)

	if uri == "" {
		return nil, fmt.Errorf("MONGO_URL not set")
	}

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	err = client.Ping(context.Background(), nil)

	if err != nil {
		return nil, err
	}

	collection := client.Database("paymentx").Collection("users")
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"email": 1}, // index in ascending order
		Options: options.Index().SetUnique(true),
	}

	_, err = collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		return nil, err
	}

	fmt.Println("✅ Connected Successfully")

	return client, nil
}
