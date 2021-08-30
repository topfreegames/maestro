// MIT License
//
// Copyright (c) 2021 TFG Co
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package framework

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

type APIClient struct {
	baseAddr    string
	httpClient  *http.Client
	marshaler   *jsonpb.Marshaler
	unmarshaler *jsonpb.Unmarshaler
}

func NewAPIClient(baseAddr string) *APIClient {
	return &APIClient{
		httpClient:  &http.Client{},
		marshaler:   &jsonpb.Marshaler{},
		unmarshaler: &jsonpb.Unmarshaler{},
		baseAddr:    baseAddr,
	}
}

func (c *APIClient) Do(verb, path string, request proto.Message, response proto.Message) error {
	buf := new(bytes.Buffer)
	err := c.marshaler.Marshal(buf, request)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequest(verb, fmt.Sprintf("%s%s", c.baseAddr, path), buf)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make the request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		if body, err := io.ReadAll(resp.Body); err == nil {
			return fmt.Errorf("failed with status %d, response body: %s", resp.StatusCode, string(body))
		}

		return fmt.Errorf("failed with status %d", resp.StatusCode)
	}

	err = c.unmarshaler.Unmarshal(resp.Body, response)
	if err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
