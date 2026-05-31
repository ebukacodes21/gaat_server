package main

import (
	"database/sql"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"

	"app/handler"
	"app/service"

	runner "app/repository"
	repository "app/repository/sqlc"
	"app/utils"
)

var ginLambda *ginadapter.GinLambda
var logger *zap.Logger

func init() {
	l, _ := zap.NewProduction()
	logger = l

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Fatal("DATABASE_URL environment variable is empty or completely unconfigured")
	}

	dbPool, err := sql.Open("postgres", dbURL)
	if err != nil {
		logger.Error("error opening postgres connection pool allocation", zap.Any("err", err))
		panic(err)
	}

	maxOpen, _ := strconv.Atoi(os.Getenv("DB_MAX_OPEN_CONNS"))
	if maxOpen == 0 {
		maxOpen = 20 // secure fallback threshold limit
	}
	maxIdle, _ := strconv.Atoi(os.Getenv("DB_MAX_IDLE_CONNS"))
	if maxIdle == 0 {
		maxIdle = 2 // keeping idle pool slim drops connection leak frequencies inside serverless instances
	}

	dbPool.SetMaxOpenConns(maxOpen)
	dbPool.SetMaxIdleConns(maxIdle)
	dbPool.SetConnMaxLifetime(5 * time.Minute) // prune stale lingering connections gracefully

	if err := dbPool.Ping(); err != nil {
		logger.Error("FATAL: unable to reach target database",
			zap.String("attempted_url", dbURL),
			zap.Any("err", err),
		)
		panic(err)
	}

	runner.RunDBMigration(dbURL)

	maker, err := utils.NewToken(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		logger.Error("unable to instantiate token maker encryption keys", zap.Any("err", err))
		panic(err)
	}

	mailer := utils.NewGAATMailer(os.Getenv("EMAIL_NAME"), os.Getenv("EMAIL_ADDRESS"), os.Getenv("EMAIL_PASSWORD"))

	db := repository.NewRepository(dbPool)
	svc := service.NewService(logger, db, maker, mailer)
	ah := handler.NewGaatServer(svc, maker)
	ginLambda = ginadapter.New(ah.R)
}

func server(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return ginLambda.Proxy(request)
}

func main() {
	lambda.Start(server)
}
