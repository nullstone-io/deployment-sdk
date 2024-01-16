package workspace

import (
	"math"
	"time"
)

func NewMetricsData() *MetricsData {
	return &MetricsData{
		Metrics: map[string]*MetricDataset{},
	}
}

type MetricsData struct {
	// Metrics are a collection of named datasets that can contain multiple series
	// This enables easy translation into UI graphs
	//   i.e. a dataset represents a single graph, a series is a plot on that graph
	// Example:
	//   "cpu" dataset
	//     "cpu-reserved" series
	//     "cpu-utilized" series
	Metrics map[string]*MetricDataset `json:"metrics"`
}

func (d *MetricsData) GetDataset(metricName string, metricType MetricDatasetType, unit string) *MetricDataset {
	if existing, ok := d.Metrics[metricName]; ok {
		return existing
	}

	ds := NewMetricDataset(metricName, metricType, unit)
	d.Metrics[metricName] = ds
	return ds
}

func NewMetricDataset(metricName string, metricType MetricDatasetType, unit string) *MetricDataset {
	return &MetricDataset{
		Name:   metricName,
		Type:   metricType,
		Unit:   unit,
		Series: map[string]*MetricSeries{},
	}
}

type MetricDataset struct {
	Name   string                   `json:"name"`
	Type   MetricDatasetType        `json:"type"`
	Unit   string                   `json:"unit"`
	Series map[string]*MetricSeries `json:"series"`
}

type MetricDatasetType string

const (
	MetricDatasetTypeUsage       = "usage"
	MetricDatasetTypeInvocations = "invocations"
	MetricDatasetTypeDuration    = "duration"
)

func (d *MetricDataset) GetSeries(metricId string) *MetricSeries {
	if existing, ok := d.Series[metricId]; ok {
		return existing
	}

	ms := NewMetricSeries(metricId)
	d.Series[metricId] = ms
	return ms
}

func NewMetricSeries(id string) *MetricSeries {
	return &MetricSeries{
		Id:         id,
		MinValue:   math.MaxFloat64,
		MaxValue:   0,
		Datapoints: make([]MetricDataPoint, 0),
	}
}

type MetricSeries struct {
	Id         string            `json:"id"`
	MinValue   float64           `json:"minValue"`
	MaxValue   float64           `json:"maxValue"`
	Datapoints []MetricDataPoint `json:"datapoints"`
}

func (m *MetricSeries) AddPoint(timestamp time.Time, value float64) {
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
