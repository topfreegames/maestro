package operations

// Definition is the operation parameters. It must be able to encode/decode
// itself.
type Definition interface {
	// Marshal encodes the definition to be stored.
	Marshal() []byte
	// Unmarshal decodes the definition into itself.
	Unmarshal(raw []byte) error
	// Name returns the definition name. This is used to compare and identify it
	// amond other definitions.
	Name() string
}
