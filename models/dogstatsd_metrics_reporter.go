// maestro
// https://github.com/topfree/ames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/topfreegames/maestro/reporters"
	reportersConstants "github.com/topfreegames/maestro/reporters/constants"
)

//DogStatsdMetricsReporter reports metrics to new relic
type DogStatsdMetricsReporter struct {
	Scheduler string
	Route     string
}

//StartSegment starts segment
func (r *DogStatsdMetricsReporter) StartSegment(name string) map[string]interface{} {
	return map[string]interface{}{
		"startTime": time.Now(),
		"segment":   strings.ToLower(name),
	}
}

//EndSegment stops segment
func (r *DogStatsdMetricsReporter) EndSegment(data map[string]interface{}, name string) {
	elapsedTime := time.Now().Sub(data["startTime"].(time.Time)).String()
	segment := data["segment"].(string)
	tags := map[string]string{
		reportersConstants.TagResponseTime: elapsedTime,
		reportersConstants.TagScheduler:    r.Scheduler,
		reportersConstants.TagSegment:      segment,
		reportersConstants.TagRoute:        r.Route,
	}
	if v, ok := data["error"].(bool); ok {
		tags[reportersConstants.TagError] = fmt.Sprintf("%t", v)
	}
	s := strings.Split(segment, "/")
	if len(s) > 1 {
		tags[reportersConstants.TagType] = s[0]
	}
	reporters.Report(reportersConstants.EventResponseTime, tags)
}

//StartDatastoreSegment starts segment
func (r *DogStatsdMetricsReporter) StartDatastoreSegment(datastore, table, operation string) map[string]interface{} {
	return map[string]interface{}{
		"startTime": time.Now(),
		"datastore": datastore,
		"operation": operation,
		"table":     strings.ToLower(table),
	}
}

//EndDatastoreSegment stops segment
func (r *DogStatsdMetricsReporter) EndDatastoreSegment(data map[string]interface{}) {
	elapsedTime := time.Now().Sub(data["startTime"].(time.Time)).String()
	datastore := data["datastore"].(string)
	operation := data["operation"].(string)
	table := data["table"].(string)
	tags := map[string]string{
		reportersConstants.TagResponseTime: elapsedTime,
		reportersConstants.TagScheduler:    r.Scheduler,
		reportersConstants.TagSegment:      strings.ToLower(fmt.Sprintf("%s/%s", datastore, operation)),
		reportersConstants.TagTable:        table,
		reportersConstants.TagType:         datastore,
		reportersConstants.TagRoute:        r.Route,
	}
	if v, ok := data["error"].(bool); ok {
		tags[reportersConstants.TagError] = fmt.Sprintf("%t", v)
	}
	reporters.Report(reportersConstants.EventResponseTime, tags)
}

//StartExternalSegment starts segment
func (r *DogStatsdMetricsReporter) StartExternalSegment(url string) map[string]interface{} {
	return map[string]interface{}{
		"startTime": time.Now(),
		"segment":   url,
	}
}

//EndExternalSegment stops segment
func (r *DogStatsdMetricsReporter) EndExternalSegment(data map[string]interface{}) {
	elapsedTime := time.Now().Sub(data["startTime"].(time.Time)).String()
	tags := map[string]string{
		reportersConstants.TagResponseTime: elapsedTime,
		reportersConstants.TagScheduler:    r.Scheduler,
		reportersConstants.TagSegment:      data["segment"].(string),
		reportersConstants.TagType:         "external",
		reportersConstants.TagRoute:        r.Route,
		reportersConstants.TagError:        fmt.Sprintf("%t", data["error"].(bool)),
	}
	if v, ok := data["error"].(bool); ok {
		tags[reportersConstants.TagError] = fmt.Sprintf("%t", v)
	}
	reporters.Report(reportersConstants.EventResponseTime, tags)
}
