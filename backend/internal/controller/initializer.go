package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/sirupsen/logrus"
)

// LDAPInitializer handles LDAP initialization
type LDAPInitializer struct {
	logger  *logrus.Logger
	timeout time.Duration
}

// NewLDAPInitializer creates a new LDAP initializer
func NewLDAPInitializer(logger *logrus.Logger, timeout time.Duration) *LDAPInitializer {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &LDAPInitializer{
		logger:  logger,
		timeout: timeout,
	}
}

// WaitForReady waits for LDAP to be ready
func (i *LDAPInitializer) WaitForReady(ctx context.Context, ldapURL string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	i.logger.WithField("url", ldapURL).Info("Waiting for LDAP to be ready")

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for LDAP: %w", ctx.Err())
		case <-ticker.C:
			conn, err := ldap.DialURL(ldapURL)
			if err != nil {
				i.logger.WithError(err).Debug("LDAP not ready yet")
				continue
			}
			conn.Close()
			i.logger.Info("LDAP is ready")
			return nil
		}
	}
}

// Initialize performs LDAP initialization
func (i *LDAPInitializer) Initialize(ctx context.Context, ldapURL, adminDN, adminPassword, configPassword, baseDN string, initData *InitDataSpec) error {
	if initData == nil {
		return nil
	}

	i.logger.WithFields(logrus.Fields{
		"url":    ldapURL,
		"baseDN": baseDN,
	}).Info("Starting LDAP initialization")

	// Step 0: Ensure custom schema (githubRepository attribute) is registered
	if err := i.ensureCustomSchema(ldapURL, configPassword); err != nil {
		i.logger.WithError(err).Warn("Failed to ensure custom schema (githubRepository may already exist)")
	}

	// Connect to LDAP
	conn, err := ldap.DialURL(ldapURL)
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP: %w", err)
	}
	defer conn.Close()

	// Bind with admin credentials
	if err := conn.Bind(adminDN, adminPassword); err != nil {
		return fmt.Errorf("failed to bind as admin: %w", err)
	}

	i.logger.Info("Connected to LDAP as admin")

	// Create organizational units
	for _, ou := range initData.OrganizationalUnits {
		if err := i.createOU(conn, baseDN, ou); err != nil {
			i.logger.WithError(err).WithField("ou", ou.Name).Warn("Failed to create OU")
		}
	}

	// Create departments
	for _, dept := range initData.Departments {
		if err := i.createDepartment(conn, baseDN, dept); err != nil {
			i.logger.WithError(err).WithField("department", dept.Name).Warn("Failed to create department")
		}
	}

	// Create users
	uidNumber := 10000
	for _, user := range initData.Users {
		uidNumber++
		if err := i.createUser(conn, baseDN, user, uidNumber); err != nil {
			i.logger.WithError(err).WithField("user", user.UID).Warn("Failed to create user")
		}
	}

	// Create groups
	gidNumber := 10000
	for _, group := range initData.Groups {
		gidNumber++
		if err := i.createGroup(conn, baseDN, group, gidNumber); err != nil {
			i.logger.WithError(err).WithField("group", group.Name).Warn("Failed to create group")
		}
	}

	i.logger.Info("LDAP initialization completed")
	return nil
}

// ensureCustomSchema registers the githubRepository attribute in cn=config
func (i *LDAPInitializer) ensureCustomSchema(ldapURL, configPassword string) error {
	i.logger.Info("Ensuring custom LDAP schema (githubRepository attribute)")

	conn, err := ldap.DialURL(ldapURL)
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP for schema: %w", err)
	}
	defer conn.Close()

	// Bind as config admin
	if err := conn.Bind("cn=admin,cn=config", configPassword); err != nil {
		return fmt.Errorf("failed to bind as config admin: %w", err)
	}

	// Check if our custom schema already exists
	searchReq := ldap.NewSearchRequest(
		"cn=schema,cn=config",
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases, 0, 0, false,
		"(cn=*devplatform)",
		[]string{"cn"},
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err == nil && len(sr.Entries) > 0 {
		i.logger.Info("Custom schema already exists, skipping")
		return nil
	}

	// Add custom schema with githubRepository attribute
	addReq := ldap.NewAddRequest("cn=devplatform,cn=schema,cn=config", nil)
	addReq.Attribute("objectClass", []string{"olcSchemaConfig"})
	addReq.Attribute("cn", []string{"devplatform"})
	addReq.Attribute("olcAttributeTypes", []string{
		"( 1.3.6.1.4.1.99999.1.1 NAME 'githubRepository' " +
			"DESC 'Repository URL (GitHub/Gitea)' " +
			"EQUALITY caseIgnoreMatch " +
			"SUBSTR caseIgnoreSubstringsMatch " +
			"SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
	})

	if err := conn.Add(addReq); err != nil {
		return fmt.Errorf("failed to add custom schema: %w", err)
	}

	i.logger.Info("Custom schema registered (githubRepository attribute)")
	return nil
}

