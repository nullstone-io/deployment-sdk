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

// MetricDataset contains all data and labels for a single chart
// The Series contained are plotted on the same chart
type MetricDataset struct {
	// Name is displayed in the title of the chart
	Name string `json:"name"`
	// Type is the kind of metric that is used to select what type of chart to use
	Type MetricDatasetType `json:"type"`
	// Unit is the unit of measurement for the entire chart
	// All Series in this MetricDataset must use the same unit of measurement
	// The unit of measurement is added to the title as `<name> (<unit>)`
	Unit   string                   `json:"unit"`
	Series map[string]*MetricSeries `json:"series"`
}

type MetricDatasetType string

const (
	MetricDatasetTypeUsage       = "usage"
	MetricDatasetTypeInvocations = "invocations"
	MetricDatasetTypeDuration    = "duration"
)

func (d *MetricDataset) GetSeries(id, metricId string) *MetricSeries {
	if existing, ok := d.Series[id]; ok {
		return existing
	}

	ms := NewMetricSeries(id, metricId)
	d.Series[id] = ms
	return ms
}

func NewMetricSeries(id, metricId string) *MetricSeries {
	return &MetricSeries{
		Id:         id,
		MetricId:   metricId,
		MinValue:   math.MaxFloat64,
		MaxValue:   0,
		Datapoints: make([]MetricDataPoint, 0),
	}
}

type MetricSeries struct {
	// Id is used as the label and unique identifier within a single chart
	Id string `json:"id"`
	// MetricId is a unique identifier across all MetricDatasets in order to safely query from the metrics provider
	MetricId string `json:"metricId"`
	// MinValue is used to identify the lowest y-value on the chart
	MinValue float64 `json:"minValue"`
	// MaxValue is used to identify the highest y-value on the chart
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
