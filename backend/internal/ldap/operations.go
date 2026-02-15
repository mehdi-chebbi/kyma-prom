package ldap

import (
	"context"
	"fmt"
	"strings"

	"github.com/devplatform/ldap-manager/internal/models"
	ldap "github.com/go-ldap/ldap/v3"
	"github.com/sirupsen/logrus"
)

// CreateUser creates a new user in LDAP
func (m *Manager) CreateUser(ctx context.Context, input *models.CreateUserInput) (*models.User, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	uidNumber := m.nextUID()
	gidNumber := m.nextGID()
	userDN := m.config.UserDN(input.UID)

	m.logger.WithFields(logrus.Fields{
		"uid":        input.UID,
		"department": input.Department,
		"uidNumber":  uidNumber,
	}).Info("Creating user")

	addRequest := ldap.NewAddRequest(userDN, nil)
	addRequest.Attribute("objectClass", []string{"inetOrgPerson", "posixAccount", "shadowAccount", "extensibleObject"})
	addRequest.Attribute("uid", []string{input.UID})
	addRequest.Attribute("cn", []string{input.CN})
	addRequest.Attribute("sn", []string{input.SN})
	addRequest.Attribute("givenName", []string{input.GivenName})
	addRequest.Attribute("mail", []string{input.Mail})
	addRequest.Attribute("departmentNumber", []string{input.Department})
	addRequest.Attribute("uidNumber", []string{fmt.Sprintf("%d", uidNumber)})
	addRequest.Attribute("gidNumber", []string{fmt.Sprintf("%d", gidNumber)})
	addRequest.Attribute("homeDirectory", []string{fmt.Sprintf("/home/%s", input.UID)})
	addRequest.Attribute("userPassword", []string{input.Password})

	if len(input.Repositories) > 0 {
		addRequest.Attribute("githubRepository", input.Repositories)
	}

	if err := conn.Add(addRequest); err != nil {
		m.logger.WithError(err).Error("Failed to create user")
		return nil, fmt.Errorf("failed to add user: %w", err)
	}

	m.logger.WithField("uid", input.UID).Info("User created successfully")
	return m.GetUser(ctx, input.UID)
}

