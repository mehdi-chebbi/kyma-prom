package graphql

import (
	"errors"
	"fmt"
	"time"

	"github.com/devplatform/codeserver-service/internal/auth"
	"github.com/devplatform/codeserver-service/internal/config"
	"github.com/devplatform/codeserver-service/internal/gitea"
	"github.com/devplatform/codeserver-service/internal/kubernetes"
	"github.com/devplatform/codeserver-service/internal/models"
	"github.com/graphql-go/graphql"
	"github.com/sirupsen/logrus"
)

type Schema struct {
	schema    graphql.Schema
	k8sClient *kubernetes.Client
	gitea     *gitea.Client
	config    *config.Config
	logger    *logrus.Logger
}

func NewSchema(k8sClient *kubernetes.Client, giteaClient *gitea.Client, cfg *config.Config, logger *logrus.Logger) *Schema {
	s := &Schema{
		k8sClient: k8sClient,
		gitea:     giteaClient,
		config:    cfg,
		logger:    logger,
	}

	instanceStatusEnum := graphql.NewEnum(graphql.EnumConfig{
		Name: "InstanceStatus",
		Values: graphql.EnumValueConfigMap{
			"PENDING":  &graphql.EnumValueConfig{Value: models.StatusPending},
			"STARTING": &graphql.EnumValueConfig{Value: models.StatusStarting},
			"RUNNING":  &graphql.EnumValueConfig{Value: models.StatusRunning},
			"STOPPING": &graphql.EnumValueConfig{Value: models.StatusStopping},
			"STOPPED":  &graphql.EnumValueConfig{Value: models.StatusStopped},
			"ERROR":    &graphql.EnumValueConfig{Value: models.StatusError},
		},
	})

	instanceType := graphql.NewObject(graphql.ObjectConfig{
		Name: "CodeServerInstance",
		Fields: graphql.Fields{
			"id":             &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"userId":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"repoName":       &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"repoOwner":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"url":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"status":         &graphql.Field{Type: graphql.NewNonNull(instanceStatusEnum)},
			"createdAt":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"lastAccessedAt": &graphql.Field{Type: graphql.String},
			"storageUsed":    &graphql.Field{Type: graphql.String},
			"errorMessage":   &graphql.Field{Type: graphql.String},
		},
	})

	provisionResultType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ProvisionResult",
		Fields: graphql.Fields{
			"instance": &graphql.Field{Type: graphql.NewNonNull(instanceType)},
			"message":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"isNew":    &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
		},
	})

	instanceStatsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "InstanceStats",
		Fields: graphql.Fields{
			"totalInstances":    &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"runningInstances":  &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"stoppedInstances":  &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"pendingInstances":  &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"totalStorageUsed":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	repositoryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Repository",
		Fields: graphql.Fields{
			"id":       &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"owner":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"fullName": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"cloneUrl": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"htmlUrl":  &graphql.Field{Type: graphql.String},
			"private":  &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
		},
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"health": &graphql.Field{
				Type:    graphql.NewNonNull(graphql.Boolean),
				Resolve: s.resolveHealth,
			},
			"myCodeServers": &graphql.Field{
				Type:    graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(instanceType))),
				Resolve: s.resolveMyCodeServers,
			},
			"codeServer": &graphql.Field{
				Type: instanceType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: s.resolveCodeServer,
			},
			"codeServerStatus": &graphql.Field{
				Type: graphql.NewNonNull(instanceStatusEnum),
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: s.resolveCodeServerStatus,
			},
			"codeServerLogs": &graphql.Field{
				Type: graphql.NewNonNull(graphql.String),
				Args: graphql.FieldConfigArgument{
					"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"lines": &graphql.ArgumentConfig{Type: graphql.Int, DefaultValue: 100},
				},
				Resolve: s.resolveCodeServerLogs,
			},
			"instanceStats": &graphql.Field{
				Type:    graphql.NewNonNull(instanceStatsType),
				Resolve: s.resolveInstanceStats,
			},
			"myRepositories": &graphql.Field{
				Type:    graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(repositoryType))),
				Resolve: s.resolveMyRepositories,
			},
		},
	})

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"provisionCodeServer": &graphql.Field{
				Type: graphql.NewNonNull(provisionResultType),
				Args: graphql.FieldConfigArgument{
					"repoOwner": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"repoName":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"branch":    &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: s.resolveProvisionCodeServer,
			},
			"stopCodeServer": &graphql.Field{
				Type: graphql.NewNonNull(graphql.Boolean),
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: s.resolveStopCodeServer,
			},
			"startCodeServer": &graphql.Field{
				Type: instanceType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: s.resolveStartCodeServer,
			},
			"deleteCodeServer": &graphql.Field{
				Type: graphql.NewNonNull(graphql.Boolean),
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: s.resolveDeleteCodeServer,
			},
			"syncRepository": &graphql.Field{
				Type: graphql.NewNonNull(graphql.Boolean),
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: s.resolveSyncRepository,
			},
		},
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
	if err != nil {
		panic(err)
	}

	s.schema = schema
	return s
}

