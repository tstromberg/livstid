package main

import (
	"flag"

	_ "image/jpeg"
	_ "image/png"

	"k8s.io/klog/v2"

	livstid "github.com/tstromberg/livstid/pkg/livstid"
)

var (
	inDir       = flag.String("in", "", "Location of input directory")
	outDir      = flag.String("out", "", "Location of output directory")
	title       = flag.String("title", "livstid 📸", "Title of photo collection")
	description = flag.String("description", "(insert description here)", "description of photo collection")
)

/*
var commit = flag.Bool("commit", false, "Commit changes")
var push = flag.Bool("push", false, "Push changes")
var watch = flag.Bool("watch", false, "Watch for changes")
*/

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if *inDir == "" {
		klog.Exitf("--in is a required flag")
	}

	if *outDir == "" {
		klog.Exitf("--out is a required flag")
	}

	c := &livstid.Config{
		InDir:       *inDir,
		OutDir:      *outDir,
		Collection:  *title,
		Description: *description,
	}

	a, err := livstid.Collect(c)
	if err != nil {
		klog.Exitf("build failed: %v", err)
	}

	if err := livstid.Render(c, a); err != nil {
		klog.Exitf("render failed: %v", err)
	}
}
