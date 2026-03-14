package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

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

	agent := NewAgent(&client, getUserMessage)
	err := agent.Run(context.TODO())
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}

type Agent struct {
	client         *anthropic.Client
	getUserMessage func() (string, bool)
}

func NewAgent(client *anthropic.Client, getUserMessage func() (string, bool)) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
	}
}

func (agent *Agent) runInference(ctx context.Context, conversation []anthropic.MessageParam) (*anthropic.Message, error) {
	MODEL := anthropic.ModelClaudeSonnet4_6
	log.Printf("Making API call to Claude with: %s", MODEL)

	message, err := agent.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     MODEL,
		MaxTokens: int64(1024),
		Messages:  conversation,
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

		for _, content := range message.Content {
			switch content.Type {
			case "text":
				fmt.Printf("\u001b[93mClaude\u001b[0m: %s\n", content.Text)
			}
		}
	}

	log.Println("Chat session ended")
	return nil
}
