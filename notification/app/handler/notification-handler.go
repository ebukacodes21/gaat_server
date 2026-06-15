package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type NotificationHandler struct {
	logger *zap.Logger
	R      *gin.Engine
}

func NewNotificationHandler(logger *zap.Logger) *NotificationHandler {
	r := gin.Default()

	nh := &NotificationHandler{
		logger: logger,
		R:      r,
	}

	return nh
}
