package elevenlabs

import "github.com/petal-labs/iris/core"

// Verified ElevenLabs model identifiers for TTS and Scribe STT.
const (
	// ModelElevenMultilingualV2 is ElevenLabs' Multilingual v2 TTS model.
	ModelElevenMultilingualV2 core.ModelID = "eleven_multilingual_v2"

	// ModelElevenFlashV25 is ElevenLabs' low-latency Flash v2.5 TTS model.
	ModelElevenFlashV25 core.ModelID = "eleven_flash_v2_5"

	// ModelElevenV3 is ElevenLabs' expressive v3 TTS model.
	ModelElevenV3 core.ModelID = "eleven_v3"

	// ModelScribeV1 is ElevenLabs' Scribe v1 speech-to-text model.
	ModelScribeV1 core.ModelID = "scribe_v1"

	// ModelScribeV2 is ElevenLabs' Scribe v2 speech-to-text model.
	ModelScribeV2 core.ModelID = "scribe_v2"
)

// models is the static catalog of known ElevenLabs models.
// Model capabilities are populated only once the corresponding Iris provider
// endpoints are implemented so unverified or unimplemented combinations are
// never advertised as supported.
var models = []core.ModelInfo{
	{
		ID:          ModelElevenMultilingualV2,
		DisplayName: "Eleven Multilingual v2",
		Capabilities: []core.Feature{
			core.FeatureSpeechSynthesis,
			core.FeatureSpeechSynthesisStreaming,
		},
	},
	{
		ID:          ModelElevenFlashV25,
		DisplayName: "Eleven Flash v2.5",
		Capabilities: []core.Feature{
			core.FeatureSpeechSynthesis,
			core.FeatureSpeechSynthesisStreaming,
		},
	},
	{
		ID:          ModelElevenV3,
		DisplayName: "Eleven v3",
		Capabilities: []core.Feature{
			core.FeatureSpeechSynthesis,
			core.FeatureSpeechSynthesisStreaming,
		},
	},
	{
		ID:          ModelScribeV1,
		DisplayName: "Scribe v1",
		Capabilities: []core.Feature{
			core.FeatureSpeechRecognition,
		},
	},
	{
		ID:          ModelScribeV2,
		DisplayName: "Scribe v2",
		Capabilities: []core.Feature{
			core.FeatureSpeechRecognition,
		},
	},
}

var modelRegistry = buildModelRegistry()

func buildModelRegistry() map[core.ModelID]*core.ModelInfo {
	registry := make(map[core.ModelID]*core.ModelInfo, len(models))
	for i := range models {
		registry[models[i].ID] = &models[i]
	}
	return registry
}

// GetModelInfo returns a defensive copy of the [core.ModelInfo] for id, or nil
// if id is not in the ElevenLabs catalog.
func GetModelInfo(id core.ModelID) *core.ModelInfo {
	m, ok := modelRegistry[id]
	if !ok {
		return nil
	}
	cp := cloneModelInfo(*m)
	return &cp
}

func cloneModelInfo(m core.ModelInfo) core.ModelInfo {
	if m.Capabilities != nil {
		caps := make([]core.Feature, len(m.Capabilities))
		copy(caps, m.Capabilities)
		m.Capabilities = caps
	}
	return m
}
