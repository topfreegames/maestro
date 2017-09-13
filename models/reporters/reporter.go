package reporters

import (
	"sync"
)

type Reporter interface {
	Report() error
}

type Reporters struct {
	reporters map[string]Reporter
}

func (r *Reporters) SetReporter(key string, value Reporter) {
	r.reporters[key] = value
}

func (r *Reporters) Report() error {
	for _, reporter := range r.reporters {
		reporter.Report()
	}
	return nil
}

var instance *Reporters
var once sync.Once

func GetInstance() *Reporters {
	once.Do(func() {
		instance = &Reporters{
			reporters: make(map[string]Reporter),
		}
	})
	return instance
}
