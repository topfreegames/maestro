package reporters

import "fmt"

type Datadog struct{}

func (d Datadog) Report(str string) error {
	fmt.Println(str)
	return nil
}
