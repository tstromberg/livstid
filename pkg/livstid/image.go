package livstid

import (
	"time"
)

// ThumbMeta describes a thumbnail.
type ThumbMeta struct {
	X       int
	Y       int
	RelPath string
	Path    string
}

// Image represents a photo with its metadata.
type Image struct {
	InPath   string
	OutPath  string
	BasePath string
	ModTime  time.Time
	RelPath  string
	Hier     []string

	Resize map[string]ThumbMeta
	Taken  time.Time

	Keywords    []string
	Title       string
	Description string

	Make  string
	Model string

	LensMake  string
	LensModel string

	Aperture    float64
	FocalLength string
	ISO         int64
	Speed       string

	Highlight bool

	Width  int64
	Height int64
}

// Album represents a collection of images.
type Album struct {
	StartTime time.Time
	EndTime   time.Time

	InPath    string
	RelPath   string
	OutPath   string
	ModTime   time.Time
	Hier      []string
	HierLevel int

	Title       string
	Description string

	Images []*Image
	Hidden bool
}
