package livstid

import (
	"time"
)

// ThumbMeta describes a thumbnail.
type ThumbMeta struct {
	RelPath string
	Path    string
	X       int
	Y       int
}

// Image represents a photo with its metadata.
type Image struct {
	ModTime     time.Time
	Taken       time.Time
	Resize      map[string]ThumbMeta
	BasePath    string
	RelPath     string
	InPath      string
	FocalLength string
	OutPath     string
	Speed       string
	Title       string
	Description string
	Make        string
	Model       string
	LensMake    string
	LensModel   string
	Hier        []string
	Keywords    []string
	Aperture    float64
	ISO         int64
	Width       int64
	Height      int64
	Highlight   bool
}

// Album represents a collection of images.
type Album struct {
	StartTime   time.Time
	EndTime     time.Time
	ModTime     time.Time
	InPath      string
	RelPath     string
	OutPath     string
	Title       string
	Description string
	Hier        []string
	Images      []*Image
	HierLevel   int
	Hidden      bool
}
