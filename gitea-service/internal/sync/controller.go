package sync

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devplatform/gitea-service/internal/config"
	"github.com/devplatform/gitea-service/internal/gitea"
	"github.com/devplatform/gitea-service/internal/ldap"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Prometheus metrics for the reconciliation controller
var (
	syncTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gitea_ldap_sync_total",
			Help: "Total number of Gitea-to-LDAP sync operations",
		},
		[]string{"type", "status"},
	)

	syncDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gitea_ldap_sync_duration_seconds",
			Help:    "Duration of sync operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"type"},
	)

	syncLastSuccess = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gitea_ldap_sync_last_success",
			Help: "Unix timestamp of last successful full reconciliation",
		},
	)

	retryQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gitea_ldap_retry_queue_size",
			Help: "Current number of items in the retry queue",
		},
	)
)

const (
	maxRetries    = 5
	retryQueueCap = 100
)

// retryItem represents a failed sync that needs to be retried
type retryItem struct {
	UID       string    `json:"uid"`
	Attempts  int       `json:"attempts"`
	NextRetry time.Time `json:"next_retry"`
}

// persistedState is the controller state saved to disk for crash recovery
type persistedState struct {
	RetryItems           []retryItem                `json:"retry_items"`
	LastReconcileSuccess time.Time                  `json:"last_reconcile_success"`
	CollabGroups         map[string]*CollabGroupMeta `json:"collab_groups,omitempty"`
}

// backoffDuration returns the backoff duration for the given attempt number
func backoffDuration(attempt int) time.Duration {
	durations := []time.Duration{
		5 * time.Second,
		15 * time.Second,
		45 * time.Second,
		2 * time.Minute,
		5 * time.Minute,
	}
	if attempt >= len(durations) {
		return durations[len(durations)-1]
	}
	return durations[attempt]
}

// Controller manages the reconciliation of Gitea repos to LDAP
type Controller struct {
	giteaService     *gitea.Service
	giteaClient      *gitea.Client
	ldapClient       *ldap.Client
	groupSyncService *GroupSyncService
	cfg              *config.Config
	logger           *logrus.Logger

	retryMu              sync.Mutex
	retryItems           []retryItem
	lastReconcileSuccess time.Time
	dataDir              string

	collabMu     sync.RWMutex
	collabGroups map[string]*CollabGroupMeta

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewController creates a new reconciliation controller
func NewController(
	giteaService *gitea.Service,
	giteaClient *gitea.Client,
	ldapClient *ldap.Client,
	cfg *config.Config,
	logger *logrus.Logger,
) *Controller {
	return &Controller{
		giteaService:     giteaService,
		giteaClient:      giteaClient,
		ldapClient:       ldapClient,
		groupSyncService: NewGroupSyncService(giteaClient, ldapClient, logger),
		cfg:              cfg,
		logger:           logger,
		retryItems:       make([]retryItem, 0),
		collabGroups:     make(map[string]*CollabGroupMeta),
		dataDir:          cfg.DataDir,
		stopCh:           make(chan struct{}),
	}
}

// stateFilePath returns the path to the persisted state file
func (c *Controller) stateFilePath() string {
	return filepath.Join(c.dataDir, "state.json")
}

// loadState restores controller state from disk after a restart
func (c *Controller) loadState() {
	path := c.stateFilePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.logger.Info("No persisted state found, starting fresh")
			return
		}
		c.logger.WithError(err).Warn("Failed to read persisted state")
		return
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		c.logger.WithError(err).Warn("Failed to parse persisted state, starting fresh")
		return
	}

	c.retryMu.Lock()
	c.retryItems = state.RetryItems
	if c.retryItems == nil {
		c.retryItems = make([]retryItem, 0)
	}
	c.lastReconcileSuccess = state.LastReconcileSuccess
	retryQueueSize.Set(float64(len(c.retryItems)))
	c.retryMu.Unlock()

	c.collabMu.Lock()
	if state.CollabGroups != nil {
		c.collabGroups = state.CollabGroups
	}
	c.collabMu.Unlock()

	if !state.LastReconcileSuccess.IsZero() {
		syncLastSuccess.Set(float64(state.LastReconcileSuccess.Unix()))
	}

	c.logger.WithFields(logrus.Fields{
		"retry_items":    len(state.RetryItems),
		"last_reconcile": state.LastReconcileSuccess.Format(time.RFC3339),
	}).Info("Restored persisted state")
}

