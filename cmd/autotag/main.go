// autotag adds suggested tags to JPEG images using Google Vertex AI
package main

import (
	"context"
	"flag"
	"log"
	"os"

	_ "image/jpeg"
	_ "image/png"

	"cloud.google.com/go/vertexai/genai"
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

	if *outDir == "" {
		klog.Fatalf("please give me an out directory to dump thumbnails into")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, os.Getenv("GCP_PROJECT_ID"), "us-central1")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			klog.Errorf("Failed to close client: %v", err)
		}
	}()

	model := client.GenerativeModel("gemini-pro-vision")

	c := &livstid.Config{
		InDirs: flag.Args(),
		OutDir: *outDir,
		Thumbnails: map[string]livstid.ThumbOpts{
			"Album": {Y: 350, Quality: 80},
			//			"Tiny": {Y: 120, Quality: 70},
		},
	}

	as, err := livstid.Collect(c)
	if err != nil {
		klog.Fatalf("unable to collect: %v", err)
	}

	e, err := exiftool.NewExiftool()
	if err != nil {
		klog.Fatalf("exiftool: %v", err)
	}
	defer func() {
		if err := e.Close(); err != nil {
			klog.Errorf("Failed to close exiftool: %v", err)
		}
	}()

	for _, a := range as.Albums {
		for _, i := range a.Images {
			if !*overwrite && len(i.Keywords) > 0 {
				klog.Infof("%s has tags: %v", i.InPath, i.Keywords)
				continue
			}
			tags, err := livstid.AutoTag(ctx, model, i)
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
}
