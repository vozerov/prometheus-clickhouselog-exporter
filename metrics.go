package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	chLogExporterErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chlogexporter_errors",
			Help: "Clickhouse Log Exporter Internal Errors",
		},
		[]string{"type"},
	)

	readLines = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "chlogexporter_read_lines",
			Help: "Total read lines count",
		},
	)

	chQueryErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "clickhouse_query_errors",
			Help: "Clickhouse Query Errors Count by Code",
		},
		[]string{"type", "code"},
	)

	chQueryCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "clickhouse_query_count",
			Help: "Clickhouse Query Count by Type",
		},
		[]string{"type"},
	)

	chQueryTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "clickhouse_query_time",
		Help:    "Time needed to process query by type",
		Buckets: []float64{1, 5, 10, 20, 30, 40, 50, 60, 120, 180, 300, 1800},
	},
		[]string{"type"},
	)

	chSelectQueryRowsRead = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "clickhouse_select_query_rows_read",
		Help:    "Number of rows read by query",
		Buckets: []float64{1000000, 10000000, 50000000, 100000000, 500000000, 1000000000, 2000000000, 3000000000, 10000000000},
	})

	chSelectQueryBytesRead = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "clickhouse_select_query_bytes_read",
		Help:    "Bytes read by query",
		Buckets: []float64{5368709120, 10737418240, 53687091200, 107374182400, 536870912000, 1073741824000},
	})

	chSelectQueryRowsPerSecond = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "clickhouse_select_query_rows_per_second",
		Help:    "Rows Per Second speed by query",
		Buckets: []float64{50000, 100000, 500000, 1000000, 2000000, 5000000, 10000000, 50000000, 100000000, 1000000000},
	})

	chSelectQueryBytesPerSecond = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "clickhouse_select_query_bytes_per_second",
		Help:    "Bytes Per Second speed by query",
		Buckets: []float64{104857600, 524288000, 1073741824, 5368709120, 21474836480, 53687091200},
	})
)

func init() {
	prometheus.MustRegister(chLogExporterErrors, readLines, chQueryErrors, chQueryCount, chQueryTime,
		chSelectQueryRowsRead, chSelectQueryBytesRead, chSelectQueryRowsPerSecond, chSelectQueryBytesPerSecond)
}
