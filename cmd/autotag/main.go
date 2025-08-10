// autotag adds suggested tags to JPEG images using Google Vertex AI.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	_ "image/jpeg"
	_ "image/png"

	"google.golang.org/genai"
	"k8s.io/klog/v2"

	"github.com/barasher/go-exiftool"
	livstid "github.com/tstromberg/livstid/pkg/livstid"
)

var (
	dryRun    = flag.Bool("n", false, "dry-run mode, don't tag things")
	overwrite = flag.Bool("o", false, "overwrite existing tags")
	outDir    = flag.String("out", "", "Location of output directory for thumbnails and cache")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.Infof("autotag starting with %d input directories", len(flag.Args()))

	if len(flag.Args()) == 0 {
		klog.Fatalf("No input directories provided. Usage: %s -out <output_dir> <input_dir1> [input_dir2 ...]", os.Args[0])
	}

	if *outDir == "" {
		klog.Fatalf("please give me an out directory to scan through")
	}

	klog.Infof("Input directories: %v", flag.Args())
	klog.Infof("Output directory: %s", *outDir)

	ctx := context.Background()
	cfg := &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_AI_API_KEY"),
	}
	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	modelName := "gemini-2.5-flash"

	c := &livstid.Config{
		InDirs: flag.Args(),
		OutDir: *outDir,
		Thumbnails: map[string]livstid.ThumbOpts{
			"Album": {Y: 350, Quality: 80},
			//			"Tiny": {Y: 120, Quality: 70},
		},
	}

	klog.Infof("Collecting images from directories...")
	as, err := livstid.Collect(c)
	if err != nil {
		klog.Fatalf("unable to collect: %v", err)
	}
	klog.Infof("Found %d albums", len(as.Albums))

	e, err := exiftool.NewExiftool()
	if err != nil {
		klog.Fatalf("exiftool: %v", err)
	}
	defer func() {
		if err := e.Close(); err != nil {
			klog.Errorf("Failed to close exiftool: %v", err)
		}
	}()

	totalImages := 0
	for _, a := range as.Albums {
		klog.Infof("Processing album: %s with %d images", a.Title, len(a.Images))
		for _, i := range a.Images {
			totalImages++
			if !*overwrite && len(i.Keywords) > 0 {
				klog.Infof("%s has tags: %v", i.InPath, i.Keywords)
				continue
			}
			tags, err := livstid.AutoTag(ctx, client, modelName, i)
			if err != nil {
				klog.Errorf("err: %v", err)
			}

			o := e.ExtractMetadata(i.InPath)
			klog.Infof("adding tags to %s: %v", i.InPath, tags)
			if len(tags) > 5 {
				tags = tags[0:5]
			}
			o[0].SetStrings("Keywords", tags)
			if !*dryRun {
				e.WriteMetadata(o)
				if o[0].Err != nil {
					klog.Errorf("Failed to write metadata for %s: %v", i.InPath, o[0].Err)
				}
			}
		}
	}

	klog.Infof("autotag completed. Processed %d total images across %d albums", totalImages, len(as.Albums))
}
