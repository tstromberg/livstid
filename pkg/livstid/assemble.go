package livstid

import (
	"errors"
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
	entityChar          = regexp.MustCompile(`%[0-9A-Fa-f]{2,4}`)
	multipleUnderscores = regexp.MustCompile(`_{2,}`)
)

// Assembly is an assembled collection of images.
type Assembly struct {
	Recent     *Album
	Images     []*Image
	Albums     []*Album
	HierAlbums []*Album
	Favorites  []*Album
	TagAlbums  []*Album
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

// Collect collects an assembly of photos.
func Collect(c *Config) (*Assembly, error) {
	is, err := findImages(c.InDirs, c.ProcessSidecars)
	if err != nil {
		return nil, err
	}

	albums := map[string]*Album{}
	hierAlbums := map[string]*Album{}
	favAlbums := map[string]*Album{}
	tagAlbums := map[string]*Album{}

	for _, i := range is {
		klog.V(1).Infof("build image: %+v", i)
		if len(c.Thumbnails) > 0 {
			i.Resize, err = thumbnails(i, c.Thumbnails, c.OutDir)
			if err != nil {
				return nil, fmt.Errorf("thumbnails: %w", err)
			}
		}

		if err := processImage(i, c.OutDir, albums, hierAlbums, favAlbums, tagAlbums); err != nil {
			continue
		}
	}

	return buildAssembly(is, albums, hierAlbums, favAlbums, tagAlbums, c.OutDir)
}

func findImages(dirs []string, processSidecars bool) ([]*Image, error) {
	is := []*Image{}
	for _, d := range dirs {
		fs, err := Find(d, processSidecars)
		if err != nil {
			return nil, fmt.Errorf("find: %w", err)
		}
		is = append(is, fs...)
	}
	return is, nil
}

func processImage(i *Image, outDir string, albums, hierAlbums, favAlbums, tagAlbums map[string]*Album) error {
	albumDir := filepath.Dir(i.InPath)
	safeRelPath := urlSafePath(i.RelPath)
	rd := filepath.Dir(i.RelPath)
	i.OutPath = filepath.Join(outDir, safeRelPath)
	hier := strings.Split(rd, string(filepath.Separator))
	if filepath.Base(rd) == "EmptyName" {
		klog.Infof("skipping EmptyName ...")
		return errors.New("skip")
	}

	// Add to regular albums
	if albums[rd] == nil {
		albums[rd] = &Album{
			InPath:  albumDir,
			RelPath: rd,
			OutPath: filepath.Join(outDir, urlSafePath(rd)),
			Images:  []*Image{},
			Title:   filepath.Base(rd),
			Hier:    hier,
		}
	}
	albums[rd].Images = append(albums[rd].Images, i)

	// Add to hierarchy albums
	addToHierAlbums(i, hier, hierAlbums, albumDir, outDir)

	// Add to favorite albums
	if slices.Contains(i.Keywords, favKeyword) {
		addToFavAlbums(i, favAlbums, rd, outDir)
	}

	// Add to tag albums
	addToTagAlbums(i, tagAlbums, rd, outDir)

	return nil
}

func addToHierAlbums(i *Image, hier []string, hierAlbums map[string]*Album, albumDir, outDir string) {
	for level := range hier {
		if level == 0 || level == len(hier) {
			continue
		}
		valbum := strings.Join(hier[0:level], "/")
		if hierAlbums[valbum] == nil {
			hierAlbums[valbum] = &Album{
				InPath:    filepath.Join(filepath.Dir(albumDir), valbum),
				OutPath:   filepath.Join(outDir, valbum),
				Images:    []*Image{},
				Title:     valbum,
				Hier:      strings.Split(valbum, string(filepath.Separator)),
				HierLevel: level,
			}
		}
		hierAlbums[valbum].Images = append(hierAlbums[valbum].Images, i)
	}
}

func addToFavAlbums(i *Image, favAlbums map[string]*Album, rd, outDir string) {
	for _, k := range i.Keywords {
		if k == favKeyword {
			k = "all"
		}

		if favAlbums[k] == nil {
			klog.Infof("FAVORITE %s: %s", k, i.BasePath)
			favAlbums[k] = &Album{
				InPath:  rd,
				OutPath: filepath.Join(outDir, "favorites", k),
				Images:  []*Image{},
				Title:   k,
				Hier:    []string{"favorites", k},
			}
		}
		favAlbums[k].Images = append(favAlbums[k].Images, i)
	}
}

func addToTagAlbums(i *Image, tagAlbums map[string]*Album, rd, outDir string) {
	for _, k := range i.Keywords {
		if tagAlbums[k] == nil {
			tagAlbums[k] = &Album{
				InPath:  rd,
				OutPath: filepath.Join(outDir, "tags", k),
				Images:  []*Image{},
				Title:   k,
				Hier:    []string{"tags", k},
			}
		}
		tagAlbums[k].Images = append(tagAlbums[k].Images, i)
	}
}

func buildAssembly(
	is []*Image,
	albums, hierAlbums, favAlbums, tagAlbums map[string]*Album,
	outDir string,
) (*Assembly, error) {
	as := filterAndSortAlbums(albums)
	fs := filterAlbumsBySize(favAlbums)
	ts := filterAlbumsBySize(tagAlbums)
	hs := albumsToSlice(hierAlbums)
	recent := createRecentAlbum(is, outDir)

	return &Assembly{
		Images:     is,
		Albums:     as,
		Favorites:  fs,
		Recent:     recent,
		HierAlbums: hs,
		TagAlbums:  ts,
	}, nil
}

func filterAndSortAlbums(albums map[string]*Album) []*Album {
	as := []*Album{}
	for _, a := range albums {
		imgs := a.Images
		if len(imgs) < minAlbumSize {
			continue
		}
		sort.Slice(imgs, func(i, j int) bool {
			return imgs[i].Taken.Before(imgs[j].Taken)
		})
		a.Images = imgs
		for i, p := range a.Images {
			klog.V(1).Infof("%s: %d = %s [%s] (taken=%s)", a.Title, i, p.InPath, p.Title, p.Taken)
		}
		as = append(as, a)
	}

	sort.Slice(as, func(i, j int) bool {
		return as[i].RelPath > as[j].RelPath
	})
	return as
}

func filterAlbumsBySize(albums map[string]*Album) []*Album {
	filtered := []*Album{}
	for _, a := range albums {
		if len(a.Images) >= minAlbumSize {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func albumsToSlice(albums map[string]*Album) []*Album {
	slice := []*Album{}
	for _, a := range albums {
		slice = append(slice, a)
	}
	return slice
}

func createRecentAlbum(is []*Image, outDir string) *Album {
	recent := &Album{Title: "Recent", Images: is, OutPath: outDir}

	ri := recent.Images
	sort.Slice(ri, func(i, j int) bool {
		return ri[i].Taken.After(ri[j].Taken)
	})

	if len(ri) > maxAlbum {
		ri = ri[0:maxAlbum]
	}

	recent.Images = ri
	return recent
}

// Validate checks the assembly for potential issues with image and album counts.
func (a *Assembly) Validate() []error {
	var errs []error

	// Check album photo count
	for _, album := range a.Albums {
		klog.Infof("%s has %d photos [level=%d]", album.Title, len(album.Images), album.HierLevel)

		maxImages := maxAlbum
		if album.HierLevel == 1 {
			maxImages = maxTopHierAlbum
		}

		if album.HierLevel > 1 {
			maxImages = maxHierAlbum
		}
		if len(album.Images) > maxImages {
			album.Hidden = true
			errs = append(errs, fmt.Errorf(
				"Album '%s' contains %d images, which exceeds the %d image limit at hierarchy level %d",
				album.Title, len(album.Images), maxImages, album.HierLevel,
			))
		}
	}

	return errs
}
