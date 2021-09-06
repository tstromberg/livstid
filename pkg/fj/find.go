package fj

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
		return i, fmt.Errorf("get Make: %w", err)
	}

	i.Model, err = fi.GetString("Model")
	if err != nil {
		return i, fmt.Errorf("get Model: %w", err)
	}

	i.LensMake, err = fi.GetString("LensMake")
	if err != nil {
		klog.Errorf("unable to get LensMake: %w", err)
	}

	i.LensModel, err = fi.GetString("LensModel")
	if err != nil {
		klog.Errorf("unable to get LensModel: %w", err)
	}

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
		return i, fmt.Errorf("get ISO: %w", err)
	}

	i.Aperture, err = fi.GetFloat("ApertureValue")
	if err != nil {
		return i, fmt.Errorf("get ApertureValue: %w", err)
	}

	i.Speed, err = fi.GetString("ShutterSpeed")
	if err != nil {
		return i, fmt.Errorf("get ShutterSpeed: %w", err)
	}

	i.FocalLength, err = fi.GetString("FocalLength")
	if err != nil {
		return i, fmt.Errorf("get FocalLength: %w", err)
	}
	i.FocalLength = strings.ReplaceAll(i.FocalLength, ".0", "")

	i.Keywords, err = fi.GetStrings("Keywords")
	if err != nil {
		klog.V(2).Infof("unable to get keywords: %w", err)
	}

	i.Description, err = fi.GetString("ImageDescription")
	if err != nil {
		klog.V(2).Infof("unable to get description: %w", err)
	}

	i.Title, err = fi.GetString("Headline")
	if err != nil {
		klog.V(2).Infof("unable to get headline: %w", err)
	}

	ds, err := fi.GetString("DateTimeOriginal")
	if err != nil {
		return i, fmt.Errorf("DateTimeOriginal: %w", err)
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
					klog.Errorf("read failure: %w", err)
					return err
				}

				i.InPath = path
				i.RelPath, err = filepath.Rel(root, path)
				if err != nil {
					return err
				}

				i.Hier = strings.Split(i.RelPath, string(filepath.Separator))

				fi, err := os.Stat(path)
				if err != nil {
					klog.Errorf("stat failure: %w", err)
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
