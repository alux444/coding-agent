package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
)

var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `Make edits to a file.
	
	Replaces 'old_string' with 'new_string' in the given file. 'old_string' and 'new_string' must be different to one another.
	
	If a the specified file with the path doesn't exist, it will be created.`,
	InputSchema: EditFileInputSchema,
	Function:    EditFile,
}

type EditFileInput struct {
	Path      string `json:"path" jsonschema_description:"The path to the file"`
	OldString string `json:"old_string" jsonschema_description:"Text to search for. Must match exactly and must have only one match exactly"`
	NewString string `json:"new_string" jsonschema_description:"Text to replace the old string with."`
}

var EditFileInputSchema = GenerateSchema[EditFileInput]()

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		return "", err
	}

	if editFileInput.Path == "" || editFileInput.OldString == editFileInput.NewString {
		log.Printf("EditFile failed: invalid input: %v", editFileInput)
		return "", fmt.Errorf("invalid input")
	}

	log.Printf("Editing file: %s, replacing '%s' with '%s'", editFileInput.Path, editFileInput.OldString, editFileInput.NewString)
	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		if os.IsNotExist(err) && editFileInput.OldString == "" {
			log.Printf("File does not exist, creating new file: %s", editFileInput.Path)
			return createNewFile(editFileInput.Path, editFileInput.NewString)
		}
		log.Printf("Failed to read file %s: %v", editFileInput.Path, err)
		return "", err
	}

	oldContent := string(content)

	// if old_string is empty, we're appending
	var newContent string
	if editFileInput.OldString == "" {
		newContent = oldContent + editFileInput.NewString
	} else {
		oldStringOccurrences := strings.Count(oldContent, editFileInput.OldString)

		if oldStringOccurrences == 0 {
			log.Printf("Old string '%s' not found in file %s", editFileInput.OldString, editFileInput.Path)
			return "", fmt.Errorf("old string not found in file")
		}

		if oldStringOccurrences > 1 {
			log.Printf("Old string '%s' found %d times in file %s. Expected exactly 1 occurrence.", editFileInput.OldString, oldStringOccurrences, editFileInput.Path)
			return "", fmt.Errorf("old string found multiple times in file")
		}

		newContent = strings.Replace(oldContent, editFileInput.OldString, editFileInput.NewString, 1)
	}

	err = os.WriteFile(editFileInput.Path, []byte(newContent), 0644)
	if err != nil {
		log.Printf("Failed to write file %s: %v", editFileInput.Path, err)
		return "", err
	}

	log.Printf("Successfully edited file %s", editFileInput.Path)
	return "success", nil
}

func createNewFile(filePath string, content string) (string, error) {
	log.Printf("Creating new file: %s", filePath)
	dir := path.Dir(filePath)
	if dir != "." {
		log.Printf("Creating directories for path: %s", dir)

		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.Printf("Failed to create directories for path %s: %v", dir, err)
			return "", fmt.Errorf("failed to create directories for path: %v", err)
		}
	}

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		log.Printf("Failed to create file %s: %v", filePath, err)
		return "", fmt.Errorf("failed to create file: %v", err)
	}

	log.Printf("Successfully created file %s", filePath)
	return "success", nil
}
