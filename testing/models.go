// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package testing

//FakeMetricsReporter is a fake metric reporter for testing
type FakeMetricsReporter struct{}

//StartSegment mocks a metric reporter
func (mr FakeMetricsReporter) StartSegment(key string) map[string]interface{} {
	return map[string]interface{}{}
}

//EndSegment mocks a metric reporter
func (mr FakeMetricsReporter) EndSegment(m map[string]interface{}, key string) {}

//StartDatastoreSegment mocks a metric reporter
func (mr FakeMetricsReporter) StartDatastoreSegment(datastore, collection, operation string) map[string]interface{} {
	return map[string]interface{}{}
}

//EndDatastoreSegment mocks a metric reporter
func (mr FakeMetricsReporter) EndDatastoreSegment(m map[string]interface{}) {}

//StartExternalSegment mocks a metric reporter
func (mr FakeMetricsReporter) StartExternalSegment(key string) map[string]interface{} {
	return map[string]interface{}{}
}

//EndExternalSegment mocks a metric reporter
func (mr FakeMetricsReporter) EndExternalSegment(m map[string]interface{}) {}
