// Simple Dashboard Server with Metrics Proxy
// Run with: node dashboard-server.js

const http = require('http');

const PORT = 9999;

const METRICS_SERVICES = {
  ldap: 'http://localhost:30007/metrics',
  gitea: 'http://localhost:30013/metrics',
  codeserver: 'http://localhost:30014/metrics'
};

const HTML_DASHBOARD = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Dev Platform Metrics Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
            color: #e4e4e7;
            padding: 20px;
        }

        .container {
            max-width: 1600px;
            margin: 0 auto;
        }

        header {
            text-align: center;
            margin-bottom: 30px;
            padding: 20px;
            background: rgba(255, 255, 255, 0.05);
            border-radius: 16px;
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.1);
        }

        h1 {
            font-size: 2.5rem;
            background: linear-gradient(90deg, #00d9ff, #00ff88);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 10px;
        }

        .subtitle {
            color: #94a3b8;
            font-size: 1rem;
        }

        .refresh-info {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 15px;
            margin-top: 15px;
        }

        .last-update {
            color: #64748b;
            font-size: 0.9rem;
        }

        .refresh-btn {
            background: linear-gradient(135deg, #00d9ff, #00ff88);
            color: #1a1a2e;
            border: none;
            padding: 10px 25px;
            border-radius: 25px;
            cursor: pointer;
            font-weight: 600;
            font-size: 0.9rem;
            transition: all 0.3s ease;
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .refresh-btn:hover {
            transform: scale(1.05);
            box-shadow: 0 5px 20px rgba(0, 217, 255, 0.3);
        }

        .refresh-btn.loading {
            opacity: 0.7;
            cursor: not-allowed;
        }

        .refresh-btn .spinner {
            width: 16px;
            height: 16px;
            border: 2px solid #1a1a2e;
            border-top-color: transparent;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            display: none;
        }

        .refresh-btn.loading .spinner {
            display: block;
        }

        @keyframes spin {
            to { transform: rotate(360deg); }
        }

        .services-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(500px, 1fr));
            gap: 25px;
            margin-bottom: 30px;
        }

        .service-card {
            background: rgba(255, 255, 255, 0.05);
            border-radius: 16px;
            padding: 25px;
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.1);
            transition: all 0.3s ease;
        }

        .service-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 10px 40px rgba(0, 0, 0, 0.3);
            border-color: rgba(255, 255, 255, 0.2);
        }

        .service-header {
            display: flex;
            align-items: center;
            gap: 15px;
            margin-bottom: 20px;
            padding-bottom: 15px;
            border-bottom: 1px solid rgba(255, 255, 255, 0.1);
        }

        .service-icon {
            width: 50px;
            height: 50px;
            border-radius: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 1.5rem;
        }

        .ldap-icon { background: linear-gradient(135deg, #8b5cf6, #6366f1); }
        .gitea-icon { background: linear-gradient(135deg, #22c55e, #16a34a); }
        .codeserver-icon { background: linear-gradient(135deg, #f97316, #ea580c); }

        .service-title {
            flex: 1;
        }

        .service-title h2 {
            font-size: 1.3rem;
            color: #fff;
            margin-bottom: 5px;
        }

        .service-title .port {
            font-size: 0.85rem;
            color: #64748b;
        }

        .status-badge {
            padding: 6px 14px;
            border-radius: 20px;
            font-size: 0.8rem;
            font-weight: 600;
        }

        .status-online {
            background: rgba(34, 197, 94, 0.2);
            color: #22c55e;
            border: 1px solid rgba(34, 197, 94, 0.3);
        }

        .status-offline {
            background: rgba(239, 68, 68, 0.2);
            color: #ef4444;
            border: 1px solid rgba(239, 68, 68, 0.3);
        }

        .status-loading {
            background: rgba(251, 191, 36, 0.2);
            color: #fbbf24;
            border: 1px solid rgba(251, 191, 36, 0.3);
        }

        .metrics-section {
            margin-bottom: 20px;
        }

        .metrics-section:last-child {
            margin-bottom: 0;
        }

        .section-title {
            font-size: 0.9rem;
            color: #94a3b8;
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-bottom: 12px;
            display: flex;
            align-items: center;
            gap: 8px;
        }

        .section-title::before {
            content: '';
            width: 4px;
            height: 16px;
            background: linear-gradient(180deg, #00d9ff, #00ff88);
            border-radius: 2px;
        }

        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
            gap: 12px;
        }

        .metric-item {
            background: rgba(0, 0, 0, 0.2);
            border-radius: 10px;
            padding: 15px;
            text-align: center;
            transition: all 0.3s ease;
            border: 1px solid rgba(255, 255, 255, 0.05);
        }

        .metric-item:hover {
            background: rgba(0, 0, 0, 0.3);
            border-color: rgba(255, 255, 255, 0.1);
        }

        .metric-value {
            font-size: 1.8rem;
            font-weight: 700;
            margin-bottom: 5px;
        }

        .metric-value.counter { color: #00d9ff; }
        .metric-value.gauge { color: #00ff88; }
        .metric-value.histogram { color: #fbbf24; }

        .metric-label {
            font-size: 0.75rem;
            color: #94a3b8;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .metric-item.highlight {
            background: linear-gradient(135deg, rgba(0, 217, 255, 0.1), rgba(0, 255, 136, 0.1));
            border-color: rgba(0, 217, 255, 0.3);
        }

        .metrics-list {
            display: flex;
            flex-direction: column;
            gap: 8px;
        }

        .metric-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 10px 15px;
            background: rgba(0, 0, 0, 0.2);
            border-radius: 8px;
            font-size: 0.9rem;
        }

        .metric-row-name {
            color: #cbd5e1;
        }

        .metric-row-value {
            font-weight: 600;
            color: #00d9ff;
        }

        .metric-row-value.success { color: #22c55e; }
        .metric-row-value.error { color: #ef4444; }

        .error-message {
            background: rgba(239, 68, 68, 0.1);
            border: 1px solid rgba(239, 68, 68, 0.3);
            border-radius: 10px;
            padding: 20px;
            text-align: center;
            color: #ef4444;
        }

        .loading-skeleton {
            background: linear-gradient(90deg, rgba(255,255,255,0.05) 25%, rgba(255,255,255,0.1) 50%, rgba(255,255,255,0.05) 75%);
            background-size: 200% 100%;
            animation: shimmer 1.5s infinite;
            border-radius: 8px;
            height: 60px;
        }

        @keyframes shimmer {
            0% { background-position: -200% 0; }
            100% { background-position: 200% 0; }
        }

        .overview-section {
            margin-bottom: 30px;
        }

        .overview-title {
            font-size: 1.2rem;
            color: #fff;
            margin-bottom: 15px;
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .overview-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
        }

        .overview-card {
            background: rgba(255, 255, 255, 0.05);
            border-radius: 12px;
            padding: 20px;
            text-align: center;
            border: 1px solid rgba(255, 255, 255, 0.1);
        }

        .overview-value {
            font-size: 2.5rem;
            font-weight: 700;
            background: linear-gradient(90deg, #00d9ff, #00ff88);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }

        .overview-label {
            color: #94a3b8;
            font-size: 0.9rem;
            margin-top: 5px;
        }

        .operations-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 10px;
        }

        .operations-table th,
        .operations-table td {
            padding: 10px 15px;
            text-align: left;
            border-bottom: 1px solid rgba(255, 255, 255, 0.05);
        }

        .operations-table th {
            color: #64748b;
            font-weight: 500;
            font-size: 0.8rem;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .operations-table td {
            color: #e4e4e7;
        }

        .operations-table tr:hover td {
            background: rgba(255, 255, 255, 0.02);
        }

        .operations-table .value-success {
            color: #22c55e;
            font-weight: 600;
        }

        .operations-table .value-error {
            color: #ef4444;
            font-weight: 600;
        }

        @media (max-width: 768px) {
            .services-grid {
                grid-template-columns: 1fr;
            }
            
            h1 {
                font-size: 1.8rem;
            }

            .metrics-grid {
                grid-template-columns: repeat(2, 1fr);
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>Dev Platform Metrics Dashboard</h1>
            <p class="subtitle">Real-time business-level metrics from all services</p>
            <div class="refresh-info">
                <span class="last-update" id="lastUpdate">Last update: Never</span>
                <button class="refresh-btn" id="refreshBtn" onclick="fetchAllMetrics()">
                    <span class="spinner"></span>
                    <span>Refresh Now</span>
                </button>
            </div>
        </header>

        <!-- Overview Section -->
        <section class="overview-section">
            <h3 class="overview-title">
                <span>📊</span> Platform Overview
            </h3>
            <div class="overview-grid" id="overviewGrid">
                <div class="overview-card">
                    <div class="overview-value" id="totalUsers">-</div>
                    <div class="overview-label">Total Users</div>
                </div>
                <div class="overview-card">
                    <div class="overview-value" id="totalGroups">-</div>
                    <div class="overview-label">Total Groups</div>
                </div>
                <div class="overview-card">
                    <div class="overview-value" id="totalDepartments">-</div>
                    <div class="overview-label">Departments</div>
                </div>
                <div class="overview-card">
                    <div class="overview-value" id="totalRepos">-</div>
                    <div class="overview-label">Repositories</div>
                </div>
                <div class="overview-card">
                    <div class="overview-value" id="totalWorkspaces">-</div>
                    <div class="overview-label">Active Workspaces</div>
                </div>
                <div class="overview-card">
                    <div class="overview-value" id="totalPRs">-</div>
                    <div class="overview-label">Open PRs</div>
                </div>
            </div>
        </section>

        <!-- Services Grid -->
        <div class="services-grid">
            <!-- LDAP Manager -->
            <div class="service-card" id="ldapCard">
                <div class="service-header">
                    <div class="service-icon ldap-icon">🔐</div>
                    <div class="service-title">
                        <h2>LDAP Manager</h2>
                        <span class="port">Port 30007</span>
                    </div>
                    <span class="status-badge status-loading" id="ldapStatus">Loading...</span>
                </div>
                <div id="ldapContent">
                    <div class="loading-skeleton"></div>
                </div>
            </div>

            <!-- Gitea Service -->
            <div class="service-card" id="giteaCard">
                <div class="service-header">
                    <div class="service-icon gitea-icon">🐙</div>
                    <div class="service-title">
                        <h2>Gitea Service</h2>
                        <span class="port">Port 30013</span>
                    </div>
                    <span class="status-badge status-loading" id="giteaStatus">Loading...</span>
                </div>
                <div id="giteaContent">
                    <div class="loading-skeleton"></div>
                </div>
            </div>

            <!-- CodeServer Service -->
            <div class="service-card" id="codeserverCard">
                <div class="service-header">
                    <div class="service-icon codeserver-icon">💻</div>
                    <div class="service-title">
                        <h2>CodeServer Service</h2>
                        <span class="port">Port 30014</span>
                    </div>
                    <span class="status-badge status-loading" id="codeserverStatus">Loading...</span>
                </div>
                <div id="codeserverContent">
                    <div class="loading-skeleton"></div>
                </div>
            </div>
        </div>
    </div>

    <script>
        // Parse Prometheus metrics text format
        function parseMetrics(text) {
            const metrics = {};
            const lines = text.split('\\n');
            
            for (const line of lines) {
                if (line.startsWith('#') || !line.trim()) continue;
                
                const match = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)((?:\\{[^}]*\\})?)?\\s+(.+)$/);
                if (match) {
                    const [, name, labelsStr, value] = match;
                    
                    let labels = {};
                    if (labelsStr && labelsStr.length > 2) {
                        const labelMatches = labelsStr.slice(1, -1).matchAll(/([a-zA-Z_][a-zA-Z0-9_]*)="([^"]*)"/g);
                        for (const [, key, val] of labelMatches) {
                            labels[key] = val;
                        }
                    }
                    
                    if (!metrics[name]) {
                        metrics[name] = [];
                    }
                    metrics[name].push({
                        value: parseFloat(value),
                        labels: labels
                    });
                }
            }
            
            return metrics;
        }

        // Get metric value
        function getMetric(metrics, name, labelFilter = null) {
            if (!metrics[name]) return null;
            
            if (!labelFilter) {
                return metrics[name][0]?.value ?? null;
            }
            
            const found = metrics[name].find(m => {
                return Object.entries(labelFilter).every(([k, v]) => m.labels[k] === v);
            });
            
            return found?.value ?? null;
        }

        // Format number
        function formatNumber(num) {
            if (num === null || num === undefined) return '-';
            if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
            if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
            return num.toLocaleString();
        }

        // Fetch metrics via proxy
        async function fetchMetrics(service) {
            try {
                const response = await fetch('/api/metrics/' + service);
                if (!response.ok) throw new Error('HTTP ' + response.status);
                const text = await response.text();
                return { success: true, data: parseMetrics(text) };
            } catch (error) {
                return { success: false, error: error.message };
            }
        }

        // Render LDAP metrics
        function renderLDAPMetrics(metrics) {
            const usersTotal = getMetric(metrics, 'ldap_users_total');
            const groupsTotal = getMetric(metrics, 'ldap_groups_total');
            const deptsTotal = getMetric(metrics, 'ldap_departments_total');
            const usersCreated = getMetric(metrics, 'ldap_users_created_total');
            const usersUpdated = getMetric(metrics, 'ldap_users_updated_total');
            const usersDeleted = getMetric(metrics, 'ldap_users_deleted_total');
            const authAttempts = getMetric(metrics, 'ldap_auth_attempts_total');
            const poolSize = getMetric(metrics, 'ldap_pool_size');
            const poolActive = getMetric(metrics, 'ldap_pool_active_connections');
            const poolIdle = getMetric(metrics, 'ldap_pool_idle_connections');
            const operations = metrics['ldap_operations_total'] || [];

            return \`
                <div class="metrics-section">
                    <div class="section-title">Entity Counts</div>
                    <div class="metrics-grid">
                        <div class="metric-item highlight">
                            <div class="metric-value gauge">\${formatNumber(usersTotal)}</div>
                            <div class="metric-label">Users</div>
                        </div>
                        <div class="metric-item highlight">
                            <div class="metric-value gauge">\${formatNumber(groupsTotal)}</div>
                            <div class="metric-label">Groups</div>
                        </div>
                        <div class="metric-item highlight">
                            <div class="metric-value gauge">\${formatNumber(deptsTotal)}</div>
                            <div class="metric-label">Departments</div>
                        </div>
                    </div>
                </div>

                <div class="metrics-section">
                    <div class="section-title">User Operations</div>
                    <div class="metrics-grid">
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(usersCreated)}</div>
                            <div class="metric-label">Created</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(usersUpdated)}</div>
                            <div class="metric-label">Updated</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(usersDeleted)}</div>
                            <div class="metric-label">Deleted</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(authAttempts)}</div>
                            <div class="metric-label">Auth Attempts</div>
                        </div>
                    </div>
                </div>

                <div class="metrics-section">
                    <div class="section-title">Connection Pool</div>
                    <div class="metrics-grid">
                        <div class="metric-item">
                            <div class="metric-value gauge">\${formatNumber(poolSize)}</div>
                            <div class="metric-label">Pool Size</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value gauge">\${formatNumber(poolActive)}</div>
                            <div class="metric-label">Active</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value gauge">\${formatNumber(poolIdle)}</div>
                            <div class="metric-label">Idle</div>
                        </div>
                    </div>
                </div>

                \${operations.length > 0 ? \`
                <div class="metrics-section">
                    <div class="section-title">Operations</div>
                    <table class="operations-table">
                        <thead>
                            <tr>
                                <th>Operation</th>
                                <th>Status</th>
                                <th>Count</th>
                            </tr>
                        </thead>
                        <tbody>
                            \${operations.map(op => \`
                                <tr>
                                    <td>\${op.labels.operation || '-'}</td>
                                    <td class="\${op.labels.success === 'true' ? 'value-success' : 'value-error'}">\${op.labels.success === 'true' ? '✓ Success' : '✗ Failed'}</td>
                                    <td>\${formatNumber(op.value)}</td>
                                </tr>
                            \`).join('')}
                        </tbody>
                    </table>
                </div>
                \` : ''}
            \`;
        }

        // Render Gitea metrics
        function renderGiteaMetrics(metrics) {
            const reposTotal = getMetric(metrics, 'gitea_repos_total');
            const usersTotal = getMetric(metrics, 'gitea_users_total');
            const prsOpen = getMetric(metrics, 'gitea_prs_open_total');
            const prsCreated = getMetric(metrics, 'gitea_prs_created_total');
            const branchesCreated = getMetric(metrics, 'gitea_branches_created_total');
            const tagsCreated = getMetric(metrics, 'gitea_tags_created_total');
            const usersSynced = getMetric(metrics, 'gitea_users_synced_total');
            const reposSynced = getMetric(metrics, 'gitea_repos_synced_total');
            const syncLastSuccess = getMetric(metrics, 'gitea_ldap_sync_last_success');

            return \`
                <div class="metrics-section">
                    <div class="section-title">Repository Stats</div>
                    <div class="metrics-grid">
                        <div class="metric-item highlight">
                            <div class="metric-value gauge">\${formatNumber(reposTotal)}</div>
                            <div class="metric-label">Total Repos</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(branchesCreated)}</div>
                            <div class="metric-label">Branches</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(tagsCreated)}</div>
                            <div class="metric-label">Tags</div>
                        </div>
                    </div>
                </div>

                <div class="metrics-section">
                    <div class="section-title">Pull Requests</div>
                    <div class="metrics-grid">
                        <div class="metric-item highlight">
                            <div class="metric-value gauge">\${formatNumber(prsOpen)}</div>
                            <div class="metric-label">Open PRs</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(prsCreated)}</div>
                            <div class="metric-label">Created</div>
                        </div>
                    </div>
                </div>

                <div class="metrics-section">
                    <div class="section-title">User Sync</div>
                    <div class="metrics-grid">
                        <div class="metric-item">
                            <div class="metric-value gauge">\${formatNumber(usersTotal)}</div>
                            <div class="metric-label">Gitea Users</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(usersSynced)}</div>
                            <div class="metric-label">Synced from LDAP</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(reposSynced)}</div>
                            <div class="metric-label">Repos Synced</div>
                        </div>
                    </div>
                </div>

                <div class="metrics-section">
                    <div class="section-title">Sync Status</div>
                    <div class="metrics-list">
                        <div class="metric-row">
                            <span class="metric-row-name">Last Successful Sync</span>
                            <span class="metric-row-value">\${syncLastSuccess > 0 ? new Date(syncLastSuccess * 1000).toLocaleString() : 'Never'}</span>
                        </div>
                    </div>
                </div>
            \`;
        }

        // Render CodeServer metrics
        function renderCodeServerMetrics(metrics) {
            const workspacesActive = getMetric(metrics, 'codeserver_workspaces_active_total');
            const workspacesCreated = getMetric(metrics, 'codeserver_workspaces_created_total');
            const workspacesDeleted = getMetric(metrics, 'codeserver_workspaces_deleted_total');
            const pvcTotal = getMetric(metrics, 'codeserver_pvc_total_bytes');
            const pvcCreated = getMetric(metrics, 'codeserver_pvc_created_total');
            const giteaApiSuccess = getMetric(metrics, 'codeserver_gitea_api_calls_total', { success: 'true' });
            const giteaApiFail = getMetric(metrics, 'codeserver_gitea_api_calls_total', { success: 'false' });
            const podCreates = getMetric(metrics, 'k8s_pod_creates_total', { success: 'true' });
            const serviceCreates = getMetric(metrics, 'k8s_service_creates_total', { success: 'true' });
            const pvcCreates = getMetric(metrics, 'k8s_pvc_creates_total', { success: 'true' });

            function formatBytes(bytes) {
                if (bytes === null || bytes === undefined) return '-';
                if (bytes === 0) return '0 B';
                const k = 1024;
                const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
                const i = Math.floor(Math.log(bytes) / Math.log(k));
                return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
            }

            return \`
                <div class="metrics-section">
                    <div class="section-title">Workspaces</div>
                    <div class="metrics-grid">
                        <div class="metric-item highlight">
                            <div class="metric-value gauge">\${formatNumber(workspacesActive)}</div>
                            <div class="metric-label">Active</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(workspacesCreated)}</div>
                            <div class="metric-label">Created</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(workspacesDeleted)}</div>
                            <div class="metric-label">Deleted</div>
                        </div>
                    </div>
                </div>

                <div class="metrics-section">
                    <div class="section-title">Storage (PVC)</div>
                    <div class="metrics-grid">
                        <div class="metric-item highlight">
                            <div class="metric-value gauge">\${formatBytes(pvcTotal)}</div>
                            <div class="metric-label">Total Storage</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(pvcCreated)}</div>
                            <div class="metric-label">PVCs Created</div>
                        </div>
                    </div>
                </div>

                <div class="metrics-section">
                    <div class="section-title">Kubernetes Operations</div>
                    <div class="metrics-grid">
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(podCreates)}</div>
                            <div class="metric-label">Pods</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(serviceCreates)}</div>
                            <div class="metric-label">Services</div>
                        </div>
                        <div class="metric-item">
                            <div class="metric-value counter">\${formatNumber(pvcCreates)}</div>
                            <div class="metric-label">PVCs</div>
                        </div>
                    </div>
                </div>

                <div class="metrics-section">
                    <div class="section-title">Gitea Integration</div>
                    <div class="metrics-list">
                        <div class="metric-row">
                            <span class="metric-row-name">API Calls (Success)</span>
                            <span class="metric-row-value success">\${formatNumber(giteaApiSuccess) || 0}</span>
                        </div>
                        <div class="metric-row">
                            <span class="metric-row-name">API Calls (Failed)</span>
                            <span class="metric-row-value error">\${formatNumber(giteaApiFail) || 0}</span>
                        </div>
                    </div>
                </div>
            \`;
        }

        // Update status badge
        function setStatus(service, status, message = null) {
            const badge = document.getElementById(service + 'Status');
            badge.className = 'status-badge';
            
            if (status === 'online') {
                badge.classList.add('status-online');
                badge.textContent = 'Online';
            } else if (status === 'offline') {
                badge.classList.add('status-offline');
                badge.textContent = message || 'Offline';
            } else {
                badge.classList.add('status-loading');
                badge.textContent = 'Loading...';
            }
        }

        // Fetch all metrics
        async function fetchAllMetrics() {
            const btn = document.getElementById('refreshBtn');
            btn.classList.add('loading');

            const [ldapResult, giteaResult, codeserverResult] = await Promise.all([
                fetchMetrics('ldap'),
                fetchMetrics('gitea'),
                fetchMetrics('codeserver')
            ]);

            // Update LDAP
            const ldapContent = document.getElementById('ldapContent');
            if (ldapResult.success) {
                setStatus('ldap', 'online');
                ldapContent.innerHTML = renderLDAPMetrics(ldapResult.data);
                document.getElementById('totalUsers').textContent = formatNumber(getMetric(ldapResult.data, 'ldap_users_total'));
                document.getElementById('totalGroups').textContent = formatNumber(getMetric(ldapResult.data, 'ldap_groups_total'));
                document.getElementById('totalDepartments').textContent = formatNumber(getMetric(ldapResult.data, 'ldap_departments_total'));
            } else {
                setStatus('ldap', 'offline', ldapResult.error);
                ldapContent.innerHTML = '<div class="error-message">Failed to fetch: ' + ldapResult.error + '</div>';
            }

            // Update Gitea
            const giteaContent = document.getElementById('giteaContent');
            if (giteaResult.success) {
                setStatus('gitea', 'online');
                giteaContent.innerHTML = renderGiteaMetrics(giteaResult.data);
                document.getElementById('totalRepos').textContent = formatNumber(getMetric(giteaResult.data, 'gitea_repos_total'));
                document.getElementById('totalPRs').textContent = formatNumber(getMetric(giteaResult.data, 'gitea_prs_open_total'));
            } else {
                setStatus('gitea', 'offline', giteaResult.error);
                giteaContent.innerHTML = '<div class="error-message">Failed to fetch: ' + giteaResult.error + '</div>';
            }

            // Update CodeServer
            const codeserverContent = document.getElementById('codeserverContent');
            if (codeserverResult.success) {
                setStatus('codeserver', 'online');
                codeserverContent.innerHTML = renderCodeServerMetrics(codeserverResult.data);
                document.getElementById('totalWorkspaces').textContent = formatNumber(getMetric(codeserverResult.data, 'codeserver_workspaces_active_total'));
            } else {
                setStatus('codeserver', 'offline', codeserverResult.error);
                codeserverContent.innerHTML = '<div class="error-message">Failed to fetch: ' + codeserverResult.error + '</div>';
            }

            document.getElementById('lastUpdate').textContent = 'Last update: ' + new Date().toLocaleString();
            btn.classList.remove('loading');
        }

        // Initial load
        fetchAllMetrics();

        // Auto-refresh every 30 seconds
        setInterval(fetchAllMetrics, 30000);
    </script>
</body>
</html>`;

// Helper function to fetch metrics from backend services
function fetchMetricsFromService(url) {
  return new Promise((resolve, reject) => {
    http.get(url, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        if (res.statusCode === 200) {
          resolve(data);
        } else {
          reject(new Error(`HTTP ${res.statusCode}`));
        }
      });
    }).on('error', reject);
  });
}

// Create HTTP server
const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://localhost:${PORT}`);
  
  // Enable CORS
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  
  if (req.method === 'OPTIONS') {
    res.writeHead(200);
    res.end();
    return;
  }
  
  // API proxy endpoints
  if (url.pathname.startsWith('/api/metrics/')) {
    const service = url.pathname.replace('/api/metrics/', '');
    const targetUrl = METRICS_SERVICES[service];
    
    if (!targetUrl) {
      res.writeHead(404, { 'Content-Type': 'text/plain' });
      res.end('Service not found');
      return;
    }
    
    try {
      const metrics = await fetchMetricsFromService(targetUrl);
      res.writeHead(200, { 'Content-Type': 'text/plain' });
      res.end(metrics);
    } catch (error) {
      res.writeHead(502, { 'Content-Type': 'text/plain' });
      res.end('Failed to fetch metrics: ' + error.message);
    }
    return;
  }
  
  // Serve dashboard HTML
  if (url.pathname === '/' || url.pathname === '/index.html') {
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(HTML_DASHBOARD);
    return;
  }
  
  res.writeHead(404, { 'Content-Type': 'text/plain' });
  res.end('Not found');
});

server.listen(PORT, () => {
  console.log(`
╔════════════════════════════════════════════════════════════╗
║         Dev Platform Metrics Dashboard Server              ║
╠════════════════════════════════════════════════════════════╣
║  Dashboard:  http://localhost:${PORT}                          ║
║                                                            ║
║  Proxying metrics from:                                    ║
║    • LDAP Manager:    http://localhost:30007/metrics       ║
║    • Gitea Service:   http://localhost:30013/metrics       ║
║    • CodeServer:      http://localhost:30014/metrics       ║
╚════════════════════════════════════════════════════════════╝
  `);
});
