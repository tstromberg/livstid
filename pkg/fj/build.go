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

//go:embed assets/album.tmpl
var albumTmpl string

//go:embed assets/album.css
var albumCSS string

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
		i.OutPath = filepath.Join(outDir, i.RelPath)

		if albums[rd] == nil {
			albums[rd] = &Album{
				InPath:  rd,
				OutPath: filepath.Join(outDir, rd),
				Images:  []*Image{},
			}
		}
		albums[rd].Images = append(albums[rd].Images, i)
	}

	if err := writeStream(outDir, is); err != nil {
		return fmt.Errorf("write stream: %w", err)
	}

	as := []*Album{}
	for _, a := range albums {
		as = append(as, a)
	}
	// TODO: Sort by date

	if err := writeAlbumIndex(outDir, as); err != nil {
		return fmt.Errorf("write stream: %w", err)
	}

	if err := writeAlbums(as); err != nil {
		return fmt.Errorf("write stream: %w", err)
	}

	return nil
}

func writeStream(outDir string, is []*Image) error {
	bs, err := renderAlbum(&Album{Title: "Daily Photos", Images: is, OutPath: outDir}, streamTmpl, streamCSS)
	if err != nil {
		return fmt.Errorf("render stream: %w", err)
	}

	p := filepath.Join(outDir, "index.html")
	klog.Infof("Writing stream index to %s", p)
	return ioutil.WriteFile(p, bs, 0o600)
}

func writeAlbumIndex(outDir string, as []*Album) error {
	bs, err := renderAlbumIndex("Albums", outDir, as, albumIdxTmpl, albumIdxCSS)
	if err != nil {
		return fmt.Errorf("render albums: %w", err)
	}

	p := filepath.Join(outDir, "albums.html")
	klog.Infof("Writing album index to %s", p)
	return ioutil.WriteFile(p, bs, 0o600)
}

func writeAlbums(as []*Album) error {
	for _, a := range as {
		bs, err := renderAlbum(a, albumTmpl, albumCSS)
		if err != nil {
			return fmt.Errorf("render album: %w", err)
		}

		p := filepath.Join(filepath.Join(a.OutPath, "index.html"))
		klog.Infof("Writing album index to %s", p)

		if p := ioutil.WriteFile(p, bs, 0o600); p != nil {
			return fmt.Errorf("write file: %w", err)
		}
	}

	return nil
}

func renderAlbum(a *Album, ts string, css string) ([]byte, error) {
	tmpl, err := template.New("album").Funcs(tmplFunctions()).Parse(ts)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	is := a.Images
	sort.Slice(is, func(i, j int) bool {
		return is[i].Taken.After(is[j].Taken)
	})

	data := struct {
		Title      string
		Stylesheet template.CSS
		Album      *Album
	}{
		Title:      a.Title,
		Stylesheet: template.CSS(css),
		Album:      a,
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	out := tpl.Bytes()
	return out, nil
}

func renderAlbumIndex(title string, outDir string, as []*Album, ts string, css string) ([]byte, error) {
	tmpl, err := template.New("album index").Funcs(tmplFunctions()).Parse(ts)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	data := struct {
		Title      string
		OutDir     string
		Stylesheet template.CSS
		Albums     []*Album
	}{
		Title:      title,
		OutDir:     outDir,
		Stylesheet: template.CSS(css),
		Albums:     as,
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	out := tpl.Bytes()
	return out, nil
}

// tmplFunctions are functions available to our templates.
func tmplFunctions() template.FuncMap {
	return template.FuncMap{
		"Odd": func(i int) bool {
			return i%2 == 1
		},
		"RelPath": func(b string, s string) string {
			r, err := filepath.Rel(b, s)
			if err != nil {
				return fmt.Sprintf("ERROR[%v]", err)
			}
			return r
		},
		"BasePath": filepath.Base,
	}
}
