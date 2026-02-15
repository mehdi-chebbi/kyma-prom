package graphql

import (
	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/models"
	"github.com/graphql-go/graphql"
	"github.com/sirupsen/logrus"
)

// defineGiteaUserType defines the GiteaUser GraphQL type
func (s *Schema) defineGiteaUserType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "GiteaUser",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.Int},
			"login":     &graphql.Field{Type: graphql.String},
			"fullName":  &graphql.Field{Type: graphql.String},
			"email":     &graphql.Field{Type: graphql.String},
			"avatarUrl": &graphql.Field{Type: graphql.String},
			"isAdmin":   &graphql.Field{Type: graphql.Boolean},
			"created":   &graphql.Field{Type: graphql.String},
		},
	})
}

// ============================================================================
// USER SYNC QUERY RESOLVERS
// ============================================================================

func (s *Schema) resolveGetGiteaUser(p graphql.ResolveParams) (interface{}, error) {
	username := p.Args["username"].(string)
	return s.giteaService.GetGiteaUser(p.Context, username)
}

func (s *Schema) resolveSearchGiteaUsers(p graphql.ResolveParams) (interface{}, error) {
	query := p.Args["query"].(string)
	limit := p.Args["limit"].(int)
	return s.giteaService.SearchGiteaUsers(p.Context, query, limit)
}

// ============================================================================
// USER SYNC MUTATION RESOLVERS
// ============================================================================

func (s *Schema) resolveSyncLDAPUser(p graphql.ResolveParams) (interface{}, error) {
	uid := p.Args["uid"].(string)
	defaultPassword := p.Args["defaultPassword"].(string)

	// Get token from context
	user, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	// Get LDAP user
	ldapUser, err := s.ldapClient.GetUser(p.Context, uid, token)
	if err != nil {
		return nil, err
	}

	// Convert to models.User
	modelsUser := &models.User{
		UID:          ldapUser.UID,
		CN:           ldapUser.CN,
		SN:           ldapUser.SN,
		GivenName:    ldapUser.GivenName,
		Mail:         ldapUser.Mail,
		Department:   ldapUser.Department,
		UIDNumber:    ldapUser.UIDNumber,
		GIDNumber:    ldapUser.GIDNumber,
		HomeDir:      ldapUser.HomeDir,
		Repositories: ldapUser.Repositories,
	}

	// Sync to Gitea
	giteaUser, err := s.giteaService.SyncLDAPUserToGitea(p.Context, modelsUser, defaultPassword)
	if err != nil {
		return nil, err
	}

	s.logger.WithField("uid", uid).Info("Synced LDAP user to Gitea")
	_ = user // mark as used

	return giteaUser, nil
}

func (s *Schema) resolveSyncAllLDAPUsers(p graphql.ResolveParams) (interface{}, error) {
	defaultPassword := p.Args["defaultPassword"].(string)

	// Get token from context
	_, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	// Sync all users
	giteaUsers, err := s.giteaService.SyncAllLDAPUsersToGitea(p.Context, token, defaultPassword)
	if err != nil {
		return nil, err
	}

	s.logger.WithField("count", len(giteaUsers)).Info("Synced all LDAP users to Gitea")

	return giteaUsers, nil
}

// ============================================================================
// REPO SYNC TYPES AND RESOLVERS (Gitea → LDAP)
// ============================================================================

// defineRepoSyncResultType defines the RepoSyncResult GraphQL type
func (s *Schema) defineRepoSyncResultType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name:        "RepoSyncResult",
		Description: "Result of syncing Gitea repos to LDAP for a user",
		Fields: graphql.Fields{
			"uid": &graphql.Field{
				Type:        graphql.String,
				Description: "User UID",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					r := p.Source.(*gitea.RepoSyncResult)
					return r.UID, nil
				},
			},
			"reposCount": &graphql.Field{
				Type:        graphql.Int,
				Description: "Number of repositories synced",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					r := p.Source.(*gitea.RepoSyncResult)
					return r.ReposCount, nil
				},
			},
			"repositories": &graphql.Field{
				Type:        graphql.NewList(graphql.String),
				Description: "List of repository full names",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					r := p.Source.(*gitea.RepoSyncResult)
					return r.Repositories, nil
				},
			},
		},
	})
}

func (s *Schema) resolveSyncGiteaReposToLDAP(p graphql.ResolveParams) (interface{}, error) {
	uid := p.Args["uid"].(string)

	// Get token from context
	_, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	result, err := s.giteaService.SyncGiteaReposToLDAP(p.Context, uid, token)
	if err != nil {
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"uid":        uid,
		"reposCount": result.ReposCount,
	}).Info("Synced Gitea repos to LDAP for user")

	return result, nil
}

func (s *Schema) resolveSyncAllGiteaReposToLDAP(p graphql.ResolveParams) (interface{}, error) {
	// Get token from context
	_, token, err := s.getUserFromContext(p.Context)
	if err != nil {
		return nil, err
	}

	results, err := s.giteaService.SyncAllGiteaReposToLDAP(p.Context, token)
	if err != nil {
		return nil, err
	}

	s.logger.WithField("count", len(results)).Info("Synced all Gitea repos to LDAP")

	return results, nil
}
