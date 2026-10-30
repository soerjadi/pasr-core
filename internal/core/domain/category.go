package domain

type Category struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	IconKey string `json:"iconKey"`
}

// CategoryInput matches the CategoryInput schema of the operator API: nil fields are left unchanged
// on update; name and iconKey are required on create, and sortOrder defaults to after the last one.
type CategoryInput struct {
	Name      *string `json:"name"`
	IconKey   *string `json:"iconKey"`
	SortOrder *int32  `json:"sortOrder"`
}