// createOU creates an organizational unit
func (i *LDAPInitializer) createOU(conn *ldap.Conn, baseDN string, ou OUSpec) error {
	dn := fmt.Sprintf("ou=%s,%s", ou.Name, baseDN)

	addReq := ldap.NewAddRequest(dn, nil)
	addReq.Attribute("objectClass", []string{"organizationalUnit"})
	addReq.Attribute("ou", []string{ou.Name})
	if ou.Description != "" {
		addReq.Attribute("description", []string{ou.Description})
	}

	err := conn.Add(addReq)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
			i.logger.WithField("ou", ou.Name).Debug("OU already exists")
			return nil
		}
		return fmt.Errorf("failed to create OU %s: %w", ou.Name, err)
	}

	i.logger.WithField("ou", ou.Name).Info("Created OU")
	return nil
}

// createDepartment creates a department under ou=departments
func (i *LDAPInitializer) createDepartment(conn *ldap.Conn, baseDN string, dept DepartmentSpec) error {
	dn := fmt.Sprintf("ou=%s,ou=departments,%s", dept.Name, baseDN)

	addReq := ldap.NewAddRequest(dn, nil)

	objectClasses := []string{"organizationalUnit"}
	if len(dept.Repositories) > 0 {
		objectClasses = append(objectClasses, "extensibleObject")
	}

	addReq.Attribute("objectClass", objectClasses)
	addReq.Attribute("ou", []string{dept.Name})

	if dept.Description != "" {
		addReq.Attribute("description", []string{dept.Description})
	}

	if dept.Manager != "" {
		managerDN := fmt.Sprintf("uid=%s,ou=users,%s", dept.Manager, baseDN)
		addReq.Attribute("manager", []string{managerDN})
	}

	if len(dept.Repositories) > 0 {
		addReq.Attribute("githubRepository", dept.Repositories)
	}

	err := conn.Add(addReq)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
			i.logger.WithField("department", dept.Name).Debug("Department already exists")
			return nil
		}
		return fmt.Errorf("failed to create department %s: %w", dept.Name, err)
	}

	i.logger.WithField("department", dept.Name).Info("Created department")
	return nil
}

// createUser creates a user under ou=users
func (i *LDAPInitializer) createUser(conn *ldap.Conn, baseDN string, user UserSpec, uidNumber int) error {
	dn := fmt.Sprintf("uid=%s,ou=users,%s", user.UID, baseDN)

	addReq := ldap.NewAddRequest(dn, nil)

	objectClasses := []string{"inetOrgPerson", "posixAccount", "shadowAccount"}
	if len(user.Repositories) > 0 {
		objectClasses = append(objectClasses, "extensibleObject")
	}

	addReq.Attribute("objectClass", objectClasses)
	addReq.Attribute("uid", []string{user.UID})
	addReq.Attribute("cn", []string{user.CommonName})
	addReq.Attribute("sn", []string{user.Surname})

	if user.GivenName != "" {
		addReq.Attribute("givenName", []string{user.GivenName})
	}

	addReq.Attribute("mail", []string{user.Email})
	addReq.Attribute("uidNumber", []string{fmt.Sprintf("%d", uidNumber)})
	addReq.Attribute("gidNumber", []string{fmt.Sprintf("%d", uidNumber)}) // Same as uidNumber
	addReq.Attribute("homeDirectory", []string{fmt.Sprintf("/home/%s", user.UID)})
	addReq.Attribute("userPassword", []string{user.Password})

	if user.Department != "" {
		addReq.Attribute("departmentNumber", []string{user.Department})
	}

	if len(user.Repositories) > 0 {
		addReq.Attribute("githubRepository", user.Repositories)
	}

	err := conn.Add(addReq)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
			i.logger.WithField("user", user.UID).Debug("User already exists")
			return nil
		}
		return fmt.Errorf("failed to create user %s: %w", user.UID, err)
	}

	i.logger.WithFields(logrus.Fields{
		"user":      user.UID,
		"uidNumber": uidNumber,
	}).Info("Created user")
	return nil
}

