package main

import (
	"flag"

	"k8s.io/klog/v2"

	"github.com/tstromberg/fj/pkg/fj"
)

var inDir = flag.String("in", "", "Location of input directory")
var outDir = flag.String("out", "", "Location of output directory")

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

	if err := fj.Build(*inDir, *outDir); err != nil {
		klog.Exitf("build failed: %v", err)
	}
}