// saveState persists controller state to disk for crash recovery
func (c *Controller) saveState() {
	c.retryMu.Lock()
	state := persistedState{
		RetryItems:           make([]retryItem, len(c.retryItems)),
		LastReconcileSuccess: c.lastReconcileSuccess,
	}
	copy(state.RetryItems, c.retryItems)
	c.retryMu.Unlock()

	c.collabMu.RLock()
	if len(c.collabGroups) > 0 {
		state.CollabGroups = make(map[string]*CollabGroupMeta, len(c.collabGroups))
		for k, v := range c.collabGroups {
			copied := *v
			copied.ExtraMembers = make([]string, len(v.ExtraMembers))
			copy(copied.ExtraMembers, v.ExtraMembers)
			state.CollabGroups[k] = &copied
		}
	}
	c.collabMu.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		c.logger.WithError(err).Error("Failed to marshal state")
		return
	}

	path := c.stateFilePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		c.logger.WithError(err).Error("Failed to create state directory")
		return
	}

	// Write atomically: temp file then rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0640); err != nil {
		c.logger.WithError(err).Error("Failed to write state file")
		return
	}

	if err := os.Rename(tmpPath, path); err != nil {
		c.logger.WithError(err).Error("Failed to rename state file")
		return
	}

	c.logger.Debug("Persisted controller state")
}

// Start begins the controller's goroutines
func (c *Controller) Start() {
	if !c.cfg.ReconcileEnabled {
		c.logger.Info("Reconciliation controller is disabled")
		return
	}

	c.logger.Info("Starting reconciliation controller")

	// Restore persisted state from disk
	c.loadState()

	// Goroutine 1: Periodic full reconciliation
	c.wg.Add(1)
	go c.reconcileLoop()

	// Goroutine 2: Webhook health check (auto-register webhook in Gitea)
	c.wg.Add(1)
	go c.webhookHealthLoop()

	// Goroutine 3: Retry queue processor
	c.wg.Add(1)
	go c.retryLoop()

	// Goroutine 4: Group/Department → Gitea team sync
	c.wg.Add(1)
	go c.groupSyncLoop()

	c.logger.WithFields(logrus.Fields{
		"reconcile_interval":     c.cfg.ReconcileInterval,
		"webhook_check_interval": c.cfg.WebhookCheckInterval,
		"group_sync_interval":    c.cfg.GroupSyncInterval,
	}).Info("Reconciliation controller started")
}

// Stop gracefully stops the controller
func (c *Controller) Stop() {
	c.logger.Info("Stopping reconciliation controller")
	close(c.stopCh)
	c.wg.Wait()
	c.saveState()
	c.logger.Info("Reconciliation controller stopped")
}

// EnqueueRetry adds a failed user sync to the retry queue
func (c *Controller) EnqueueRetry(uid string) {
	c.retryMu.Lock()

	// Check if already in queue
	for _, item := range c.retryItems {
		if item.UID == uid {
			c.retryMu.Unlock()
			return
		}
	}

	if len(c.retryItems) >= retryQueueCap {
		c.logger.Warn("Retry queue is full, dropping oldest item")
		c.retryItems = c.retryItems[1:]
	}

	c.retryItems = append(c.retryItems, retryItem{
		UID:       uid,
		Attempts:  0,
		NextRetry: time.Now().Add(backoffDuration(0)),
	})

	retryQueueSize.Set(float64(len(c.retryItems)))
	c.retryMu.Unlock()

	c.logger.WithField("uid", uid).Info("Enqueued user for retry sync")
	c.saveState()
}

// SetupHTTPHandlers registers the controller's HTTP handlers on the given mux
func (c *Controller) SetupHTTPHandlers(mux *http.ServeMux) {
	// Webhook endpoint - receives Gitea webhook events
	mux.HandleFunc("/webhook/gitea", c.webhookHandler)

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())
}

