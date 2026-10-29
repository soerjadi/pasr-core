package domain

import "time"

// Review matches the Review schema in docs/api/openapi.yaml.
type Review struct {
	ID         int64     `json:"id"`
	Rating     int32     `json:"rating"`
	Text       string    `json:"text"`
	AuthorName string    `json:"authorName"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ReviewPage matches the ReviewPage schema.
type ReviewPage = Page[Review]
