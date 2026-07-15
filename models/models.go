package models

type VersionResponse struct {
	Version  string `json:"version"`
	EmbaPath string `json:"emba_path"`
}

type ScanCreateResponse struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type TaskResponse struct {
	TaskID         string  `json:"task_id"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	CreatedAt      string  `json:"created_at"`
	CompletedAt    *string `json:"completed_at"`
	ExitCode       *int    `json:"exit_code"`
}

type TaskListItem struct {
	TaskID         string  `json:"task_id"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	CreatedAt      string  `json:"created_at"`
	CompletedAt    *string `json:"completed_at"`
}

type ScanListResponse struct {
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Items    []TaskListItem  `json:"items"`
}
