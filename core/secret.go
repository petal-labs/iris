package core

// Secret wraps a sensitive string value with protection against accidental logging.
// The underlying value is never exposed through String(), GoString(), or JSON marshaling.
//
// Use Expose() to access the actual value when needed (e.g., for HTTP headers).
//
// Example:
//
//	secret := NewSecret("sk-abc123")
//	fmt.Println(secret)        // prints: [REDACTED]
//	fmt.Printf("%#v", secret)  // prints: core.Secret{[REDACTED]}
//	secret.Expose()            // returns: "sk-abc123"
type Secret struct {
	value string
}

// NewSecret creates a new Secret from a string value.
func NewSecret(value string) Secret {
	return Secret{value: value}
}

// String returns a redacted placeholder.
// This prevents accidental logging of the secret value.
// Implements fmt.Stringer.
func (s Secret) String() string {
	return "[REDACTED]"
}

// GoString returns a redacted placeholder for %#v formatting.
// Implements fmt.GoStringer.
func (s Secret) GoString() string {
	return "core.Secret{[REDACTED]}"
}

// MarshalJSON returns a redacted JSON string.
// This prevents accidental JSON serialization of the secret value.
func (s Secret) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED]"`), nil
}

// MarshalText returns a redacted text representation.
// This prevents accidental text serialization (e.g., in YAML).
// Implements encoding.TextMarshaler.
func (s Secret) MarshalText() ([]byte, error) {
	return []byte("[REDACTED]"), nil
}

// Expose returns the actual secret value.
// Use this only when the value is genuinely needed (e.g., for authentication headers).
//
// Security note: Be careful not to log or serialize the returned value.
func (s Secret) Expose() string {
	return s.value
}

// IsEmpty returns true if the secret value is empty.
func (s Secret) IsEmpty() bool {
	return s.value == ""
}
