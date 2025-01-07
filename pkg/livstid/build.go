package livstid

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/otiai10/copy"
	"k8s.io/klog/v2"
)

//go:embed assets/ng2/recent.tmpl
var streamTmpl string

//go:embed assets/ng2/index.tmpl
var idxTmpl string

//go:embed assets/ng2/album.tmpl
var albumTmpl string

var assetsDir = "pkg/livstid/assets/ng2"

var favKeyword = "fav"

func Build(inDir string, outDir string) error {
	klog.Infof("build: %s -> %s", inDir, outDir)

	is, err := Find(inDir)
	if err != nil {
		return fmt.Errorf("find: %w", err)
	}

	albums := map[string]*Album{}
	favs := map[string]*Album{}
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
				Hier:    strings.Split(rd, string(filepath.Separator)),
			}
		}
		albums[rd].Images = append(albums[rd].Images, i)

		if !slices.Contains(i.Keywords, favKeyword) {
			continue
		}

		for _, k := range i.Keywords {
			if k == favKeyword {
				k = "all"
			}

			if favs[k] == nil {
				favs[k] = &Album{
					InPath:  rd,
					OutPath: filepath.Join(outDir, "tags", k),
					Images:  []*Image{},
					Title:   k,
					Hier:    []string{"tags", k},
				}
			}
			favs[k].Images = append(favs[k].Images, i)

		}
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

	fs := []*Album{}
	for _, f := range favs {
		if len(f.Images) > 1 {
			fs = append(fs, f)
		}
	}

	if err := writeAlbumIndex(outDir, as, fs); err != nil {
		return fmt.Errorf("write album index: %w", err)
	}

	if err := writeAlbums(as); err != nil {
		return fmt.Errorf("write albums: %w", err)
	}

	if err := writeAlbums(fs); err != nil {
		return fmt.Errorf("write favorites: %w", err)
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

func writeAlbumIndex(outDir string, as []*Album, fs []*Album) error {
	klog.Infof("writing album index with %d albums ...", len(as))
	bs, err := renderAlbumIndex("Albums", outDir, as, fs, idxTmpl)
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

		if err := os.MkdirAll(a.OutPath, 0o755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}

		p := filepath.Join(filepath.Join(a.OutPath, "index.html"))
		klog.Infof("Writing album index to %s", p)

		if err := os.WriteFile(p, bs, 0o600); err != nil {
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

func renderAlbumIndex(title string, outDir string, as []*Album, fs []*Album, ts string) ([]byte, error) {
	tmpl, err := template.New("album index").Funcs(tmplFunctions()).Parse(ts)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outDir), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	data := struct {
		Title     string
		OutDir    string
		Albums    []*Album
		Favorites []*Album
	}{
		Title:     title,
		OutDir:    outDir,
		Albums:    as,
		Favorites: fs,
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
