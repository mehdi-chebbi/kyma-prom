package controller

// InitDataSpec defines initialization data for LDAP
type InitDataSpec struct {
	// OrganizationalUnits to create
	OrganizationalUnits []OUSpec `json:"organizationalUnits,omitempty"`

	// Departments to create under ou=departments
	Departments []DepartmentSpec `json:"departments,omitempty"`

	// Users to create under ou=users
	Users []UserSpec `json:"users,omitempty"`

	// Groups to create under ou=groups
	Groups []GroupSpec `json:"groups,omitempty"`
}

// OUSpec defines an organizational unit
type OUSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// DepartmentSpec defines a department
type DepartmentSpec struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Manager      string   `json:"manager,omitempty"`
	Repositories []string `json:"repositories,omitempty"`
}

// UserSpec defines a user
type UserSpec struct {
	UID          string   `json:"uid"`
	CommonName   string   `json:"cn"`
	Surname      string   `json:"sn"`
	GivenName    string   `json:"givenName,omitempty"`
	Email        string   `json:"mail"`
	Password     string   `json:"password"`
	Department   string   `json:"department,omitempty"`
	Repositories []string `json:"repositories,omitempty"`
}

// GroupSpec defines a group
type GroupSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Members     []string `json:"members,omitempty"`
}
