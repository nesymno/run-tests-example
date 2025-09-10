package types

import (
	"time"
)

type TestData struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Data string `json:"data"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Database  string    `json:"database"`
	Cache     string    `json:"cache"`
}
