package livstid

// Config holds configuration for livstid.
type Config struct {
	Thumbnails      map[string]ThumbOpts
	OutDir          string
	Collection      string
	Description     string
	RCloneTarget    string
	InDirs          []string
	ProcessSidecars bool
}

// TakeoutSidecar is a JSON file for EXIF overrides that is compatible with Google Takeout.
type TakeoutSidecar struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	// Not compatible
	Tags []string `json:"tags"`
}
