package metrics

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Collector interface {
	writePrometheus(*strings.Builder)
}

type Registry struct {
	mu         sync.RWMutex
	collectors []Collector
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) MustRegister(collectors ...Collector) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.collectors = append(r.collectors, collectors...)
}

func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		r.mu.RLock()
		collectors := append([]Collector(nil), r.collectors...)
		r.mu.RUnlock()

		var builder strings.Builder
		for _, collector := range collectors {
			collector.writePrometheus(&builder)
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(builder.String()))
	})
}

type CounterVec struct {
	name       string
	help       string
	labelNames []string

	mu     sync.RWMutex
	series map[string]*counterSeries
}

type counterSeries struct {
	labelValues []string
	value       float64
}

func NewCounterVec(name, help string, labelNames []string) *CounterVec {
	return &CounterVec{
		name:       name,
		help:       help,
		labelNames: append([]string(nil), labelNames...),
		series:     make(map[string]*counterSeries),
	}
}

func (c *CounterVec) Inc(labelValues ...string) {
	c.Add(1, labelValues...)
}

func (c *CounterVec) Add(value float64, labelValues ...string) {
	if value < 0 {
		panic("counter cannot decrease")
	}

	seriesKey, values := labelsKey(c.labelNames, labelValues)

	c.mu.Lock()
	defer c.mu.Unlock()

	series, ok := c.series[seriesKey]
	if !ok {
		series = &counterSeries{labelValues: values}
		c.series[seriesKey] = series
	}

	series.value += value
}

func (c *CounterVec) writePrometheus(builder *strings.Builder) {
	writeMetricHeader(builder, c.name, c.help, "counter")

	c.mu.RLock()
	series := make([]counterSeries, 0, len(c.series))
	for _, value := range c.series {
		series = append(series, counterSeries{
			labelValues: append([]string(nil), value.labelValues...),
			value:       value.value,
		})
	}
	c.mu.RUnlock()

	sort.Slice(series, func(i, j int) bool {
		return strings.Join(series[i].labelValues, "\xff") < strings.Join(series[j].labelValues, "\xff")
	})

	for _, value := range series {
		writeSample(builder, c.name, c.labelNames, value.labelValues, value.value)
	}
}

type HistogramVec struct {
	name       string
	help       string
	labelNames []string
	buckets    []float64

	mu     sync.RWMutex
	series map[string]*histogramSeries
}

type histogramSeries struct {
	labelValues []string
	buckets     []uint64
	count       uint64
	sum         float64
}

func NewHistogram(name, help string, buckets []float64, labelNames []string) *HistogramVec {
	if len(buckets) == 0 {
		panic("histogram requires at least one bucket")
	}

	copiedBuckets := append([]float64(nil), buckets...)
	sort.Float64s(copiedBuckets)

	return &HistogramVec{
		name:       name,
		help:       help,
		labelNames: append([]string(nil), labelNames...),
		buckets:    copiedBuckets,
		series:     make(map[string]*histogramSeries),
	}
}

func (h *HistogramVec) Observe(value float64, labelValues ...string) {
	seriesKey, values := labelsKey(h.labelNames, labelValues)

	h.mu.Lock()
	defer h.mu.Unlock()

	series, ok := h.series[seriesKey]
	if !ok {
		series = &histogramSeries{
			labelValues: values,
			buckets:     make([]uint64, len(h.buckets)),
		}
		h.series[seriesKey] = series
	}

	series.count++
	series.sum += value

	for i, bucket := range h.buckets {
		if value <= bucket {
			series.buckets[i]++
		}
	}
}

