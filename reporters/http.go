// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2018 Top Free Games <backend@tfgco.com>

package reporters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	httpExtensions "github.com/topfreegames/extensions/http"
	handlers "github.com/topfreegames/maestro/reporters/http"
)

// HTTP reports metrics to an http endpoint
type HTTP struct {
	client handlers.Client
	region string
	logger logrus.FieldLogger
}

// HTTPClient implements reporters::http.Client interface
type HTTPClient struct {
	client *http.Client
	putURL string
}

// NewHTTPClient ctor
func NewHTTPClient(putURL string, timeout time.Duration) *HTTPClient {
	client := httpExtensions.New()
	client.Timeout = timeout
	return &HTTPClient{
		client: client,
		putURL: putURL,
	}
}

// Send makes a PUT HTTP call to the underlying client
func (c *HTTPClient) Send(opts map[string]interface{}) error {
	bodyMap := map[string]interface{}{}
	bodyMap["tags"] = opts["tags"]
	delete(opts, "tags")
	bodyMap["metadata"] = opts
	b, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, c.putURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode > 299 {
		return fmt.Errorf("error status code: %d", res.StatusCode)
	}
	return nil
}

// Report finds a matching handler to some 'event' metric and delegates
// further actions to it
// HTTP reports have the following format in json:
// { "tags": [...], "metadata": {...} }
// tags = opts[tags]
// metadata = opts - opts[tags]
func (h *HTTP) Report(event string, opts map[string]interface{}) error {
	handlerI, prs := handlers.Find(event)
	if !prs {
		return fmt.Errorf("reportHandler for %s doesn't exist", event)
	}
	tags := []string{"maestro", event, h.region}
	if game, ok := opts["game"].(string); ok {
		tags = append(tags, game)
	}
	delete(opts, "game")
	opts["tags"] = tags
	if opts["error"] != nil {
		if err, ok := opts["error"].(error); ok {
			opts["error"] = err.Error()
		}
	}
	opts["region"] = h.region
	handler := handlerI.(func(handlers.Client, map[string]interface{}) error)
	err := handler(h.client, opts)
	if err != nil {
		return fmt.Errorf("failed to report %s: %w", event, err)
	}
	return nil
}

// MakeHTTP adds an HTTP struct to the Reporters' singleton
func MakeHTTP(config *viper.Viper, logger logrus.FieldLogger, r *Reporters) {
	httpR, err := NewHTTP(config, logger.WithField("reporter", "http"))
	if err != nil {
		logger.Error(err)
		return
	}
	r.SetReporter("http", httpR)
}

func loadDefaultHTTPConfigs(c *viper.Viper) {
	c.SetDefault("reporters.http.putURL", "http://localhost:8080")
	c.SetDefault("reporters.http.region", "test")
	c.SetDefault("reporters.http.timeout", "5s")
}

// NewHTTP creates an HTTP struct using putURL and region from config
func NewHTTP(config *viper.Viper, logger logrus.FieldLogger) (*HTTP, error) {
	loadDefaultHTTPConfigs(config)
	putURL := config.GetString("reporters.http.putURL")
	region := config.GetString("reporters.http.region")
	timeout := config.GetDuration("reporters.http.timeout")
	client := NewHTTPClient(putURL, timeout)
	httpR := &HTTP{client: client, region: region, logger: logger}
	return httpR, nil
}

// MakeHTTPWithClient is the same as MakeHTTP with the flexibility of
// setting any handlers.Client instead of depending only on the config
func MakeHTTPWithClient(
	client handlers.Client, config *viper.Viper,
	logger *logrus.Logger, r *Reporters,
) {
	httpR := &HTTP{client: client, region: config.GetString("reporters.http.region"), logger: logger}
	r.SetReporter("http", httpR)
}
