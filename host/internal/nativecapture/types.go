package nativecapture

type WindowInfo struct {
	Handle      uint64 `json:"handle"`
	Title       string `json:"title"`
	ProcessID   uint32 `json:"process_id"`
	ProcessName string `json:"process_name"`
	ClassName   string `json:"class_name"`
}

type SnapshotRequest struct {
	Handle     uint64
	OutputPath string
}

type SnapshotResult struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
