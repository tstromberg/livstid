package fj

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
	"github.com/otiai10/copy"
	"github.com/rwcarlsen/goexif/exif"
	"k8s.io/klog/v2"
)

// ThumbOpts are thumbnail soptions
type ThumbOpts struct {
	X       int
	Y       int
	Quality int
}

var defaultThumbOpts = map[string]ThumbOpts{
	"133y":  {Y: 133, Quality: 75},
	"512x":  {X: 512, Quality: 80},
	"2048x": {X: 2048, Quality: 85},
}

func thumbnails(i Image, outDir string) (map[string]ThumbMeta, error) {
	klog.Infof("creating thumbnails for %s in %s", i.Path, outDir)
	fullDest := filepath.Join(outDir, i.RelPath)
	klog.Infof("relpath: %s -- full dest: %s", i.RelPath, fullDest)

	sst, err := os.Stat(i.Path)
	if err != nil {
		return nil, err
	}

	dst, err := os.Stat(fullDest)
	updated := false

	if err != nil {
		updated = true
		klog.Infof("updating %s: does not exist", fullDest)
	}

	if err == nil && sst.Size() != dst.Size() {
		updated = true
		klog.Infof("updating %s: size mismatch", fullDest)
	}

	if err == nil && sst.ModTime().After(dst.ModTime()) {
		klog.Infof("updating %s: source newer", fullDest)
		updated = true
	}

	if updated {
		err := copy.Copy(i.Path, fullDest)
		if err != nil {
			return nil, fmt.Errorf("copy: %v", err)
		}
	}

	thumbDir := filepath.Join(outDir, filepath.Dir(i.RelPath), "thumbs")
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %v", err)
	}

	base := strings.Split(filepath.Base(i.Path), ".")[0]
	var img image.Image

	thumbs := map[string]ThumbMeta{}

	for name, t := range defaultThumbOpts {
		thumbName := fmt.Sprintf("%s@%s.jpg", base, name)
		thumbDest := filepath.Join(thumbDir, thumbName)
		fullThumbDest := filepath.Join(outDir, thumbDest)

		st, err := os.Stat(fullThumbDest)
		if err == nil && st.Size() > int64(128) && !updated {
			klog.Infof("%s exists (%d bytes)", fullThumbDest, st.Size())
			rt, err := readThumb(fullThumbDest)
			if err == nil {
				rt.RelPath = thumbDest
				thumbs[name] = *rt
				continue
			}
			klog.Warningf("unable to read thumb: %v", err)
		}

		if img == nil {
			klog.Infof("opening %s ...", fullDest)
			img, err = imgio.Open(fullDest)
			if err != nil {
				return nil, err
			}
		}

		ct, err := createThumb(img, fullThumbDest, t)
		if err != nil {
			return nil, fmt.Errorf("create thumb: %w", err)
		}

		ct.RelPath = thumbDest
		thumbs[name] = *ct
	}

	klog.Infof("thumbs: %+v", thumbs)
	return thumbs, nil
}

func createThumb(i image.Image, path string, t ThumbOpts) (*ThumbMeta, error) {
	klog.Infof("creating thumb: %s", path)
	x := t.X
	y := t.Y

	if t.X == 0 {
		scale := i.Bounds().Dy() / t.Y
		x = int(i.Bounds().Dx() / scale)
	}

	if t.Y == 0 {
		scale := i.Bounds().Dx() / t.X
		y = int(i.Bounds().Dy() / scale)
	}

	rimg := transform.Resize(i, x, y, transform.Lanczos)
	if err := imgio.Save(path, rimg, imgio.JPEGEncoder(t.Quality)); err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}

	return &ThumbMeta{X: rimg.Bounds().Dx(), Y: rimg.Bounds().Dy()}, nil
}

func readThumb(path string) (*ThumbMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	ex, err := exif.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	gx, err := ex.Get(exif.ImageWidth)
	if err != nil {
		return nil, fmt.Errorf("imgwidth: %w", err)
	}
	x, err := strconv.Atoi(gx.String())
	if err != nil {
		return nil, err
	}

	gy, err := ex.Get(exif.ImageLength)
	if err != nil {
		return nil, fmt.Errorf("imglen: %w", err)
	}

	y, err := strconv.Atoi(gy.String())
	if err != nil {
		return nil, err
	}

	return &ThumbMeta{X: x, Y: y}, nil
}
