package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	// ControllerName is the name of this controller
	ControllerName = "openldap-controller"

	// ManifestFile is the embedded OpenLDAP manifest
	ManifestFile = "manifests/01-openldap.yaml"

	// DefaultNamespace is the default namespace for OpenLDAP
	DefaultNamespace = "dev-platform"

	// DefaultLDAPURL is the default internal LDAP service URL
	DefaultLDAPURL = "ldap://openldap-internal.dev-platform.svc.cluster.local:389"

	// DefaultBaseDN is the default base DN
	DefaultBaseDN = "dc=devplatform,dc=local"

	// DefaultAdminDN is the default admin DN
	DefaultAdminDN = "cn=admin,dc=devplatform,dc=local"

	// DefaultAdminPassword is the default admin password
	DefaultAdminPassword = "admin123"
)

// Controller manages OpenLDAP deployment and initialization
type Controller struct {
	// Kubernetes client
	kubeClient kubernetes.Interface

	// REST config for dynamic client
	config *rest.Config

	// Manifest applier
	applier *ManifestApplier

	// LDAP initializer
	initializer *LDAPInitializer

	// Logger
	logger *logrus.Logger

	// Namespace to watch
	namespace string

	// StatefulSet informer
	informer cache.SharedIndexInformer

	// Initialization state
	initialized bool
	initMu      sync.RWMutex

	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Stop channel
	stopCh chan struct{}
}

// ControllerConfig holds configuration for the controller
type ControllerConfig struct {
	Namespace      string
	LDAPTimeout    time.Duration
	LDAPURL        string
	BaseDN         string
	AdminDN        string
	AdminPassword  string
	ConfigPassword string
	InitData       *InitDataSpec
}

// NewController creates a new OpenLDAP controller
func NewController(config *rest.Config, cfg *ControllerConfig, logger *logrus.Logger) (*Controller, error) {
	if cfg == nil {
		cfg = &ControllerConfig{
			Namespace:     DefaultNamespace,
			LDAPTimeout:   30 * time.Second,
			LDAPURL:       DefaultLDAPURL,
			BaseDN:        DefaultBaseDN,
			AdminDN:       DefaultAdminDN,
			AdminPassword: DefaultAdminPassword,
			InitData:      DefaultInitData(),
		}
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	applier, err := NewManifestApplier(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest applier: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Controller{
		kubeClient:  kubeClient,
		config:      config,
		applier:     applier,
		initializer: NewLDAPInitializer(logger, cfg.LDAPTimeout),
		logger:      logger,
		namespace:   cfg.Namespace,
		ctx:         ctx,
		cancel:      cancel,
		stopCh:      make(chan struct{}),
	}

	// Setup StatefulSet informer to watch for OpenLDAP readiness
	c.setupInformer()

	return c, nil
}

// setupInformer sets up the StatefulSet informer
func (c *Controller) setupInformer() {
	c.informer = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = "app=openldap"
				return c.kubeClient.AppsV1().StatefulSets(c.namespace).List(c.ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = "app=openldap"
				return c.kubeClient.AppsV1().StatefulSets(c.namespace).Watch(c.ctx, options)
			},
		},
		&appsv1.StatefulSet{},
		30*time.Second,
		cache.Indexers{},
	)

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.handleStatefulSetUpdate,
	})
}

// Run starts the controller
func (c *Controller) Run(cfg *ControllerConfig) error {
	c.logger.WithFields(logrus.Fields{
		"controller": ControllerName,
		"namespace":  c.namespace,
	}).Info("Starting OpenLDAP controller")

	// Step 1: Apply OpenLDAP manifests
	c.logger.Info("Applying OpenLDAP manifests")
	if err := c.applier.ApplyManifestFile(c.ctx, ManifestFile); err != nil {
		return fmt.Errorf("failed to apply manifests: %w", err)
	}

	// Step 2: Start informer to watch for StatefulSet readiness
	go c.informer.Run(c.stopCh)

	// Wait for cache sync
	if !cache.WaitForCacheSync(c.stopCh, c.informer.HasSynced) {
		return fmt.Errorf("failed to sync informer cache")
	}

	c.logger.Info("Waiting for OpenLDAP StatefulSet to be ready")

	// Step 3: Wait for StatefulSet to be ready, then initialize
	go c.watchAndInitialize(cfg)

	// Wait for stop signal
	<-c.stopCh
	return nil
}

