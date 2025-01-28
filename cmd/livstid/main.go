package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

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
	rcloneFlag  = flag.String("rclone", "", "rclone target to sync directory contents to")
)

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
		InDir:        *inDir,
		OutDir:       *outDir,
		Collection:   *title,
		Description:  *description,
		RCloneTarget: *rcloneFlag,
	}

	a, err := build(c)
	if err != nil {
		klog.Exitf("build failed: %v", err)
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

// build collects, renders, and syncs
func build(c *livstid.Config) (*livstid.Assembly, error) {
	a, err := livstid.Collect(c)
	if err != nil {
		return a, fmt.Errorf("collect: %w", err)
	}

	errs := a.Validate()
	if len(errs) > 0 {
		for _, err := range errs {
			klog.Errorf("validation error: %v", err)
		}
		return a, err
	}

	if err := livstid.Render(c, a); err != nil {
		return a, fmt.Errorf("render: %w", err)
	}

	if c.RCloneTarget != "" {
		if err := rcloneSync(c); err != nil {
			return a, fmt.Errorf("clone: %w", err)
		}
	}

	return a, nil
}

// rcloneSync synchronizes the website to a remote crlone target
func rcloneSync(c *livstid.Config) error {
	klog.Infof("rclone syncing to %s ...", c.RCloneTarget)
	path, err := exec.LookPath("rclone")
	if err != nil {
		return fmt.Errorf("rclone not installed in $PATH")
	}

	start := time.Now()
	cmd := exec.Command(path, "sync", filepath.Join(c.OutDir, "/"), filepath.Join(c.RCloneTarget, "/"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v failed: %w", cmd, err)
	}
	klog.Infof("rclone sync to %s completed in %s", c.RCloneTarget, time.Since(start))
	return nil
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

				// TODO: dedup events in quick succession
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
					klog.Infof("watch event: %s", event)
					a, err := build(c)
					if err != nil {
						klog.Exitf("build failed: %v", err)
					}

					if err := updateWatchPaths(c, w, a, event.Name); err != nil {
						klog.Exitf("watch update failed: %v", err)
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

	if err := updateWatchPaths(c, w, a, ""); err != nil {
		return err
	}

	<-make(chan struct{})
	return nil
}

// updateWatchPaths updates the watch list with new paths
func updateWatchPaths(c *livstid.Config, w *fsnotify.Watcher, a *livstid.Assembly, path string) error {
	exists := map[string]bool{}
	for _, d := range w.WatchList() {
		exists[d] = true
	}

	dirs := []string{c.InDir}
	if path != "" {
		dirs = append(dirs, path)
	}

	for _, aa := range a.Albums {
		dirs = append(dirs, filepath.Join(c.InDir, aa.InPath))
		dirs = append(dirs, filepath.Dir(filepath.Join(c.InDir, aa.InPath)))
	}

	slices.Sort(dirs)
	dirs = slices.Compact(dirs)

	for _, d := range dirs {
		if exists[d] {
			continue
		}

		if err := w.Add(d); err != nil {
			return err
		}
	}

	return nil
}
