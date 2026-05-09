package models

type FileEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type VersionEntry struct {
	Skript    string            `json:"skript"`
	Minecraft string            `json:"minecraft"`
	Addons    map[string]string `json:"addons"`
	Files     []FileEntry       `json:"files"`
}

type Package struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Author      string                  `json:"author"`
	Latest      string                  `json:"latest"`
	Versions    map[string]VersionEntry `json:"versions"`
}

type PackageSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Latest      string `json:"latest"`
}
