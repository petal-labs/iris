// Package gemini provides a Google Gemini API provider implementation for Iris.
package gemini

import "encoding/json"

// geminiRequest represents a request to the Gemini generateContent API.
type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"system_instruction,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
	Tools             []geminiTool     `json:"tools,omitempty"`
}

// geminiContent represents a content block (user or model turn).
type geminiContent struct {
	Role  string       `json:"role,omitempty"` // "user" or "model"
	Parts []geminiPart `json:"parts"`
}

// geminiPart represents a part within content (text, function call, image, etc).
type geminiPart struct {
	Text             string              `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResp `json:"functionResponse,omitempty"`
	Thought          *bool               `json:"thought,omitempty"`
	InlineData       *geminiInlineData   `json:"inlineData,omitempty"`
	FileData         *geminiFileData     `json:"fileData,omitempty"`
}

// geminiFileData represents a reference to an uploaded file.
type geminiFileData struct {
	MimeType string `json:"mime_type,omitempty"`
	FileURI  string `json:"file_uri"`
}

// geminiGenConfig holds generation configuration.
type geminiGenConfig struct {
	Temperature     *float32           `json:"temperature,omitempty"`
	MaxOutputTokens *int               `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  *geminiThinkConfig `json:"thinkingConfig,omitempty"`
}

// geminiThinkConfig configures thinking/reasoning mode.
type geminiThinkConfig struct {
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`  // Gemini 3: "minimal"/"low"/"medium"/"high"
	ThinkingBudget  *int   `json:"thinkingBudget,omitempty"` // Gemini 2.5: token count
	IncludeThoughts bool   `json:"includeThoughts,omitempty"`
}

// geminiTool represents a tool definition.
type geminiTool struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

// geminiFunctionDecl declares a function the model can call.
type geminiFunctionDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// geminiFunctionCall represents a function call from the model.
type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// geminiFunctionResp provides a function response back to the model.
type geminiFunctionResp struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

// geminiResponse represents a response from the Gemini API.
type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage      `json:"usageMetadata,omitempty"`
}

// geminiCandidate represents a response candidate.
type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

// geminiUsage tracks token usage.
type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	ThoughtsTokenCount   int `json:"thoughtsTokenCount,omitempty"`
}

// geminiErrorResponse represents an error response from the API.
type geminiErrorResponse struct {
	Error geminiError `json:"error"`
}

// geminiError contains error details.
type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}
