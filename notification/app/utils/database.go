package utils

import (
	"app/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type DatabaseContract interface {
	CreateNotification(ctx context.Context, req *models.NotificationPayload) error
	GetNotifications(ctx context.Context, resId string) ([]*models.Notification, error)
	MarkAsRead(ctx context.Context, id string) (string, error)
	MarkAllAsRead(ctx context.Context, id string) (string, error)
}

type Repository struct {
	client           *mongo.Client
	database         *mongo.Database
	notificationColl *mongo.Collection
}

func NewRepository(client *mongo.Client, databaseName string) DatabaseContract {
	db := client.Database(databaseName)

	return &Repository{
		client:           client,
		database:         db,
		notificationColl: db.Collection("notifications"),
	}
}

func (r *Repository) CreateNotification(ctx context.Context, req *models.NotificationPayload) error {
	id, err := bson.ObjectIDFromHex(req.RestaurantID)
	if err != nil {
		return fmt.Errorf("%s invalid restaurant id", req.RestaurantID)
	}

	notification := &models.Notification{
		ID:           bson.NewObjectID(),
		RestaurantID: id,
		Title:        req.Title,
		Body:         req.Body,
		IsRead:       false,
		CreatedAt:    time.Now(),
	}

	if _, err = r.notificationColl.InsertOne(ctx, notification); err != nil {
		return fmt.Errorf("%v unable to create notification", err)
	}
	return nil
}

func (r *Repository) GetNotifications(ctx context.Context, resId string) ([]*models.Notification, error) {
	id, err := bson.ObjectIDFromHex(resId)
	if err != nil {
		return nil, fmt.Errorf("%s invalid restaurant id", resId)
	}

	cursor, err := r.notificationColl.Find(ctx, bson.M{"restaurantID": id})
	if err != nil {
		return nil, fmt.Errorf("unable to find notifications: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*models.Notification
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("decoding error: %w", err)
	}

	return results, nil
}

func (r *Repository) MarkAsRead(ctx context.Context, notificationId string) (string, error) {
	var notification *models.Notification
	id, err := bson.ObjectIDFromHex(notificationId)
	if err != nil {
		return "", fmt.Errorf("%s invalid notification id", notificationId)
	}

	err = r.notificationColl.FindOne(ctx, bson.M{"_id": id}).Decode(&notification)
	if err == mongo.ErrNoDocuments {
		return "", fmt.Errorf("notification with id %s not found", id)
	} else if err != nil {
		return "", fmt.Errorf("failed to query notification: %w", err)
	}

	update := bson.M{"$set": bson.M{"isRead": true}}
	_, err = r.notificationColl.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return "", fmt.Errorf("failed to update notification status: %w", err)
	}
	return "mark read successful", nil
}

func (r *Repository) MarkAllAsRead(ctx context.Context, resId string) (string, error) {
	rid, err := bson.ObjectIDFromHex(resId)
	if err != nil {
		return "", fmt.Errorf("%s is an invalid restaurant ID", rid)
	}

	filter := bson.M{"restaurantID": rid, "isRead": false}
	update := bson.M{"$set": bson.M{"isRead": true}}

	result, err := r.notificationColl.UpdateMany(ctx, filter, update)
	if err != nil {
		return "", fmt.Errorf("failed to update notifications: %w", err)
	}

	if result.MatchedCount == 0 {
		return "no unread notifications found", nil
	}

	return fmt.Sprintf("marked %d notifications as read", result.ModifiedCount), nil
}
