//go:build ignore

// gen/main.go regenerates exiftool-based golden JSON files and the generated
// EXIF field tables used by the decoder.
//
//go:generate go run main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	testdataDir          = "../testdata"
	exifManifestPath     = "exif_fields_reference.json"
	exifFieldsOutputPath = "../metadecoder_exif_fields.go"
	legacyPentaxGroup    = "Maker" + "Notes"
	orderedValueSep      = "\x1f"
)

type tagEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type exifFieldManifest struct {
	EXIF    []tagEntry `json:"exif"`
	GPS     []tagEntry `json:"gps"`
	Interop []tagEntry `json:"interop"`
}

type manifestEntry struct {
	id   uint16
	name string
}

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
	"EXIF":      true,
	"IPTC":      true,
	"QuickTime": true,
	"XMP":       true,
	"XML":       true,
	"Composite": true,
}

func main() {
	if err := generateGoldenFiles(); err != nil {
		fmt.Fprintf(os.Stderr, "generate golden files: %v\n", err)
		os.Exit(1)
	}
	if err := generateEXIFFields(); err != nil {
		fmt.Fprintf(os.Stderr, "generate exif fields: %v\n", err)
		os.Exit(1)
	}
}

func generateGoldenFiles() error {
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		return fmt.Errorf("read testdata: %w", err)
	}

	videoExts := map[string]bool{
		".mp4": true,
		".mov": true,
		".m4v": true,
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !videoExts[ext] {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	generated := 0
	for _, name := range names {
		videoPath := filepath.Join(testdataDir, name)
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
	if !bytes.Contains(raw, []byte(`"`+legacyPentaxGroup+`"`)) {
		return raw, nil
	}

	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, fmt.Errorf("decode exiftool json: %w", err)
	}

	for _, result := range results {
		migratePentaxGroup(result)
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

func generateEXIFFields() error {
	manifest, err := loadManifest(exifManifestPath)
	if err != nil {
		return err
	}

	src, err := renderEXIFFields(manifest)
	if err != nil {
		return err
	}

	formatted, err := format.Source(src)
	if err != nil {
		return fmt.Errorf("format generated exif fields: %w", err)
	}

	if err := os.WriteFile(exifFieldsOutputPath, formatted, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", exifFieldsOutputPath, err)
	}

	fmt.Printf("generated %s\n", exifFieldsOutputPath)
	return nil
}

func loadManifest(path string) (exifFieldManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return exifFieldManifest{}, fmt.Errorf("read %s: %w", path, err)
	}

	var manifest exifFieldManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return exifFieldManifest{}, fmt.Errorf("decode %s: %w", path, err)
	}

	for _, section := range []struct {
		name    string
		entries []tagEntry
	}{
		{name: "exif", entries: manifest.EXIF},
		{name: "gps", entries: manifest.GPS},
		{name: "interop", entries: manifest.Interop},
	} {
		if _, err := parseManifestSection(section.name, section.entries); err != nil {
			return exifFieldManifest{}, err
		}
	}

	return manifest, nil
}

func parseManifestSection(sectionName string, entries []tagEntry) ([]manifestEntry, error) {
	parsed := make([]manifestEntry, 0, len(entries))
	seen := make(map[uint16]bool, len(entries))
	lastID := uint16(0)
	for i, entry := range entries {
		if entry.Name == "" {
			return nil, fmt.Errorf("%s[%d]: empty name", sectionName, i)
		}
		id, err := parseTagID(entry.ID)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", sectionName, i, err)
		}
		if seen[id] {
			return nil, fmt.Errorf("%s[%d]: duplicate tag id %s", sectionName, i, entry.ID)
		}
		if i > 0 && id <= lastID {
			return nil, fmt.Errorf("%s[%d]: ids must be strictly ascending", sectionName, i)
		}
		seen[id] = true
		lastID = id
		parsed = append(parsed, manifestEntry{
			id:   id,
			name: entry.Name,
		})
	}
	return parsed, nil
}

func parseTagID(raw string) (uint16, error) {
	if len(raw) != 6 || !strings.HasPrefix(raw, "0x") {
		return 0, fmt.Errorf("invalid tag id %q", raw)
	}
	value, err := strconv.ParseUint(raw[2:], 16, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid tag id %q: %w", raw, err)
	}
	return uint16(value), nil
}

func renderEXIFFields(manifest exifFieldManifest) ([]byte, error) {
	exifEntries, err := parseManifestSection("exif", manifest.EXIF)
	if err != nil {
		return nil, err
	}
	gpsEntries, err := parseManifestSection("gps", manifest.GPS)
	if err != nil {
		return nil, err
	}
	interopEntries, err := parseManifestSection("interop", manifest.Interop)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by go generate ./gen; DO NOT EDIT.")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "package videometa")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "// exifFields maps EXIF tag IDs to their names.\n")
	fmt.Fprintf(&out, "// Names are generated from %s and match the committed exiftool 13.50 reference set.\n", exifManifestPath)
	writeTagMap(&out, "exifFields", exifEntries)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "// exifFieldsGPS maps GPS IFD tag IDs to their names.")
	writeTagMap(&out, "exifFieldsGPS", gpsEntries)
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "// exifIFDPointers maps tag IDs that are pointers to sub-IFDs.")
	fmt.Fprintln(&out, "var exifIFDPointers = map[uint16]string{")
	fmt.Fprintln(&out, "\t0x8769: \"ExifIFD\",")
	fmt.Fprintln(&out, "\t0x8825: \"GPSInfoIFD\",")
	fmt.Fprintln(&out, "\t0xA005: \"InteropIFD\",")
	fmt.Fprintln(&out, "}")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "// exifInteropFields maps Interoperability IFD tag IDs.")
	writeTagMap(&out, "exifInteropFields", interopEntries)

	return out.Bytes(), nil
}

func writeTagMap(out *bytes.Buffer, name string, entries []manifestEntry) {
	fmt.Fprintf(out, "var %s = map[uint16]string{\n", name)
	for _, entry := range entries {
		fmt.Fprintf(out, "\t0x%04X: %q,\n", entry.id, entry.name)
	}
	fmt.Fprintln(out, "}")
}
