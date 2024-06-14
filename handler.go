package handler

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
)

const loggerKey = "logger"
const metricsKey = "metrics"

func GetLogger(ctx context.Context) *slog.Logger {
	val := ctx.Value(loggerKey)
	if val != nil {
		return val.(*slog.Logger)
	}
	return slog.Default()
}

func GetMetricsChan(ctx context.Context) chan MetricsInput {
	val := ctx.Value(metricsKey)
	if val == nil {
		ch := make(chan MetricsInput, 0)
		return ch
	}
	m := val.(*metrics)
	return m.inputChan
}

func GetNewContextWithLogger(parent context.Context, logger *slog.Logger) context.Context {
	newContext := context.WithValue(parent, loggerKey, logger)
	return newContext
}

type Handler[T interface{}, U interface{}] func(ctx context.Context, event T) (U, error)

func WithLogger[T interface{}, U interface{}](handlerFunc Handler[T, U], m *metrics) Handler[T, U] {
	return func(ctx context.Context, event T) (U, error) {
		// Perform pre-handler tasks here
		newContext := ContextWithLogger(ctx)
		newContext = context.WithValue(newContext, metricsKey, m)

		response, err := handlerFunc(newContext, event)
		if err != nil {
			logger := GetLogger(ctx)
			logger.Error("lambda execution failed", "error", err.Error())
		}

		return response, err
	}
}

func ContextWithLogger(ctx context.Context) context.Context {
	traceId := os.Getenv("_X_AMZN_TRACE_ID")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if traceId != "" {
		parts := strings.Split(traceId, ";")
		if len(parts) > 0 {
			logger = logger.With("trace_id", strings.Replace(parts[0], "Root=", "", 1))
		}
	}
	newContext := context.WithValue(ctx, loggerKey, logger)
	return newContext
}

func MustGetEnv(key string) string {
	val := os.Getenv(key)
	if strings.Trim(val, " ") == "" {
		panic(fmt.Errorf("environment variable for '%s' has not been set", key))
	}
	return val
}

func MustGetEnvInt(key string) int {
	v := MustGetEnv(key)
	i, err := strconv.Atoi(v)
	if err != nil {
		panic(err)
	}
	return i
}

// BuildAndStart configures a logger, instruments the handler with OpenTelemetry, instruments the AWS SDK, and then starts the lambda
func BuildAndStart[T interface{}, U interface{}](getHandler func(awsConfig aws.Config) Handler[T, U]) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRetryer(func() aws.Retryer {
		return retry.NewStandard(func(so *retry.StandardOptions) {
			//Use a large number so that the SDK client shouldn't run out of retry attempts
			//Note that this is not the number of times it will retry per API call but the number of times
			//it might retry during the client lifetime
			so.RateLimiter = ratelimit.NewTokenRateLimit(1_000_000)
		})
	}))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	cfgWithoutXRay, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	//Instrument the AWS SDK - this needs to happen before any service clients (e.g. s3Client) are created
	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	//Pass the AWS config to the get handler - service clients can be created in this method
	handlerFn := getHandler(cfg)

	cwc := cloudwatch.NewFromConfig(cfgWithoutXRay)
	m := setup(ctx, cwc)
	wrappedHandler := WithLogger(handlerFn, &m)
	m.Start()

	lambda.StartWithOptions(wrappedHandler, lambda.WithEnableSIGTERM(func() {
		log.Println("sigterm")
		m.Shutdown()
	}))
}

func BuildAndStartCustomResource(getHandler func(awsConfig aws.Config) cfn.CustomResourceFunction) {

	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	//Instrument the AWS SDK - this needs to happen before any service clients (e.g. s3Client) are created
	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	//Pass the AWS config to the get handler - service clients can be created in this method
	handlerFn := getHandler(cfg)

	lambda.Start(cfn.LambdaWrap(func(ctx context.Context, event cfn.Event) (physicalResourceID string, data map[string]interface{}, err error) {
		newContext := ContextWithLogger(ctx)
		return handlerFn(newContext, event)
	}))
}
