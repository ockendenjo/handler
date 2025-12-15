package handler

import (
	"context"
	"io"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-xray-sdk-go/v2/instrumentation/awsv2"
)

type Builder[T any, U any] struct {
	ctx        context.Context
	awsConfig  aws.Config
	logWriter  *io.Writer
	getHandler func(awsConfig aws.Config) Handler[T, U]
}

func Build[T interface{}, U interface{}](getHandler func(awsConfig aws.Config) Handler[T, U]) *Builder[T, U] {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRetryer(func() aws.Retryer {
		return retry.NewStandard(func(so *retry.StandardOptions) {
			//Use a large number so that the SDK client shouldn't run out of retry attempts
			//Note that this is not the number of times it will retry per API call but the number of times
			//it might retry during the client lifetime
			so.RateLimiter = ratelimit.NewTokenRateLimit(1_000_000)
		})
	}), config.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	return &Builder[T, U]{
		ctx:        ctx,
		awsConfig:  cfg,
		getHandler: getHandler,
	}
}

func (b *Builder[T, U]) WithLogWriter(getWriter func(aws.Config) *io.Writer) *Builder[T, U] {
	b.logWriter = getWriter(b.awsConfig)
	return b
}

func (b *Builder[T, U]) Start() {
	if IsLambda() {
		awsv2.AWSV2Instrumentor(&b.awsConfig.APIOptions)
		handlerFn := b.getHandler(b.awsConfig)
		lambda.Start(withLogger(handlerFn, b.logWriter))
		return
	}

	startLambdaLocally(b.ctx, b.awsConfig, b.getHandler)
}

// BuildAndStart configures a logger, instruments the handler with OpenTelemetry, instruments the AWS SDK, and then starts the lambda
func BuildAndStart[T interface{}, U interface{}](getHandler func(awsConfig aws.Config) Handler[T, U]) {
	Build(getHandler).Start()
}
