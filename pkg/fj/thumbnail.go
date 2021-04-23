package fj

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
	"github.com/otiai10/copy"
	"k8s.io/klog/v2"
)

var (
	ThumbDateFormat = "2006-01-02"
	ModTimeFormat   = "150405"
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
		return nil, fmt.Errorf("stat: %v", err)
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

	var img image.Image
	thumbs := map[string]ThumbMeta{}

	for name, t := range defaultThumbOpts {
		relPath := thumbRelPath(i, t)
		klog.Infof("thumb relpath: %s", relPath)
		fullPath := filepath.Join(outDir, relPath)

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return nil, fmt.Errorf("mkdir: %v", err)
		}

		st, err := os.Stat(fullPath)
		if err == nil && st.Size() > int64(128) && !updated {
			klog.Infof("%s exists (%d bytes)", fullPath, st.Size())
			rt, err := readThumb(fullPath)
			if err == nil {
				rt.RelPath = relPath
				klog.Infof("found thumb: %+v", *rt)
				thumbs[name] = *rt
				continue
			}
			klog.Warningf("unable to read thumb: %v", err)
		}

		if img == nil {
			klog.Infof("opening %s ...", i.Path)
			img, err = imgio.Open(i.Path)
			if err != nil {
				return nil, fmt.Errorf("imgio.Open: %v", err)
			}
		}

		ct, err := createThumb(img, fullPath, t)
		if err != nil {
			klog.Errorf("create failed: %v", err)
			return nil, fmt.Errorf("create thumb: %w", err)
		}

		ct.RelPath = relPath
		thumbs[name] = *ct
		klog.Infof("created thumb: %+v", ct)
	}

	return thumbs, nil
}

func createThumb(i image.Image, path string, t ThumbOpts) (*ThumbMeta, error) {
	klog.Infof("creating %dx%d thumb: %s - %+v", t.X, t.Y, path, i.Bounds())
	x := t.X
	y := t.Y

	if i.Bounds().Dy() == 0 {
		return nil, fmt.Errorf("no Y for %+v", i)
	}

	if i.Bounds().Dx() == 0 {
		return nil, fmt.Errorf("no X for %+v", i)
	}

	if t.X == 0 {
		scale := float64(i.Bounds().Dy()) / float64(t.Y)
		x = int(float64(i.Bounds().Dx()) / scale)
	}

	if t.Y == 0 {
		scale := float64(i.Bounds().Dx()) / float64(t.X)
		klog.Infof("scale = %d (%d / %d)", scale, i.Bounds().Dx(), t.X)
		y = int(float64(i.Bounds().Dy()) / scale)
	}

	rimg := transform.Resize(i, x, y, transform.Lanczos)
	if err := imgio.Save(path, rimg, imgio.JPEGEncoder(t.Quality)); err != nil {
		klog.Errorf("save failed: %v", err)
		return nil, fmt.Errorf("save: %w", err)
	}

	klog.Infof("created")
	return &ThumbMeta{X: rimg.Bounds().Dx(), Y: rimg.Bounds().Dy()}, nil
}

func readThumb(path string) (*ThumbMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	image, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("unable to decode: %v", err)
	}

	return &ThumbMeta{X: image.Width, Y: image.Height}, nil
}

// thumbRelPath returns a relative path to a thumbnail, optimizing for both cache busting and SEO
func thumbRelPath(i Image, t ThumbOpts) string {
	base := filepath.Base(i.RelPath)
	ext := filepath.Ext(base)
	noExt := strings.TrimSuffix(base, ext)

	thumbDir := filepath.Join(filepath.Dir(i.RelPath), "_")
	dimensions := ""
	if t.X != 0 {
		dimensions = fmt.Sprintf("x%d", t.X)
	}
	if t.Y != 0 {
		dimensions = fmt.Sprintf("y%d", t.Y)
	}

	// ModTimeFormat is important to catch minor adjustments
	newBase := fmt.Sprintf("%s_%s_%s@%s_%s.jpg", noExt, i.Title, i.Taken.Format(ThumbDateFormat), dimensions, i.ModTime.Format(ModTimeFormat))
	return filepath.Join(thumbDir, newBase)
}
