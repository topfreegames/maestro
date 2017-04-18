// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

//MixedMetricsReporter calls other metrics reporters
type MixedMetricsReporter struct {
	MetricsReporters []MetricsReporter
	Func             func(name string, f func() error) error
}

//NewMixedMetricsReporter ctor
func NewMixedMetricsReporter() *MixedMetricsReporter {
	return &MixedMetricsReporter{
		MetricsReporters: []MetricsReporter{},
	}
}

//WithSegment that calls all the other metrics reporters
func (m *MixedMetricsReporter) WithSegment(name string, f func() error) error {
	if m == nil {
		return f()
	}

	for _, mr := range m.MetricsReporters {
		data := mr.StartSegment(name)
		defer mr.EndSegment(data, name)
	}

	return f()
}

//WithDatastoreSegment that calls all the other metrics reporters
func (m *MixedMetricsReporter) WithDatastoreSegment(table, operation string, f func() error) error {
	if m == nil {
		return f()
	}

	for _, mr := range m.MetricsReporters {
		data := mr.StartDatastoreSegment("Postgres", table, operation)
		defer mr.EndDatastoreSegment(data)
	}

	return f()
}

//WithRedisSegment with redis segment
func (m *MixedMetricsReporter) WithRedisSegment(operation string, f func() error) error {
	if m == nil {
		return f()
	}

	for _, mr := range m.MetricsReporters {
		data := mr.StartDatastoreSegment("Redis", "redis", operation)
		defer mr.EndDatastoreSegment(data)
	}

	return f()
}

//WithExternalSegment that calls all the other metrics reporters
func (m *MixedMetricsReporter) WithExternalSegment(url string, f func() error) error {
	if m == nil {
		return f()
	}

	for _, mr := range m.MetricsReporters {
		data := mr.StartExternalSegment(url)
		defer mr.EndExternalSegment(data)
	}

	return f()
}

//AddReporter to metrics reporter
func (m *MixedMetricsReporter) AddReporter(mr MetricsReporter) {
	m.MetricsReporters = append(m.MetricsReporters, mr)
}
