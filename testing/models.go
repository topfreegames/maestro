// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package testing

import (
	"fmt"

	"github.com/topfreegames/maestro/models"
)

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

// SchedulerEventMatcher matches SchedulerEvent based on the information passed.
type SchedulerEventMatcher struct {
	ExpectedName          string
	ExpectedSchedulerName string
	ExpectedMetadata      map[string]interface{}
}

func (sem *SchedulerEventMatcher) Matches(x interface{}) bool {
	event := x.(*models.SchedulerEvent)

	if sem.ExpectedName != "" && event.Name != sem.ExpectedName {
		return false
	}

	if sem.ExpectedSchedulerName != "" && event.SchedulerName != sem.ExpectedSchedulerName {
		return false
	}

	if len(sem.ExpectedMetadata) > 0 {
		for name, value := range sem.ExpectedMetadata {
			eventMetadataValue, ok := event.Metadata[name]
			if !ok || eventMetadataValue != value {
				return false
			}
		}
	}

	return true
}

func (sem *SchedulerEventMatcher) String() string {
	return fmt.Sprintf(
		"expected event \"%s\", schedulerName \"%s\" and metadata %v",
		sem.ExpectedName,
		sem.ExpectedSchedulerName,
		sem.ExpectedMetadata,
	)
}
