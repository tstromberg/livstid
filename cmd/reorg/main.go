// reorg reorganizes a photo album directory based on the date
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"k8s.io/klog/v2"

	livstid "github.com/tstromberg/livstid/pkg/livstid"
)

var (
	dryRun           = flag.Bool("n", false, "dry-run mode, don't move things")
	datePrefix       = regexp.MustCompile(`^\d{4}[.\-][\d.\-]+[ _-]`)
	apostropheSuffix = regexp.MustCompile(`_s$`)
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	c := &livstid.Config{
		InDirs: flag.Args(),
	}

	as, err := livstid.Collect(c)
	if err != nil {
		klog.Fatalf("unable to collect: %v", err)
	}

	for _, a := range as.Albums {
		if a.InPath == "" || a.InPath == "." {
			klog.Fatalf("invalid album path: %+v", a)
		}

		year := 0
		month := 0
		for _, i := range a.Images {
			if !i.Taken.IsZero() {
				year = i.Taken.Year()
				month = int(i.Taken.Month())
			}
		}

		if year == 0 {
			klog.Infof("no year in %s", a.InPath)
			continue
		}
		base := filepath.Base(a.InPath)
		base = datePrefix.ReplaceAllString(base, "")
		base = apostropheSuffix.ReplaceAllString(base, `'s`)
		// fix bad apostrophes
		base = strings.ReplaceAll(base, "_s ", "'s ")
		base = strings.ReplaceAll(base, "_ ", " ")
		base = strings.ReplaceAll(base, " _", " ")
		base = strings.ReplaceAll(base, "#", "")
		base = strings.TrimSpace(base)

		newPath := fmt.Sprintf("%s/%d/%02d/%s", filepath.Dir(a.InPath), year, month, base)
		if *dryRun {
			klog.Infof("dry run: %s -> %s", a.InPath, newPath)
			continue
		}
		klog.Infof("%s -> %s", a.InPath, newPath)
		if err := os.MkdirAll(filepath.Dir(newPath), 0o750); err != nil {
			klog.Errorf("Failed to create directory %s: %v", filepath.Dir(newPath), err)
			continue
		}
		if err := os.Rename(a.InPath, newPath); err != nil {
			klog.Errorf("Failed to rename %s to %s: %v", a.InPath, newPath, err)
		}
	}
}