// createGroup creates a group under ou=groups
func (i *LDAPInitializer) createGroup(conn *ldap.Conn, baseDN string, group GroupSpec, gidNumber int) error {
	dn := fmt.Sprintf("cn=%s,ou=groups,%s", group.Name, baseDN)

	addReq := ldap.NewAddRequest(dn, nil)

	addReq.Attribute("objectClass", []string{"groupOfNames", "posixGroup"})
	addReq.Attribute("cn", []string{group.Name})
	addReq.Attribute("gidNumber", []string{fmt.Sprintf("%d", gidNumber)})

	if group.Description != "" {
		addReq.Attribute("description", []string{group.Description})
	}

	// Groups need at least one member
	members := group.Members
	if len(members) == 0 {
		// Add a placeholder member (will be the admin)
		members = []string{fmt.Sprintf("cn=admin,%s", baseDN)}
	}

	memberDNs := make([]string, len(members))
	for i, m := range members {
		if m == fmt.Sprintf("cn=admin,%s", baseDN) || m[:3] == "cn=" || m[:4] == "uid=" {
			memberDNs[i] = m
		} else {
			memberDNs[i] = fmt.Sprintf("uid=%s,ou=users,%s", m, baseDN)
		}
	}

	addReq.Attribute("member", memberDNs)

	err := conn.Add(addReq)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
			i.logger.WithField("group", group.Name).Debug("Group already exists")
			return nil
		}
		return fmt.Errorf("failed to create group %s: %w", group.Name, err)
	}

	i.logger.WithFields(logrus.Fields{
		"group":     group.Name,
		"gidNumber": gidNumber,
		"members":   len(memberDNs),
	}).Info("Created group")
	return nil
}

// parseInitData parses InitDataSpec from JSON string
func parseInitData(jsonStr string) (*InitDataSpec, error) {
	if jsonStr == "" {
		return nil, nil
	}

	var data InitDataSpec
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("failed to parse init data JSON: %w", err)
	}

	return &data, nil
}

// ComputeInitDataHash computes a hash of the init data for change detection
func ComputeInitDataHash(initData *InitDataSpec) string {
	if initData == nil {
		return ""
	}

	data, err := json.Marshal(initData)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8]) // First 8 bytes as hex
}

// DefaultInitData returns the default initialization data
func DefaultInitData() *InitDataSpec {
	return &InitDataSpec{
		OrganizationalUnits: []OUSpec{
			{Name: "users", Description: "Users"},
			{Name: "groups", Description: "Groups"},
			{Name: "departments", Description: "Departments"},
		},
		Departments: []DepartmentSpec{
			{
				Name:        "engineering",
				Description: "Engineering Department",
			},
			{
				Name:        "devops",
				Description: "DevOps Department",
			},
			{
				Name:        "datascience",
				Description: "Data Science Department",
			},
			{
				Name:        "frontend",
				Description: "Frontend Department",
			},
		},
		Users: []UserSpec{
			{
				UID:        "john.doe",
				CommonName: "John Doe",
				Surname:    "Doe",
				GivenName:  "John",
				Email:      "john.doe@devplatform.local",
				Password:   "password123",
				Department: "engineering",
				Repositories: []string{
					"https://github.com/devplatform/backend",
					"https://github.com/devplatform/frontend",
				},
			},
			{
				UID:        "jane.smith",
				CommonName: "Jane Smith",
				Surname:    "Smith",
				GivenName:  "Jane",
				Email:      "jane.smith@devplatform.local",
				Password:   "password123",
				Department: "engineering",
			},
			{
				UID:        "bob.wilson",
				CommonName: "Bob Wilson",
				Surname:    "Wilson",
				GivenName:  "Bob",
				Email:      "bob.wilson@devplatform.local",
				Password:   "password123",
				Department: "devops",
			},
			{
				UID:        "alice.chen",
				CommonName: "Alice Chen",
				Surname:    "Chen",
				GivenName:  "Alice",
				Email:      "alice.chen@devplatform.local",
				Password:   "password123",
				Department: "datascience",
			},
			{
				UID:        "mike.jones",
				CommonName: "Mike Jones",
				Surname:    "Jones",
				GivenName:  "Mike",
				Email:      "mike.jones@devplatform.local",
				Password:   "password123",
				Department: "frontend",
			},
		},
		Groups: []GroupSpec{
			{
				Name:        "developers",
				Description: "All developers",
				Members:     []string{"john.doe", "jane.smith", "mike.jones"},
			},
			{
				Name:        "admins",
				Description: "System administrators",
				Members:     []string{"bob.wilson"},
			},
			{
				Name:        "leads",
				Description: "Team leads",
				Members:     []string{"jane.smith"},
			},
		},
	}
}
