package fj

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"sort"

	"k8s.io/klog/v2"
)

//go:embed assets/stream.tmpl
var streamTmpl string

//go:embed assets/stream.css
var streamCSS string

//go:embed assets/album.tmpl
var albumTmpl string

//go:embed assets/album.css
var albumCSS string

func Build(inDir string, outDir string) error {
	klog.Infof("build: %s -> %s", inDir, outDir)

	is, err := Find(inDir)

	for _, i := range is {
		klog.Infof("build image: %+v", i)
		i.Thumbnails, err = thumbnails(*i, outDir)
		if err != nil {
			return fmt.Errorf("thumbnails: %v", err)
		}

		i.ThumbPath = i.Thumbnails["512x"].RelPath

		if i.ThumbPath == "" {
			return fmt.Errorf("unable to find thumb for %+v", i)
		}

		klog.Infof("thumbpath: %s", i.ThumbPath)
	}

	html, err := renderStream("fj stream", is)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(outDir, "index.html"), []byte(html), 0644)
	return err
}

func renderStream(title string, is []*Image) (string, error) {
	funcMap := template.FuncMap{
		"Odd": func(i int) bool {
			if i%2 == 1 {
				return true
			}
			return false
		},
	}
	tmpl, err := template.New("stream").Funcs(funcMap).Parse(streamTmpl)
	if err != nil {
		return "", fmt.Errorf("parse: %v", err)
	}

	sort.Slice(is, func(i, j int) bool {
		return is[i].Taken.After(is[j].Taken)
	})

	data := struct {
		Title      string
		Stylesheet template.CSS
		Images     []*Image
	}{
		Title:      title,
		Stylesheet: template.CSS(streamCSS),
		Images:     is,
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}

	out := tpl.String()
	return out, nil
}

func renderAlbum(title string, is []*Image) (string, error) {
	funcMap := template.FuncMap{}
	tmpl, err := template.New("album").Funcs(funcMap).Parse(albumTmpl)
	if err != nil {
		return "", fmt.Errorf("parse: %v", err)
	}

	sort.Slice(is, func(i, j int) bool {
		return is[i].Taken.After(is[j].Taken)
	})

	data := struct {
		Title      string
		Stylesheet template.CSS
		Images     []*Image
	}{
		Title:      title,
		Stylesheet: template.CSS(streamCSS),
		Images:     is,
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}

	out := tpl.String()
	return out, nil
}
