package reporters

type Datadog struct{}

func (d Datadog) Report() error {
	return nil
}
