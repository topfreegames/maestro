package entities

import (
	"fmt"
	"strconv"
	"strings"
)

const portRangeDelimiter = "-"

// PortRange represents the which ports will be available for allocation.
type PortRange struct {
	Start int32
	End   int32
}

// Total returns the total ports avaiable.
func (p *PortRange) Total() int32 {
	return (p.End - p.Start) + 1
}

// ParsePortRange parses the provided string into a PortRange. The string must
// follow the format:
//   * Two positive numbers (start and end) separated by an "-" (hyphen);
//   * End must equal or higher than the start, if it is lower the function will
//     return an error;
// Some examples are:
//   * "1000-2000": {Start: 1000, End: 2000};
//   * "1000-1000": {Start: 1000, End: 1000};
//   * "1000-999": Returns an error;
//   * "1000": Returns an error;
//   * "abc": Returns an error;
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
