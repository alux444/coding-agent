package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

var CodeSearchDefinition = ToolDefinition{
	Name: "code_search",
	Description: `Search for code patterns using ripgrep (rg)

	Use this to find code patters, function definitions, variable usage or any text in the codebase.
	Search by pattern, file type or directory
	`,
	InputSchema: CodeSearchInputSchema,
	Function:    CodeSearch,
}

type CodeSearchInput struct {
	Pattern       string `json:"pattern" jsonschema_description:"The search pattern. Can be a regex pattern supported by ripgrep."`
	Path          string `json:"path,omitempty" jsonschema_description:"Optional path to search in (file or directory)"`
	FileType      string `json:"file_type,omitempty" jsonschema_description:"Optional file type to search for (e.g. 'go', 'js', 'py')"`
	CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema_description:"Whether the search should be case sensitive. Defaults to false."`
}

var CodeSearchInputSchema = GenerateSchema[CodeSearchInput]()

func CodeSearch(input json.RawMessage) (string, error) {
	codeSearchInput := CodeSearchInput{}
	err := json.Unmarshal(input, &codeSearchInput)
	if err != nil {
		return "", err
	}

	if codeSearchInput.Pattern == "" {
		log.Printf("CodeSearch failed: empty search pattern")
		return "", nil
	}

	log.Printf("Searching for pattern: '%s' in path: '%s' with file type: '%s'", codeSearchInput.Pattern, codeSearchInput.Path, codeSearchInput.FileType)

	args := []string{"rg", "--line-number", "--with-filename", "--color=never"}

	if !codeSearchInput.CaseSensitive {
		args = append(args, "--ignore-case")
	}

	if codeSearchInput.FileType != "" {
		args = append(args, "--type", codeSearchInput.FileType)
	}

	args = append(args, codeSearchInput.Pattern)

	if codeSearchInput.Path != "" {
		args = append(args, codeSearchInput.Path)
	} else {
		args = append(args, ".")
	}

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.Output()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			log.Printf("No matches found for pattern: '%s'", codeSearchInput.Pattern)
			return "", nil
		}
		log.Printf("CodeSearch command failed: %v", err)
		return "", fmt.Errorf("code search failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	lines := strings.Split(result, "\n")

	if len(lines) > 20 {
		log.Printf("CodeSearch found %d results, truncating to 20", len(lines))
		lines = lines[:20]
	}

	finalResult := strings.Join(lines, "\n")
	log.Printf("CodeSearch found %d results for pattern: '%s'", len(lines), codeSearchInput.Pattern)
	return finalResult, nil
}
