// Example: Multi-turn Conversation
//
// This example demonstrates how to maintain context across
// multiple turns in a conversation by including previous messages.
//
// Run with:
//
//	export OPENAI_API_KEY=your-key
//	go run main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/petal-labs/iris/core"
	"github.com/petal-labs/iris/providers/openai"
)

// Conversation maintains the message history
type Conversation struct {
	client   *core.Client
	model    core.ModelID
	messages []core.Message
}

func NewConversation(client *core.Client, model core.ModelID, systemPrompt string) *Conversation {
	conv := &Conversation{
		client:   client,
		model:    model,
		messages: []core.Message{},
	}
	if systemPrompt != "" {
		conv.messages = append(conv.messages, core.Message{
			Role:    core.RoleSystem,
			Content: systemPrompt,
		})
	}
	return conv
}

func (c *Conversation) Send(ctx context.Context, userMessage string) (string, error) {
	// Add user message to history
	c.messages = append(c.messages, core.Message{
		Role:    core.RoleUser,
		Content: userMessage,
	})

	// Build request with full history
	builder := c.client.Chat(c.model)
	for _, msg := range c.messages {
		switch msg.Role {
		case core.RoleSystem:
			builder = builder.System(msg.Content)
		case core.RoleUser:
			builder = builder.User(msg.Content)
		case core.RoleAssistant:
			builder = builder.Assistant(msg.Content)
		}
	}

	// Get response
	resp, err := builder.GetResponse(ctx)
	if err != nil {
		// Remove the failed user message
		c.messages = c.messages[:len(c.messages)-1]
		return "", err
	}

	// Add assistant response to history
	c.messages = append(c.messages, core.Message{
		Role:    core.RoleAssistant,
		Content: resp.Output,
	})

	return resp.Output, nil
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY environment variable not set")
		os.Exit(1)
	}

	provider := openai.New(apiKey)
	client := core.NewClient(provider)

	// Create conversation with system prompt
	conv := NewConversation(
		client,
		"gpt-4o-mini",
		"You are a helpful programming tutor. Keep responses concise but informative. Remember the context of our conversation.",
	)

	fmt.Println("Chat with the AI (type 'quit' to exit)")
	fmt.Println("The assistant remembers previous messages in the conversation.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if strings.ToLower(input) == "quit" {
			fmt.Println("Goodbye!")
			break
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		response, err := conv.Send(ctx, input)
		cancel()

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Printf("Assistant: %s\n\n", response)
	}
}
