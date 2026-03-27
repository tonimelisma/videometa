package videometa

import (
	"fmt"
	"os"
)

func ExampleDecode() {
	f, err := os.Open("testdata/minimal.mp4")
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()

	_, err = Decode(Options{
		R:       f,
		Sources: QUICKTIME | CONFIG,
		HandleTag: func(ti TagInfo) error {
			if ti.Namespace == "moov/mvhd" && ti.Tag == "TimeScale" {
				fmt.Printf("%s %s=%v\n", ti.Namespace, ti.Tag, ti.Value)
				return ErrStopWalking
			}
			return nil
		},
	})
	if err != nil {
		panic(err)
	}

	// Output:
	// moov/mvhd TimeScale=1000
}

func ExampleDecodeAll_namespaceAware() {
	f, err := os.Open("testdata/minimal.mp4")
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()

	metadata, err := DecodeAll(Options{
		R:       f,
		Sources: QUICKTIME | CONFIG,
	})
	if err != nil {
		panic(err)
	}

	mvhd := metadata.Tags.QuickTime().Namespace("moov/mvhd")
	timeScale := mvhd.Find("TimeScale")
	fmt.Printf("timeScale=%v codec=%s\n", timeScale[0].Value, metadata.VideoConfig.Codec)

	// Output:
	// timeScale=1000 codec=avc1
}
