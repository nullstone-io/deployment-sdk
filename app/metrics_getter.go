package app

import (
	"context"
	"math"
	"time"
)

type MetricsGetter interface {
	Get(ctx context.Context, options MetricsGetterOptions) (*MetricsData, error)
}

type MetricsGetterOptions struct {
	StartTime *time.Time
	EndTime   *time.Time

	Metrics []string
}

func NewMetricsData() *MetricsData {
	return &MetricsData{
		Metrics: map[string]*MetricsDataMetric{},
	}
}

type MetricsData struct {
	Metrics map[string]*MetricsDataMetric `json:"metrics"`
}

func (d *MetricsData) Metric(metricName string) *MetricsDataMetric {
	if existing, ok := d.Metrics[metricName]; ok {
		return existing
	}

	mdm := NewMetricsDataMetric(metricName)
	d.Metrics[metricName] = mdm
	return mdm
}

func NewMetricsDataMetric(metricName string) *MetricsDataMetric {
	return &MetricsDataMetric{
		Name:       metricName,
		MinValue:   math.MaxFloat64,
		MaxValue:   0,
		Datapoints: make([]MetricDataPoint, 0),
	}
}

type MetricsDataMetric struct {
	Name       string            `json:"name"`
	MinValue   float64           `json:"minValue"`
	MaxValue   float64           `json:"maxValue"`
	Datapoints []MetricDataPoint `json:"datapoints"`
}

func (m *MetricsDataMetric) AddPoint(timestamp time.Time, value float64) {
	if value > m.MaxValue {
		m.MaxValue = value
	}
	if value < m.MinValue {
		m.MinValue = value
	}
	m.Datapoints = append(m.Datapoints, MetricDataPoint{
		Timestamp: timestamp,
		Value:     value,
	})
}

type MetricDataPoint struct {
	Timestamp time.Time `json:"t"`
	Value     float64   `json:"v"`
}