// watchAndInitialize waits for OpenLDAP to be ready and runs initialization
func (c *Controller) watchAndInitialize(cfg *ControllerConfig) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if c.isInitialized() {
				continue
			}

			// Check if StatefulSet is ready
			ss, err := c.kubeClient.AppsV1().StatefulSets(c.namespace).Get(c.ctx, "openldap", metav1.GetOptions{})
			if err != nil {
				c.logger.WithError(err).Debug("StatefulSet not found yet")
				continue
			}

			if ss.Status.ReadyReplicas < 1 {
				c.logger.WithField("readyReplicas", ss.Status.ReadyReplicas).Debug("StatefulSet not ready")
				continue
			}

			c.logger.Info("OpenLDAP StatefulSet is ready, starting initialization")

			// Wait a bit for LDAP to fully start
			time.Sleep(10 * time.Second)

			// Run initialization
			if err := c.runInitialization(cfg); err != nil {
				c.logger.WithError(err).Error("Failed to initialize LDAP")
				continue
			}

			c.setInitialized(true)
			c.logger.Info("OpenLDAP initialization complete")
		}
	}
}

// runInitialization runs the LDAP initialization
func (c *Controller) runInitialization(cfg *ControllerConfig) error {
	ldapURL := cfg.LDAPURL
	if ldapURL == "" {
		ldapURL = DefaultLDAPURL
	}

	baseDN := cfg.BaseDN
	if baseDN == "" {
		baseDN = DefaultBaseDN
	}

	adminDN := cfg.AdminDN
	if adminDN == "" {
		adminDN = DefaultAdminDN
	}

	adminPassword := cfg.AdminPassword
	if adminPassword == "" {
		adminPassword = DefaultAdminPassword
	}

	initData := cfg.InitData
	if initData == nil {
		initData = DefaultInitData()
	}

	// Wait for LDAP to be ready
	ctx, cancel := context.WithTimeout(c.ctx, 2*time.Minute)
	defer cancel()

	if err := c.initializer.WaitForReady(ctx, ldapURL); err != nil {
		return fmt.Errorf("LDAP not ready: %w", err)
	}

	configPassword := cfg.ConfigPassword
	if configPassword == "" {
		configPassword = "config123"
	}

	// Run initialization
	return c.initializer.Initialize(ctx, ldapURL, adminDN, adminPassword, configPassword, baseDN, initData)
}

// handleStatefulSetUpdate handles StatefulSet update events
func (c *Controller) handleStatefulSetUpdate(oldObj, newObj interface{}) {
	newSS := newObj.(*appsv1.StatefulSet)

	c.logger.WithFields(logrus.Fields{
		"name":          newSS.Name,
		"readyReplicas": newSS.Status.ReadyReplicas,
		"replicas":      *newSS.Spec.Replicas,
	}).Debug("StatefulSet updated")
}

// Stop stops the controller
func (c *Controller) Stop() {
	c.logger.Info("Stopping OpenLDAP controller")
	c.cancel()
	close(c.stopCh)
}

// isInitialized returns whether LDAP has been initialized
func (c *Controller) isInitialized() bool {
	c.initMu.RLock()
	defer c.initMu.RUnlock()
	return c.initialized
}

// setInitialized sets the initialization state
func (c *Controller) setInitialized(v bool) {
	c.initMu.Lock()
	defer c.initMu.Unlock()
	c.initialized = v
}

// Delete removes all OpenLDAP resources
func (c *Controller) Delete() error {
	c.logger.Info("Deleting OpenLDAP resources")
	return c.applier.DeleteManifestFile(c.ctx, ManifestFile)
}
