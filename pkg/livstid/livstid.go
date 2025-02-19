package livstid

type Config struct {
	InDirs          []string
	OutDir          string
	Collection      string
	Description     string
	RCloneTarget    string
	Thumbnails      map[string]ThumbOpts
	ProcessSidecars bool
}

// Sidecar is a JSON file for EXIF overrides that is compatible with Google Takeout
type TakeoutSidecar struct {
	Title       string
	Description string
	// Not compatible
	Tags []string
}
