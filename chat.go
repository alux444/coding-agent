package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"hello/tools"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/joho/godotenv"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, falling back to environment variables")
	}

	log.Println("Starting chat...")

	client := anthropic.NewClient()
	log.Println("Anthropic client started")

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	agentTools := []tools.ToolDefinition{tools.ReadFileDefinition, tools.ListFilesDefinition}
	log.Printf("Initialised %d tools", len(agentTools))

	agent := NewAgent(&client, getUserMessage, agentTools)
	err := agent.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}

type Agent struct {
	client         *anthropic.Client
	getUserMessage func() (string, bool)
	tools          []tools.ToolDefinition
}

func NewAgent(client *anthropic.Client, getUserMessage func() (string, bool), tools []tools.ToolDefinition) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
		tools:          tools,
	}
}

func (agent *Agent) runInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {
	anthropicTools := []anthropic.ToolUnionParam{}
	for _, tool := range agent.tools {
		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: tool.InputSchema,
			},
		})
	}

	MODEL := anthropic.ModelClaudeSonnet4_6
	log.Printf("Making API call to Claude with: %s", MODEL)

	message, err := agent.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     MODEL,
		MaxTokens: int64(1024),
		Messages:  conversation,
		Tools:     anthropicTools,
	})

	if err != nil {
		log.Printf("API call failed: %v", err)
	} else {
		log.Printf("API call successful")
	}

	return message, err
}

func (agent *Agent) Run(ctx context.Context) error {
	conversation := []anthropic.MessageParam{}

	fmt.Println("Chat with Claude (use 'ctrl-c' to quit)")
	log.Println("Starting chat session")

	for {
		userInput, ok := agent.promptUserInput()
		if !ok {
			break
		}
		if userInput == "" {
			continue
		}

		conversation = append(conversation, anthropic.NewUserMessage(anthropic.NewTextBlock(userInput)))

		var err error
		conversation, err = agent.runTurn(ctx, conversation)
		if err != nil {
			return err
		}
	}

	log.Println("Chat session ended")
	return nil
}

func (agent *Agent) promptUserInput() (string, bool) {
	fmt.Print("\u001b[94mYou\u001b[0m: ")
	input, ok := agent.getUserMessage()
	if !ok {
		log.Println("User input ended, breaking from chat")
	} else if input == "" {
		log.Println("Skipping empty message")
	} else {
		log.Printf("User input received: %q", input)
	}
	return input, ok
}

func (agent *Agent) runTurn(ctx context.Context, conversation []anthropic.MessageParam) ([]anthropic.MessageParam, error) {
	message, err := agent.runInference(ctx, conversation)
	if err != nil {
		log.Printf("Error during inference: %v", err)
		return conversation, err
	}
	conversation = append(conversation, message.ToParam())

	for {
		toolResults, hasToolUse := agent.processMessageContent(message)
		if !hasToolUse {
			break
		}

		log.Printf("Sending %d tool results back to Claude", len(toolResults))
		conversation = append(conversation, anthropic.NewUserMessage(toolResults...))

		message, err = agent.runInference(ctx, conversation)
		if err != nil {
			log.Printf("Error during inference after tool use: %v", err)
			return conversation, err
		}
		conversation = append(conversation, message.ToParam())
	}

	return conversation, nil
}

func (agent *Agent) processMessageContent(message *anthropic.Message) ([]anthropic.ContentBlockParamUnion, bool) {
	log.Printf("Processing %d content blocks from Claude", len(message.Content))

	var toolResults []anthropic.ContentBlockParamUnion
	var hasToolUse bool

	for _, content := range message.Content {
		switch content.Type {
		case "text":
			fmt.Printf("\u001b[93mClaude\u001b[0m: %s\n", content.Text)
		case "tool_use":
			hasToolUse = true
			toolResults = append(toolResults, agent.executeToolUse(content.AsToolUse()))
		}
	}

	return toolResults, hasToolUse
}

func (agent *Agent) executeToolUse(toolUse anthropic.ToolUseBlock) anthropic.ContentBlockParamUnion {
	log.Printf("Claude used tool: %s with input: %s", toolUse.Name, string(toolUse.Input))
	fmt.Printf("\u001b[96mtool\u001b[0m: %s(%s)\n", toolUse.Name, string(toolUse.Input))

	result, err := agent.callTool(toolUse.Name, toolUse.Input)
	if err != nil {
		fmt.Printf("\u001b[91merror\u001b[0m: %s\n", err.Error())
		log.Printf("Error executing tool %s: %v", toolUse.Name, err)
		return anthropic.NewToolResultBlock(toolUse.ID, err.Error(), true)
	}

	fmt.Printf("\u001b[92mresult\u001b[0m: %s\n", result)
	log.Printf("Tool %s executed successfully", toolUse.Name)
	return anthropic.NewToolResultBlock(toolUse.ID, result, false)
}

func (agent *Agent) callTool(name string, input json.RawMessage) (string, error) {
	for _, tool := range agent.tools {
		if tool.Name == name {
			log.Printf("Executing tool: %s", name)
			return tool.Function(input)
		}
	}
	return "", fmt.Errorf("tool %s not found", name)
}
