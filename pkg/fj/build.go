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

//go:embed assets/album_index.tmpl
var albumIdxTmpl string

//go:embed assets/album_index.css
var albumIdxCSS string

func Build(inDir string, outDir string) error {
	klog.Infof("build: %s -> %s", inDir, outDir)

	albums := map[string]*Album{}
	is, err := Find(inDir)
	if err != nil {
		return fmt.Errorf("find: %w", err)
	}

	for _, i := range is {
		klog.Infof("build image: %+v", i)
		i.Thumbnails, err = thumbnails(*i, outDir)
		if err != nil {
			return fmt.Errorf("thumbnails: %w", err)
		}

		i.ThumbPath = i.Thumbnails["512x"].RelPath

		if i.ThumbPath == "" {
			return fmt.Errorf("unable to find thumb for %+v", i)
		}

		klog.Infof("thumbpath: %s", i.ThumbPath)

		rd := filepath.Dir(i.RelPath)
		if albums[rd] == nil {
			albums[rd] = &Album{
				RelPath: rd,
				Images:  []*Image{},
			}
		}
		albums[rd].Images = append(albums[rd].Images, i)
	}

	bs, err := renderStream("fj stream", is)
	if err != nil {
		return fmt.Errorf("render stream: %w", err)
	}

	if err = ioutil.WriteFile(filepath.Join(outDir, "index.html"), bs, 0o600); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	abs := []*Album{}
	for _, a := range albums {
		abs = append(abs, a)
	}
	// TODO: Sort by date

	bs, err = renderAlbumIndex("fj albums", abs)
	if err != nil {
		return fmt.Errorf("render albums: %w", err)
	}

	if err = ioutil.WriteFile(filepath.Join(outDir, "albums.html"), bs, 0o600); err != nil {
		return fmt.Errorf("write albums: %w", err)
	}

	return nil
}

func renderStream(title string, is []*Image) ([]byte, error) {
	funcMap := template.FuncMap{
		"Odd": func(i int) bool {
			return i%2 == 1
		},
	}
	tmpl, err := template.New("stream").Funcs(funcMap).Parse(streamTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
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
		return nil, fmt.Errorf("execute: %w", err)
	}

	out := tpl.Bytes()
	return out, nil
}

func renderAlbumIndex(title string, as []*Album) ([]byte, error) {
	funcMap := template.FuncMap{}
	tmpl, err := template.New("album").Funcs(funcMap).Parse(albumIdxTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	data := struct {
		Title      string
		Stylesheet template.CSS
		Albums     []*Album
	}{
		Title:      title,
		Stylesheet: template.CSS(albumIdxCSS),
		Albums:     as,
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	out := tpl.Bytes()
	return out, nil
}
