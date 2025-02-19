package livstid

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otiai10/copy"
	"k8s.io/klog/v2"
)

//go:embed assets/ng2/recent.tmpl
var streamTmpl string

//go:embed assets/ng2/index.tmpl
var idxTmpl string

//go:embed assets/ng2/album.tmpl
var albumTmpl string

//go:embed assets/ng2/style.css
var styleText string

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

	if err := writeAlbums(c, a.TagAlbums); err != nil {
		return fmt.Errorf("write tags: %w", err)
	}

	if err := writeAlbums(c, a.HierAlbums); err != nil {
		return fmt.Errorf("write hier albums: %w", err)
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
		klog.V(1).Infof("copying %d assets from %s", len(ms), src)
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
	klog.V(1).Infof("writing recent with %d images ...", len(a.Images))

	bs, err := renderAlbum(c, a, streamTmpl)
	if err != nil {
		return fmt.Errorf("render stream: %w", err)
	}

	path := filepath.Join(c.OutDir, "recent/all/index.html")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	klog.V(1).Infof("Writing stream index to %s", path)
	return os.WriteFile(path, bs, 0o644)
}

func writeIndex(c *Config, a *Assembly) error {
	klog.V(1).Infof("writing album index with %d albums ...", len(a.Albums))
	bs, err := renderAlbumIndex(c, a, idxTmpl)
	if err != nil {
		return fmt.Errorf("render albums: %w", err)
	}

	p := filepath.Join(c.OutDir, "index.html")
	klog.V(1).Infof("Writing album index to %s", p)
	return os.WriteFile(p, bs, 0o644)
}

func writeAlbums(c *Config, as []*Album) error {
	klog.Infof("Writing out %d albums ...", len(as))
	for _, a := range as {
		klog.V(1).Infof("rendering album %s [%s] with %d images ...", a.Title, a.OutPath, len(a.Images))
		bs, err := renderAlbum(c, a, albumTmpl)
		if err != nil {
			return fmt.Errorf("render album: %w", err)
		}

		if err := os.MkdirAll(a.OutPath, 0o755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}

		p := filepath.Join(filepath.Join(a.OutPath, "index.html"))
		klog.V(1).Infof("Writing album index to %s", p)

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

	data := struct {
		Title      string
		Collection string
		Album      *Album
		Style      template.CSS
	}{
		Collection: c.Collection,
		Title:      a.Title,
		Album:      a,
		Style:      template.CSS(styleText),
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
		Recent      *Album
		Style       template.CSS
	}{
		Collection:  c.Collection,
		Description: c.Description,
		OutDir:      c.OutDir,
		Albums:      a.Albums,
		Favorites:   a.Favorites,
		Recent:      a.Recent,
		Style:       template.CSS(styleText),
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
		"Upward": func(hier []string, num int) string {
			if len(hier)-1 == num {
				return ""
			}
			parts := len(hier) - num - 1
			relPath := []string{}
			for i := 0; i < parts; i++ {
				relPath = append(relPath, "..")
			}

			klog.V(1).Infof("upward %s [len=%d] num=%d parts=%d - returning %v", hier, len(hier), num, parts, relPath)
			return strings.Join(relPath, "/")
		},
		"ToRoot": func(hier []string) string {
			relPath := []string{}
			for range hier {
				relPath = append(relPath, "..")
			}
			return strings.Join(relPath, "/")
		},
		"ImageURL": func(b string, s string) string {
			r, err := filepath.Rel(b, s)
			if err != nil {
				return fmt.Sprintf("ERROR[%v]", err)
			}
			return filepath.Dir(r) + "/#nanogallery/i/0/" + filepath.Base(r)
		},
		"Random": func(as []*Album) *Image {
			if len(as) == 0 {
				return &Image{}
			}
			is := []*Image{}
			for _, a := range as {
				is = append(is, a.Images...)
			}
			return is[rand.Intn(len(is))]
		},
		"MostRecentTime": func(a *Album) time.Time {
			d := time.Time{}
			for _, i := range a.Images {
				if i.Taken.After(d) {
					d = i.Taken
				}
			}
			return d
		},
		"First": func(a *Album) *Image {
			return a.Images[0]
		},
		"RandInHier": func(as []*Album, top string) *Image {
			is := []*Image{}
			for _, a := range as {
				if a.Hier[0] == top {
					is = append(is, a.Images...)
				}
			}
			return is[rand.Intn(len(is))]
		},

		"BasePath": filepath.Base,
	}
}
