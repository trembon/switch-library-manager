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

type SwitchTitle struct {
	Name        string `json:"name"`
	TitleId     string `json:"titleId"`
	Icon        string `json:"icon"`
	Region      string `json:"region"`
	ReleaseDate string `json:"release_date"`
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
