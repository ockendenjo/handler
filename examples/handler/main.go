package main

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/ockendenjo/handler"
)

func main() {
	handler.BuildAndStart(func(awsConfig aws.Config) handler.Handler[*inputEvent, *outputEvent] {
		//Set up any AWS SDK clients here (using awsConfig)

		return func(ctx *handler.Context, event *inputEvent) (*outputEvent, error) {
			//This function is invoked for every lambda invocation
			if event.Foo < 0 {
				return nil, errors.New("foo must not be less than zero")
			}
			return &outputEvent{Bar: 2 * event.Foo}, nil
		}
	})
}

type inputEvent struct {
	Foo int `json:"foo"`
}

type outputEvent struct {
	Bar int `json:"bar"`
}
