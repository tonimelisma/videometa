package videometa

import (
	"encoding/json"
	"os"
	"slices"
	"strings"

	qt "github.com/frankban/quicktest"
)

type orderedGoldenGroupFile struct {
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

func orderedGoldenPathFor(flatGoldenPath string) string {
	return strings.TrimSuffix(flatGoldenPath, ".exiftool.json") + ".exiftool.ordered.json"
}

func loadOrderedGolden(c *qt.C, path string) orderedGoldenGroupFile {
	c.Helper()

	data, err := os.ReadFile(path)
	c.Assert(err, qt.IsNil)

	var golden orderedGoldenGroupFile
	err = json.Unmarshal(data, &golden)
	c.Assert(err, qt.IsNil)
	return golden
}

func (g orderedGoldenGroupFile) Group(name string) ([]orderedGoldenTag, bool) {
	for _, group := range g.Groups {
		if group.Name == name {
			return group.Tags, true
		}
	}
	return nil, false
}

func actualOrderedGroupTags(tags Tags, group string) []TagInfo {
	all := tags.All()
	ordered := make([]TagInfo, 0, len(all))
	for _, tag := range all {
		if goldenFamily0GroupName(tag) == group {
			ordered = append(ordered, tag)
		}
	}
	return ordered
}

func goldenFamily0GroupName(tag TagInfo) string {
	switch tag.Source {
	case QUICKTIME:
		return "QuickTime"
	case VENDOR:
		if strings.HasSuffix(tag.Namespace, "/nrtm") {
			return "XML"
		}
		return "QuickTime"
	case COMPOSITE:
		return "Composite"
	default:
		return ""
	}
}

func compareOrderedGoldenTags(c *qt.C, got []TagInfo, want []orderedGoldenTag, groupName string) {
	c.Helper()

	c.Assert(len(got), qt.Equals, len(want), qt.Commentf(
		"%s occurrence count mismatch: got %d tags, want %d", groupName, len(got), len(want)))

	gotByTag := make(map[string][]TagInfo)
	for _, gotTag := range got {
		gotByTag[gotTag.Tag] = append(gotByTag[gotTag.Tag], gotTag)
	}

	wantByTag := make(map[string][]orderedGoldenTag)
	var wantTagOrder []string
	for _, wantTag := range want {
		if _, seen := wantByTag[wantTag.Tag]; !seen {
			wantTagOrder = append(wantTagOrder, wantTag.Tag)
		}
		wantByTag[wantTag.Tag] = append(wantByTag[wantTag.Tag], wantTag)
	}

	var extraTags []string
	for tagName := range gotByTag {
		if _, ok := wantByTag[tagName]; !ok {
			extraTags = append(extraTags, tagName)
		}
	}
	slices.Sort(extraTags)
	c.Assert(extraTags, qt.HasLen, 0, qt.Commentf("%s unexpected repeated tags: %v", groupName, extraTags))

	for _, tagName := range wantTagOrder {
		wantOccurrences := wantByTag[tagName]
		gotOccurrences, ok := gotByTag[tagName]
		c.Assert(ok, qt.IsTrue, qt.Commentf("%s missing ordered occurrences for tag %q", groupName, tagName))
		c.Assert(len(gotOccurrences), qt.Equals, len(wantOccurrences), qt.Commentf(
			"%s.%s occurrence count mismatch: got %d, want %d",
			groupName, tagName, len(gotOccurrences), len(wantOccurrences)))

		for i := range wantOccurrences {
			gotTag := gotOccurrences[i]
			wantTag := wantOccurrences[i]

			if binaryPlaceholder, ok := wantTag.Value.(string); ok && strings.HasPrefix(binaryPlaceholder, "(Binary data") {
				gotValue := formatValueForGolden(gotTag.Value)
				c.Assert(len(gotValue) > 0, qt.IsTrue, qt.Commentf(
					"%s.%s occurrence %d expected binary placeholder, got %T",
					groupName, tagName, i, gotTag.Value))
				continue
			}

			if orderedGoldenValueMatches(gotTag.Value, wantTag.Value) {
				continue
			}

			assertGoldenValue(c, groupName, tagName, gotTag.Value, wantTag.Value)
		}
	}
}

func orderedGoldenValueMatches(got any, want any) bool {
	if goldenValueMatches(got, want) {
		return true
	}

	if wantString, ok := want.(string); ok {
		if gotSlice, isSlice := got.([]string); isSlice {
			candidates := []string{
				strings.Join(gotSlice, "."),
				strings.Join(gotSlice, ", "),
				strings.Join(gotSlice, " "),
			}
			if len(gotSlice) == 1 {
				candidates = append(candidates, gotSlice[0])
			}
			for _, candidate := range candidates {
				if strings.TrimRight(candidate, " ") == strings.TrimRight(wantString, " ") {
					return true
				}
			}
		}
	}

	gotString, gotOK := got.(string)
	wantString, wantOK := want.(string)
	if !gotOK || !wantOK {
		return false
	}

	// exiftool's ordered text output trims right-padding that -json preserves.
	// Keep the flat JSON layer exact and let the ordered layer focus on
	// duplicate/order coverage.
	if strings.TrimRight(gotString, " ") == strings.TrimRight(wantString, " ") {
		return true
	}

	return false
}
