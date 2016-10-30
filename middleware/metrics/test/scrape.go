// Adapted by Miek Gieben for CoreDNS testing.
//
// License from prom2json
// Copyright 2014 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package test will scrape a target and you can inspect the variables.
// Basic usage:
//
//	result := Scrape("http://localhost:9153/metrics")
//	v := MetricValue("coredns_cache_capacity", result)
//
package test

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"testing"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/prometheus/common/expfmt"

	dto "github.com/prometheus/client_model/go"
)

type (
	// MetricFamily holds a prometheus metric.
	MetricFamily struct {
		Name    string        `json:"name"`
		Help    string        `json:"help"`
		Type    string        `json:"type"`
		Metrics []interface{} `json:"metrics,omitempty"` // Either metric or summary.
	}

	// metric is for all "single value" metrics.
	metric struct {
		Labels map[string]string `json:"labels,omitempty"`
		Value  string            `json:"value"`
	}

	summary struct {
		Labels    map[string]string `json:"labels,omitempty"`
		Quantiles map[string]string `json:"quantiles,omitempty"`
		Count     string            `json:"count"`
		Sum       string            `json:"sum"`
	}

	histogram struct {
		Labels  map[string]string `json:"labels,omitempty"`
		Buckets map[string]string `json:"buckets,omitempty"`
		Count   string            `json:"count"`
		Sum     string            `json:"sum"`
	}
)

// Scrape returns the all the vars a []*metricFamily.
func Scrape(t *testing.T, url string) []*MetricFamily {
	mfChan := make(chan *dto.MetricFamily, 1024)

	go fetchMetricFamilies(t, url, mfChan)

	result := []*MetricFamily{}
	for mf := range mfChan {
		result = append(result, newMetricFamily(mf))
	}
	return result
}

// MetricValue returns the value associated with name as a string as well as the labels.
// It only returns the first metrics of the slice.
func MetricValue(name string, mfs []*MetricFamily) (string, map[string]string) {
	for _, mf := range mfs {
		if mf.Name == name {
			// Only works with Gauge and Counter...
			return mf.Metrics[0].(metric).Value, mf.Metrics[0].(metric).Labels
		}
	}
	return "", nil
}

// MetricValueLabel returns the value for name *and* label *value*.
func MetricValueLabel(name, label string, mfs []*MetricFamily) (string, map[string]string) {
	// bit hacky is this really handy...?
	for _, mf := range mfs {
		if mf.Name == name {
			for _, m := range mf.Metrics {
				for _, v := range m.(metric).Labels {
					if v == label {
						return m.(metric).Value, m.(metric).Labels
					}
				}

			}
		}
	}
	return "", nil
}

func newMetricFamily(dtoMF *dto.MetricFamily) *MetricFamily {
	mf := &MetricFamily{
		Name:    dtoMF.GetName(),
		Help:    dtoMF.GetHelp(),
		Type:    dtoMF.GetType().String(),
		Metrics: make([]interface{}, len(dtoMF.Metric)),
	}
	for i, m := range dtoMF.Metric {
		if dtoMF.GetType() == dto.MetricType_SUMMARY {
			mf.Metrics[i] = summary{
				Labels:    makeLabels(m),
				Quantiles: makeQuantiles(m),
				Count:     fmt.Sprint(m.GetSummary().GetSampleCount()),
				Sum:       fmt.Sprint(m.GetSummary().GetSampleSum()),
			}
		} else if dtoMF.GetType() == dto.MetricType_HISTOGRAM {
			mf.Metrics[i] = histogram{
				Labels:  makeLabels(m),
				Buckets: makeBuckets(m),
				Count:   fmt.Sprint(m.GetHistogram().GetSampleCount()),
				Sum:     fmt.Sprint(m.GetSummary().GetSampleSum()),
			}
		} else {
			mf.Metrics[i] = metric{
				Labels: makeLabels(m),
				Value:  fmt.Sprint(value(m)),
			}
		}
	}
	return mf
}

func value(m *dto.Metric) float64 {
	if m.Gauge != nil {
		return m.GetGauge().GetValue()
	}
	if m.Counter != nil {
		return m.GetCounter().GetValue()
	}
	if m.Untyped != nil {
		return m.GetUntyped().GetValue()
	}
	return 0.
}

func makeLabels(m *dto.Metric) map[string]string {
	result := map[string]string{}
	for _, lp := range m.Label {
		result[lp.GetName()] = lp.GetValue()
	}
	return result
}

func makeQuantiles(m *dto.Metric) map[string]string {
	result := map[string]string{}
	for _, q := range m.GetSummary().Quantile {
		result[fmt.Sprint(q.GetQuantile())] = fmt.Sprint(q.GetValue())
	}
	return result
}

func makeBuckets(m *dto.Metric) map[string]string {
	result := map[string]string{}
	for _, b := range m.GetHistogram().Bucket {
		result[fmt.Sprint(b.GetUpperBound())] = fmt.Sprint(b.GetCumulativeCount())
	}
	return result
}

func fetchMetricFamilies(t *testing.T, url string, ch chan<- *dto.MetricFamily) {
	defer close(ch)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("creating GET request for URL %q failed: %s", url, err)
	}
	req.Header.Add("Accept", acceptHeader)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("executing GET request for URL %q failed: %s", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET request for URL %q returned HTTP status %s", url, resp.Status)
	}

	mediatype, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err == nil && mediatype == "application/vnd.google.protobuf" &&
		params["encoding"] == "delimited" &&
		params["proto"] == "io.prometheus.client.MetricFamily" {
		for {
			mf := &dto.MetricFamily{}
			if _, err = pbutil.ReadDelimited(resp.Body, mf); err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("reading metric family protocol buffer failed: %s", err)
			}
			ch <- mf
		}
	} else {
		// We could do further content-type checks here, but the
		// fallback for now will anyway be the text format
		// version 0.0.4, so just go for it and see if it works.
		var parser expfmt.TextParser
		metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
		if err != nil {
			t.Fatal("reading text format failed:", err)
		}
		for _, mf := range metricFamilies {
			ch <- mf
		}
	}
}

const acceptHeader = `application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited;q=0.7,text/plain;version=0.0.4;q=0.3`
