package handler

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/stretchr/testify/assert"
)

func Test_Metrics(t *testing.T) {
	ctx := ContextWithLogger(context.Background())
	logger := GetLogger(ctx)

	client := &TestClient{}
	m := setup(ctx, client)
	m.Start()

	time.Sleep(3 * time.Second)
	metric := MetricsInput{Namespace: "foo", MetricName: "FooThing", Value: 12}
	logger.Info("Send metric", "data", metric)
	m.inputChan <- metric

	time.Sleep(3 * time.Second)
	metric = MetricsInput{Namespace: "bar", MetricName: "BarThing", Value: 1, Dimensions: map[string]string{"group": "a"}}
	logger.Info("Send metric", "data", metric)
	m.inputChan <- metric

	time.Sleep(3 * time.Second)
	metric = MetricsInput{Namespace: "foo", MetricName: "FooThing", Value: 12}
	logger.Info("Send metric", "data", metric)
	m.inputChan <- metric

	time.Sleep(3 * time.Second)
	assert.Equal(t, 2, client.counter)
	metric = MetricsInput{Namespace: "bar", MetricName: "BarThing", Value: 5}
	logger.Info("Send metric", "data", metric)
	m.inputChan <- metric

	m.Shutdown()
	assert.Equal(t, 3, client.counter)
}

type TestClient struct {
	counter int
	params  *cloudwatch.PutMetricDataInput
}

func (tc *TestClient) PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error) {
	tc.counter++
	tc.params = params
	GetLogger(ctx).Info("PutMetricData called", "params", params)
	return &cloudwatch.PutMetricDataOutput{}, nil
}
