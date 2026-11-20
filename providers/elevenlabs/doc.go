// Package elevenlabs provides an ElevenLabs speech API provider for Iris.
//
// ElevenLabs is a speech-focused provider (text-to-speech synthesis and
// speech-to-text transcription). It implements [core.Provider] so it can be
// registered with [github.com/petal-labs/iris/providers] and wrapped by
// [core.Client], while chat methods ([ElevenLabs.Chat] and
// [ElevenLabs.StreamChat]) return [core.ErrNotSupported].
//
// # Construction and Authentication
//
// Create a provider from an explicit API key or from the ELEVENLABS_API_KEY
// environment variable:
//
//	p, err := elevenlabs.NewFromEnv(
//	    elevenlabs.WithTimeout(30 * time.Second),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	client := core.NewClient(p)
//
// The API key is stored as a [core.Secret] and transmitted in the xi-api-key
// HTTP header. Formatting [Config] or [ElevenLabs] with fmt (String/GoString)
// or marshaling [Config] to JSON redacts the API key and omits custom header
// values.
//
// # Concurrency and Timeouts
//
// [ElevenLabs] is immutable after construction and safe for concurrent use
// across goroutines. Per-provider timeouts configured via [WithTimeout] apply
// to unary operations unless the caller's context already carries a deadline
// or [WithTimeout](0) disables the default timeout.
//
// # Current Limitations
//
// This package currently implements provider configuration, authentication,
// typed error normalization, timeout handling, and model catalog metadata.
// Synchronous TTS, HTTP streaming TTS, file-based Scribe transcription, and
// real-time voice protocols are implemented in subsequent issues.
package elevenlabs
