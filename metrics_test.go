package handler

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_metrics(t *testing.T) {
	t.Setenv("METRIC_NAMESPACE", "MyTestApp")

	testcases := []struct {
		name    string
		handler Handler[inputEvent, outputEvent]
		checkFn func(t *testing.T, logMap map[string]json.RawMessage)
	}{
		{
			name: "metrics without dimensions",
			handler: func(ctx *Context, event inputEvent) (outputEvent, error) {
				ctx.Metric("myMetric").
					Value(5)
				return outputEvent{}, nil
			},
			checkFn: func(t *testing.T, logMap map[string]json.RawMessage) {
				require.Contains(t, logMap, "_aws")
				assert.JSONEq(t, `5`, string(logMap["myMetric"]))

				var cwm cwMetrics
				err := json.Unmarshal(logMap["_aws"], &cwm)
				require.NoError(t, err)
				require.Len(t, cwm.Metrics, 1)

				exp := cwMetricOuter{
					Namespace:  "MyTestApp",
					Dimensions: [][]string{},
					Metrics: []cwMetricInner{
						{Name: "myMetric"},
					},
				}
				assert.Equal(t, exp, cwm.Metrics[0])
			},
		},
		{
			name: "metrics with unit",
			handler: func(ctx *Context, event inputEvent) (outputEvent, error) {
				ctx.Metric("myMetric").Unit("Milliseconds").Value(123)
				return outputEvent{}, nil
			},
			checkFn: func(t *testing.T, logMap map[string]json.RawMessage) {
				require.Contains(t, logMap, "_aws")
				assert.JSONEq(t, `123`, string(logMap["myMetric"]))

				var cwm cwMetrics
				err := json.Unmarshal(logMap["_aws"], &cwm)
				require.NoError(t, err)
				require.Len(t, cwm.Metrics, 1)

				exp := cwMetricOuter{
					Namespace:  "MyTestApp",
					Dimensions: [][]string{},
					Metrics: []cwMetricInner{{
						Name: "myMetric",
						Unit: aws.String("Milliseconds"),
					}},
				}
				assert.Equal(t, exp, cwm.Metrics[0])
			},
		},
		{
			name: "metrics with dimension",
			handler: func(ctx *Context, event inputEvent) (outputEvent, error) {
				ctx.Metric("myMetric").
					Dimension("InterfaceID", "IF-021").
					Unit("Milliseconds").
					Value(123)
				return outputEvent{}, nil
			},
			checkFn: func(t *testing.T, logMap map[string]json.RawMessage) {
				require.Contains(t, logMap, "_aws")
				assert.JSONEq(t, `123`, string(logMap["myMetric"]))
				assert.JSONEq(t, `"IF-021"`, string(logMap["InterfaceID"]))

				var cwm cwMetrics
				err := json.Unmarshal(logMap["_aws"], &cwm)
				require.NoError(t, err)
				require.Len(t, cwm.Metrics, 1)

				exp := cwMetricOuter{
					Namespace:  "MyTestApp",
					Dimensions: [][]string{{"InterfaceID"}},
					Metrics: []cwMetricInner{{
						Name: "myMetric",
						Unit: aws.String("Milliseconds"),
					}},
				}
				assert.Equal(t, exp, cwm.Metrics[0])
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}

			h := func(ctx *Context, event inputEvent) (outputEvent, error) {
				//Bodge the slog.Logger to be able to capture the logging result
				ctx.storyLogger.slogger = getJSONSlogFromWriter(buf)
				return tc.handler(ctx, event)
			}

			wrappedHandler := withLogger(h, nil)
			_, err := wrappedHandler(t.Context(), inputEvent{Foo: 1})
			require.NoError(t, err)

			b := buf.Bytes()
			var logMap map[string]json.RawMessage
			err = json.Unmarshal(b, &logMap)
			require.NoError(t, err)
			tc.checkFn(t, logMap)
		})
	}
}
