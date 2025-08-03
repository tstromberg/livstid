// livstid builds hierarchical static photo albums
package main

import (
	"context"
	"errors"
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
	"github.com/tstromberg/livstid/pkg/manage"
)

var (
	outFlag    = flag.String("out", "", "Location of output directory")
	titleFlag  = flag.String("title", "livstid 📸", "Title of photo collection")
	descFlag   = flag.String("description", "(insert description here)", "description of photo collection")
	listenFlag = flag.Bool("listen", false, "serve content via HTTP (read-only)")
	manageFlag = flag.Bool("manage", false, "serve content via HTTP (writes)")
	addrFlag   = flag.String("addr", "localhost:12800", "host:port to bind to in listen mode")
	watchFlag  = flag.Bool("watch", false, "watch for changes to inDir and rebuild")
	rcloneFlag = flag.String("rclone", "", "rclone target to sync directory contents to")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if len(flag.Args()) == 0 {
		klog.Exitf("required arguments: directories to process")
	}

	if *outFlag == "" {
		klog.Exitf("--out is a required flag")
	}

	c := &livstid.Config{
		InDirs:       flag.Args(),
		OutDir:       *outFlag,
		Collection:   *titleFlag,
		Description:  *descFlag,
		RCloneTarget: *rcloneFlag,
		Thumbnails: map[string]livstid.ThumbOpts{
			"Tiny":     {Y: 120, Quality: 70},
			"Album":    {Y: 350, Quality: 80},
			"Recent":   {X: 512, Quality: 85},
			"Recent2X": {X: 1024, Quality: 85},
			"View":     {X: 1920, Quality: 85},
		},
		ProcessSidecars: false,
	}
	var wg sync.WaitGroup
	if *manageFlag {
		wg.Add(1)
		go func() {
			defer wg.Done()
			serveDynamic(c, *outFlag, *addrFlag)
		}()
	} else if *listenFlag {
		wg.Add(1)
		go func() {
			defer wg.Done()
			serveStatic(*outFlag, *addrFlag)
		}()
	}

	a, err := build(c)
	if err != nil {
		klog.Exitf("build failed: %v", err)
	}

	if *watchFlag {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := watch(c, a); err != nil {
				klog.Errorf("Watch error: %v", err)
			}
		}()
	}

	wg.Wait()
}

// build collects, renders, and syncs.
func build(c *livstid.Config) (*livstid.Assembly, error) {
	a, err := livstid.Collect(c)
	if err != nil {
		return a, fmt.Errorf("collect: %w", err)
	}

	errs := a.Validate()
	if len(errs) > 0 {
		for _, err := range errs {
			klog.Errorf("*** validation error: %v", err)
		}
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

// rcloneSync synchronizes the website to a remote crlone target.
func rcloneSync(c *livstid.Config) error {
	klog.Infof("rclone syncing to %s ...", c.RCloneTarget)
	path, err := exec.LookPath("rclone")
	if err != nil {
		return errors.New("rclone not installed in $PATH")
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "sync", c.OutDir+"/", c.RCloneTarget+"/")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v failed: %w", cmd, err)
	}
	klog.Infof("rclone sync to %s completed in %s", c.RCloneTarget, time.Since(start))
	return nil
}

// serveStatic serves a static web directory via HTTP.
func serveStatic(path string, addr string) {
	fs := http.FileServer(http.Dir(path))
	http.Handle("/", fs)

	klog.Infof("Listening on %s...", addr)
	server := &http.Server{
		Addr:         addr,
		Handler:      nil,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	err := server.ListenAndServe()
	if err != nil {
		klog.Exitf("listen failed: %v", err)
	}
}

// serveDynamic serves a dynamic website with management enabled.
func serveDynamic(c *livstid.Config, path string, addr string) {
	m := manage.New(c, path)
	fs := http.FileServer(http.Dir(path))
	http.Handle("/", fs)
	http.HandleFunc("/hide", m.HideHandler())
	server := &http.Server{
		Addr:         addr,
		Handler:      nil,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		klog.Errorf("HTTP server error: %v", err)
	}
}

// watch watches a directory for changes and rebuilds.
func watch(c *livstid.Config, a *livstid.Assembly) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("new watches: %w", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			klog.Errorf("Failed to close watcher: %v", err)
		}
	}()

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return
				}

				// TODO: dedup events in quick succession
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) ||
					event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
					klog.Infof("watch event: %s", event)
					assembly, err := build(c)
					if err != nil {
						klog.Exitf("build failed: %v", err)
					}

					if err := updateWatchPaths(c, w, assembly, event.Name); err != nil {
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

// updateWatchPaths updates the watch list with new paths.
func updateWatchPaths(c *livstid.Config, w *fsnotify.Watcher, a *livstid.Assembly, path string) error {
	exists := map[string]bool{}
	for _, d := range w.WatchList() {
		exists[d] = true
	}

	dirs := c.InDirs
	if path != "" {
		dirs = append(dirs, path)
	}

	for _, aa := range a.Albums {
		dirs = append(dirs, aa.InPath, filepath.Dir(aa.InPath))
	}

	slices.Sort(dirs)
	dirs = slices.Compact(dirs)

	for _, d := range dirs {
		if exists[d] {
			continue
		}

		if err := w.Add(d); err != nil {
			return fmt.Errorf("watcher add: %w", err)
		}
	}

	return nil
}
