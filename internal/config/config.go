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

package config

import "time"

// Config is used to fetch configurations using paths. The interface provides a
// a way to fetch configurations in specific types. Paths are set in strings and
// can have separated "scopes" using ".". An example of path would be:
// "api.metrics.enabled".
type Config interface {
	// GetString returns the configuration path as a string. Default: ""
	GetString(string) string
	// GetInt returns the configuration path as an int. Default: 0
	GetInt(string) int
	// GetFloat64 returns the configuration path as a float64. Default: 0.0
	GetFloat64(string) float64
	// GetBool returns the configuration path as a boolean. Default: false
	GetBool(string) bool
	// GetDuration returns a time.Duration of the config. Default: 0
	GetDuration(string) time.Duration
}
