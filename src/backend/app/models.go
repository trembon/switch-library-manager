package app

type Pair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LocalLibraryData struct {
	LibraryData []LibraryTemplateData `json:"library_data"`
	Issues      []Pair                `json:"issues"`
	NumFiles    int                   `json:"num_files"`
}

type OrganizationResult struct {
	Status        string           `json:"status"`
	ConflictCount int              `json:"conflict_count"`
	Library       LocalLibraryData `json:"library"`
}

type SwitchTitle struct {
	Name        string `json:"name"`
	TitleId     string `json:"titleId"`
	Icon        string `json:"icon"`
	Region      string `json:"region"`
	ReleaseDate string `json:"release_date"`
	TitleMetadata
}

// TitleMetadata is optional catalog information shown in extra table columns.
// List values are sent as received; the frontend joins them for display.
type TitleMetadata struct {
	Publisher     string   `json:"publisher"`
	Genres        []string `json:"genres"`
	Languages     []string `json:"languages"`
	Players       int      `json:"players"`
	AgeRating     int      `json:"age_rating"`
	RatingContent []string `json:"rating_content"`
}

type LibraryTemplateData struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Dlc     string `json:"dlc"`
	TitleId string `json:"titleId"`
	Path    string `json:"path"`
	Icon    string `json:"icon"`
	Update  int    `json:"update"`
	Region  string `json:"region"`
	Type    string `json:"type"`
	// UpdateID is the title ID of the installed update, empty without one.
	UpdateID string `json:"update_id"`
	// FileSize is the base file's size in bytes; zero when it is unknown,
	// such as for split files.
	FileSize    int64  `json:"file_size"`
	ReleaseDate string `json:"release_date"`
	TitleMetadata
}

type ProgressUpdate struct {
	Curr    int    `json:"curr"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

type OrganizationPreview struct {
	RootFolder string                     `json:"root_folder"`
	Entries    []OrganizationPreviewEntry `json:"entries"`
}

type OrganizationPreviewEntry struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}
