//go:build ignore

// gen/main.go runs exiftool on all test video files and saves
// the JSON output as golden files in testdata/.
//
//go:generate go run main.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	testdataDir := filepath.Join("..", "testdata")
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read testdata: %v\n", err)
		os.Exit(1)
	}

	videoExts := map[string]bool{
		".mp4": true,
		".mov": true,
		".m4v": true,
	}

	generated := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !videoExts[ext] {
			continue
		}

		videoPath := filepath.Join(testdataDir, e.Name())
		goldenPath := filepath.Join(testdataDir, e.Name()+".exiftool.json")

		// Run exiftool -n -json to get numeric values.
		cmd := exec.Command("exiftool", "-n", "-json", "-g", videoPath)
		output, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "exiftool %s: %v\n", e.Name(), err)
			continue
		}

		if err := os.WriteFile(goldenPath, output, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", goldenPath, err)
			os.Exit(1)
		}
		fmt.Printf("generated %s\n", goldenPath)
		generated++
	}

	fmt.Printf("done: %d golden files generated\n", generated)
}