func (s *Schema) GetSchema() graphql.Schema {
	return s.schema
}

func (s *Schema) resolveHealth(p graphql.ResolveParams) (interface{}, error) {
	if err := s.k8sClient.HealthCheck(p.Context); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *Schema) resolveMyCodeServers(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return nil, errors.New("unauthorized")
	}

	pods, err := s.k8sClient.ListUserInstances(p.Context, userID)
	if err != nil {
		return nil, err
	}

	instances := make([]*models.CodeServerInstance, 0, len(pods))
	for _, pod := range pods {
		instances = append(instances, s.k8sClient.PodToInstance(&pod))
	}

	return instances, nil
}

func (s *Schema) resolveCodeServer(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return nil, errors.New("unauthorized")
	}

	pod, err := s.k8sClient.GetCodeServerPod(p.Context, userID)
	if err != nil {
		return nil, err
	}

	return s.k8sClient.PodToInstance(pod), nil
}

func (s *Schema) resolveCodeServerStatus(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return nil, errors.New("unauthorized")
	}

	status, _, err := s.k8sClient.GetPodStatus(p.Context, userID)
	return status, err
}

func (s *Schema) resolveCodeServerLogs(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return nil, errors.New("unauthorized")
	}

	lines := int64(p.Args["lines"].(int))
	return s.k8sClient.GetPodLogs(p.Context, userID, lines)
}

func (s *Schema) resolveInstanceStats(p graphql.ResolveParams) (interface{}, error) {
	pods, err := s.k8sClient.ListUserInstances(p.Context, "")
	if err != nil {
		return nil, err
	}

	stats := &models.InstanceStats{}
	for _, pod := range pods {
		stats.TotalInstances++
		instance := s.k8sClient.PodToInstance(&pod)
		switch instance.Status {
		case models.StatusRunning:
			stats.RunningInstances++
		case models.StatusStopped:
			stats.StoppedInstances++
		case models.StatusPending, models.StatusStarting:
			stats.PendingInstances++
		}
	}

	pvcs, err := s.k8sClient.ListPVCs(p.Context)
	if err == nil {
		var totalBytes int64
		for _, pvc := range pvcs {
			if storage, ok := pvc.Status.Capacity["storage"]; ok {
				totalBytes += storage.Value()
			}
		}
		stats.TotalStorageUsed = formatBytes(totalBytes)
	}

	return stats, nil
}

func (s *Schema) resolveMyRepositories(p graphql.ResolveParams) (interface{}, error) {
	token := auth.GetTokenFromContext(p.Context)
	return s.gitea.GetUserRepositories(p.Context, token)
}

func (s *Schema) resolveProvisionCodeServer(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return nil, errors.New("unauthorized")
	}

	repoOwner := p.Args["repoOwner"].(string)
	repoName := p.Args["repoName"].(string)
	branch := ""
	if b, ok := p.Args["branch"].(string); ok && b != "" {
		branch = b
	}
	token := auth.GetTokenFromContext(p.Context)

	s.logger.WithFields(logrus.Fields{
		"user":      userID,
		"repoOwner": repoOwner,
		"repoName":  repoName,
		"branch":    branch,
	}).Info("Provisioning code-server")

	// Validate repo access via gitea-service
	hasAccess, err := s.gitea.ValidateRepoAccess(p.Context, token, repoOwner, repoName)
	if err != nil {
		s.logger.WithError(err).Error("Failed to validate repo access")
		return nil, errors.New("failed to validate repository access")
	}
	if !hasAccess {
		return nil, errors.New("repository access denied")
	}

	// Get clone URL
	cloneURL, err := s.gitea.GetRepoCloneURL(p.Context, token, repoOwner, repoName)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get clone URL")
		return nil, errors.New("failed to get repository URL")
	}

	if err := s.k8sClient.EnsureNamespace(p.Context); err != nil {
		s.logger.WithError(err).Error("Failed to ensure namespace")
		return nil, errors.New("failed to prepare environment")
	}

	_, err = s.k8sClient.EnsurePVC(p.Context, userID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create PVC")
		return nil, errors.New("failed to create storage")
	}

	existingPod, err := s.k8sClient.GetCodeServerPod(p.Context, userID)
	if err == nil && existingPod != nil {
		return &models.ProvisionResult{
			Instance: s.k8sClient.PodToInstance(existingPod),
			Message:  "Using existing instance",
			IsNew:    false,
		}, nil
	}

	pod, err := s.k8sClient.CreateCodeServerPod(p.Context, userID, cloneURL, repoName, repoOwner, branch)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create pod")
		return nil, errors.New("failed to create instance")
	}

	_, err = s.k8sClient.EnsureService(p.Context, userID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to create service")
	}

	if err := s.k8sClient.EnsureVirtualService(p.Context, userID); err != nil {
		s.logger.WithError(err).Warn("Failed to create VirtualService")
	}

	if err := s.k8sClient.EnsureDestinationRule(p.Context, userID); err != nil {
		s.logger.WithError(err).Warn("Failed to create DestinationRule")
	}

	timeout := time.Duration(s.config.CodeServerTimeout) * time.Second
	if err := s.k8sClient.WaitForPodReady(p.Context, userID, timeout); err != nil {
		s.logger.WithError(err).Warn("Pod not ready within timeout")
	}

	pod, _ = s.k8sClient.GetCodeServerPod(p.Context, userID)

	return &models.ProvisionResult{
		Instance: s.k8sClient.PodToInstance(pod),
		Message:  "Instance created successfully",
		IsNew:    true,
	}, nil
}

