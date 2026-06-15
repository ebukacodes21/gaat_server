package service

import (
	"go.uber.org/zap"
)

type NotificationService struct {
	logger *zap.Logger
}

func NewNotificationService(logger *zap.Logger) *NotificationService {
	nh := &NotificationService{
		logger: logger,
	}

	return nh
}
