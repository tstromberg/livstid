package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"sync"

	_ "image/jpeg"
	_ "image/png"

	"k8s.io/klog/v2"

	"github.com/fsnotify/fsnotify"
	livstid "github.com/tstromberg/livstid/pkg/livstid"
)

var (
	inDir       = flag.String("in", "", "Location of input directory")
	outDir      = flag.String("out", "", "Location of output directory")
	title       = flag.String("title", "livstid 📸", "Title of photo collection")
	description = flag.String("description", "(insert description here)", "description of photo collection")
	listen      = flag.Bool("listen", false, "serve content via HTTP")
	addr        = flag.String("addr", "localhost:12800", "host:port to bind to in listen mode")
	watchFlag   = flag.Bool("watch", false, "watch for changes to inDir and rebuild")
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

	var wg sync.WaitGroup
	if *watchFlag {
		wg.Add(1)
		go func() {
			defer wg.Done()
			watch(c, a)
		}()
	}

	if *listen {
		wg.Add(1)
		go func() {
			defer wg.Done()
			serve(*outDir, *addr)
		}()
	}

	wg.Wait()
}

// serve serves a static web directory via HTTP
func serve(path string, addr string) {
	fs := http.FileServer(http.Dir(path))
	http.Handle("/", fs)

	klog.Infof("Listening on %s...", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		klog.Exitf("listen failed: %v", err)
	}
}

// watch watches a directory for changes and rebuilds
func watch(c *livstid.Config, a *livstid.Assembly) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("new watches: %w", err)
	}
	defer w.Close()

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
					a, err := livstid.Collect(c)
					if err != nil {
						klog.Exitf("build failed: %v", err)
					}

					if err := livstid.Render(c, a); err != nil {
						klog.Exitf("render failed: %v", err)
					}
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	dirs := []string{
		c.InDir,
	}
	for _, aa := range a.Albums {
		klog.Infof("%s", aa.InPath)
		dirs = append(dirs, filepath.Join(c.InDir, aa.InPath))
		dirs = append(dirs, filepath.Dir(filepath.Join(c.InDir, aa.InPath)))
	}

	slices.Sort(dirs)
	dirs = slices.Compact(dirs)

	klog.Infof("watching %d dirs ...", len(dirs))
	for _, d := range dirs {
		err = w.Add(d)
		if err != nil {
			log.Fatal(err)
		}
	}

	<-make(chan struct{})
	return nil
}
