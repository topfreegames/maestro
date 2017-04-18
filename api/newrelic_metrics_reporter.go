// maestro
// https://github.com/topfree/ames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api

import newrelic "github.com/newrelic/go-agent"

//NewRelicMetricsReporter reports metrics to new relic
type NewRelicMetricsReporter struct {
	App         *App
	Transaction newrelic.Transaction
}

//StartSegment starts segment
func (r *NewRelicMetricsReporter) StartSegment(name string) map[string]interface{} {
	if r.Transaction == nil {
		return nil
	}

	return map[string]interface{}{
		"segment": newrelic.StartSegment(r.Transaction, name),
	}
}

//EndSegment stops segment
func (r *NewRelicMetricsReporter) EndSegment(data map[string]interface{}, name string) {
	if r.Transaction == nil {
		return
	}

	data["segment"].(newrelic.Segment).End()
}

//StartDatastoreSegment starts segment
func (r *NewRelicMetricsReporter) StartDatastoreSegment(datastore, table, operation string) map[string]interface{} {
	if r.Transaction == nil {
		return nil
	}

	db := newrelic.DatastoreProduct(datastore)

	s := newrelic.DatastoreSegment{
		Product:    db,
		Collection: table,
		Operation:  operation,
	}
	s.StartTime = newrelic.StartSegmentNow(r.Transaction)

	return map[string]interface{}{
		"segment": s,
	}
}

//EndDatastoreSegment stops segment
func (r *NewRelicMetricsReporter) EndDatastoreSegment(data map[string]interface{}) {
	if r.Transaction == nil {
		return
	}

	data["segment"].(newrelic.DatastoreSegment).End()
}

//StartExternalSegment starts segment
func (r *NewRelicMetricsReporter) StartExternalSegment(url string) map[string]interface{} {
	if r.Transaction == nil {
		return nil
	}

	s := newrelic.ExternalSegment{
		StartTime: newrelic.StartSegmentNow(r.Transaction),
		URL:       url,
	}

	return map[string]interface{}{
		"segment": s,
	}
}

//EndExternalSegment stops segment
func (r *NewRelicMetricsReporter) EndExternalSegment(data map[string]interface{}) {
	if r.Transaction == nil {
		return
	}

	data["segment"].(newrelic.ExternalSegment).End()
}
