package fj

import (
	"time"
)

// ThumbMeta describes a thumbnail.
type ThumbMeta struct {
	X       int
	Y       int
	RelPath string
}

type Image struct {
	InPath  string
	OutPath string
	ModTime time.Time
	RelPath string
	Hier    []string

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

	Width  int64
	Height int64
}

type Album struct {
	StartTime time.Time
	EndTime   time.Time

	InPath  string
	OutPath string
	ModTime time.Time
	Hier    []string

	Title       string
	Description string

	Images []*Image
}
