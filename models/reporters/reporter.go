package reporters

type Reporter interface {
	Report() error
}

type Reporters struct {
	Reporters []Reporter
}

func (r Reporters) Report() error {
	for _, reporter := range r.Reporters {
		reporter.Report()
	}
	return nil
}
