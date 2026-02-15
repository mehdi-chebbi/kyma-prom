import { useEffect, useState, useRef, useCallback } from "react";
import {
  listMyCodeServers,
  stopCodeServer,
  startCodeServer,
  deleteCodeServer,
  syncRepository,
  getCodeServerLogs,
  getInstanceStats,
} from "../../services/codeserverService";
import type { CodeServerInstance, InstanceStats } from "../../services/codeserverService";
import { FilterBar } from "../../components/filter-bar/filter-bar";
import WidgetCard from "../../components/widget-card/widget-card";
import { Modal } from "../../components/modal/modal";
import { ConfirmationModal } from "../../components/confirmation-modal/confirmation-modal";
import { useDebounce } from "../../utils/useDebounce";
import { useDelayedLoading } from "../../utils/useDelayedLoading";
import "./workspaces.css";

const REFRESH_INTERVAL = 30_000;

const STATUS_LABELS: Record<string, string> = {
  RUNNING: "Running",
  STOPPED: "Stopped",
  PENDING: "Pending",
  STARTING: "Starting",
  STOPPING: "Stopping",
  ERROR: "Error",
};

export default function Workspaces() {
  const [instances, setInstances] = useState<CodeServerInstance[]>([]);
  const [stats, setStats] = useState<InstanceStats | null>(null);
  const [loading, setLoading] = useState(true);
  const showSkeleton = useDelayedLoading(loading);

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search);

  const [statusFilter, setStatusFilter] = useState("");

  // Action loading states
  const [actionLoading, setActionLoading] = useState<Record<string, string>>({});

  // Delete confirmation
  const [deleteTarget, setDeleteTarget] = useState<CodeServerInstance | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Logs viewer
  const [logsTarget, setLogsTarget] = useState<CodeServerInstance | null>(null);
  const [logs, setLogs] = useState("");
  const [logsLoading, setLogsLoading] = useState(false);

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = useCallback(async () => {
    try {
      const [instanceList, instanceStats] = await Promise.all([
        listMyCodeServers(),
        getInstanceStats(),
      ]);
      setInstances(instanceList);
      setStats(instanceStats);
    } catch {
      // silently fail on auto-refresh
    } finally {
      setLoading(false);
    }
  }, []);

  // Initial load + auto-refresh
  useEffect(() => {
    fetchData();
    timerRef.current = setInterval(fetchData, REFRESH_INTERVAL);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [fetchData]);

  // Filter instances
  const filtered = instances.filter((inst) => {
    const matchesSearch =
      !debouncedSearch ||
      `${inst.repoOwner}/${inst.repoName}`
        .toLowerCase()
        .includes(debouncedSearch.toLowerCase());
    const matchesStatus = !statusFilter || inst.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  // ─── Actions ──────────────────────────────────────────────

  const setAction = (id: string, action: string) =>
    setActionLoading((prev) => ({ ...prev, [id]: action }));
  const clearAction = (id: string) =>
    setActionLoading((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });

  const handleStop = async (inst: CodeServerInstance) => {
    setAction(inst.id, "stopping");
    try {
      await stopCodeServer(inst.id);
      await fetchData();
    } finally {
      clearAction(inst.id);
    }
  };

  const handleStart = async (inst: CodeServerInstance) => {
    setAction(inst.id, "starting");
    try {
      await startCodeServer(inst.id);
      await fetchData();
    } finally {
      clearAction(inst.id);
    }
  };

  const handleSync = async (inst: CodeServerInstance) => {
    setAction(inst.id, "syncing");
    try {
      await syncRepository(inst.id);
      await fetchData();
    } finally {
      clearAction(inst.id);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteCodeServer(deleteTarget.id);
      setDeleteTarget(null);
      await fetchData();
    } finally {
      setDeleting(false);
    }
  };

  const handleViewLogs = async (inst: CodeServerInstance) => {
    setLogsTarget(inst);
    setLogsLoading(true);
    try {
      const content = await getCodeServerLogs(inst.id, 200);
      setLogs(content);
    } catch {
      setLogs("Failed to fetch logs.");
    } finally {
      setLogsLoading(false);
    }
  };

  return (
    <div className="workspaces-page">
      <div className="workspaces-topbar">
        <h2 className="workspaces-heading">Workspaces</h2>
        <div className="workspaces-topbar-right">
          <FilterBar
            filters={[
              {
                key: "search",
                type: "text",
                placeholder: "Search workspaces\u2026",
                value: search,
                onChange: setSearch,
              },
              {
                key: "status",
                type: "select",
                placeholder: "All statuses",
                value: statusFilter,
                onChange: setStatusFilter,
                options: [
                  { value: "", label: "All statuses" },
                  { value: "RUNNING", label: "Running" },
                  { value: "STOPPED", label: "Stopped" },
                  { value: "PENDING", label: "Pending" },
                  { value: "ERROR", label: "Error" },
                ],
              },
            ]}
          />
        </div>
      </div>

      {/* Stats widgets */}
      {stats && (
        <div className="workspaces-stats">
          <WidgetCard title="Total">{stats.totalInstances}</WidgetCard>
          <WidgetCard title="Running">{stats.runningInstances}</WidgetCard>
          <WidgetCard title="Stopped">{stats.stoppedInstances}</WidgetCard>
          <WidgetCard title="Pending">{stats.pendingInstances}</WidgetCard>
          <WidgetCard title="Storage">{stats.totalStorageUsed || "0 B"}</WidgetCard>
        </div>
      )}

      {/* Instance cards */}
      <div className="workspaces-grid">
        {showSkeleton && instances.length === 0 && (
          <p className="widget-text">Loading workspaces...</p>
        )}

        {!loading && filtered.length === 0 && (
          <p className="widget-text">No workspaces found.</p>
        )}

        {filtered.map((inst) => {
          const busy = actionLoading[inst.id];
          const repoFullName = `${inst.repoOwner}/${inst.repoName}`;
          return (
            <div key={inst.id} className="workspace-card">
              <div className="workspace-card-header">
                <div className="workspace-card-title">
                  <span className="workspace-repo">{repoFullName}</span>
                  <span className={`workspace-status-badge status-${inst.status.toLowerCase()}`}>
                    {STATUS_LABELS[inst.status] || inst.status}
                  </span>
                </div>
                <span className="workspace-date">
                  {new Date(inst.createdAt).toLocaleDateString()}
                </span>
              </div>

              {inst.errorMessage && (
                <p className="workspace-error">{inst.errorMessage}</p>
              )}

              {inst.storageUsed && (
                <span className="workspace-storage">Storage: {inst.storageUsed}</span>
              )}

              <div className="workspace-actions">
                {inst.status === "RUNNING" && inst.url && (
                  <a
                    href={inst.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="ws-btn ws-btn-primary"
                  >
                    Open IDE
                  </a>
                )}

                {inst.status === "RUNNING" && (
                  <>
                    <button
                      className="ws-btn ws-btn-secondary"
                      disabled={!!busy}
                      onClick={() => handleSync(inst)}
                    >
                      {busy === "syncing" ? "Syncing\u2026" : "Sync"}
                    </button>
                    <button
                      className="ws-btn ws-btn-warning"
                      disabled={!!busy}
                      onClick={() => handleStop(inst)}
                    >
                      {busy === "stopping" ? "Stopping\u2026" : "Stop"}
                    </button>
                  </>
                )}

                {inst.status === "STOPPED" && (
                  <button
                    className="ws-btn ws-btn-primary"
                    disabled={!!busy}
                    onClick={() => handleStart(inst)}
                  >
                    {busy === "starting" ? "Starting\u2026" : "Start"}
                  </button>
                )}

                <button
                  className="ws-btn ws-btn-secondary"
                  onClick={() => handleViewLogs(inst)}
                >
                  Logs
                </button>

                <button
                  className="ws-btn ws-btn-danger"
                  disabled={!!busy}
                  onClick={() => setDeleteTarget(inst)}
                >
                  Delete
                </button>
              </div>
            </div>
          );
        })}
      </div>

      {/* Delete confirmation */}
      {deleteTarget && (
        <ConfirmationModal
          message={`Are you sure you want to delete the workspace for "${deleteTarget.repoOwner}/${deleteTarget.repoName}"? This will remove all workspace data.`}
          onCancel={() => setDeleteTarget(null)}
          onConfirm={handleDeleteConfirm}
          loading={deleting}
        />
      )}

      {/* Logs viewer */}
      {logsTarget && (
        <Modal
          title="Workspace Logs"
          subtitle={`${logsTarget.repoOwner}/${logsTarget.repoName}`}
          onClose={() => {
            setLogsTarget(null);
            setLogs("");
          }}
          width={720}
        >
          {logsLoading ? (
            <p className="widget-text">Loading logs...</p>
          ) : (
            <pre className="workspace-logs-content">{logs || "No logs available."}</pre>
          )}
        </Modal>
      )}
    </div>
  );
}
