package ollama

// ollamaRequest is the request body for the Ollama chat API.
type ollamaRequest struct {
	Model     string          `json:"model"`
	Messages  []ollamaMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	Tools     []ollamaTool    `json:"tools,omitempty"`
	Think     *bool           `json:"think,omitempty"`
	Format    interface{}     `json:"format,omitempty"`
	Options   *ollamaOptions  `json:"options,omitempty"`
	KeepAlive string          `json:"keep_alive,omitempty"`
}

// ollamaMessage represents a message in the Ollama chat API.
type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Images    []string         `json:"images,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	Thinking  string           `json:"thinking,omitempty"`
	ToolName  string           `json:"tool_name,omitempty"`
}

// ollamaOptions contains model parameters for the Ollama API.
type ollamaOptions struct {
	Temperature float32  `json:"temperature,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
	TopP        float32  `json:"top_p,omitempty"`
	TopK        int      `json:"top_k,omitempty"`
	Seed        int      `json:"seed,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

// ollamaResponse is the response from the Ollama chat API.
type ollamaResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            ollamaMessage `json:"message"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason,omitempty"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
	Error              string        `json:"error,omitempty"`
}

// ollamaToolCall represents a tool call from the model.
type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

// ollamaFunctionCall contains the function name and arguments.
type ollamaFunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ollamaTool represents a tool definition for the Ollama API.
type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

// ollamaToolFunction defines a function that can be called by the model.
type ollamaToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ollamaErrorResponse is the error response format from Ollama.
type ollamaErrorResponse struct {
	Error string `json:"error"`
}
