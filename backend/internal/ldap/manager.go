package ldap

import (
	"context"
	"net"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devplatform/ldap-manager/internal/config"
	"github.com/devplatform/ldap-manager/internal/models"
	ldap "github.com/go-ldap/ldap/v3"
	"github.com/sirupsen/logrus"
)

// Manager handles LDAP connections and operations
type Manager struct {
	config         *config.Config
	pool           chan *ldap.Conn
	poolSize       int
	mu             sync.RWMutex
	closed         bool
	logger         *logrus.Logger
	uidCounter     int32
	gidCounter     int32
	totalRequests  int64
	createdAt      time.Time
}

// NewManager creates a new LDAP manager with connection pool
func NewManager(cfg *config.Config, logger *logrus.Logger) (*Manager, error) {
	m := &Manager{
		config:     cfg,
		pool:       make(chan *ldap.Conn, cfg.LDAPPoolSize),
		poolSize:   cfg.LDAPPoolSize,
		logger:     logger,
		uidCounter: int32(cfg.StartingUID),
		gidCounter: int32(cfg.StartingGID),
		createdAt:  time.Now(),
	}

	// Pre-populate the connection pool
	for i := 0; i < cfg.LDAPPoolSize; i++ {
		conn, err := m.createConnection()
		if err != nil {
			m.logger.WithError(err).Error("Failed to create initial connection")
			// Close existing connections
			close(m.pool)
			for c := range m.pool {
				c.Close()
			}
			return nil, fmt.Errorf("failed to initialize connection pool: %w", err)
		}
		m.pool <- conn
	}

	m.logger.WithField("pool_size", cfg.LDAPPoolSize).Info("LDAP connection pool initialized")
	return m, nil
}

// createConnection creates a new LDAP connection
func (m *Manager) createConnection() (*ldap.Conn, error) {
	m.logger.WithField("url", m.config.LDAPURL).Debug("Creating new LDAP connection")

	conn, err := ldap.DialURL(m.config.LDAPURL, ldap.DialWithDialer(&net.Dialer{
		Timeout: m.config.LDAPConnTimeout,
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to dial LDAP: %w", err)
	}

	// Bind with admin credentials
	err = conn.Bind(m.config.LDAPBindDN, m.config.LDAPBindPassword)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to bind: %w", err)
	}

	return conn, nil
}

// getConnection retrieves a connection from the pool
func (m *Manager) getConnection(ctx context.Context) (*ldap.Conn, error) {
	atomic.AddInt64(&m.totalRequests, 1)

	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("connection pool is closed")
	}
	m.mu.RUnlock()

	select {
	case conn := <-m.pool:
		// Test the connection
		if !m.testConnection(conn) {
			m.logger.Debug("Connection test failed, creating new connection")
			conn.Close()
			newConn, err := m.createConnection()
			if err != nil {
				return nil, fmt.Errorf("failed to create new connection: %w", err)
			}
			return newConn, nil
		}
		return conn, nil
	case <-time.After(m.config.LDAPPoolTimeout):
		return nil, fmt.Errorf("timeout waiting for connection from pool")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// returnConnection returns a connection to the pool
func (m *Manager) returnConnection(conn *ldap.Conn) {
	if conn == nil {
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		conn.Close()
		return
	}

	// Non-blocking send
	select {
	case m.pool <- conn:
	default:
		// Pool is full, close the connection
		m.logger.Warn("Connection pool full, closing connection")
		conn.Close()
	}
}

// testConnection tests if a connection is still alive
func (m *Manager) testConnection(conn *ldap.Conn) bool {
	if conn == nil {
		return false
	}

	// Perform a simple search to test the connection
	searchRequest := ldap.NewSearchRequest(
		m.config.LDAPBaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0,
		int(m.config.LDAPConnTimeout.Seconds()),
		false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)

	_, err := conn.Search(searchRequest)
	return err == nil
}

// HealthCheck performs a health check on the LDAP connection
func (m *Manager) HealthCheck(ctx context.Context) error {
	conn, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer m.returnConnection(conn)

	if !m.testConnection(conn) {
		return fmt.Errorf("health check failed: connection test failed")
	}

	return nil
}

// GetStats returns connection pool statistics
func (m *Manager) GetStats() *models.Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	available := len(m.pool)
	inUse := m.poolSize - available

	return &models.Stats{
		PoolSize:      m.poolSize,
		Available:     available,
		InUse:         inUse,
		TotalRequests: int(atomic.LoadInt64(&m.totalRequests)),
	}
}

// Close closes all connections in the pool
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	close(m.pool)

	// Close all connections
	count := 0
	for conn := range m.pool {
		conn.Close()
		count++
	}

	m.logger.WithField("connections_closed", count).Info("LDAP connection pool closed")
	return nil
}

// nextUID returns the next available UID number
func (m *Manager) nextUID() int {
	return int(atomic.AddInt32(&m.uidCounter, 1))
}

// nextGID returns the next available GID number
func (m *Manager) nextGID() int {
	return int(atomic.AddInt32(&m.gidCounter, 1))
}
