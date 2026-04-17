package graphql

import (
	"time"

	"github.com/devplatform/ldap-manager/internal/models"
	"github.com/graphql-go/graphql"
)

// defineStatsType defines the Stats GraphQL type
func (s *Schema) defineStatsType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Stats",
		Fields: graphql.Fields{
			"totalConnections":  &graphql.Field{Type: graphql.Int},
			"activeConnections": &graphql.Field{Type: graphql.Int},
			"poolSize":          &graphql.Field{Type: graphql.Int},
		},
	})
}

// defineHealthType defines the Health GraphQL type
func (s *Schema) defineHealthType() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "Health",
		Fields: graphql.Fields{
			"status":    &graphql.Field{Type: graphql.String},
			"timestamp": &graphql.Field{Type: graphql.String},
		},
	})
}

// ============================================================================
// COMMON RESOLVERS (Health, Stats)
// ============================================================================

func (s *Schema) resolveHealth(p graphql.ResolveParams) (interface{}, error) {
	ldapHealthy := s.ldapMgr.HealthCheck(p.Context) == nil
	status := "healthy"
	if !ldapHealthy {
		status = "unhealthy"
	}

	return &models.HealthStatus{
		Status:    status,
		Timestamp: time.Now().Unix(),
		LDAP:      ldapHealthy,
	}, nil
}

func (s *Schema) resolveStats(p graphql.ResolveParams) (interface{}, error) {
	return s.ldapMgr.GetStats(), nil
}
