// reorg reorganizes a photo album directory based on the date
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"k8s.io/klog/v2"

	livstid "github.com/tstromberg/livstid/pkg/livstid"
)

var dryRun = flag.Bool("n", false, "dry-run mode, don't move things")

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	c := &livstid.Config{
		InDir: flag.Args()[0],
	}

	as, err := livstid.Collect(c)
	if err != nil {
		klog.Fatalf("unable to collect: %v", err)
	}

	for _, a := range as.Albums {
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
		// fix bad apostrophes
		base = strings.ReplaceAll(base, "_s ", "'s ")

		new := fmt.Sprintf("%s/%d/%02d/%s", filepath.Dir(a.InPath), year, month, base)
		klog.Infof("%s -> %s", a.InPath, new)
		if !*dryRun {
			os.MkdirAll(filepath.Dir(new), 0o755)
			os.Rename(a.InPath, new)
		}
	}
}
