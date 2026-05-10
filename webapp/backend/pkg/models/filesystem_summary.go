package models

type FilesystemSummaryWrapper struct {
	Success bool `json:"success"`
	Data    struct {
		Filesystems map[string][]FilesystemCapacity  `json:"filesystems"`
		Hosts       map[string]*FilesystemHostStatus `json:"hosts"`
	} `json:"data"`
}
