package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	lambdaSdk "github.com/aws/aws-sdk-go-v2/service/lambda"
)

func IsLambda() bool {
	return os.Getenv("LAMBDA_TASK_ROOT") != ""
}

var envVarMap = map[string]string{}

const (
	envKeyDebugPort    = "LAMBDA_DEBUG_PORT"
	envKeyFunctionName = "LAMBDA_FUNCTION_NAME"
)

func getLocalAddr() string {
	debugPort := os.Getenv(envKeyDebugPort)
	if debugPort != "" {
		return ":" + debugPort
	}
	return ":8000"
}

func startLambdaLocally[T interface{}, U interface{}](ctx context.Context, cfg aws.Config, getHandler func(awsConfig aws.Config) Handler[T, U]) {
	fmt.Println("Running lambda locally - need to load environment variables from AWS")
	funcName := getLambdaFunctionName()
	errLog := log.New(os.Stderr, "", log.LstdFlags)

	if funcName != "" {
		fmt.Printf("Loading environment variables from lambda: %s\n", funcName)
		lambdaClient := lambdaSdk.NewFromConfig(cfg)
		res, err := lambdaClient.GetFunctionConfiguration(ctx, &lambdaSdk.GetFunctionConfigurationInput{
			FunctionName: aws.String(funcName),
		})
		if err != nil {
			errLog.Printf("unable to read environment vars for lambda function %s: %s", funcName, err.Error())
			errLog.Println("Ensure the AWS_PROFILE is set in the run configuration")
			os.Exit(1)
		}

		if res.Environment != nil {
			envVarMap = res.Environment.Variables
		}
	} else {
		fmt.Println("No function name provided - any environment variables will need to be manually set")
	}

	handlerFn := getHandler(cfg)
	addr := getLocalAddr()

	http.HandleFunc("/", buildHandleRoot(addr))
	http.HandleFunc("/endpoint", buildHandleLocalEndpoint(handlerFn))

	fmt.Printf("Starting server http://localhost%s\n", addr)
	fmt.Printf("POST requests to http://localhost%s/endpoint using command:\n\n", addr)
	fmt.Printf("curl -X POST -H 'Content-Type: application/json' -d @payload.json http://localhost%s/endpoint\n", addr)

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 3 * time.Second,
	}
	err := server.ListenAndServe()
	if err != nil {
		var bindErr *net.OpError
		if errors.As(err, &bindErr) && strings.Contains(bindErr.Error(), "address already in use") {
			portOnly := strings.Replace(addr, ":", "", 1)
			err = fmt.Errorf("the port %s is already in use. Set the %s environment variable to use a different port", portOnly, envKeyDebugPort)
		}
		panic(err)
	}
}

func buildHandleRoot(addr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lines := []string{
			"Save the JSON payload to a file - e.g. payload.json",
			fmt.Sprintf("curl -X POST -H \"Content-Type: application/json\" -d @payload.json http://localhost%s/endpoint", addr),
		}

		_, wErr := w.Write([]byte(strings.Join(lines, "\n\n")))
		if wErr != nil {
			logger := log.New(os.Stderr, "", 0)
			logger.Println(wErr)
		}
	}
}

func buildHandleLocalEndpoint[T any, U any](handler Handler[T, U]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		handleError := func(err error) {
			w.WriteHeader(http.StatusInternalServerError)
			_, wErr := w.Write([]byte(err.Error()))
			if wErr != nil {
				logger := log.New(os.Stderr, "", 0)
				logger.Println(wErr)
			}
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

		//Some lambda handlers require a context with deadline
		ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(1*time.Hour))
		defer cancel()

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		hctx := GetWithSlogLogger(ctx, logger)
		res, err := handler(hctx, input)
		if err != nil {
			handleError(err)
			return
		}

		outputBytes, err := json.Marshal(res)
		if err != nil {
			handleError(err)
			return
		}

		_, wErr := w.Write(outputBytes)
		if wErr != nil {
			logger := log.New(os.Stderr, "", 0)
			logger.Println(wErr)
		}
	}
}

func getLambdaFunctionName() string {
	trimAndRemoveLogPrefix := func(s string) string {
		return strings.TrimPrefix(strings.TrimSpace(s), "/aws/lambda/")
	}

	envFuncName := os.Getenv(envKeyFunctionName)
	if envFuncName != "" {
		return trimAndRemoveLogPrefix(envFuncName)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter lambda function name, log group name (or re-run with env var %s set): ", envKeyFunctionName)
	text, _ := reader.ReadString('\n')

	return trimAndRemoveLogPrefix(text)
}
