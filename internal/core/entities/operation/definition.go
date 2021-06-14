package operation

type Definition interface {
	ShouldExecute(currentOperations []Operation) bool

	Marshal() []byte
	Unmarshal() ([]byte, error)
	Name() string
}
