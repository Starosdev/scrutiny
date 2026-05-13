package models

type FilesystemSummaryWrapper struct {
	Data struct {
		Filesystems map[string][]FilesystemCapacity  `json:"filesystems"`
		Hosts       map[string]*FilesystemHostStatus `json:"hosts"`
	} `json:"data"`
	Success bool `json:"success"`
}
