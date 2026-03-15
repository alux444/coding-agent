package tools

import (
	"encoding/json"
	"log"
	"os/exec"
	"strings"
)

var BashDefinition = ToolDefinition{
	Name:        "bash",
	Description: "Execute a bash command and return its output. This is used for running shell commands",
	InputSchema: BashInputSchema,
	Function:    Bash,
}

type BashInput struct {
	Command string `json:"command" jsonschema_description:"The bash command to execute."`
}

var BashInputSchema = GenerateSchema[BashInput]()

func Bash(input json.RawMessage) (string, error) {
	bashInput := BashInput{}
	err := json.Unmarshal(input, &bashInput)
	if err != nil {
		return "", err
	}

	log.Printf("Executing bash command: %s", bashInput.Command)
	cmd := exec.Command("bash", "-c", bashInput.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Bash command failed: %v, output: %s", err, string(output))
		return "", err
	}

	log.Printf("Bash command succeeded, output: %s", string(output))
	return strings.TrimSpace(string(output)), nil
}
