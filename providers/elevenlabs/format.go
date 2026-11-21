package elevenlabs

import (
	"fmt"
	"mime"

	"github.com/petal-labs/iris/v2/core"
)

// DefaultTTSFormat is the default audio format used for ElevenLabs TTS requests
// when SpeechRequest.Format is zero-valued.
var DefaultTTSFormat = core.AudioFormatMP3_44100_128

// resolveOutputFormat maps a provider-agnostic core.AudioFormat to ElevenLabs'
// output_format query parameter and returns the canonical resolved AudioFormat.
//
// A zero-value format resolves to DefaultTTSFormat ("mp3_44100_128").
// Unsupported encodings, sample rates, bitrates, or multi-channel requests
// return an error wrapping [core.ErrInvalidAudioFormat].
func resolveOutputFormat(f core.AudioFormat) (string, core.AudioFormat, error) {
	if f.IsZero() {
		return "mp3_44100_128", DefaultTTSFormat, nil
	}
	if err := f.Validate(); err != nil {
		return "", core.AudioFormat{}, err
	}
	if f.Channels > 1 {
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs TTS only supports mono (1 channel), got %d", core.ErrInvalidAudioFormat, f.Channels)
	}

	switch f.Encoding {
	case core.AudioEncodingMP3:
		return resolveMP3Format(f)
	case core.AudioEncodingPCM:
		return resolvePCMFormat(f)
	case core.AudioEncodingULaw:
		if f.SampleRate != 0 && f.SampleRate != 8000 {
			return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs ulaw requires 8000Hz sample rate, got %dHz", core.ErrInvalidAudioFormat, f.SampleRate)
		}
		return "ulaw_8000", core.AudioFormatULaw8000, nil
	case core.AudioEncodingALaw:
		if f.SampleRate != 0 && f.SampleRate != 8000 {
			return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs alaw requires 8000Hz sample rate, got %dHz", core.ErrInvalidAudioFormat, f.SampleRate)
		}
		return "alaw_8000", core.AudioFormatALaw8000, nil
	case core.AudioEncodingOpus:
		return resolveOpusFormat(f)
	default:
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs TTS does not support encoding %q", core.ErrInvalidAudioFormat, f.Encoding)
	}
}

func resolveMP3Format(f core.AudioFormat) (string, core.AudioFormat, error) {
	sr := f.SampleRate
	br := f.BitRate
	if sr == 0 {
		sr = 44100
	}
	switch sr {
	case 22050:
		if br == 0 || br == 32 {
			return "mp3_22050_32", core.AudioFormatMP3_22050_32, nil
		}
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs mp3 at 22050Hz only supports 32kbps, got %dkbps", core.ErrInvalidAudioFormat, br)
	case 44100:
		if br == 0 {
			br = 128
		}
		switch br {
		case 32, 64, 96, 128, 192:
			resolved := core.AudioFormat{
				Encoding:   core.AudioEncodingMP3,
				SampleRate: 44100,
				BitRate:    br,
				Channels:   1,
				MIMEType:   core.AudioEncodingMP3.MIMEType(),
			}
			return fmt.Sprintf("mp3_44100_%d", br), resolved, nil
		default:
			return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs mp3 at 44100Hz does not support %dkbps", core.ErrInvalidAudioFormat, br)
		}
	default:
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs mp3 does not support %dHz sample rate", core.ErrInvalidAudioFormat, sr)
	}
}

func resolvePCMFormat(f core.AudioFormat) (string, core.AudioFormat, error) {
	sr := f.SampleRate
	if sr == 0 {
		sr = 16000
	}
	switch sr {
	case 8000, 16000, 22050, 24000, 44100, 48000:
		resolved := core.AudioFormat{
			Encoding:   core.AudioEncodingPCM,
			SampleRate: sr,
			Channels:   1,
			MIMEType:   core.AudioEncodingPCM.MIMEType(),
		}
		return fmt.Sprintf("pcm_%d", sr), resolved, nil
	default:
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs pcm does not support %dHz sample rate", core.ErrInvalidAudioFormat, sr)
	}
}

func resolveOpusFormat(f core.AudioFormat) (string, core.AudioFormat, error) {
	sr := f.SampleRate
	if sr == 0 {
		sr = 48000
	}
	if sr != 48000 {
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs opus requires 48000Hz sample rate, got %dHz", core.ErrInvalidAudioFormat, sr)
	}
	br := f.BitRate
	if br == 0 {
		br = 64
	}
	switch br {
	case 32, 64, 96, 128, 192:
		resolved := core.AudioFormat{
			Encoding:   core.AudioEncodingOpus,
			SampleRate: 48000,
			BitRate:    br,
			Channels:   1,
			MIMEType:   core.AudioEncodingOpus.MIMEType(),
		}
		return fmt.Sprintf("opus_48000_%d", br), resolved, nil
	default:
		return "", core.AudioFormat{}, fmt.Errorf("%w: elevenlabs opus at 48000Hz does not support %dkbps", core.ErrInvalidAudioFormat, br)
	}
}

func responseMIMEType(contentType string, fallback string) string {
	if contentType == "" {
		return fallback
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType == "" {
		return fallback
	}
	return mediaType
}
