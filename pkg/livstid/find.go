package livstid

import (
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

func read(path string, et *exiftool.Exiftool) (Image, error) {
	fis := et.ExtractMetadata(path)
	fi := fis[0]
	i := Image{}
	var err error

	if fi.Err != nil {
		return i, fmt.Errorf("extract fail for %q: %w", path, fi.Err)
	}

	for k, v := range fi.Fields {
		klog.V(2).Infof("%q=%v\n", k, v)
	}

	i.Make, err = fi.GetString("Make")
	if err != nil {
		klog.Warningf("unable to get make for %s: %v", path, err)
	}

	i.Model, err = fi.GetString("Model")
	if err != nil {
		klog.V(1).Infof("unable to get model for %s: %v", path, err)
	}

	i.LensMake, err = fi.GetString("LensMake")
	i.LensModel, err = fi.GetString("LensModel")

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
	i.Keywords, err = fi.GetStrings("Keywords")
	i.Description, err = fi.GetString("ImageDescription")

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

func Find(root string) ([]*Image, error) {
	found := []*Image{}

	et, err := exiftool.NewExiftool()
	if err != nil {
		klog.Exitf("exiftool failed: %v\n", err)
	}
	defer et.Close()

	err = godirwalk.Walk(root, &godirwalk.Options{
		Callback: func(path string, de *godirwalk.Dirent) error {
			if filepath.Base(path)[0] == '.' {
				return godirwalk.SkipThis
			}

			if strings.HasSuffix(path, "jpg") {
				klog.Infof("found %s", path)
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

				fi, err := os.Stat(path)
				if err != nil {
					klog.Errorf("stat failure: %v", err)
					return err
				}

				i.ModTime = fi.ModTime()

				found = append(found, &i)
			}

			return nil
		},
	})

	return found, err
}
