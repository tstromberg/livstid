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

var (
	favKeyword          = "fav"
	maxAlbum            = 24
	maxHierAlbum        = 48
	maxTopHierAlbum     = 365
	minAlbumSize        = 4
	entityChar          = regexp.MustCompile(`\%[0-9A-Fa-f]{2,4}`)
	multipleUnderscores = regexp.MustCompile(`_{2,}`)
)

// an Assembly is an assembled collection of images.
type Assembly struct {
	Images     []*Image
	Albums     []*Album
	HierAlbums []*Album
	Favorites  []*Album
	Recent     *Album
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

	out := strings.ToLower(strings.Join(o, "/"))
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
	hierAlbums := map[string]*Album{}
	favs := map[string]*Album{}
	for _, i := range is {
		klog.V(1).Infof("build image: %+v", i)
		if c.BuildThumbnails {
			i.Resize, err = thumbnails(*i, outDir)
			if err != nil {
				return nil, fmt.Errorf("thumbnails: %w", err)
			}
		}

		safeRelPath := urlSafePath(i.RelPath)
		rd := filepath.Dir(i.RelPath)
		i.OutPath = filepath.Join(outDir, safeRelPath)
		hier := strings.Split(rd, string(filepath.Separator))
		if filepath.Base(rd) == "EmptyName" {
			klog.Infof("skipping EmptyName ...")
			continue
		}

		if albums[rd] == nil {
			albums[rd] = &Album{
				InPath:  filepath.Join(c.InDir, rd),
				OutPath: filepath.Join(outDir, urlSafePath(rd)),
				Images:  []*Image{},
				Title:   filepath.Base(rd),
				Hier:    hier,
			}
		}
		albums[rd].Images = append(albums[rd].Images, i)

		// virtual albums based on hierarchy
		for level := range hier {
			if level == 0 || level == len(hier) {
				continue
			}
			valbum := strings.Join(hier[0:level], "/")
			if hierAlbums[valbum] == nil {
				hierAlbums[valbum] = &Album{
					InPath:    filepath.Join(c.InDir, valbum),
					OutPath:   filepath.Join(outDir, valbum),
					Images:    []*Image{},
					Title:     valbum,
					Hier:      strings.Split(valbum, string(filepath.Separator)),
					HierLevel: level,
				}
			}
			hierAlbums[valbum].Images = append(hierAlbums[valbum].Images, i)
		}

		if !slices.Contains(i.Keywords, favKeyword) {
			continue
		}

		for _, k := range i.Keywords {
			if k == favKeyword {
				k = "all"
			}

			if favs[k] == nil {
				klog.Infof("FAVORITE %s: %s", k, i.BasePath)
				favs[k] = &Album{
					InPath:  rd,
					OutPath: filepath.Join(outDir, "favorites", k),
					Images:  []*Image{},
					Title:   k,
					Hier:    []string{"favorites", k},
				}
			}
			favs[k].Images = append(favs[k].Images, i)

		}
	}

	as := []*Album{}
	for _, a := range albums {
		is := a.Images
		if len(is) < minAlbumSize {
			continue
		}
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
		if len(f.Images) >= minAlbumSize {
			fs = append(fs, f)
		}
	}

	hs := []*Album{}
	for _, f := range hierAlbums {
		hs = append(fs, f)
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
		Images:     is,
		Albums:     as,
		Favorites:  fs,
		Recent:     recent,
		HierAlbums: hs,
	}, nil
}

// Validate checks the assembly for potential issues with image and album counts
func (a *Assembly) Validate() []error {
	var errors []error

	// Check album photo count
	for _, album := range a.Albums {
		klog.Infof("%s has %d photos [level=%d]", album.Title, len(album.Images), album.HierLevel)

		max := maxAlbum
		if album.HierLevel == 1 {
			max = maxTopHierAlbum
		}

		if album.HierLevel > 1 {
			max = maxHierAlbum
		}
		if len(album.Images) > max {
			album.Hidden = true
			errors = append(errors, fmt.Errorf("Album '%s' contains %d images, which exceeds the %d image limit at hierarchy level %d", album.Title, len(album.Images), max, album.HierLevel))
		}
	}

	return errors
}
