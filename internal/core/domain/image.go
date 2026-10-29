package domain

// ImageSet matches the ImageSet schema in docs/api/openapi.yaml: public URLs of the pre-resized variants.
type ImageSet struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

// ImageKeys are the storage object keys of the variants. They are persisted in uploads.keys
// and turned into URLs by the storage adapter.
type ImageKeys struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

// ImageUpload matches the ImageUpload schema in docs/api/openapi.yaml.
type ImageUpload struct {
	ID    int64    `json:"id"`
	Image ImageSet `json:"image"`
}

// All returns the keys in a stable order.
func (k ImageKeys) All() []string { return []string{k.Small, k.Medium, k.Large} }
