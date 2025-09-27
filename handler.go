package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/cfn"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	lambdaSdk "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-xray-sdk-go/v2/instrumentation/awsv2"
)

const loggerKey = "logger"

func GetLogger(ctx context.Context) *slog.Logger {
	val := ctx.Value(loggerKey)
	if val != nil {
		return val.(*slog.Logger)
	}
	return slog.Default()
}

func GetNewContextWithLogger(parent context.Context, logger *slog.Logger) context.Context {
	newContext := context.WithValue(parent, loggerKey, logger)
	return newContext
}

type Handler[T interface{}, U interface{}] func(ctx context.Context, event T) (U, error)

func WithLogger[T interface{}, U interface{}](handlerFunc Handler[T, U]) Handler[T, U] {
	return func(ctx context.Context, event T) (U, error) {
		// Perform pre-handler tasks here
		newContext := ContextWithLogger(ctx)

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
	val, found := envVarMap[key]
	if found {
		return val
	}
	val = os.Getenv(key)
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

	if IsLambda() {
		awsv2.AWSV2Instrumentor(&cfg.APIOptions)
		handlerFn := getHandler(cfg)
		lambda.Start(WithLogger(handlerFn))
		return
	}

	startLambdaLocally(ctx, cfg, getHandler)
}

func IsLambda() bool {
	return os.Getenv("LAMBDA_TASK_ROOT") != ""
}

var envVarMap = map[string]string{}

func startLambdaLocally[T interface{}, U interface{}](ctx context.Context, cfg aws.Config, getHandler func(awsConfig aws.Config) Handler[T, U]) {
	fmt.Println("Running lambda locally - need to load environment variables from AWS")
	funcName := getLambdaFunctionName()

	if funcName != "" {
		fmt.Printf("Loading environment variables from lambda: %s\n", funcName)
		lambdaClient := lambdaSdk.NewFromConfig(cfg)
		res, err := lambdaClient.GetFunctionConfiguration(ctx, &lambdaSdk.GetFunctionConfigurationInput{
			FunctionName: aws.String(funcName),
		})
		if err != nil {
			panic(fmt.Errorf("unable to read environment vars for lambda function %s: %w", funcName, err))
		}

		if res.Environment != nil {
			envVarMap = res.Environment.Variables
		}
	} else {
		fmt.Println("No function name provided - any environment variables will need to be manually set")
	}

	handlerFn := getHandler(cfg)

	http.HandleFunc("/", handleLocalRoot)
	http.HandleFunc("/endpoint", buildHandleLocalEndpoint(handlerFn))

	fmt.Println("Starting server")
	fmt.Println("View UI at http://localhost:8000")
	fmt.Println("POST requests to http://localhost:8000/endpoint")
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		panic(err)
	}
}

func handleLocalRoot(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("The UI has not yet been built yet\n\nPOST requests to /endpoint"))
}

func buildHandleLocalEndpoint[T any, U any](handler Handler[T, U]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		handleError := func(err error) {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
		}

		bytes, err := io.ReadAll(r.Body)
		if err != nil {
			handleError(err)
			return
		}

		var input T
		err = json.Unmarshal(bytes, &input)
		if err != nil {
			handleError(err)
			return
		}

		res, err := handler(r.Context(), input)
		if err != nil {
			handleError(err)
			return
		}

		outputBytes, err := json.Marshal(res)
		if err != nil {
			handleError(err)
			return
		}

		w.Write(outputBytes)
	}
}

func getLambdaFunctionName() string {
	envFuncName := os.Getenv("LAMBDA_FUNCTION_NAME")
	if envFuncName != "" {
		return envFuncName
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Lambda function name: ")
	text, _ := reader.ReadString('\n')

	return strings.TrimSpace(text)
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
