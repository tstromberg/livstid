package fj

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"

	"k8s.io/klog/v2"
)

//go:embed stream.tmpl
var streamTmpl string

func Build(inDir string, outDir string) error {
	klog.Infof("build: %s -> %s", inDir, outDir)

	is, err := Find(inDir)
	klog.Infof("images: %+v", is)

	for _, i := range is {
		i.Thumbnails, err = thumbnails(*i, outDir)
		i.ThumbPath = i.Thumbnails["512x"].RelPath
	}

	html, err := renderStream("fj stream", is)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(outDir, "index.html"), []byte(html), 0644)
	return err
}

func renderStream(title string, is []*Image) (string, error) {
	funcMap := template.FuncMap{}
	tmpl, err := template.New("stream").Funcs(funcMap).Parse(streamTmpl)
	if err != nil {
		return "", fmt.Errorf("parse: %v", err)
	}

	data := struct {
		Title  string
		Images []*Image
	}{
		Title:  title,
		Images: is,
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}

	out := tpl.String()
	return out, nil
}
