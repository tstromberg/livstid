package livstid

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/barasher/go-exiftool"
	"github.com/karrick/godirwalk"
	"k8s.io/klog/v2"
)

var exifDate = "2006:01:02 15:04:05"

func read(path string, et *exiftool.Exiftool) (*Image, error) {
	fis := et.ExtractMetadata(path)
	fi := fis[0]
	i := &Image{}
	var err error

	if fi.Err != nil {
		return i, fmt.Errorf("extract fail for %q: %w", path, fi.Err)
	}

	for k, v := range fi.Fields {
		klog.V(2).Infof("%q=%v\n", k, v)
	}

	i.Make, err = fi.GetString("Make")
	if err != nil {
		klog.V(1).Infof("unable to get make for %s: %v", path, err)
	}

	i.Make = strings.TrimSpace(strings.ReplaceAll(i.Make, "CORPORATION", ""))

	i.Model, err = fi.GetString("Model")
	if err != nil {
		klog.V(1).Infof("unable to get model for %s: %v", path, err)
	}

	i.Model = strings.TrimSpace(strings.ReplaceAll(i.Model, i.Make, ""))

	i.LensMake, _ = fi.GetString("LensMake")
	i.LensModel, _ = fi.GetString("LensModel")

	i.Height, err = fi.GetInt("ImageHeight")
	if err != nil {
		return i, fmt.Errorf("get ImageHeight: %w", err)
	}

	i.Width, err = fi.GetInt("ImageWidth")
	if err != nil {
		return i, fmt.Errorf("get ImageWidth: %w", err)
	}

	i.ISO, err = fi.GetInt("ISO")
	if err != nil {
		klog.V(1).Infof("unable to get ISO for %s: %v", path, err)
	}

	i.Aperture, err = fi.GetFloat("ApertureValue")
	if err != nil {
		klog.V(1).Infof("unable to get aperture for %s: %v", path, err)
	}

	i.Speed, err = fi.GetString("ShutterSpeed")
	if err != nil {
		klog.V(1).Infof("unable to get shutter speed for %s: %v", path, err)
	}

	i.FocalLength, err = fi.GetString("FocalLength")
	if err != nil {
		klog.V(1).Infof("unable to get focal length for %s: %v", path, err)
	}

	i.FocalLength = strings.ReplaceAll(i.FocalLength, ".0", "")
	i.Keywords, _ = fi.GetStrings("Keywords")
	i.Description, _ = fi.GetString("ImageDescription")

	i.Title, err = fi.GetString("Headline")
	if err != nil {
		klog.V(2).Infof("unable to get headline: %v", err)
	}

	ds, err := fi.GetString("DateTimeOriginal")
	if err != nil {
		klog.V(1).Infof("unable to get date time for %s: %v", path, err)
		return i, nil
	}

	i.Taken, err = time.Parse(exifDate, ds)
	if err != nil {
		return i, fmt.Errorf("parse time %q: %w", ds, err)
	}

	return i, nil
}

func removeDupes(is []*Image) []*Image {
	seen := map[string]*Image{}
	for _, i := range is {
		key := fmt.Sprintf("%s-%s-%d", i.Taken, i.Speed, i.ISO)
		if seen[key] == nil {
			seen[key] = i
			continue
		}
		klog.Infof("photo dupe found: %s (choosing best)", i.InPath)

		if len(i.Description) > len(seen[key].Description) {
			klog.V(1).Infof("will use %s instead!", i.BasePath)
			seen[key] = i
			continue
		}

		// use the longest base path? so that we include '-edited' photos.
		if len(i.BasePath) > len(seen[key].BasePath) {
			klog.V(1).Infof("will use %s instead!", i.BasePath)
			seen[key] = i
		}
	}

	result := []*Image{}
	for _, k := range seen {
		result = append(result, k)
	}
	return result
}

// Find searches for images in a directory tree.
func Find(root string, sidecars bool) ([]*Image, error) {
	klog.Infof("finding files in %s ...", root)
	found := []*Image{}

	et, err := exiftool.NewExiftool()
	if err != nil {
		klog.Exitf("exiftool failed: %v\n", err)
	}
	defer func() {
		if err := et.Close(); err != nil {
			klog.Errorf("Failed to close exiftool: %v", err)
		}
	}()

	err = godirwalk.Walk(root, &godirwalk.Options{
		Callback: func(path string, _ *godirwalk.Dirent) error {
			if filepath.Base(path)[0] == '.' {
				return godirwalk.SkipThis
			}

			if strings.HasSuffix(path, "jpg") {
				klog.V(1).Infof("found %s", path)
				fi, err := os.Stat(path)
				if err != nil {
					klog.Errorf("stat failure: %v", err)
					return err
				}

				i, err := read(path, et)
				if err != nil {
					klog.Errorf("read failure: %v", err)
					return err
				}

				i.InPath = path
				i.RelPath, err = filepath.Rel(root, path)
				if err != nil {
					return err
				}
				i.BasePath = urlSafePath(filepath.Base(path))

				i.Hier = strings.Split(i.RelPath, string(filepath.Separator))

				i.ModTime = fi.ModTime()

				if sidecars {
					if err := processSidecars(i); err != nil {
						klog.Errorf("sidecars: %v", err)
					}
				}
				found = append(found, i)
			}

			return nil
		},
	})

	return removeDupes(found), err
}

func processSidecars(i *Image) error {
	// so far we only process Google Takeout sidecars
	tp := i.InPath + ".json"
	if _, err := os.Stat(tp); err != nil {
		return nil // no sidecar file, not an error
	}
	bs, err := os.ReadFile(tp)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	side := &TakeoutSidecar{}
	if err := json.Unmarshal(bs, side); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	if side.Description != "" {
		i.Title = side.Description
		klog.Infof("%s: found sidecar title: %q", i.BasePath, i.Title)
	}
	return nil
}