// GetUser retrieves a user by UID
func (m *Manager) GetUser(ctx context.Context, uid string) (*models.User, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	searchRequest := ldap.NewSearchRequest(
		m.config.UsersDN(),
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		fmt.Sprintf("(uid=%s)", ldap.EscapeFilter(uid)),
		[]string{"uid", "cn", "sn", "givenName", "mail", "departmentNumber", "uidNumber", "gidNumber", "homeDirectory", "githubRepository"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("user not found: %s", uid)
	}

	return m.entryToUser(result.Entries[0]), nil
}

// ListUsers lists users with optional filtering
func (m *Manager) ListUsers(ctx context.Context, filter *models.SearchFilter) ([]*models.User, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	filterStr := "(objectClass=inetOrgPerson)"
	if filter != nil {
		filters := []string{"(objectClass=inetOrgPerson)"}
		if filter.UID != "" {
			filters = append(filters, fmt.Sprintf("(uid=*%s*)", ldap.EscapeFilter(filter.UID)))
		}
		if filter.CN != "" {
			filters = append(filters, fmt.Sprintf("(cn=*%s*)", ldap.EscapeFilter(filter.CN)))
		}
		if filter.SN != "" {
			filters = append(filters, fmt.Sprintf("(sn=*%s*)", ldap.EscapeFilter(filter.SN)))
		}
		if filter.GivenName != "" {
			filters = append(filters, fmt.Sprintf("(givenName=*%s*)", ldap.EscapeFilter(filter.GivenName)))
		}
		if filter.Mail != "" {
			filters = append(filters, fmt.Sprintf("(mail=*%s*)", ldap.EscapeFilter(filter.Mail)))
		}
		if filter.Department != "" {
			filters = append(filters, fmt.Sprintf("(departmentNumber=%s)", ldap.EscapeFilter(filter.Department)))
		}
		if filter.UIDNumber != 0 {
			filters = append(filters, fmt.Sprintf("(uidNumber=%d)", filter.UIDNumber))
		}
		if filter.GIDNumber != 0 {
			filters = append(filters, fmt.Sprintf("(gidNumber=%d)", filter.GIDNumber))
		}
		if filter.Repository != "" {
			filters = append(filters, fmt.Sprintf("(githubRepository=*%s*)", ldap.EscapeFilter(filter.Repository)))
		}
		if len(filters) > 1 {
			filterStr = fmt.Sprintf("(&%s)", strings.Join(filters, ""))
		}
	}

	searchRequest := ldap.NewSearchRequest(
		m.config.UsersDN(),
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filterStr,
		[]string{"uid", "cn", "sn", "givenName", "mail", "departmentNumber", "uidNumber", "gidNumber", "homeDirectory", "githubRepository"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	users := make([]*models.User, 0, len(result.Entries))
	for _, entry := range result.Entries {
		users = append(users, m.entryToUser(entry))
	}

	return users, nil
}

// UpdateUser updates user attributes
func (m *Manager) UpdateUser(ctx context.Context, input *models.UpdateUserInput) (*models.User, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	userDN := m.config.UserDN(input.UID)

	m.logger.WithField("uid", input.UID).Info("Updating user")

	modifyRequest := ldap.NewModifyRequest(userDN, nil)

	if input.CN != nil {
		modifyRequest.Replace("cn", []string{*input.CN})
	}
	if input.SN != nil {
		modifyRequest.Replace("sn", []string{*input.SN})
	}
	if input.GivenName != nil {
		modifyRequest.Replace("givenName", []string{*input.GivenName})
	}
	if input.Mail != nil {
		modifyRequest.Replace("mail", []string{*input.Mail})
	}
	if input.Department != nil {
		modifyRequest.Replace("departmentNumber", []string{*input.Department})
	}
	if input.Password != nil {
		modifyRequest.Replace("userPassword", []string{*input.Password})
	}
	if input.Repositories != nil {
		if len(input.Repositories) > 0 {
			modifyRequest.Replace("githubRepository", input.Repositories)
		} else {
			modifyRequest.Delete("githubRepository", nil)
		}
	}

	if err := conn.Modify(modifyRequest); err != nil {
		m.logger.WithError(err).Error("Failed to update user")
		return nil, fmt.Errorf("failed to modify user: %w", err)
	}

	m.logger.WithField("uid", input.UID).Info("User updated successfully")
	return m.GetUser(ctx, input.UID)
}

// DeleteUser deletes a user from LDAP
func (m *Manager) DeleteUser(ctx context.Context, uid string) error {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	userDN := m.config.UserDN(uid)

	m.logger.WithField("uid", uid).Info("Deleting user")

	deleteRequest := ldap.NewDelRequest(userDN, nil)
	if err := conn.Del(deleteRequest); err != nil {
		m.logger.WithError(err).Error("Failed to delete user")
		return fmt.Errorf("failed to delete user: %w", err)
	}

	m.logger.WithField("uid", uid).Info("User deleted successfully")
	return nil
}

// Authenticate authenticates a user with their password
func (m *Manager) Authenticate(ctx context.Context, uid, password string) (*models.User, error) {
	// First, get the user to retrieve their DN
	user, err := m.GetUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Create a new connection for authentication (don't use pool)
	conn, err := ldap.DialURL(m.config.LDAPURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Try to bind with user credentials
	userDN := m.config.UserDN(uid)
	if err := conn.Bind(userDN, password); err != nil {
		m.logger.WithFields(logrus.Fields{
			"uid": uid,
		}).Warn("Authentication failed")
		return nil, fmt.Errorf("authentication failed")
	}

	m.logger.WithField("uid", uid).Info("User authenticated successfully")
	return user, nil
}

// CreateDepartment creates a new department
func (m *Manager) CreateDepartment(ctx context.Context, input *models.CreateDepartmentInput) (*models.Department, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	deptDN := m.config.DepartmentDN(input.OU)

	m.logger.WithField("ou", input.OU).Info("Creating department")

	addRequest := ldap.NewAddRequest(deptDN, nil)
	addRequest.Attribute("objectClass", []string{"organizationalUnit", "extensibleObject"})
	addRequest.Attribute("ou", []string{input.OU})

	if input.Description != "" {
		addRequest.Attribute("description", []string{input.Description})
	}
	if input.Manager != "" {
		managerDN := m.config.UserDN(input.Manager)
		addRequest.Attribute("manager", []string{managerDN})
	}
	if len(input.Repositories) > 0 {
		addRequest.Attribute("githubRepository", input.Repositories)
	}

	if err := conn.Add(addRequest); err != nil {
		m.logger.WithError(err).Error("Failed to create department")
		return nil, fmt.Errorf("failed to add department: %w", err)
	}

	m.logger.WithField("ou", input.OU).Info("Department created successfully")
	return m.GetDepartment(ctx, input.OU)
}

// GetDepartment retrieves a department by OU
func (m *Manager) GetDepartment(ctx context.Context, ou string) (*models.Department, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	searchRequest := ldap.NewSearchRequest(
		m.config.DepartmentsDN(),
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		fmt.Sprintf("(ou=%s)", ldap.EscapeFilter(ou)),
		[]string{"ou", "description", "manager", "githubRepository"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("department not found: %s", ou)
	}

	dept := m.entryToDepartment(result.Entries[0])

	// Get members using the same connection (avoid pool exhaustion)
	memberFilter := fmt.Sprintf("(&(objectClass=inetOrgPerson)(departmentNumber=%s))", ldap.EscapeFilter(ou))
	memberSearch := ldap.NewSearchRequest(
		m.config.UsersDN(),
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		memberFilter,
		[]string{"uid"},
		nil,
	)
	memberResult, err := conn.Search(memberSearch)
	if err != nil {
		m.logger.WithError(err).Warn("Failed to get department members")
	} else {
		memberUIDs := make([]string, 0, len(memberResult.Entries))
		for _, mEntry := range memberResult.Entries {
			memberUIDs = append(memberUIDs, mEntry.GetAttributeValue("uid"))
		}
		dept.Members = memberUIDs
	}

	return dept, nil
}

// ListDepartments lists all departments
func (m *Manager) ListDepartments(ctx context.Context) ([]*models.Department, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	searchRequest := ldap.NewSearchRequest(
		m.config.DepartmentsDN(),
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		"(objectClass=organizationalUnit)",
		[]string{"ou", "description", "manager", "githubRepository"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	departments := make([]*models.Department, 0, len(result.Entries))
	for _, entry := range result.Entries {
		dept := m.entryToDepartment(entry)

		// Get members using the same connection (avoid pool exhaustion)
		ou := entry.GetAttributeValue("ou")
		memberFilter := fmt.Sprintf("(&(objectClass=inetOrgPerson)(departmentNumber=%s))", ldap.EscapeFilter(ou))
		memberSearch := ldap.NewSearchRequest(
			m.config.UsersDN(),
			ldap.ScopeSingleLevel,
			ldap.NeverDerefAliases,
			0,
			0,
			false,
			memberFilter,
			[]string{"uid"},
			nil,
		)
		memberResult, err := conn.Search(memberSearch)
		if err == nil {
			memberUIDs := make([]string, 0, len(memberResult.Entries))
			for _, mEntry := range memberResult.Entries {
				memberUIDs = append(memberUIDs, mEntry.GetAttributeValue("uid"))
			}
			dept.Members = memberUIDs
		}

		departments = append(departments, dept)
	}

	return departments, nil
}

// DeleteDepartment deletes a department
func (m *Manager) DeleteDepartment(ctx context.Context, ou string) error {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	deptDN := m.config.DepartmentDN(ou)

	m.logger.WithField("ou", ou).Info("Deleting department")

	deleteRequest := ldap.NewDelRequest(deptDN, nil)
	if err := conn.Del(deleteRequest); err != nil {
		m.logger.WithError(err).Error("Failed to delete department")
		return fmt.Errorf("failed to delete department: %w", err)
	}

	m.logger.WithField("ou", ou).Info("Department deleted successfully")
	return nil
}

// AssignRepositoryToDepartment assigns repositories to a department
func (m *Manager) AssignRepositoryToDepartment(ctx context.Context, ou string, repos []string) error {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	deptDN := m.config.DepartmentDN(ou)

	m.logger.WithFields(logrus.Fields{
		"ou":    ou,
		"repos": len(repos),
	}).Info("Assigning repositories to department")

	modifyRequest := ldap.NewModifyRequest(deptDN, nil)
	modifyRequest.Replace("githubRepository", repos)

	if err := conn.Modify(modifyRequest); err != nil {
		m.logger.WithError(err).Error("Failed to assign repositories")
		return fmt.Errorf("failed to assign repositories: %w", err)
	}

	m.logger.WithField("ou", ou).Info("Repositories assigned successfully")
	return nil
}

// GetUsersByDepartment retrieves all users in a department
func (m *Manager) GetUsersByDepartment(ctx context.Context, department string) ([]*models.User, error) {
	filter := &models.SearchFilter{
		Department: department,
	}
	return m.ListUsers(ctx, filter)
}

// CreateGroup creates a new group
func (m *Manager) CreateGroup(ctx context.Context, cn, description string) (*models.Group, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	gidNumber := m.nextGID()
	groupDN := m.config.GroupDN(cn)

	m.logger.WithField("cn", cn).Info("Creating group")

	addRequest := ldap.NewAddRequest(groupDN, nil)
	addRequest.Attribute("objectClass", []string{"groupOfNames", "posixGroup"})
	addRequest.Attribute("cn", []string{cn})
	addRequest.Attribute("gidNumber", []string{fmt.Sprintf("%d", gidNumber)})
	// groupOfNames requires at least one member, use a placeholder
	addRequest.Attribute("member", []string{"cn=placeholder,ou=groups," + m.config.LDAPBaseDN})

	if description != "" {
		addRequest.Attribute("description", []string{description})
	}

	if err := conn.Add(addRequest); err != nil {
		m.logger.WithError(err).Error("Failed to create group")
		return nil, fmt.Errorf("failed to add group: %w", err)
	}

	m.logger.WithField("cn", cn).Info("Group created successfully")
	return m.GetGroup(ctx, cn)
}

// GetGroup retrieves a group by CN
func (m *Manager) GetGroup(ctx context.Context, cn string) (*models.Group, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	searchRequest := ldap.NewSearchRequest(
		m.config.GroupsDN(),
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		fmt.Sprintf("(cn=%s)", ldap.EscapeFilter(cn)),
		[]string{"cn", "gidNumber", "description", "member", "githubRepository"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("group not found: %s", cn)
	}

	return m.entryToGroup(result.Entries[0]), nil
}

// ListGroups lists all groups
func (m *Manager) ListGroups(ctx context.Context) ([]*models.Group, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	searchRequest := ldap.NewSearchRequest(
		m.config.GroupsDN(),
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		"(objectClass=groupOfNames)",
		[]string{"cn", "gidNumber", "description", "member", "githubRepository"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	groups := make([]*models.Group, 0, len(result.Entries))
	for _, entry := range result.Entries {
		groups = append(groups, m.entryToGroup(entry))
	}

	return groups, nil
}

// AddUserToGroup adds a user to a group
func (m *Manager) AddUserToGroup(ctx context.Context, uid, groupCN string) error {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	userDN := m.config.UserDN(uid)
	groupDN := m.config.GroupDN(groupCN)

	m.logger.WithFields(logrus.Fields{
		"uid":   uid,
		"group": groupCN,
	}).Info("Adding user to group")

	modifyRequest := ldap.NewModifyRequest(groupDN, nil)
	modifyRequest.Add("member", []string{userDN})

	if err := conn.Modify(modifyRequest); err != nil {
		m.logger.WithError(err).Error("Failed to add user to group")
		return fmt.Errorf("failed to add user to group: %w", err)
	}

	// Cascade: sync user's repos from all their groups (including this new one)
	if err := m.syncUserReposFromGroups(conn, uid); err != nil {
		m.logger.WithError(err).Warn("Failed to cascade repos after adding user to group")
	}

	m.logger.WithFields(logrus.Fields{
		"uid":   uid,
		"group": groupCN,
	}).Info("User added to group successfully")
	return nil
}

// AssignRepositoriesToGroup assigns repositories to a group
func (m *Manager) AssignRepositoriesToGroup(ctx context.Context, cn string, repositories []string) (*models.Group, error) {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	groupDN := m.config.GroupDN(cn)

	m.logger.WithFields(logrus.Fields{
		"group":        cn,
		"repositories": repositories,
	}).Info("Assigning repositories to group")

	// First, check if group needs extensibleObject class for custom attributes
	searchRequest := ldap.NewSearchRequest(
		groupDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"objectClass"},
		nil,
	)

	searchResult, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search group: %w", err)
	}

	if len(searchResult.Entries) == 0 {
		return nil, fmt.Errorf("group not found: %s", cn)
	}

	// Check if group has extensibleObject class
	objectClasses := searchResult.Entries[0].GetAttributeValues("objectClass")
	hasExtensible := false
	for _, oc := range objectClasses {
		if oc == "extensibleObject" {
			hasExtensible = true
			break
		}
	}

	modifyRequest := ldap.NewModifyRequest(groupDN, nil)

	// Add extensibleObject if not present (allows custom attributes)
	if !hasExtensible {
		modifyRequest.Add("objectClass", []string{"extensibleObject"})
	}

	// Replace githubRepository attribute with new values
	modifyRequest.Replace("githubRepository", repositories)

	if err := conn.Modify(modifyRequest); err != nil {
		m.logger.WithError(err).Error("Failed to assign repositories to group")
		return nil, fmt.Errorf("failed to modify group: %w", err)
	}

	// Cascade: sync repos for all group members
	memberSearch := ldap.NewSearchRequest(
		m.config.GroupDN(cn),
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=*)",
		[]string{"member"},
		nil,
	)

	memberResult, err := conn.Search(memberSearch)
	if err == nil && len(memberResult.Entries) > 0 {
		for _, memberDN := range memberResult.Entries[0].GetAttributeValues("member") {
			if strings.Contains(memberDN, "placeholder") {
				continue
			}
			parts := strings.Split(memberDN, ",")
			if len(parts) > 0 {
				memberUID := strings.TrimPrefix(parts[0], "uid=")
				if err := m.syncUserReposFromGroups(conn, memberUID); err != nil {
					m.logger.WithFields(logrus.Fields{
						"uid":   memberUID,
						"group": cn,
					}).WithError(err).Warn("Failed to cascade repos to group member")
				}
			}
		}
	}

	m.logger.WithField("group", cn).Info("Repositories assigned to group successfully")
	return m.GetGroup(ctx, cn)
}

// syncUserReposFromGroups recalculates a user's githubRepository attribute
// as the union of all repos from groups the user belongs to.
// Must be called with an existing connection (not from pool).
func (m *Manager) syncUserReposFromGroups(conn *ldap.Conn, uid string) error {
	userDN := m.config.UserDN(uid)

	// Find all groups this user belongs to
	groupSearch := ldap.NewSearchRequest(
		m.config.GroupsDN(),
		ldap.ScopeSingleLevel,
		ldap.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(&(objectClass=groupOfNames)(member=%s))", ldap.EscapeFilter(userDN)),
		[]string{"cn", "githubRepository"},
		nil,
	)

	groupResult, err := conn.Search(groupSearch)
	if err != nil {
		return fmt.Errorf("failed to search user groups: %w", err)
	}

	// Collect union of all group repos
	repoSet := make(map[string]struct{})
	for _, entry := range groupResult.Entries {
		for _, repo := range entry.GetAttributeValues("githubRepository") {
			repoSet[repo] = struct{}{}
		}
	}

	repos := make([]string, 0, len(repoSet))
	for repo := range repoSet {
		repos = append(repos, repo)
	}

	// Update user's githubRepository
	modifyRequest := ldap.NewModifyRequest(userDN, nil)
	modifyRequest.Replace("githubRepository", repos)

	if err := conn.Modify(modifyRequest); err != nil {
		return fmt.Errorf("failed to sync repos for user %s: %w", uid, err)
	}

	m.logger.WithFields(logrus.Fields{
		"uid":   uid,
		"repos": len(repos),
	}).Info("Synced user repos from groups")

	return nil
}

// Helper functions to convert LDAP entries to models

func (m *Manager) entryToUser(entry *ldap.Entry) *models.User {
	uidNumber := 0
	gidNumber := 0
	fmt.Sscanf(entry.GetAttributeValue("uidNumber"), "%d", &uidNumber)
	fmt.Sscanf(entry.GetAttributeValue("gidNumber"), "%d", &gidNumber)

	return &models.User{
		UID:          entry.GetAttributeValue("uid"),
		CN:           entry.GetAttributeValue("cn"),
		SN:           entry.GetAttributeValue("sn"),
		GivenName:    entry.GetAttributeValue("givenName"),
		Mail:         entry.GetAttributeValue("mail"),
		Department:   entry.GetAttributeValue("departmentNumber"),
		UIDNumber:    uidNumber,
		GIDNumber:    gidNumber,
		HomeDir:      entry.GetAttributeValue("homeDirectory"),
		Repositories: entry.GetAttributeValues("githubRepository"),
		DN:           entry.DN,
	}
}

func (m *Manager) entryToDepartment(entry *ldap.Entry) *models.Department {
	manager := entry.GetAttributeValue("manager")
	// Extract UID from manager DN if present
	if manager != "" {
		parts := strings.Split(manager, ",")
		if len(parts) > 0 {
			uidPart := strings.TrimPrefix(parts[0], "uid=")
			manager = uidPart
		}
	}

	return &models.Department{
		OU:           entry.GetAttributeValue("ou"),
		Description:  entry.GetAttributeValue("description"),
		Manager:      manager,
		Members:      []string{}, // Will be populated by caller
		Repositories: entry.GetAttributeValues("githubRepository"),
		DN:           entry.DN,
	}
}

func (m *Manager) entryToGroup(entry *ldap.Entry) *models.Group {
	gidNumber := 0
	fmt.Sscanf(entry.GetAttributeValue("gidNumber"), "%d", &gidNumber)

	members := entry.GetAttributeValues("member")
	memberUIDs := make([]string, 0, len(members))

	for _, memberDN := range members {
		// Skip placeholder
		if strings.Contains(memberDN, "placeholder") {
			continue
		}
		// Extract UID from DN
		parts := strings.Split(memberDN, ",")
		if len(parts) > 0 {
			uidPart := strings.TrimPrefix(parts[0], "uid=")
			memberUIDs = append(memberUIDs, uidPart)
		}
	}

	return &models.Group{
		CN:           entry.GetAttributeValue("cn"),
		Description:  entry.GetAttributeValue("description"),
		GIDNumber:    gidNumber,
		Members:      memberUIDs,
		Repositories: entry.GetAttributeValues("githubRepository"),
		DN:           entry.DN,
	}
}

// DeleteGroup deletes a group by CN
func (m *Manager) DeleteGroup(ctx context.Context, cn string) error {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	groupDN := m.config.GroupDN(cn)

	m.logger.WithField("cn", cn).Info("Deleting group")

	delRequest := ldap.NewDelRequest(groupDN, nil)
	if err := conn.Del(delRequest); err != nil {
		m.logger.WithError(err).Error("Failed to delete group")
		return fmt.Errorf("failed to delete group: %w", err)
	}

	m.logger.WithField("cn", cn).Info("Group deleted successfully")
	return nil
}

// RemoveUserFromGroup removes a user from a group
func (m *Manager) RemoveUserFromGroup(ctx context.Context, uid, groupCN string) error {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer m.returnConnection(conn)

	userDN := m.config.UserDN(uid)
	groupDN := m.config.GroupDN(groupCN)

	m.logger.WithFields(logrus.Fields{
		"uid":   uid,
		"group": groupCN,
	}).Info("Removing user from group")

	modifyRequest := ldap.NewModifyRequest(groupDN, nil)
	modifyRequest.Delete("member", []string{userDN})

	if err := conn.Modify(modifyRequest); err != nil {
		m.logger.WithError(err).Error("Failed to remove user from group")
		return fmt.Errorf("failed to remove user from group: %w", err)
	}

	// Cascade: recalculate user's repos from remaining groups
	if err := m.syncUserReposFromGroups(conn, uid); err != nil {
		m.logger.WithError(err).Warn("Failed to cascade repo removal after removing user from group")
	}

	m.logger.WithFields(logrus.Fields{
		"uid":   uid,
		"group": groupCN,
	}).Info("User removed from group successfully")
	return nil
}