func (h *HistogramVec) writePrometheus(builder *strings.Builder) {
	writeMetricHeader(builder, h.name, h.help, "histogram")

	h.mu.RLock()
	series := make([]histogramSeries, 0, len(h.series))
	for _, value := range h.series {
		series = append(series, histogramSeries{
			labelValues: append([]string(nil), value.labelValues...),
			buckets:     append([]uint64(nil), value.buckets...),
			count:       value.count,
			sum:         value.sum,
		})
	}
	h.mu.RUnlock()

	sort.Slice(series, func(i, j int) bool {
		return strings.Join(series[i].labelValues, "\xff") < strings.Join(series[j].labelValues, "\xff")
	})

	labels := append(append([]string(nil), h.labelNames...), "le")
	for _, value := range series {
		for i, upperBound := range h.buckets {
			labelValues := append(append([]string(nil), value.labelValues...), formatFloat(upperBound))
			writeSample(builder, h.name+"_bucket", labels, labelValues, float64(value.buckets[i]))
		}

		infLabels := append(append([]string(nil), value.labelValues...), "+Inf")
		writeSample(builder, h.name+"_bucket", labels, infLabels, float64(value.count))
		writeSample(builder, h.name+"_sum", h.labelNames, value.labelValues, value.sum)
		writeSample(builder, h.name+"_count", h.labelNames, value.labelValues, float64(value.count))
	}
}

type GaugeSample struct {
	LabelValues []string
	Value       float64
}

type GaugeFunc struct {
	name       string
	help       string
	labelNames []string
	fn         func() []GaugeSample
}

func NewGaugeFunc(name, help string, labelNames []string, fn func() []GaugeSample) *GaugeFunc {
	return &GaugeFunc{
		name:       name,
		help:       help,
		labelNames: append([]string(nil), labelNames...),
		fn:         fn,
	}
}

func (g *GaugeFunc) writePrometheus(builder *strings.Builder) {
	writeMetricHeader(builder, g.name, g.help, "gauge")

	samples := append([]GaugeSample(nil), g.fn()...)
	sort.Slice(samples, func(i, j int) bool {
		return strings.Join(samples[i].LabelValues, "\xff") < strings.Join(samples[j].LabelValues, "\xff")
	})

	for _, sample := range samples {
		if len(sample.LabelValues) != len(g.labelNames) {
			panic(fmt.Sprintf("metric %s: got %d labels, want %d", g.name, len(sample.LabelValues), len(g.labelNames)))
		}
		writeSample(builder, g.name, g.labelNames, sample.LabelValues, sample.Value)
	}
}

func labelsKey(labelNames, labelValues []string) (string, []string) {
	if len(labelValues) != len(labelNames) {
		panic(fmt.Sprintf("got %d labels, want %d", len(labelValues), len(labelNames)))
	}

	values := append([]string(nil), labelValues...)
	return strings.Join(values, "\xff"), values
}

func writeMetricHeader(builder *strings.Builder, name, help, metricType string) {
	builder.WriteString("# HELP ")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(escapeHelp(help))
	builder.WriteByte('\n')
	builder.WriteString("# TYPE ")
	builder.WriteString(name)
	builder.WriteByte(' ')
	builder.WriteString(metricType)
	builder.WriteByte('\n')
}

func writeSample(builder *strings.Builder, name string, labelNames, labelValues []string, value float64) {
	builder.WriteString(name)
	if len(labelNames) > 0 {
		builder.WriteByte('{')
		for i, labelName := range labelNames {
			if i > 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(labelName)
			builder.WriteString("=\"")
			builder.WriteString(escapeLabelValue(labelValues[i]))
			builder.WriteByte('"')
		}
		builder.WriteByte('}')
	}
	builder.WriteByte(' ')
	builder.WriteString(formatFloat(value))
	builder.WriteByte('\n')
}

func escapeHelp(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return value
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}

func formatFloat(value float64) string {
	if math.IsInf(value, 1) {
		return "+Inf"
	}
	if math.IsInf(value, -1) {
		return "-Inf"
	}
	if math.IsNaN(value) {
		return "NaN"
	}

	return strconv.FormatFloat(value, 'f', -1, 64)
}
