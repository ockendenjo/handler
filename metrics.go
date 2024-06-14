package handler

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type metrics struct {
	context   context.Context
	inputChan chan MetricsInput
	lock      sync.Mutex
	client    MetricClient
	run       bool
	stopChan  chan bool
	data      map[string][]types.MetricDatum
}

func setup(ctx context.Context, client MetricClient) metrics {
	return metrics{
		context:   ctx,
		inputChan: make(chan MetricsInput, 100),
		lock:      sync.Mutex{},
		client:    client,
		run:       false,
		stopChan:  make(chan bool),
		data:      map[string][]types.MetricDatum{},
	}
}

func (m *metrics) Shutdown() {
	m.stopChan <- true
	m.flush()
}

func (m *metrics) Start() {
	m.run = true
	go m.startCollector()
	go m.startWriter()
}

func (m *metrics) startCollector() {
	for {
		select {
		case input := <-m.inputChan:
			m.lock.Lock()
			namespaceMap := m.data[input.Namespace]
			m.data[input.Namespace] = append(namespaceMap, convertInput(input))
			m.lock.Unlock()
		case <-m.stopChan:
			return
		}
	}
}

func convertInput(input MetricsInput) types.MetricDatum {

	d := make([]types.Dimension, len(input.Dimensions))

	i := 0
	for k, v := range input.Dimensions {
		d[i] = types.Dimension{Name: &k, Value: &v}
		i++
	}
	now := time.Now()

	return types.MetricDatum{
		MetricName: &input.MetricName,
		Value:      &input.Value,
		Dimensions: d,
		Timestamp:  &now,
	}
}

func (m *metrics) startWriter() {
	for m.run {
		time.Sleep(10 * time.Second)
		m.flush()
	}
}

func (m *metrics) flush() {
	m.lock.Lock()
	mapCopy := map[string][]types.MetricDatum{}
	found := false
	for k, v := range m.data {
		mapCopy[k] = v
		found = true
	}
	clear(m.data)
	m.lock.Unlock()

	if !found {
		return
	}

	for ns, data := range mapCopy {
		namespace := ns

		_, err := m.client.PutMetricData(m.context, &cloudwatch.PutMetricDataInput{
			MetricData: data,
			Namespace:  &namespace,
		})
		if err != nil {
			GetLogger(m.context).Info("PutMetricData returned error", "err", err)
		}
	}
}

type MetricsInput struct {
	Namespace  string
	MetricName string
	Value      float64
	Dimensions map[string]string
}

type MetricClient interface {
	PutMetricData(ctx context.Context, params *cloudwatch.PutMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.PutMetricDataOutput, error)
}
