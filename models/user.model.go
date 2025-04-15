package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type User struct {
	ID       primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name     string             `json:"name,omitempty" validate:"required,min=3,max=50"`
	Email    string             `json:"email,omitempty" validate:"required,email"`
	Password string             `json:"password" validate:"required,password"`
}
