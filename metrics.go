package handler

import (
	"os"
	"time"
)

func (h *Context) Metric(metricName string) *MetricBuilder {
	return &MetricBuilder{
		handlerCtx: h,
		name:       metricName,
	}
}

type MetricBuilder struct {
	handlerCtx *Context
	name       string
	dimensions map[string]any
	unit       *string
	value      any
}

func (m *MetricBuilder) Dimension(key string, value any) *MetricBuilder {
	if m.dimensions == nil {
		m.dimensions = make(map[string]any)
	}
	m.dimensions[key] = value
	return m
}

func (m *MetricBuilder) Unit(value string) *MetricBuilder {
	m.unit = &value
	return m
}

func (m *MetricBuilder) Value(value any) {
	m.value = value
	m.handlerCtx.metrics = append(m.handlerCtx.metrics, m)
}

func (h *Context) addMetricsToLogging() {
	if len(h.metrics) < 1 {
		return
	}

	metricList := make([]cwMetricOuter, 0, len(h.metrics))

	namespace := os.Getenv("METRIC_NAMESPACE")

	for _, m := range h.metrics {

		dimensions := make([][]string, 0, 1)
		if len(m.dimensions) > 0 {
			dimKeys := make([]string, 0, len(m.dimensions))
			for k, v := range m.dimensions {
				dimKeys = append(dimKeys, k)
				h.storyLogger.AddParam(k, v)
			}
			dimensions = append(dimensions, dimKeys)
		}

		outer := cwMetricOuter{
			Namespace:  namespace,
			Dimensions: dimensions,
			Metrics: []cwMetricInner{{
				Name: m.name,
				Unit: m.unit,
			}},
		}
		metricList = append(metricList, outer)

		h.storyLogger.AddParam(m.name, m.value)
	}

	awsMetrics := cwMetrics{
		Metrics:   metricList,
		Timestamp: time.Now().UnixMilli(),
	}

	h.storyLogger.AddParam("_aws", awsMetrics)
}

type cwMetrics struct {
	Metrics   []cwMetricOuter `json:"CloudWatchMetrics"`
	Timestamp int64           `json:"Timestamp"`
}

type cwMetricOuter struct {
	Namespace  string          `json:"Namespace"`
	Dimensions [][]string      `json:"Dimensions"`
	Metrics    []cwMetricInner `json:"Metrics"`
}

type cwMetricInner struct {
	Name              string  `json:"Name"`
	Unit              *string `json:"Unit,omitempty"`
	StorageResolution *int    `json:"StorageResolution,omitempty"`
}
