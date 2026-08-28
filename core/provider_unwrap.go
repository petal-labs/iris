package core

const maxProviderUnwrapDepth = 64

// ProviderUnwrapper is implemented by provider decorators that expose the
// provider they wrap. Capability helpers follow this chain so optional
// interfaces remain discoverable through decorators.
type ProviderUnwrapper interface {
	Unwrap() Provider
}

func asProviderCapability[T any](provider Provider) (T, bool) {
	var zero T
	for range maxProviderUnwrapDepth {
		if provider == nil {
			return zero, false
		}
		if capability, ok := any(provider).(T); ok {
			return capability, true
		}
		unwrapper, ok := provider.(ProviderUnwrapper)
		if !ok {
			return zero, false
		}
		provider = unwrapper.Unwrap()
	}
	return zero, false
}
