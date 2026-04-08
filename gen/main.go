//go:build ignore

// gen/main.go regenerates exiftool-based golden JSON files for supported
// video-native metadata groups.
//
//go:generate go run main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	testdataDir       = "../testdata"
	legacyPentaxGroup = "Maker" + "Notes"
	orderedValueSep   = "\x1f"
)

type orderedGoldenFile struct {
	SourceFile string               `json:"sourceFile"`
	Groups     []orderedGoldenGroup `json:"groups"`
}

type orderedGoldenGroup struct {
	Name string             `json:"name"`
	Tags []orderedGoldenTag `json:"tags"`
}

type orderedGoldenTag struct {
	Tag   string `json:"tag"`
	Value any    `json:"value"`
}

var orderedGoldenGroups = map[string]bool{
	"QuickTime": true,
	"XML":       true,
	"Composite": true,
}

var goldenFixtureFiles = []string{
	"IMG_5179.MOV",
	"dji_inspire3_car_4k120_rec709.mov",
	"dji_ronin4d_4k_prores4444_25fps.mov",
	"exiftool_quicktime.mov",
	"google.mp4",
	"gopro_action.mp4",
	"minimal.mp4",
	"nonfaststart.mp4",
	"sony_a6700.mp4",
	"with_audio.mp4",
	"with_gps.mp4",
}

func main() {
	if err := generateGoldenFiles(); err != nil {
		fmt.Fprintf(os.Stderr, "generate golden files: %v\n", err)
		os.Exit(1)
	}
}

func generateGoldenFiles() error {
	generated := 0
	for _, name := range goldenFixtureFiles {
		videoPath := filepath.Join(testdataDir, name)
		if _, err := os.Stat(videoPath); err != nil {
			return fmt.Errorf("stat %s: %w", videoPath, err)
		}
		goldenPath := filepath.Join(testdataDir, name+".exiftool.json")
		orderedGoldenPath := filepath.Join(testdataDir, name+".exiftool.ordered.json")

		cmd := exec.Command("exiftool", "-n", "-json", "-g", "--File:all", "--ExifTool:all", videoPath)
		output, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "exiftool %s: %v\n", name, err)
			continue
		}

		normalizedOutput, err := normalizeGoldenJSON(output)
		if err != nil {
			return fmt.Errorf("normalize %s: %w", goldenPath, err)
		}

		if err := os.WriteFile(goldenPath, normalizedOutput, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", goldenPath, err)
		}
		fmt.Printf("generated %s\n", goldenPath)

		orderedOutput, err := generateOrderedGolden(videoPath)
		if err != nil {
			return fmt.Errorf("generate ordered golden %s: %w", name, err)
		}
		if err := os.WriteFile(orderedGoldenPath, orderedOutput, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", orderedGoldenPath, err)
		}
		fmt.Printf("generated %s\n", orderedGoldenPath)
		generated += 2
	}

	fmt.Printf("done: %d golden files generated\n", generated)
	return nil
}

func normalizeGoldenJSON(raw []byte) ([]byte, error) {
	if !bytes.Contains(raw, []byte(`"`+legacyPentaxGroup+`"`)) &&
		!bytes.Contains(raw, []byte(`"EXIF"`)) &&
		!bytes.Contains(raw, []byte(`"IPTC"`)) &&
		!bytes.Contains(raw, []byte(`"XMP"`)) {
		return raw, nil
	}

	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, fmt.Errorf("decode exiftool json: %w", err)
	}

	for _, result := range results {
		migratePentaxGroup(result)
		delete(result, "EXIF")
		delete(result, "IPTC")
		delete(result, "XMP")
	}

	normalized, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode normalized json: %w", err)
	}
	return append(normalized, '\n'), nil
}

func migratePentaxGroup(result map[string]any) {
	rawPentax, ok := result[legacyPentaxGroup]
	if !ok {
		return
	}

	pentax, ok := rawPentax.(map[string]any)
	if !ok || len(pentax) == 0 {
		delete(result, legacyPentaxGroup)
		return
	}

	quickTime, ok := result["QuickTime"].(map[string]any)
	if !ok || quickTime == nil {
		quickTime = make(map[string]any, len(pentax))
		result["QuickTime"] = quickTime
	}
	for key, value := range pentax {
		quickTime[key] = value
	}
	delete(result, legacyPentaxGroup)
}

func generateOrderedGolden(videoPath string) ([]byte, error) {
	cmd := exec.Command("exiftool", "-a", "-n", "-G0", "-S", "-sep", orderedValueSep, "--File:all", "--ExifTool:all", videoPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run exiftool ordered output: %w", err)
	}

	golden, err := parseOrderedGolden(output, videoPath)
	if err != nil {
		return nil, err
	}

	normalized, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode ordered golden: %w", err)
	}
	return append(normalized, '\n'), nil
}

func parseOrderedGolden(raw []byte, videoPath string) (orderedGoldenFile, error) {
	type groupBucket struct {
		name string
		tags []orderedGoldenTag
	}

	var groupOrder []string
	groups := make(map[string]*groupBucket)
	lines := strings.Split(string(raw), "\n")
	for lineNumber, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "[") {
			return orderedGoldenFile{}, fmt.Errorf("parse ordered golden line %d: missing group prefix", lineNumber+1)
		}

		closeIdx := strings.IndexByte(line, ']')
		if closeIdx < 0 {
			return orderedGoldenFile{}, fmt.Errorf("parse ordered golden line %d: missing closing bracket", lineNumber+1)
		}

		group := line[1:closeIdx]
		if group == legacyPentaxGroup {
			group = "QuickTime"
		}
		if !orderedGoldenGroups[group] {
			continue
		}

		payload := strings.TrimSpace(line[closeIdx+1:])
		colonIdx := strings.Index(payload, ":")
		if colonIdx < 0 {
			return orderedGoldenFile{}, fmt.Errorf("parse ordered golden line %d: missing tag/value separator", lineNumber+1)
		}

		tag := strings.TrimSpace(payload[:colonIdx])
		if tag == "" {
			return orderedGoldenFile{}, fmt.Errorf("parse ordered golden line %d: empty tag", lineNumber+1)
		}
		valueText := strings.TrimSpace(payload[colonIdx+1:])
		var value any = valueText
		if strings.Contains(valueText, orderedValueSep) {
			value = strings.Split(valueText, orderedValueSep)
		}

		bucket, found := groups[group]
		if !found {
			bucket = &groupBucket{name: group}
			groups[group] = bucket
			groupOrder = append(groupOrder, group)
		}
		bucket.tags = append(bucket.tags, orderedGoldenTag{
			Tag:   tag,
			Value: value,
		})
	}

	orderedGroups := make([]orderedGoldenGroup, 0, len(groupOrder))
	for _, name := range groupOrder {
		bucket := groups[name]
		orderedGroups = append(orderedGroups, orderedGoldenGroup{
			Name: name,
			Tags: bucket.tags,
		})
	}

	return orderedGoldenFile{
		SourceFile: videoPath,
		Groups:     orderedGroups,
	}, nil
}
