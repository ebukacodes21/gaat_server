package utils

import (
	"errors"
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
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, ErrorRes(err))
		}

		ctx.Set(AuthorizationPayloadKey, payload)
		ctx.Next()
	}
}
