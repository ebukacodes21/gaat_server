package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type MessageAttributes struct {
	ActionType struct {
		Type  string `json:"Type"`
		Value string `json:"Value"`
	} `json:"actionType"`
}

type SNSMessage struct {
	Type              string            `json:"Type"`
	MessageId         string            `json:"MessageId"`
	TopicArn          string            `json:"TopicArn"`
	Message           string            `json:"Message"`
	Timestamp         string            `json:"Timestamp"`
	SignatureVersion  string            `json:"SignatureVersion"`
	Signature         string            `json:"Signature"`
	SigningCertURL    string            `json:"SigningCertURL"`
	UnsubscribeURL    string            `json:"UnsubscribeURL"`
	MessageAttributes MessageAttributes `json:"MessageAttributes"`
}

type EmailPayload struct {
	Email   string `json:"Email"`
	Subject string `json:"Subject"`
	Content string `json:"Content"`
}

type InviteLawyerPayload struct {
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Content string `json:"content"`
}

type AddAdminPayload struct {
	AdminID          string `json:"AdminID"`
	Email            string `json:"Email"`
	Password         string `json:"Password"`
	VerificationCode string `json:"VerificationCode"`
}

type NotificationPayload struct {
	RestaurantID string `json:"restaurantID"`
	Title        string `json:"title"`
	Body         string `json:"body"`
}

type Notification struct {
	ID           bson.ObjectID `bson:"_id"`
	RestaurantID bson.ObjectID `bson:"restaurantID"`
	Title        string        `bson:"title"`
	Body         string        `bson:"body"`
	IsRead       bool          `bson:"isRead"`
	CreatedAt    time.Time     `bson:"createdAt"`
}