// webhookHandler handles Gitea webhook events for real-time repo sync
func (c *Controller) webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.logger.WithError(err).Error("Failed to read webhook body")
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Always validate webhook signature - no trust policy
	if c.cfg.GiteaWebhookSecret == "" {
		c.logger.Error("Webhook secret not configured, rejecting request")
		http.Error(w, "webhook secret not configured", http.StatusInternalServerError)
		return
	}

	sig := r.Header.Get("X-Gitea-Signature")
	if !verifyWebhookSignature(body, sig, c.cfg.GiteaWebhookSecret) {
		c.logger.Warn("Invalid webhook signature")
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// Only handle repository events
	eventType := r.Header.Get("X-Gitea-Event")
	if eventType != "repository" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ignored", "event": eventType})
		return
	}

	syncTotal.WithLabelValues("webhook", "received").Inc()

	// Parse the webhook payload
	var payload struct {
		Action string `json:"action"`
		Sender struct {
			Login string `json:"login"`
		} `json:"sender"`
		Repository struct {
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
			FullName string `json:"full_name"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		c.logger.WithError(err).Error("Failed to parse webhook payload")
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	ownerLogin := payload.Repository.Owner.Login
	if ownerLogin == "" {
		c.logger.Warn("Webhook payload missing repository owner")
		http.Error(w, "missing repository owner", http.StatusBadRequest)
		return
	}

	c.logger.WithFields(logrus.Fields{
		"action": payload.Action,
		"owner":  ownerLogin,
		"repo":   payload.Repository.FullName,
	}).Info("Received Gitea repository webhook")

	// Get a Keycloak service token to authenticate with LDAP Manager
	serviceToken, err := c.getKeycloakToken()
	if err != nil {
		c.logger.WithError(err).Error("Failed to get Keycloak token for webhook")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Sync the repo owner's repos to LDAP
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	start := time.Now()
	result, err := c.giteaService.SyncGiteaReposToLDAP(ctx, ownerLogin, serviceToken)
	duration := time.Since(start).Seconds()
	syncDuration.WithLabelValues("webhook").Observe(duration)

	if err != nil {
		syncTotal.WithLabelValues("webhook", "error").Inc()
		c.logger.WithFields(logrus.Fields{
			"owner": ownerLogin,
		}).WithError(err).Error("Webhook repo sync failed, enqueuing for retry")
		c.EnqueueRetry(ownerLogin)
		http.Error(w, "sync failed", http.StatusInternalServerError)
		return
	}

	syncTotal.WithLabelValues("webhook", "success").Inc()

	c.logger.WithFields(logrus.Fields{
		"owner":      ownerLogin,
		"reposCount": result.ReposCount,
	}).Info("Webhook repo sync completed")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "synced",
		"uid":        result.UID,
		"reposCount": result.ReposCount,
	})
}

// reconcileLoop runs full reconciliation at configured intervals
func (c *Controller) reconcileLoop() {
	defer c.wg.Done()

	// Run initial reconciliation after a short delay to let services warm up
	select {
	case <-time.After(30 * time.Second):
	case <-c.stopCh:
		return
	}

	// Sync LDAP users to Gitea first so they exist for repo/group sync
	c.runUserSync()
	c.runFullReconcile()

	ticker := time.NewTicker(c.cfg.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.runFullReconcile()
		case <-c.stopCh:
			return
		}
	}
}

// runFullReconcile performs a full sync of all Gitea repos to LDAP
func (c *Controller) runFullReconcile() {
	c.logger.Info("Starting full reconciliation")
	start := time.Now()

	// Sync LDAP users to Gitea first so they exist for repo sync
	c.runUserSync()

	token, err := c.getKeycloakToken()
	if err != nil {
		c.logger.WithError(err).Error("Failed to get Keycloak token for reconciliation")
		syncTotal.WithLabelValues("reconcile", "error").Inc()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	results, err := c.giteaService.SyncAllGiteaReposToLDAP(ctx, token)
	duration := time.Since(start).Seconds()
	syncDuration.WithLabelValues("reconcile").Observe(duration)

	if err != nil {
		c.logger.WithError(err).Error("Full reconciliation failed")
		syncTotal.WithLabelValues("reconcile", "error").Inc()
		return
	}

	syncTotal.WithLabelValues("reconcile", "success").Inc()
	now := time.Now()
	syncLastSuccess.Set(float64(now.Unix()))

	c.retryMu.Lock()
	c.lastReconcileSuccess = now
	c.retryMu.Unlock()

	c.logger.WithFields(logrus.Fields{
		"users_synced": len(results),
		"duration_s":   fmt.Sprintf("%.2f", duration),
	}).Info("Full reconciliation completed")

	c.saveState()
}

// runUserSync syncs all LDAP users to Gitea so they exist before repo/group sync
func (c *Controller) runUserSync() {
	c.logger.Info("Starting LDAP → Gitea user sync")
	start := time.Now()

	token, err := c.getKeycloakToken()
	if err != nil {
		c.logger.WithError(err).Error("Failed to get Keycloak token for user sync")
		syncTotal.WithLabelValues("user_sync", "error").Inc()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	users, err := c.giteaService.SyncAllLDAPUsersToGitea(ctx, token, c.cfg.UserSyncDefaultPassword)
	duration := time.Since(start).Seconds()
	syncDuration.WithLabelValues("user_sync").Observe(duration)

	if err != nil {
		c.logger.WithError(err).Error("LDAP user sync failed")
		syncTotal.WithLabelValues("user_sync", "error").Inc()
		return
	}

	syncTotal.WithLabelValues("user_sync", "success").Inc()
	c.logger.WithFields(logrus.Fields{
		"users_synced": len(users),
		"duration_s":   fmt.Sprintf("%.2f", duration),
	}).Info("LDAP user sync completed")
}

// webhookHealthLoop periodically checks that the Gitea webhook exists
func (c *Controller) webhookHealthLoop() {
	defer c.wg.Done()

	// Initial webhook registration after a short delay
	select {
	case <-time.After(10 * time.Second):
	case <-c.stopCh:
		return
	}

	c.ensureWebhook()

	ticker := time.NewTicker(c.cfg.WebhookCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.ensureWebhook()
		case <-c.stopCh:
			return
		}
	}
}

// ensureWebhook checks and creates the Gitea system webhook if needed
func (c *Controller) ensureWebhook() {
	targetURL := fmt.Sprintf("http://%s/webhook/gitea", c.cfg.WebhookTargetHost)

	if err := c.giteaClient.EnsureWebhook(targetURL, c.cfg.GiteaWebhookSecret); err != nil {
		c.logger.WithError(err).Warn("Failed to ensure Gitea webhook")
	} else {
		c.logger.Debug("Webhook health check passed")
	}
}

// retryLoop processes the retry queue
func (c *Controller) retryLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.processRetryQueue()
		case <-c.stopCh:
			return
		}
	}
}

// processRetryQueue processes items that are ready for retry
func (c *Controller) processRetryQueue() {
	c.retryMu.Lock()
	now := time.Now()

	// Find items ready for retry
	ready := make([]retryItem, 0)
	remaining := make([]retryItem, 0)

	for _, item := range c.retryItems {
		if now.After(item.NextRetry) {
			ready = append(ready, item)
		} else {
			remaining = append(remaining, item)
		}
	}

	c.retryItems = remaining
	c.retryMu.Unlock()

	if len(ready) == 0 {
		return
	}

	token, err := c.getKeycloakToken()
	if err != nil {
		c.logger.WithError(err).Error("Failed to get Keycloak token for retry")
		// Re-enqueue all items
		c.retryMu.Lock()
		c.retryItems = append(c.retryItems, ready...)
		c.retryMu.Unlock()
		return
	}

	for _, item := range ready {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		start := time.Now()
		_, err := c.giteaService.SyncGiteaReposToLDAP(ctx, item.UID, token)
		duration := time.Since(start).Seconds()
		syncDuration.WithLabelValues("retry").Observe(duration)
		cancel()

		if err != nil {
			syncTotal.WithLabelValues("retry", "error").Inc()
			item.Attempts++

			if item.Attempts >= maxRetries {
				c.logger.WithFields(logrus.Fields{
					"uid":      item.UID,
					"attempts": item.Attempts,
				}).Error("Max retries reached, dropping item")
				continue
			}

			item.NextRetry = time.Now().Add(backoffDuration(item.Attempts))
			c.retryMu.Lock()
			c.retryItems = append(c.retryItems, item)
			c.retryMu.Unlock()

			c.logger.WithFields(logrus.Fields{
				"uid":        item.UID,
				"attempt":    item.Attempts,
				"next_retry": item.NextRetry.Format(time.RFC3339),
			}).Warn("Retry sync failed, re-enqueued")
		} else {
			syncTotal.WithLabelValues("retry", "success").Inc()
			c.logger.WithField("uid", item.UID).Info("Retry sync succeeded")
		}
	}

	retryQueueSize.Set(float64(len(c.retryItems)))
	c.saveState()
}

// getKeycloakToken obtains a service token via Keycloak client credentials grant
func (c *Controller) getKeycloakToken() (string, error) {
	if c.cfg.KeycloakClientSecret == "" {
		return "", fmt.Errorf("KEYCLOAK_CLIENT_SECRET is required for controller auth")
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.cfg.KeycloakClientID)
	data.Set("client_secret", c.cfg.KeycloakClientSecret)

	tokenURL := c.cfg.GetKeycloakTokenURL()

	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to request Keycloak token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Keycloak response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Keycloak token request failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse Keycloak token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("Keycloak returned empty access token")
	}

	c.logger.WithFields(logrus.Fields{
		"client_id":  c.cfg.KeycloakClientID,
		"expires_in": tokenResp.ExpiresIn,
	}).Debug("Obtained Keycloak service token")

	return tokenResp.AccessToken, nil
}

// ============================================================================
// GROUP SYNC (4th goroutine)
// ============================================================================

// groupSyncLoop periodically syncs LDAP groups and departments to Gitea teams
func (c *Controller) groupSyncLoop() {
	defer c.wg.Done()

	// Initial delay to let services warm up
	select {
	case <-time.After(20 * time.Second):
	case <-c.stopCh:
		return
	}

	c.runGroupSync()

	ticker := time.NewTicker(c.cfg.GroupSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.runGroupSync()
		case <-c.stopCh:
			return
		}
	}
}

// runGroupSync performs one cycle of LDAP → Gitea team synchronization
func (c *Controller) runGroupSync() {
	c.logger.Info("Starting group sync cycle")
	start := time.Now()

	token, err := c.getKeycloakToken()
	if err != nil {
		c.logger.WithError(err).Error("Failed to get Keycloak token for group sync")
		syncTotal.WithLabelValues("group_sync", "error").Inc()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	orgName := c.cfg.GetDefaultOwner()
	var syncErrors int

	// Sync departments that have repositories
	departments, err := c.ldapClient.ListAllDepartments(ctx, token)
	if err != nil {
		c.logger.WithError(err).Error("Failed to list departments for group sync")
		syncTotal.WithLabelValues("group_sync", "error").Inc()
		return
	}

	for _, dept := range departments {
		if len(dept.Repositories) == 0 {
			continue
		}

		result, err := c.groupSyncService.SyncDepartmentToTeam(ctx, dept.OU, orgName, dept.OU, "write", token)
		if err != nil {
			c.logger.WithError(err).Errorf("Failed to sync department %s", dept.OU)
			syncErrors++
			continue
		}

		c.logger.WithFields(logrus.Fields{
			"department":       dept.OU,
			"membersAdded":     result.MembersAdded,
			"reposAdded":       result.RepositoriesAdded,
			"managerGranted":   result.ManagerGranted,
		}).Info("Department synced to Gitea team")
	}

	// Sync groups that have repositories
	groups, err := c.ldapClient.ListAllGroups(ctx, token)
	if err != nil {
		c.logger.WithError(err).Error("Failed to list groups for group sync")
		syncTotal.WithLabelValues("group_sync", "error").Inc()
		return
	}

	for _, group := range groups {
		if len(group.Repositories) == 0 {
			continue
		}

		c.collabMu.RLock()
		meta, isCollab := c.collabGroups[group.CN]
		c.collabMu.RUnlock()

		if isCollab {
			// This is a dynamic collab group — resolve membership from dept + extras
			result, err := c.groupSyncService.SyncCollabGroup(ctx, group.CN, meta, orgName, "write", token)
			if err != nil {
				c.logger.WithError(err).Errorf("Failed to sync collab group %s", group.CN)
				syncErrors++
				continue
			}
			c.logger.WithFields(logrus.Fields{
				"collabGroup":    group.CN,
				"membersAdded":   result.MembersAdded,
				"reposAdded":     result.RepositoriesAdded,
				"managerGranted": result.ManagerGranted,
			}).Info("Collab group synced to Gitea team")
		} else {
			// Regular LDAP group — sync directly
			result, err := c.groupSyncService.SyncGroupToTeam(ctx, group.CN, orgName, group.CN, "write", "")
			if err != nil {
				c.logger.WithError(err).Errorf("Failed to sync group %s", group.CN)
				syncErrors++
				continue
			}
			c.logger.WithFields(logrus.Fields{
				"group":        group.CN,
				"membersAdded": result.MembersAdded,
				"reposAdded":   result.RepositoriesAdded,
			}).Info("Group synced to Gitea team")
		}
	}

	duration := time.Since(start).Seconds()
	syncDuration.WithLabelValues("group_sync").Observe(duration)

	if syncErrors > 0 {
		syncTotal.WithLabelValues("group_sync", "partial").Inc()
	} else {
		syncTotal.WithLabelValues("group_sync", "success").Inc()
	}

	c.logger.WithFields(logrus.Fields{
		"departments": len(departments),
		"groups":      len(groups),
		"errors":      syncErrors,
		"duration_s":  fmt.Sprintf("%.2f", duration),
	}).Info("Group sync cycle completed")
}

// ============================================================================
// COLLAB GROUP MANAGEMENT (used by collab_service.go)
// ============================================================================

// RegisterCollabGroup saves collab group metadata and persists state
func (c *Controller) RegisterCollabGroup(groupCN string, meta *CollabGroupMeta) {
	c.collabMu.Lock()
	c.collabGroups[groupCN] = meta
	c.collabMu.Unlock()

	c.logger.WithFields(logrus.Fields{
		"groupCN":        groupCN,
		"baseDepartment": meta.BaseDepartment,
		"extraMembers":   len(meta.ExtraMembers),
	}).Info("Registered collab group")

	c.saveState()
}

// UnregisterCollabGroup removes collab group metadata and persists state
func (c *Controller) UnregisterCollabGroup(groupCN string) {
	c.collabMu.Lock()
	delete(c.collabGroups, groupCN)
	c.collabMu.Unlock()

	c.logger.WithField("groupCN", groupCN).Info("Unregistered collab group")
	c.saveState()
}

// GetCollabGroupMeta returns the collab group metadata, or nil if not a collab group
func (c *Controller) GetCollabGroupMeta(groupCN string) *CollabGroupMeta {
	c.collabMu.RLock()
	defer c.collabMu.RUnlock()
	return c.collabGroups[groupCN]
}

// ListCollabGroups returns a copy of all collab group metadata
func (c *Controller) ListCollabGroups() map[string]*CollabGroupMeta {
	c.collabMu.RLock()
	defer c.collabMu.RUnlock()

	result := make(map[string]*CollabGroupMeta, len(c.collabGroups))
	for k, v := range c.collabGroups {
		copied := *v
		copied.ExtraMembers = make([]string, len(v.ExtraMembers))
		copy(copied.ExtraMembers, v.ExtraMembers)
		result[k] = &copied
	}
	return result
}

// GetGroupSyncService returns the group sync service for direct use by other layers
func (c *Controller) GetGroupSyncService() *GroupSyncService {
	return c.groupSyncService
}

// verifyWebhookSignature validates the HMAC-SHA256 signature from Gitea
func verifyWebhookSignature(payload []byte, signature string, secret string) bool {
	if signature == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}
