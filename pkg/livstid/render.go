package livstid

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
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

var assetsDir = "pkg/livstid/assets/ng2"

func Render(c *Config, a *Assembly) error {
	if err := copyAssets(assetsDir, c.OutDir); err != nil {
		return fmt.Errorf("copyAssets: %w", err)
	}

	if err := writeAlbums(c, a.Albums); err != nil {
		return fmt.Errorf("write albums: %w", err)
	}

	if err := writeAlbums(c, a.Favorites); err != nil {
		return fmt.Errorf("write favorites: %w", err)
	}

	if err := writeRecent(c, a.Recent); err != nil {
		return fmt.Errorf("write stream: %w", err)
	}

	if err := writeIndex(c, a); err != nil {
		return fmt.Errorf("write index: %w", err)
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

func writeRecent(c *Config, a *Album) error {
	klog.Infof("writing recent with %d images ...", len(a.Images))

	bs, err := renderAlbum(c, a, streamTmpl)
	if err != nil {
		return fmt.Errorf("render stream: %w", err)
	}

	path := filepath.Join(c.OutDir, "recent.html")
	klog.Infof("Writing stream index to %s", path)
	return os.WriteFile(path, bs, 0o644)
}

func writeIndex(c *Config, a *Assembly) error {
	klog.Infof("writing album index with %d albums ...", len(a.Albums))
	bs, err := renderAlbumIndex(c, a, idxTmpl)
	if err != nil {
		return fmt.Errorf("render albums: %w", err)
	}

	p := filepath.Join(c.OutDir, "index.html")
	klog.Infof("Writing album index to %s", p)
	return os.WriteFile(p, bs, 0o644)
}

func writeAlbums(c *Config, as []*Album) error {
	for _, a := range as {
		klog.Infof("rendering album %s with %d images ...", a.OutPath, len(a.Images))
		bs, err := renderAlbum(c, a, albumTmpl)
		if err != nil {
			return fmt.Errorf("render album: %w", err)
		}

		if err := os.MkdirAll(a.OutPath, 0o755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}

		p := filepath.Join(filepath.Join(a.OutPath, "index.html"))
		klog.Infof("Writing album index to %s", p)

		if err := os.WriteFile(p, bs, 0o644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
	}

	return nil
}

func renderAlbum(c *Config, a *Album, templateString string) ([]byte, error) {
	tmpl, err := template.New("album").Funcs(tmplFunctions()).Parse(templateString)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	is := a.Images
	sort.Slice(is, func(i, j int) bool {
		return is[i].Taken.After(is[j].Taken)
	})

	data := struct {
		Title      string
		Collection string
		Album      *Album
	}{
		Collection: c.Collection,
		Title:      a.Title,
		Album:      a,
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	out := tpl.Bytes()
	return out, nil
}

func renderAlbumIndex(c *Config, a *Assembly, ts string) ([]byte, error) {
	tmpl, err := template.New("album index").Funcs(tmplFunctions()).Parse(ts)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	data := struct {
		Collection  string
		Description string
		OutDir      string
		Albums      []*Album
		Favorites   []*Album
	}{
		Collection:  c.Collection,
		Description: c.Description,
		OutDir:      c.OutDir,
		Albums:      a.Albums,
		Favorites:   a.Favorites,
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
