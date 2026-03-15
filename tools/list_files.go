package tools

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories at a given path. If no path is provided, list files in the current working directory",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		panic(err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	log.Printf("Listing files in dir: %s", dir)

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// skip .dev
		if info.IsDir() && (relativePath == ".dev" || strings.HasPrefix(relativePath, ".dev/")) {
			return filepath.SkipDir
		}

		if relativePath != "." {
			if info.IsDir() {
				files = append(files, relativePath+"/")
			} else {
				files = append(files, relativePath)
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Failed to list files in directory %s: %v", dir, err)
		return "", err
	}

	log.Printf("Successfully listed files in directory %s: found %d items", dir, len(files))

	result, err := json.Marshal(files)
	if err != nil {
		log.Printf("Failed to marshal file list result: %v", err)
		return "", err
	}

	return string(result), nil
}
