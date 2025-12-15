package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ockendenjo/handler"
)

func main() {
	handler.BuildAndStart(func(awsConfig aws.Config) handler.SQSHandler {
		h := &sqsHandler{}
		return handler.GetSQSHandler(h, nil)
	})
}

type inputEvent struct {
	Foo int `json:"foo"`
}

type sqsHandler struct {
}

func (h *sqsHandler) ProcessSQSEvent(ctx *handler.Context, genericType inputEvent, attributes map[string]events.SQSMessageAttribute) error {
	return nil
}