func (s *Schema) resolveStopCodeServer(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return false, errors.New("unauthorized")
	}

	if err := s.k8sClient.DeleteCodeServerPod(p.Context, userID); err != nil {
		return false, err
	}

	if err := s.k8sClient.DeleteService(p.Context, userID); err != nil {
		s.logger.WithError(err).Warn("Failed to delete service")
	}

	if err := s.k8sClient.DeleteVirtualService(p.Context, userID); err != nil {
		s.logger.WithError(err).Warn("Failed to delete VirtualService")
	}

	return true, nil
}

func (s *Schema) resolveStartCodeServer(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return nil, errors.New("unauthorized")
	}

	token := auth.GetTokenFromContext(p.Context)

	pvc, err := s.k8sClient.GetPVC(p.Context, userID)
	if err != nil {
		return nil, errors.New("no existing workspace found")
	}

	repoName := pvc.Labels["repo"]
	repoOwner := pvc.Labels["repo-owner"]
	if repoName == "" || repoOwner == "" {
		return nil, errors.New("workspace metadata missing")
	}

	cloneURL, err := s.gitea.GetRepoCloneURL(p.Context, token, repoOwner, repoName)
	if err != nil {
		return nil, err
	}

	branchFromPVC := pvc.Labels["branch"]
	pod, err := s.k8sClient.CreateCodeServerPod(p.Context, userID, cloneURL, repoName, repoOwner, branchFromPVC)
	if err != nil {
		return nil, err
	}

	s.k8sClient.EnsureService(p.Context, userID)
	s.k8sClient.EnsureVirtualService(p.Context, userID)

	return s.k8sClient.PodToInstance(pod), nil
}

func (s *Schema) resolveDeleteCodeServer(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return false, errors.New("unauthorized")
	}

	s.k8sClient.DeleteCodeServerPod(p.Context, userID)
	s.k8sClient.DeleteService(p.Context, userID)
	s.k8sClient.DeleteVirtualService(p.Context, userID)
	s.k8sClient.DeleteDestinationRule(p.Context, userID)

	if err := s.k8sClient.DeletePVC(p.Context, userID); err != nil {
		return false, err
	}

	return true, nil
}

func (s *Schema) resolveSyncRepository(p graphql.ResolveParams) (interface{}, error) {
	userID := auth.GetUserFromContext(p.Context)
	if userID == "" {
		return false, errors.New("unauthorized")
	}

	token := auth.GetTokenFromContext(p.Context)

	pod, err := s.k8sClient.GetCodeServerPod(p.Context, userID)
	if err != nil {
		return false, errors.New("instance not found")
	}

	repoName := pod.Annotations["codeserver.devplatform/repo-name"]
	repoOwner := pod.Annotations["codeserver.devplatform/repo-owner"]

	cloneURL, err := s.gitea.GetRepoCloneURL(p.Context, token, repoOwner, repoName)
	if err != nil {
		return false, err
	}

	if err := s.k8sClient.DeleteCodeServerPod(p.Context, userID); err != nil {
		return false, err
	}

	branchFromPod := pod.Annotations["codeserver.devplatform/branch"]
	_, err = s.k8sClient.CreateCodeServerPod(p.Context, userID, cloneURL, repoName, repoOwner, branchFromPod)
	if err != nil {
		return false, err
	}

	return true, nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), []string{"KB", "MB", "GB", "TB"}[exp])
}
