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

package entities

import (
	"fmt"
	"strconv"
	"strings"
)

// PortRange represents the which ports will be available for allocation.
type PortRange struct {
	Start int32 `validate:"ltfield=End"`
	End   int32 `validate:"gtfield=Start"`
}

func NewPortRange(start int32, end int32) *PortRange {
	return &PortRange{
		Start: start,
		End:   end}
}

// Total returns the total ports available.
func (p *PortRange) Total() int32 {
	return (p.End - p.Start) + 1
}

// ParsePortRange parses the provided string into a PortRange. The string must
// follow the format:
//   - Two positive numbers (start and end) separated by an "-" (hyphen);
//   - End must equal or higher than the start, if it is lower the function will
//     return an error;
//
// Some examples are:
//   - "1000-2000": {Start: 1000, End: 2000};
//   - "1000-1000": {Start: 1000, End: 1000};
//   - "1000-999": Returns an error;
//   - "1000": Returns an error;
//   - "abc": Returns an error;
func ParsePortRange(rangeStr string) (*PortRange, error) {
	split := strings.Split(rangeStr, "-")
	if len(split) != 2 {
		return nil, fmt.Errorf("invalid port range format it must follow \"start-end\"")
	}

	start, err := strconv.Atoi(split[0])
	if err != nil {
		return nil, fmt.Errorf("failed to covert start, invalid port range value: %s", err)
	}

	end, err := strconv.Atoi(split[1])
	if err != nil {
		return nil, fmt.Errorf("failed to covert end, invalid port range value: %s", err)
	}

	if start > end {
		return nil, fmt.Errorf("port range end must be higher than start")
	}

	return &PortRange{Start: int32(start), End: int32(end)}, nil
}
