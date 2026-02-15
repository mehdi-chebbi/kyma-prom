package models

// User represents an LDAP user with all attributes
type User struct {
	UID          string   `json:"uid"`
	CN           string   `json:"cn"`
	SN           string   `json:"sn"`
	GivenName    string   `json:"givenName"`
	Mail         string   `json:"mail"`
	Department   string   `json:"department"`
	UIDNumber    int      `json:"uidNumber"`
	GIDNumber    int      `json:"gidNumber"`
	HomeDir      string   `json:"homeDirectory"`
	Repositories []string `json:"repositories"`
	DN           string   `json:"dn"`
}

// Department represents an organizational unit in LDAP
type Department struct {
	OU           string   `json:"ou"`
	Description  string   `json:"description"`
	Manager      string   `json:"manager,omitempty"`
	Members      []string `json:"members"`
	Repositories []string `json:"repositories"`
	DN           string   `json:"dn"`
}

// Group represents an LDAP group
type Group struct {
	CN           string   `json:"cn"`
	Description  string   `json:"description,omitempty"`
	GIDNumber    int      `json:"gidNumber"`
	Members      []string `json:"members"`
	Repositories []string `json:"repositories"`
	DN           string   `json:"dn"`
}

// CreateUserInput contains fields for creating a new user
type CreateUserInput struct {
	UID          string   `json:"uid"`
	CN           string   `json:"cn"`
	SN           string   `json:"sn"`
	GivenName    string   `json:"givenName"`
	Mail         string   `json:"mail"`
	Department   string   `json:"department"`
	Password     string   `json:"password"`
	Repositories []string `json:"repositories"`
}

// UpdateUserInput contains fields for updating a user
type UpdateUserInput struct {
	UID          string   `json:"uid"`
	CN           *string  `json:"cn,omitempty"`
	SN           *string  `json:"sn,omitempty"`
	GivenName    *string  `json:"givenName,omitempty"`
	Mail         *string  `json:"mail,omitempty"`
	Department   *string  `json:"department,omitempty"`
	Password     *string  `json:"password,omitempty"`
	Repositories []string `json:"repositories,omitempty"`
}

// CreateDepartmentInput contains fields for creating a department
type CreateDepartmentInput struct {
	OU           string   `json:"ou"`
	Description  string   `json:"description"`
	Manager      string   `json:"manager,omitempty"`
	Repositories []string `json:"repositories,omitempty"`
}

// SearchFilter contains optional filters for user searches
type SearchFilter struct {
	UID        string `json:"uid,omitempty"`
	CN         string `json:"cn,omitempty"`
	SN         string `json:"sn,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
	Mail       string `json:"mail,omitempty"`
	Department string `json:"department,omitempty"`
	UIDNumber  int    `json:"uidNumber,omitempty"`
	GIDNumber  int    `json:"gidNumber,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// AuthPayload is returned after successful authentication
type AuthPayload struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// Stats contains connection pool statistics
type Stats struct {
	PoolSize      int `json:"poolSize"`
	Available     int `json:"available"`
	InUse         int `json:"inUse"`
	TotalRequests int `json:"totalRequests"`
}

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
	LDAP      bool   `json:"ldap"`
}
