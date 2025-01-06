package fj

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/otiai10/copy"
	"k8s.io/klog/v2"
)

//go:embed assets/ng2/recent.tmpl
var streamTmpl string

//go:embed assets/ng2/index.tmpl
var idxTmpl string

//go:embed assets/ng2/album.tmpl
var albumTmpl string

var assetsDir = "pkg/fj/assets/ng2"

func Build(inDir string, outDir string) error {
	klog.Infof("build: %s -> %s", inDir, outDir)

	albums := map[string]*Album{}
	is, err := Find(inDir)
	if err != nil {
		return fmt.Errorf("find: %w", err)
	}

	for _, i := range is {
		klog.Infof("build image: %+v", i)
		i.Resize, err = thumbnails(*i, outDir)
		if err != nil {
			return fmt.Errorf("thumbnails: %w", err)
		}

		rd := filepath.Dir(i.RelPath)
		i.OutPath = filepath.Join(outDir, i.RelPath)

		if albums[rd] == nil {
			albums[rd] = &Album{
				InPath:  rd,
				OutPath: filepath.Join(outDir, rd),
				Images:  []*Image{},
				Title:   filepath.Base(rd),
			}
		}
		albums[rd].Images = append(albums[rd].Images, i)
	}

	if err := copyAssets(assetsDir, outDir); err != nil {
		return fmt.Errorf("copyAssets: %w", err)
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

func copyAssets(inDir string, outDir string) error {
	for _, ext := range []string{"png", "css", "jpg", "gif"} {
		src := fmt.Sprintf("%s/*.%s", inDir, ext)
		ms, err := filepath.Glob(src)
		klog.Infof("copying %d assets from %s", len(ms), src)
		if err != nil {
			return err
		}
		for _, m := range ms {
			if err := copy.Copy(m, filepath.Join(outDir, "_", filepath.Base(m))); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeStream(outDir string, is []*Image) error {
	klog.Infof("writing stream with %d images ...", len(is))
	bs, err := renderAlbum(&Album{Title: "Recent", Images: is, OutPath: outDir}, streamTmpl)
	if err != nil {
		return fmt.Errorf("render stream: %w", err)
	}

	p := filepath.Join(outDir, "recent.html")
	klog.Infof("Writing stream index to %s", p)
	return ioutil.WriteFile(p, bs, 0o600)
}

func writeAlbumIndex(outDir string, as []*Album) error {
	klog.Infof("writing album index with %d albums ...", len(as))
	bs, err := renderAlbumIndex("Albums", outDir, as, idxTmpl)
	if err != nil {
		return fmt.Errorf("render albums: %w", err)
	}

	p := filepath.Join(outDir, "index.html")
	klog.Infof("Writing album index to %s", p)
	return ioutil.WriteFile(p, bs, 0o600)
}

func writeAlbums(as []*Album) error {
	for _, a := range as {
		klog.Infof("rendering album %s with %d images ...", a.OutPath, len(a.Images))
		bs, err := renderAlbum(a, albumTmpl)
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

func renderAlbum(a *Album, ts string) ([]byte, error) {
	tmpl, err := template.New("album").Funcs(tmplFunctions()).Parse(ts)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	is := a.Images
	sort.Slice(is, func(i, j int) bool {
		return is[i].Taken.After(is[j].Taken)
	})

	data := struct {
		Title string
		Album *Album
	}{
		Title: a.Title,
		Album: a,
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	out := tpl.Bytes()
	return out, nil
}

func renderAlbumIndex(title string, outDir string, as []*Album, ts string) ([]byte, error) {
	tmpl, err := template.New("album index").Funcs(tmplFunctions()).Parse(ts)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	data := struct {
		Title     string
		OutDir    string
		Albums    []*Album
		Favorites []*Album
	}{
		Title:  title,
		OutDir: outDir,
		Albums: as,
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
