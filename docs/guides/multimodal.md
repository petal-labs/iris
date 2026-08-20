# Multimodal Messages and Image Generation

Sending images to a model, and generating or editing images with providers that
support it.

## Multimodal Messages

Send text and images together with `UserMultimodal` or the `UserWithImageURL` convenience method:

```go
resp, err := client.Chat(model).
    UserMultimodal().
        Text("What's in this image?").
        ImageURLWithDetail("https://example.com/image.jpg", core.ImageDetailHigh).
        Done().
    GetResponse(ctx)
```

Image URLs and base64 data URLs are mapped for Anthropic, OpenAI Chat Completions and Responses, Azure AI Foundry, and Gemini. File IDs and document parts remain endpoint-specific. When requests execute through `core.Client`, configure `WithWarningHandler` to observe any part that the selected provider, model endpoint, or message role cannot transmit; the client invokes that handler before an unsupported part is omitted.

## Image Generation

Generate images using OpenAI's image models:

```go
provider := openai.New(os.Getenv("OPENAI_API_KEY"))

// Generate an image
resp, err := provider.GenerateImage(ctx, &core.ImageGenerateRequest{
    Model:   openai.ModelGPTImage2,
    Prompt:  "A serene mountain landscape at sunset",
    Size:    core.ImageSize1024x1024,
    Quality: core.ImageQualityHigh,
})

// Save the image
data, _ := resp.Data[0].GetBytes()
os.WriteFile("landscape.png", data, 0644)
```

### Streaming Partial Images

```go
stream, _ := provider.StreamImage(ctx, &core.ImageGenerateRequest{
    Model:         openai.ModelGPTImage1,
    Prompt:        "A futuristic cityscape",
    PartialImages: 3,
})

for chunk := range stream.Ch {
    // Process partial image
    fmt.Printf("Partial %d received\n", chunk.PartialImageIndex)
}

final := <-stream.Final
// Save final image
```

### Editing Images

```go
imageData, _ := os.ReadFile("input.png")

resp, _ := provider.EditImage(ctx, &core.ImageEditRequest{
    Model:  openai.ModelGPTImage1,
    Prompt: "Add a rainbow in the sky",
    Images: []core.ImageInput{
        {Data: imageData},
    },
    InputFidelity: core.ImageInputFidelityHigh,
})
```

### Supported Image Models

| Model | Description |
|-------|-------------|
| `gpt-image-2` | Latest GPT Image model |
| `gpt-image-1.5` | GPT Image 1.5 |
| `gpt-image-1` | Standard GPT Image |
| `gpt-image-1-mini` | Fast, cost-effective |
| `dall-e-3` | High quality (deprecated May 2026) |
| `dall-e-2` | Lower cost, inpainting (deprecated May 2026) |

---

See also: [Timeouts and Warning Hooks](timeouts-and-warnings.md) · [Documentation index](../README.md)
