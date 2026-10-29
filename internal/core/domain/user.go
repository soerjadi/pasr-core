package domain

type Role string

const (
	RoleBuyer    Role = "buyer"
	RoleOperator Role = "operator"
)

// Claims is the identity carried by an access token.
type Claims struct {
	UserID int64
	Role   Role
}

// User matches the User schema in docs/api/openapi.yaml. Role is internal and never serialized.
type User struct {
	ID    int64         `json:"id"`
	Phone string        `json:"phone"`
	Name  string        `json:"name"`
	Role  Role          `json:"-"`
	Store *StoreSummary `json:"store,omitempty"`
}
