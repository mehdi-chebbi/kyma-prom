import { useEffect, useState, useRef } from "react";
import { ProjectCard } from "../../components/project-card/project-card";
import WidgetCard from "../../components/widget-card/widget-card";
import {
  myRepositories,
  searchRepositories,
  updateRepository,
  createRepository,
  deleteRepository,
  listBranches,
} from "../../services/repositoryService";
import type { Branch } from "../../services/repositoryService";
import {
  provisionCodeServer,
  listMyCodeServers,
} from "../../services/codeserverService";
import type { CodeServerInstance } from "../../services/codeserverService";
import type {
  CreateRepositoryInput,
  Repository,
  RepositoryPage,
} from "../../GQL/models/repository";
import "./projects.css";
import { useDebounce } from "../../utils/useDebounce";
import { FilterBar } from "../../components/filter-bar/filter-bar";
import { useDelayedLoading } from "../../utils/useDelayedLoading";
import { Modal } from "../../components/modal/modal";
import { RepositoryForm } from "./repository-form/repository-form";
import { ConfirmationModal } from "../../components/confirmation-modal/confirmation-modal";
import { MigrateRepositoryModal } from "./migrate-modal/migrate-modal";

const PAGE_SIZE = 5;

export default function Projects() {
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search);
  const [offset, setOffset] = useState(0);
  const [hasMore, setHasMore] = useState(true);
  const [loading, setLoading] = useState(false);
  const showLoadingMessage = useDelayedLoading(loading);
  const loaderRef = useRef<HTMLDivElement | null>(null);

  const [showModal, setShowModal] = useState(false);
  const [editingRepo, setEditingRepo] = useState<Repository | null>(null);
  const [formData, setFormData] = useState<
    Partial<Repository & CreateRepositoryInput>
  >({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const [migrateModal, setMigrateModal] = useState(false);

  const [deleteRepo, setDeleteRepo] = useState<Repository | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [provisioningId, setProvisioningId] = useState<string | number | null>(
    null,
  );

  /* ---------------- Branch Picker State ---------------- */
  const [branchPickerRepo, setBranchPickerRepo] = useState<Repository | null>(
    null,
  );
  const [branches, setBranches] = useState<Branch[]>([]);
  const [selectedBranch, setSelectedBranch] = useState("");
  const [loadingBranches, setLoadingBranches] = useState(false);

  /* ---------------- Code Server Instances ---------------- */
  const [codeServers, setCodeServers] = useState<
    Map<string, CodeServerInstance>
  >(new Map());

  const fetchCodeServers = async () => {
    try {
      const instances = await listMyCodeServers();
      const map = new Map<string, CodeServerInstance>();
      instances.forEach((inst) => {
        map.set(`${inst.repoOwner}/${inst.repoName}`, inst);
      });
      setCodeServers(map);
    } catch {
      // ignore — code server may not be available
    }
  };

  useEffect(() => {
    fetchCodeServers();
  }, []);

  /* ---------------- Fetch repositories ---------------- */
  const fetchRepositories = async (reset = false) => {
    if (loading) return;

    setLoading(true);

    try {
      const currentOffset = reset ? 0 : offset;

      const res: RepositoryPage =
        debouncedSearch.trim().length > 0
          ? await searchRepositories(debouncedSearch, {
              limit: PAGE_SIZE,
              offset: currentOffset,
            })
          : await myRepositories({
              limit: PAGE_SIZE,
              offset: currentOffset,
            });

      setRepositories((prev) => {
        const merged = reset ? res.items : [...prev, ...res.items];

        const uniqueMap = new Map<string | number, Repository>();
        merged.forEach((repo) => uniqueMap.set(repo.id, repo));
        const uniqueRepos = Array.from(uniqueMap.values());

        return uniqueRepos;
      });

      setOffset(currentOffset + res.items.length);
      setHasMore(res.hasMore);
    } finally {
      setLoading(false);
    }
  };

  /* ---------------- Reset on search ---------------- */
  useEffect(() => {
    setOffset(0);
    setHasMore(true);
    fetchRepositories(true);
  }, [debouncedSearch]);

  /* ---------------- Infinite scroll ---------------- */
  useEffect(() => {
    const onScroll = () => {
      if (!hasMore || loading) return;
      if (
        window.innerHeight + window.scrollY >=
        document.body.offsetHeight - 10
      ) {
        fetchRepositories();
      }
    };

    window.addEventListener("scroll", onScroll);
    return () => window.removeEventListener("scroll", onScroll);
  }, [hasMore, loading, debouncedSearch]);

  /* ---------------- Form Handlers ---------------- */
  const handleEditClick = async (repo: Repository) => {
    setEditingRepo(repo);
    console.log(repo);
    setLoadingBranches(true);
    try {
      const branchList = await listBranches(repo.owner.login, repo.name);
      setBranches(branchList);
    } catch {
      setBranches([{ name: repo.defaultBranch || "main" }]);
    } finally {
      setLoadingBranches(false);
      setFormData({
        name: repo.name,
        description: repo.description,
        defaultBranch: repo.defaultBranch,
        private: repo.private,
      });
      setSubmitError(null);
      setShowModal(true);
    }
  };

  const handleCreateClick = () => {
    setEditingRepo(null);
    setFormData({ name: "", description: "", private: false });
    setSubmitError(null);
    setShowModal(true);
  };

  const handleMigrateRepo = () => {
    setMigrateModal(true);
  };
  const handleSubmit = async () => {
    if (!formData.name) {
      setSubmitError("Repository name is required");
      return;
    }

    setSubmitting(true);
    setSubmitError(null);

    try {
      if (editingRepo) {
        await updateRepository(
          editingRepo.owner.login,
          formData.name,
          formData.description,
          !!formData.private,
          formData.defaultBranch || "",
        );
      } else {
        await createRepository({
          name: formData.name,
          description: formData.description,
          private: !!formData.private,
          autoInit: !!formData.autoInit,
          gitignores: formData.gitignores,
          license: formData.license,
          defaultBranch: formData.defaultBranch || "main", // fallback to 'main'
        });
      }
      setShowModal(false);
      fetchRepositories(true);
    } catch (err: any) {
      setSubmitError(err.message || "Failed to save repository");
    } finally {
      setSubmitting(false);
    }
  };

  const handleOpenCode = async (repo: Repository) => {
    setBranchPickerRepo(repo);
    setSelectedBranch(repo.defaultBranch || "main");
    setLoadingBranches(true);
    try {
      const branchList = await listBranches(repo.owner.login, repo.name);
      setBranches(branchList);
    } catch {
      setBranches([{ name: repo.defaultBranch || "main" }]);
    } finally {
      setLoadingBranches(false);
    }
  };

  const confirmOpenCode = async () => {
    if (!branchPickerRepo) return;
    const repo = branchPickerRepo;
    setBranchPickerRepo(null);
    setProvisioningId(repo.id);
    try {
      const result = await provisionCodeServer(
        repo.owner.login,
        repo.name,
        selectedBranch,
      );
      if (result.instance.url) {
        window.open(result.instance.url, "_blank");
      }
      fetchCodeServers();
    } catch (err: any) {
      alert(err.message || "Failed to start code server");
    } finally {
      setProvisioningId(null);
    }
  };

  /* ---------------- Delete Handlers ---------------- */
  const handleDeleteClick = (repo: Repository) => {
    setDeleteRepo(repo);
  };

  const confirmDelete = async () => {
    if (!deleteRepo) return;
    setDeleting(true);
    try {
      await deleteRepository(deleteRepo.owner.login, deleteRepo.name);
      setDeleteRepo(null);
      fetchRepositories(true);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="projects-page">
      <div className="projects-topbar">
        <div />
        <div className="projects-topbar-right">
          <FilterBar
            filters={[
              {
                key: "search",
                type: "text",
                placeholder: "Search repositories…",
                value: search,
                onChange: setSearch,
              },
            ]}
            actions={[
              <button className="projects-add-btn" onClick={handleCreateClick}>
                Add New
              </button>,
              <button className="projects-add-btn" onClick={handleMigrateRepo}>
                Migrate
              </button>,
            ]}
          />
        </div>
      </div>

      <div className="projects-grid">
        <div className="projects-column">
          <WidgetCard title="Last 30 days" actionLabel="Upgrade">
            <ul className="usage-list">
              <li>
                <span>Edge Requests</span>
                <span>15 / 1M</span>
              </li>
              <li>
                <span>Edge Request CPU Duration</span>
                <span>0s / 1h</span>
              </li>
              <li>
                <span>Fast Data Transfer</span>
                <span>166.7 KB / 100 GB</span>
              </li>
            </ul>
          </WidgetCard>
          <WidgetCard
            title="Alerts"
            actionLabel="Upgrade to Observability Plus"
          >
            <p className="widget-text">
              Automatically monitor your projects for anomalies.
            </p>
          </WidgetCard>
          <WidgetCard title="Recent Previews">
            <p className="widget-text">
              Your recent deployments will appear here.
            </p>
          </WidgetCard>
        </div>

        <div className="projects-column">
          {repositories.map((repo) => {
            const csInstance = codeServers.get(repo.fullName);
            return (
              <ProjectCard
                key={repo.id}
                icon="📦"
                title={repo.name}
                repoName={repo.fullName}
                repoLink={repo.htmlUrl}
                dateRange={new Date(repo.updatedAt).toLocaleDateString()}
                branch={repo.defaultBranch}
                status={repo.private ? "🔒" : "◉"}
                stars={repo.stars}
                forks={repo.forks}
                onOpenCode={() => handleOpenCode(repo)}
                openingCode={provisioningId === repo.id}
                onEdit={() => handleEditClick(repo)}
                onDelete={() => handleDeleteClick(repo)}
                codeServerStatus={
                  (csInstance?.status as
                    | "RUNNING"
                    | "STOPPED"
                    | "PENDING"
                    | "STARTING"
                    | undefined) ?? null
                }
                codeServerUrl={csInstance?.url}
              />
            );
          })}

          {showLoadingMessage && <p className="widget-text">Loading…</p>}
          {!showLoadingMessage && repositories.length === 0 && (
            <p className="widget-text">No repositories found.</p>
          )}

          <div ref={loaderRef} style={{ height: 1 }} />
        </div>
      </div>

      {showModal && (
        <Modal
          title={editingRepo ? "Update Repository" : "Create Repository"}
          subtitle={
            editingRepo
              ? "Modify repository details below."
              : "Fill in the information below to add a new repository."
          }
          onClose={() => setShowModal(false)}
          footer={
            <>
              <button
                className="cancel-btn"
                onClick={() => setShowModal(false)}
                disabled={submitting}
              >
                Cancel
              </button>
              <button
                className="submit-btn"
                onClick={handleSubmit}
                disabled={submitting}
              >
                {submitting ? "Saving..." : editingRepo ? "Update" : "Create"}
              </button>
            </>
          }
        >
          {submitError && <div className="form-error">{submitError}</div>}
          <RepositoryForm
            formData={formData}
            setFormData={setFormData}
            edit={!!editingRepo}
            loadingBranches={loadingBranches}
            branches={branches}
          />
        </Modal>
      )}

      {deleteRepo && (
        <ConfirmationModal
          message={`Are you sure you want to delete repository "${deleteRepo.fullName}"? This action cannot be undone.`}
          onCancel={() => setDeleteRepo(null)}
          onConfirm={confirmDelete}
          loading={deleting}
        />
      )}

      {migrateModal && (
        <MigrateRepositoryModal
          onClose={() => setMigrateModal(false)}
          isOpen={migrateModal}
          onSuccess={() => fetchRepositories(true)}
        />
      )}

      {branchPickerRepo && (
        <Modal
          title="Open in Code Server"
          subtitle={`Select a branch for ${branchPickerRepo.fullName}`}
          onClose={() => setBranchPickerRepo(null)}
          width={400}
          footer={
            <>
              <button
                className="cancel-btn"
                onClick={() => setBranchPickerRepo(null)}
              >
                Cancel
              </button>
              <button
                className="submit-btn"
                onClick={confirmOpenCode}
                disabled={loadingBranches || !selectedBranch}
              >
                Open
              </button>
            </>
          }
        >
          {loadingBranches ? (
            <p className="widget-text">Loading branches...</p>
          ) : (
            <div className="form-group">
              <label htmlFor="branch-select">Branch</label>
              <select
                id="branch-select"
                className="branch-select"
                value={selectedBranch}
                onChange={(e) => setSelectedBranch(e.target.value)}
              >
                {branches.map((b) => (
                  <option key={b.name} value={b.name}>
                    {b.name}
                  </option>
                ))}
              </select>
            </div>
          )}
        </Modal>
      )}
    </div>
  );
}
