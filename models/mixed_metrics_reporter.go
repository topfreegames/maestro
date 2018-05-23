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

	var err error
	data := make([]map[string]interface{}, len(m.MetricsReporters))
	for i, mr := range m.MetricsReporters {
		data[i] = mr.StartSegment(name)
	}
	defer func() {
		for i, mr := range m.MetricsReporters {
			if len(data) > i {
				if v := data[i]; v != nil {
					data[i]["error"] = err != nil
					mr.EndSegment(data[i], name)
				}
			}
		}
	}()

	err = f()
	return err
}

//WithDatastoreSegment that calls all the other metrics reporters
func (m *MixedMetricsReporter) WithDatastoreSegment(table, operation string, f func() error) error {
	if m == nil {
		return f()
	}

	var err error
	data := make([]map[string]interface{}, len(m.MetricsReporters))
	for i, mr := range m.MetricsReporters {
		data[i] = mr.StartDatastoreSegment("Postgres", table, operation)
	}
	defer func() {
		for i, mr := range m.MetricsReporters {
			data[i]["error"] = err != nil
			mr.EndDatastoreSegment(data[i])
		}
	}()

	err = f()
	return err
}

//WithRedisSegment with redis segment
func (m *MixedMetricsReporter) WithRedisSegment(operation string, f func() error) error {
	if m == nil {
		return f()
	}

	var err error
	data := make([]map[string]interface{}, len(m.MetricsReporters))
	for i, mr := range m.MetricsReporters {
		data[i] = mr.StartDatastoreSegment("Redis", "redis", operation)
	}
	defer func() {
		for i, mr := range m.MetricsReporters {
			data[i]["error"] = err != nil
			mr.EndDatastoreSegment(data[i])
		}
	}()

	err = f()
	return err
}

//WithExternalSegment that calls all the other metrics reporters
func (m *MixedMetricsReporter) WithExternalSegment(url string, f func() error) error {
	if m == nil {
		return f()
	}

	var err error
	data := make([]map[string]interface{}, len(m.MetricsReporters))
	for i, mr := range m.MetricsReporters {
		data[i] = mr.StartExternalSegment(url)
	}
	defer func() {
		for i, mr := range m.MetricsReporters {
			data[i]["error"] = err != nil
			mr.EndExternalSegment(data[i])
		}
	}()

	err = f()
	return err
}

//AddReporter to metrics reporter
func (m *MixedMetricsReporter) AddReporter(mr MetricsReporter) {
	m.MetricsReporters = append(m.MetricsReporters, mr)
}
