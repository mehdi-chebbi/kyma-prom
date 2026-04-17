package graphql

import (
	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/sync"
	"github.com/graphql-go/graphql"
)

// defineTeamType defines the Team GraphQL type
func (s *Schema) defineTeamType(userType, repositoryType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Team",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:        graphql.Int,
				Description: "Team ID",
			},
			"name": &graphql.Field{
				Type:        graphql.String,
				Description: "Team name",
			},
			"description": &graphql.Field{
				Type:        graphql.String,
				Description: "Team description",
			},
			"permission": &graphql.Field{
				Type:        teamPermissionEnum,
				Description: "Team permission level",
			},
			"members": &graphql.Field{
				Type:        graphql.NewList(userType),
				Description: "Team members",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					team := p.Source.(*gitea.Team)
					return s.giteaClient.ListTeamMembers(p.Context, team.ID)
				},
			},
			"repositories": &graphql.Field{
				Type:        graphql.NewList(repositoryType),
				Description: "Team repositories",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					team := p.Source.(*gitea.Team)
					return s.giteaClient.ListTeamRepositories(p.Context, team.ID)
				},
			},
		},
	})
}

// teamPermissionEnum defines the TeamPermission enum
var teamPermissionEnum = graphql.NewEnum(graphql.EnumConfig{
	Name:        "TeamPermission",
	Description: "Permission level for team access to repositories",
	Values: graphql.EnumValueConfigMap{
		"READ": &graphql.EnumValueConfig{
			Value:       "read",
			Description: "Read-only access (can view and clone)",
		},
		"WRITE": &graphql.EnumValueConfig{
			Value:       "write",
			Description: "Write access (can push changes)",
		},
		"ADMIN": &graphql.EnumValueConfig{
			Value:       "admin",
			Description: "Admin access (full control)",
		},
	},
})

// defineSyncResultType defines the SyncResult GraphQL type
func (s *Schema) defineSyncResultType(teamType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "SyncResult",
		Fields: graphql.Fields{
			"team": &graphql.Field{
				Type:        teamType,
				Description: "The synced team",
			},
			"membersAdded": &graphql.Field{
				Type:        graphql.Int,
				Description: "Number of members added",
			},
			"membersFailed": &graphql.Field{
				Type:        graphql.Int,
				Description: "Number of members that failed to add",
			},
			"repositoriesAdded": &graphql.Field{
				Type:        graphql.Int,
				Description: "Number of repositories added",
			},
			"repositoriesFailed": &graphql.Field{
				Type:        graphql.Int,
				Description: "Number of repositories that failed to add",
			},
			"errors": &graphql.Field{
				Type:        graphql.NewList(graphql.String),
				Description: "List of errors encountered during sync",
			},
		},
	})
}

// Team mutation resolvers

func (s *Schema) resolveCreateTeam(p graphql.ResolveParams) (interface{}, error) {
	orgName := p.Args["orgName"].(string)
	name := p.Args["name"].(string)
	permission := p.Args["permission"].(string)

	description := ""
	if desc, ok := p.Args["description"].(string); ok {
		description = desc
	}

	return s.giteaClient.CreateTeam(p.Context, orgName, &gitea.CreateTeamRequest{
		Name:        name,
		Description: description,
		Permission:  permission,
	})
}

func (s *Schema) resolveAddTeamMember(p graphql.ResolveParams) (interface{}, error) {
	teamID := int64(p.Args["teamId"].(int))
	username := p.Args["username"].(string)

	if err := s.giteaClient.AddTeamMember(p.Context, teamID, username); err != nil {
		return nil, err
	}

	// Return the updated team
	return s.giteaClient.GetTeam(p.Context, teamID)
}

func (s *Schema) resolveRemoveTeamMember(p graphql.ResolveParams) (interface{}, error) {
	teamID := int64(p.Args["teamId"].(int))
	username := p.Args["username"].(string)

	if err := s.giteaClient.RemoveTeamMember(p.Context, teamID, username); err != nil {
		return nil, err
	}

	// Return the updated team
	return s.giteaClient.GetTeam(p.Context, teamID)
}

func (s *Schema) resolveAddTeamRepository(p graphql.ResolveParams) (interface{}, error) {
	teamID := int64(p.Args["teamId"].(int))
	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)

	if err := s.giteaClient.AddTeamRepository(p.Context, teamID, owner, repo); err != nil {
		return nil, err
	}

	// Return the updated team
	return s.giteaClient.GetTeam(p.Context, teamID)
}

func (s *Schema) resolveRemoveTeamRepository(p graphql.ResolveParams) (interface{}, error) {
	teamID := int64(p.Args["teamId"].(int))
	owner := p.Args["owner"].(string)
	repo := p.Args["repo"].(string)

	if err := s.giteaClient.RemoveTeamRepository(p.Context, teamID, owner, repo); err != nil {
		return nil, err
	}

	// Return the updated team
	return s.giteaClient.GetTeam(p.Context, teamID)
}

func (s *Schema) resolveSyncGroupToTeam(p graphql.ResolveParams) (interface{}, error) {
	groupCN := p.Args["groupCn"].(string)
	orgName := p.Args["orgName"].(string)
	permission := p.Args["permission"].(string)

	teamName := ""
	if name, ok := p.Args["teamName"].(string); ok {
		teamName = name
	}

	// Create sync service
	syncService := sync.NewGroupSyncService(s.giteaClient, s.ldapClient, s.logger)

	// Perform sync
	return syncService.SyncGroupToTeam(p.Context, groupCN, orgName, teamName, permission, "")
}

// Team query resolvers

func (s *Schema) resolveListTeams(p graphql.ResolveParams) (interface{}, error) {
	orgName := p.Args["orgName"].(string)

	page := 1
	if p, ok := p.Args["page"].(int); ok {
		page = p
	}

	limit := 50
	if l, ok := p.Args["limit"].(int); ok {
		limit = l
	}

	return s.giteaClient.ListTeams(p.Context, orgName, page, limit)
}

func (s *Schema) resolveGetTeam(p graphql.ResolveParams) (interface{}, error) {
	teamID := int64(p.Args["teamId"].(int))
	return s.giteaClient.GetTeam(p.Context, teamID)
}
