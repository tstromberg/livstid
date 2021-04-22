package fj

import (
	"fmt"
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
		return i, fmt.Errorf("extract fail for %q: %v", path, fi.Err)
	}

	for k, v := range fi.Fields {
		klog.V(2).Infof("%q=%v\n", k, v)
	}

	i.Make, err = fi.GetString("Make")
	if err != nil {
		return i, fmt.Errorf("Make: %v", err)
	}

	i.Model, err = fi.GetString("Model")
	if err != nil {
		return i, fmt.Errorf("Model: %v", err)
	}

	i.LensMake, err = fi.GetString("LensMake")
	if err != nil {
		return i, fmt.Errorf("LensMake: %v", err)
	}

	i.LensModel, err = fi.GetString("LensModel")
	if err != nil {
		return i, fmt.Errorf("LensModel: %v", err)
	}

	i.Height, err = fi.GetInt("ImageHeight")
	if err != nil {
		return i, fmt.Errorf("ImageHeight: %v", err)
	}

	i.Width, err = fi.GetInt("ImageWidth")
	if err != nil {
		return i, fmt.Errorf("ImageWidth: %v", err)
	}

	i.ISO, err = fi.GetInt("ISO")
	if err != nil {
		return i, fmt.Errorf("ISO: %v", err)
	}

	i.Aperture, err = fi.GetFloat("ApertureValue")
	if err != nil {
		return i, fmt.Errorf("ApertureValue: %v", err)
	}

	i.Speed, err = fi.GetString("ShutterSpeed")
	if err != nil {
		return i, fmt.Errorf("ShutterSpeed: %v", err)
	}

	i.FocalLength, err = fi.GetString("FocalLength")
	if err != nil {
		return i, fmt.Errorf("FocalLength: %v", err)
	}
	i.FocalLength = strings.Replace(i.FocalLength, ".0", "", -1)

	i.Keywords, err = fi.GetStrings("Keywords")
	if err != nil {
		klog.Errorf("unable to get keywords: %v", err)
	}

	i.Description, err = fi.GetString("ImageDescription")
	if err != nil {
		klog.Errorf("unable to get description: %v", err)
	}

	i.Title, err = fi.GetString("Headline")
	if err != nil {
		klog.Errorf("unable to get headline: %v", err)
	}

	ds, err := fi.GetString("DateTimeOriginal")
	if err != nil {
		return i, fmt.Errorf("DateTimeOriginal: %v", err)
	}

	i.Taken, err = time.Parse(exifDate, ds)
	if err != nil {
		return i, fmt.Errorf("parse time %q: %v", ds, err)
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
			if strings.Contains(path, ".git") {
				return godirwalk.SkipThis
			}

			if strings.HasSuffix(path, "jpg") {
				klog.Infof("found %s", path)
				i, err := read(path, et)
				if err != nil {
					return err
				}

				i.Path = path
				i.RelPath, err = filepath.Rel(root, path)
				if err != nil {
					return err
				}

				found = append(found, &i)
			}

			return nil
		},
	})

	return found, err
}
