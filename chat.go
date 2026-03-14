package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
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

	tools := []ToolDefinition{ReadFileDefinition}
	log.Printf("Initialised %d tools", len(tools))

	agent := NewAgent(&client, getUserMessage, tools)
	err := agent.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}

type Agent struct {
	client         *anthropic.Client
	getUserMessage func() (string, bool)
	tools          []ToolDefinition
}

func NewAgent(client *anthropic.Client, getUserMessage func() (string, bool), tools []ToolDefinition) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
		tools:          tools,
	}
}

type ToolDefinition struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	InputSchema anthropic.ToolInputSchemaParam `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Use this for viewing the contents of a file. Do not use with directory names.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of a file in the working directory"`
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

// produces correct JSON schema by reflecting struct tags at runtime
func GenerateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T

	schema := reflector.Reflect(v)
	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		panic(err)
	}

	log.Printf("Reading file: %s", readFileInput.Path)
	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		log.Printf("Failed to read file %s: %v", readFileInput.Path, err)
		return "", err
	}

	log.Printf("Successfully read file %s (%d bytes)", readFileInput.Path, len(content))
	return string(content), nil
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

	log.Println("Starting chat session")
	fmt.Println("Chat with Claude (use 'ctrl-c' to quit)")

	for {
		blueYou := "\u001b[94mYou\u001b[0m: "
		fmt.Print(blueYou)
		userInput, ok := agent.getUserMessage()
		if !ok {
			log.Println("User input ended, breaking from chat")
			break
		}

		if userInput == "" {
			log.Println("Skipping empty message")
			continue
		}

		log.Printf("User input received: %q", userInput)

		userMessage := anthropic.NewUserMessage(anthropic.NewTextBlock(userInput))
		conversation = append(conversation, userMessage)

		log.Printf("Sending message to Claude")

		message, err := agent.runInference(ctx, conversation)
		if err != nil {
			log.Printf("Error during inference: %v", err)
			return err
		}
		conversation = append(conversation, message.ToParam())

		log.Printf("Received response from Claude - %d content blocks", len(message.Content))
		for {
			var toolResults []anthropic.ContentBlockParamUnion
			var hasToolUse bool

			log.Printf("Processing %d content blocks from Claude", len(message.Content))

			for _, content := range message.Content {
				switch content.Type {
				case "text":
					fmt.Printf("\u001b[93mClaude\u001b[0m: %s\n", content.Text)
				case "tool_use":
					hasToolUse = true
					toolUse := content.AsToolUse()
					log.Printf("Claude used tool: %s with input: %s", toolUse.Name, string(toolUse.Input))
					fmt.Printf("\u001b[96mtool\u001b[0m: %s(%s)\n", toolUse.Name, string(toolUse.Input))

					// find and use tool
					var toolResult string
					var toolError error
					var toolFound bool

					for _, tool := range agent.tools {
						if tool.Name == toolUse.Name {
							log.Printf("Executing tool: %s", tool.Name)
							toolResult, toolError = tool.Function(toolUse.Input)
							fmt.Printf("\u001b[92mresult\u001b[0m: %s\n", toolResult)

							if toolError != nil {
								fmt.Printf("\u001b[91merror\u001b[0m: %s\n", toolError.Error())
								log.Printf("Error executing tool %s: %v", tool.Name, toolError)
							} else {
								log.Printf("Tool %s executed successfully", tool.Name)
							}

							toolFound = true
							break
						}
					}

					if !toolFound {
						toolResult = fmt.Sprintf("Error: tool %s not found", toolUse.Name)
						fmt.Printf("\u001b[91merror\u001b[0m: %s\n", toolError.Error())
					}

					if toolError != nil {
						toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, toolError.Error(), true))
					} else {
						toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, toolResult, false))
					}
				}
			}

			if !hasToolUse {
				break
			}

			log.Printf("Sending %d tool results back to Claude", len(toolResults))
			toolResultMessage := anthropic.NewUserMessage(toolResults...)
			conversation = append(conversation, toolResultMessage)

			message, err = agent.runInference(ctx, conversation)
			if err != nil {
				log.Printf("Error during inference after tool use: %v", err)
				return err
			}
			conversation = append(conversation, message.ToParam())

			log.Printf("Received followup response wth %d content blocks", len(message.Content))
		}

	}
	log.Println("Chat session ended")
	return nil
}
