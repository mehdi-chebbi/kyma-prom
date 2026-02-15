import { useEffect, useState } from "react";
import type { Repository, RepositoryPage } from "../../GQL/models/repository";
import {
  listRepositories,
  searchRepositories,
  createRepository,
  updateRepository,
  deleteRepository,
} from "../../services/repositoryService";
import { RepoForm, EMPTY_REPO_FORM, type RepoFormData } from "./repo-form/repo-form";
import { MigrateRepositoryModal } from "../projects/migrate-modal/migrate-modal";

import { DataTable } from "../../components/data-table/data-table";
import { FilterBar } from "../../components/filter-bar/filter-bar";
import { Modal } from "../../components/modal/modal";
import { ConfirmationModal } from "../../components/confirmation-modal/confirmation-modal";
import { useDebounce } from "../../utils/useDebounce";
import { useDelayedLoading } from "../../utils/useDelayedLoading";
import { getGraphQLErrorMessage } from "../../utils/getGraphQLErrorMessage";

import "./repos-dashboard.css";

type RepoRow = Repository & { dn: string };

const toRow = (r: Repository): RepoRow => ({ ...r, dn: r.fullName });

export const ReposDashboard = () => {
  const [reposPage, setReposPage] = useState<RepositoryPage | null>(null);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(8);

  const [loading, setLoading] = useState(true);
  const showSkeleton = useDelayedLoading(loading);

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search);
  const [visibilityFilter, setVisibilityFilter] = useState("");

  const [showModal, setShowModal] = useState(false);
  const [editingRepo, setEditingRepo] = useState<Repository | null>(null);
  const [formData, setFormData] = useState<RepoFormData>(EMPTY_REPO_FORM);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const [deleteRepo, setDeleteRepo] = useState<Repository | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [showMigrate, setShowMigrate] = useState(false);

  const fetchRepos = async () => {
    try {
      setLoading(true);
      const offset = (page - 1) * pageSize;

      let data: RepositoryPage;

      if (debouncedSearch.trim()) {
        data = await searchRepositories(debouncedSearch.trim(), {
          limit: pageSize,
          offset,
        });
      } else {
        data = await listRepositories({ limit: pageSize, offset });
      }

      if (visibilityFilter) {
        const isPrivate = visibilityFilter === "private";
        data = {
          ...data,
          items: data.items.filter((r) => r.private === isPrivate),
          total: data.items.filter((r) => r.private === isPrivate).length,
        };
      }

      setReposPage(data);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch, visibilityFilter]);

  useEffect(() => {
    fetchRepos();
  }, [page, debouncedSearch, visibilityFilter]);

  const handleCreateClick = () => {
    setEditingRepo(null);
    setFormData(EMPTY_REPO_FORM);
    setSubmitError(null);
    setShowModal(true);
  };

  const handleEditClick = (row: RepoRow) => {
    setEditingRepo(row);
    setFormData({
      name: row.name,
      description: row.description ?? "",
      private: row.private,
      defaultBranch: row.defaultBranch,
      autoInit: false,
      gitignores: "",
      license: "",
    });
    setSubmitError(null);
    setShowModal(true);
  };

  const handleSubmit = async () => {
    setSubmitError(null);
    setSubmitting(true);
    try {
      if (editingRepo) {
        await updateRepository(
          editingRepo.owner.login,
          editingRepo.name,
          formData.description || undefined,
          formData.private,
          formData.defaultBranch,
        );
      } else {
        await createRepository({
          name: formData.name,
          description: formData.description || undefined,
          private: formData.private,
          defaultBranch: formData.defaultBranch || "main",
          autoInit: formData.autoInit,
          gitignores: formData.gitignores || undefined,
          license: formData.license || undefined,
        });
      }
      setShowModal(false);
      fetchRepos();
    } catch (err) {
      setSubmitError(getGraphQLErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteClick = (row: RepoRow) => {
    setDeleteRepo(row);
  };

  const confirmDelete = async () => {
    if (!deleteRepo) return;
    setDeleting(true);
    try {
      await deleteRepository(deleteRepo.owner.login, deleteRepo.name);
      fetchRepos();
    } finally {
      setDeleting(false);
      setDeleteRepo(null);
    }
  };

  const rows: RepoRow[] = (reposPage?.items ?? []).map(toRow);

  return (
    <div className="dashboard-page">
      <div className="dashboard-page-title">
        <h1>Repositories</h1>
        <p>Manage all repositories across the platform.</p>
      </div>

      <FilterBar
        filters={[
          {
            key: "search",
            type: "text",
            placeholder: "Search by name...",
            value: search,
            onChange: setSearch,
          },
          {
            key: "visibility",
            type: "select",
            placeholder: "All Visibility",
            value: visibilityFilter,
            options: [
              { label: "Public", value: "public" },
              { label: "Private", value: "private" },
            ],
            onChange: setVisibilityFilter,
          },
        ]}
        actions={
          <div className="actions-group">
            <button className="sync-btn" onClick={() => setShowMigrate(true)}>
              Migrate
            </button>
            <button className="create-btn" onClick={handleCreateClick}>
              + Create Repo
            </button>
          </div>
        }
      />

      <div className="table-wrapper">
        <DataTable<RepoRow>
          data={rows}
          loading={showSkeleton}
          page={page}
          pageSize={pageSize}
          total={reposPage?.total ?? 0}
          onPageChange={setPage}
          columns={[
            {
              key: "fullName",
              header: "Name",
              sortable: true,
            },
            {
              key: "owner",
              header: "Owner",
              render: (r) => r.owner.login,
            },
            {
              key: "private",
              header: "Visibility",
              render: (r) => (
                <span className={`visibility-badge ${r.private ? "private" : "public"}`}>
                  {r.private ? "Private" : "Public"}
                </span>
              ),
            },
            {
              key: "stars",
              header: "Stars",
              sortable: true,
            },
            {
              key: "forks",
              header: "Forks",
              sortable: true,
            },
            {
              key: "defaultBranch",
              header: "Branch",
            },
          ]}
          onEdit={handleEditClick}
          onDelete={handleDeleteClick}
        />
      </div>

      {showModal && (
        <Modal
          title={editingRepo ? "Update Repository" : "Create Repository"}
          subtitle={
            editingRepo
              ? "Modify the repository settings below."
              : "Fill in the information below to create a new repository."
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
          <RepoForm
            editingRepo={editingRepo}
            formData={formData}
            setFormData={setFormData}
          />
        </Modal>
      )}

      {deleteRepo && (
        <ConfirmationModal
          message={`Are you sure you want to delete "${deleteRepo.fullName}"? This action cannot be undone.`}
          onCancel={() => setDeleteRepo(null)}
          onConfirm={confirmDelete}
          loading={deleting}
        />
      )}

      <MigrateRepositoryModal
        isOpen={showMigrate}
        onClose={() => setShowMigrate(false)}
        onSuccess={fetchRepos}
      />
    </div>
  );
};
