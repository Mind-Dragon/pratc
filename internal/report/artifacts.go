package report

import (
	"fmt"
	"os"
)

func readAnalyzeArtifact(inputDir string) ([]byte, error) {
	for _, name := range []string{"step-2-analyze.json", "analyze.json"} {
		path := inputDir + "/" + name
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read analyze file %s: %w", path, err)
		}
	}
	return nil, fmt.Errorf("failed to read analyze file: missing step-2-analyze.json or analyze.json")
}
