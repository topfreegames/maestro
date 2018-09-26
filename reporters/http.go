// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2018 Top Free Games <backend@tfgco.com>

package reporters

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	handlers "github.com/topfreegames/maestro/reporters/http"
)

// HTTP reports metrics to an http endpoint
type HTTP struct {
	client *HTTPClient
	region string
	logger *logrus.Logger
}

// HTTPClient implements reporters::http.Client interface
type HTTPClient struct {
	client *http.Client
	putURL string
}

// NewHTTPClient ctor
func NewHTTPClient(putURL string) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{},
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
	req, err := http.NewRequest(http.MethodPut, c.putURL, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode > 299 {
		return fmt.Errorf("reporters.HTTP error status code: %d", res.StatusCode)
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
	if prs == false {
		return fmt.Errorf("reportHandler for %s doesn't exist", event)
	}
	opts["tags"] = []string{"maestro", event, h.region}
	if opts["error"] != nil {
		if err, ok := opts["error"].(error); ok {
			opts["error"] = err.Error()
		}
	}
	opts["region"] = h.region
	handler :=
		handlerI.(func(handlers.Client, map[string]interface{}) error)
	err := handler(h.client, opts)
	if err != nil {
		h.logger.Error(err)
	}
	return err
}

// MakeHTTP adds an HTTP struct to the Reporters' singleton
func MakeHTTP(config *viper.Viper, logger *logrus.Logger, r *Reporters) {
	httpR, err := NewHTTP(config, logger)

	if err == nil {
		r.SetReporter("http", httpR)
	}
}

func loadDefaultHTTPConfigs(c *viper.Viper) {
	c.SetDefault("reporters.http.putURL", "http://localhost:8080")
	c.SetDefault("reporters.http.region", "test")
}

// NewHTTP creates an HTTP struct using putURL and region from config
func NewHTTP(config *viper.Viper, logger *logrus.Logger) (*HTTP, error) {
	loadDefaultHTTPConfigs(config)
	putURL := config.GetString("reporters.http.putURL")
	region := config.GetString("reporters.http.region")
	client := NewHTTPClient(putURL)
	httpR := &HTTP{client: client, region: region}
	return httpR, nil
}
