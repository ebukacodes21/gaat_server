package utils

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	AuthorizationHeaderKey  = "authorization"
	AuthorizationTypeBearer = "bearer"
	AuthorizationPayloadKey = "authorization_payload"
)

func AuthMiddleware(maker TokenMaker) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authorizationHeader := ctx.GetHeader(AuthorizationHeaderKey)
		if len(authorizationHeader) == 0 {
			err := errors.New("authorization header is not provided")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorRes(err))
		}

		fields := strings.Fields(authorizationHeader)
		if len(fields) < 2 {
			err := errors.New("invalid authorization")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorRes(err))
		}

		authorizationType := strings.ToLower(fields[0])
		if authorizationType != AuthorizationTypeBearer {
			err := errors.New("unsupported format")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorRes(err))
		}

		accessToken := fields[1]
		payload, err := maker.VerifyToken(accessToken)
		if err != nil {
			log.Print(err)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorRes(err))
		}

		ctx.Set(AuthorizationPayloadKey, payload)
		ctx.Next()
	}
}

var RoleRank = map[string]int{
	"admin":      4,
	"supervisor": 3,
	"staff":      2,
	"user":       1,
}

func HasPermission(userRole string, requiredRole string) bool {
	userRank, ok1 := RoleRank[userRole]
	requiredRank, ok2 := RoleRank[requiredRole]

	if !ok1 || !ok2 {
		return false
	}

	return userRank >= requiredRank
}

func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, exists := c.Get(AuthorizationPayloadKey)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		payload := raw.(*Payload)

		if !HasPermission(payload.Role, requiredRole) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "unauthorized to access this route",
			})
			return
		}

		c.Next()
	}
}
