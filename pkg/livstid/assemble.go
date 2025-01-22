package livstid

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"k8s.io/klog/v2"
)

var favKeyword = "fav"
var maxAlbum = 24
var minTagAlbumSize = 3
var entityChar = regexp.MustCompile(`\%[0-9A-Fa-f]{2,4}`)
var multipleUnderscores = regexp.MustCompile(`_{2,}`)

// an Assembly is an assembled collection of images.
type Assembly struct {
	Images    []*Image
	Albums    []*Album
	Favorites []*Album
	Recent    *Album
}

func urlSafePath(in string) string {
	o := []string{}
	parts := strings.Split(in, "/")

	for _, p := range parts {
		p = url.QueryEscape(p)
		p = strings.ReplaceAll(p, "%2C", ",")
		p = strings.ReplaceAll(p, "+", "_")
		p = entityChar.ReplaceAllString(p, "_")
		p = strings.ReplaceAll(p, " ", "_")
		p = strings.ReplaceAll(p, "_-_", "-")
		p = multipleUnderscores.ReplaceAllString(p, "_")
		o = append(o, p)
	}

	out := strings.Join(o, "/")
	klog.V(1).Infof("%s -> %s", in, out)
	return out
}

// Collect collects an assembly of photos
func Collect(c *Config) (*Assembly, error) {
	inDir := c.InDir
	outDir := c.OutDir

	klog.V(1).Infof("collect: %s -> %s", inDir, outDir)

	is, err := Find(inDir)
	if err != nil {
		return nil, fmt.Errorf("find: %w", err)
	}

	albums := map[string]*Album{}
	favs := map[string]*Album{}
	for _, i := range is {
		klog.V(1).Infof("build image: %+v", i)
		i.Resize, err = thumbnails(*i, outDir)
		if err != nil {
			return nil, fmt.Errorf("thumbnails: %w", err)
		}

		safeRelPath := urlSafePath(i.RelPath)
		rd := filepath.Dir(i.RelPath)
		i.OutPath = filepath.Join(outDir, safeRelPath)

		if albums[rd] == nil {
			albums[rd] = &Album{
				InPath:  rd,
				OutPath: filepath.Join(outDir, urlSafePath(rd)),
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

	as := []*Album{}
	for _, a := range albums {
		is := a.Images
		sort.Slice(is, func(i, j int) bool {
			return is[i].Taken.Before(is[j].Taken)
		})
		a.Images = is
		for i, p := range a.Images {
			klog.V(1).Infof("%s: %d = %s [%s] (taken=%s)", a.Title, i, p.InPath, p.Title, p.Taken)
		}
		as = append(as, a)
	}

	sort.Slice(as, func(i, j int) bool {
		return as[i].InPath > as[j].InPath
	})

	fs := []*Album{}
	for _, f := range favs {
		if len(f.Images) >= minTagAlbumSize {
			fs = append(fs, f)
		}
	}

	recent := &Album{Title: "Recent", Images: is, OutPath: outDir}

	ri := recent.Images
	sort.Slice(ri, func(i, j int) bool {
		return ri[i].Taken.After(ri[j].Taken)
	})

	if len(ri) > maxAlbum {
		ri = ri[0:maxAlbum]
	}

	recent.Images = ri

	return &Assembly{
		Images:    is,
		Albums:    as,
		Favorites: fs,
		Recent:    recent,
	}, nil
}
