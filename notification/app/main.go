package main

import (
	"app/handler"
	"app/models"
	"app/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"go.uber.org/zap"
)

var (
	logger    *zap.Logger
	ginLambda *ginadapter.GinLambda
	ctx       = context.Background()
)

func init() {
	l, _ := zap.NewProduction()
	logger = l

	nh := handler.NewNotificationHandler(logger)
	ginLambda = ginadapter.New(nh.R)
}

func notificationHandlerAPI(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return ginLambda.Proxy(request)
}

func notificationHandlerSQS(sqsEvent events.SQSEvent) error {
	mailer := utils.NewGMailer(os.Getenv("EMAIL_SENDER"), os.Getenv("EMAIL_SENDER_ADDRESS"), os.Getenv("EMAIL_SENDER_PASSWORD"))
	for _, message := range sqsEvent.Records {
		logger.Info("received SQS message", zap.String("message", message.Body))

		var snsMessage models.SNSMessage
		err := json.Unmarshal([]byte(message.Body), &snsMessage)
		if err != nil {
			return fmt.Errorf("failed to unmarshal SQS message body: %w", err)
		}

		if snsMessage.MessageAttributes.ActionType.Value == "SEND_EMAIL" {
			var data *models.EmailPayload
			err := json.Unmarshal([]byte(snsMessage.Message), &data)
			if err != nil {
				return fmt.Errorf("failed to unmarshal payload: %w", err)
			}

			to := []string{data.Email}

			if err := mailer.SendMail(data.Subject, data.Content, to, nil, nil, nil); err != nil {
				return fmt.Errorf("failed to send verify email")
			}
		}
	}

	return nil
}

func multiHandler(request any) (any, error) {
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	var temp map[string]interface{}
	if err := json.Unmarshal(requestBytes, &temp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %v", err)
	}

	if _, exists := temp["Records"]; exists {
		var sqsEvent events.SQSEvent
		if err := json.Unmarshal(requestBytes, &sqsEvent); err != nil {
			return nil, fmt.Errorf("failed to unmarshal SQS event: %v", err)
		}
		return nil, notificationHandlerSQS(sqsEvent)
	}

	var apiRequest events.APIGatewayProxyRequest
	if err := json.Unmarshal(requestBytes, &apiRequest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API Gateway request: %v", err)
	}
	return notificationHandlerAPI(apiRequest)
}

func main() {
	lambda.Start(multiHandler)
}
