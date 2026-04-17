import { useEffect, useState } from "react";
import { useAtom } from "jotai";
import {
  listUsers,
  createUser,
  updateUser,
  deleteUser,
} from "../../services/userService";
import {
  syncAllLDAPUsersToGitea,
  syncAllGiteaReposToLDAP,
} from "../../services/syncService";

import type { User, CreateUserInput, UpdateUserInput } from "../../GQL/models/user";
import type { Group } from "../../GQL/models/group";
import type { Repository } from "../../GQL/models/repository";
import { UserForm } from "./user-form/user-form";
import { listAllDepartments } from "../../services/departmentService";
import { listAllGroups } from "../../services/groupService";
import { listRepositories } from "../../services/repositoryService";

import "./users-dashboard.css";
import { ConfirmationModal } from "../../components/confirmation-modal/confirmation-modal";
import { FilterBar } from "../../components/filter-bar/filter-bar";
import { DataTable } from "../../components/data-table/data-table";
import { useDebounce } from "../../utils/useDebounce";
import { useDelayedLoading } from "../../utils/useDelayedLoading";
import { Modal } from "../../components/modal/modal";
import { getGraphQLErrorMessage } from "../../utils/getGraphQLErrorMessage";

import {
  usersPageDataAtom,
  usersPageNumAtom,
  usersSearchAtom,
  usersMailSearchAtom,
  usersDeptFilterAtom,
  departmentsAtom,
} from "../../store/atoms";

export const UsersDashboard = () => {
  const [usersPage, setUsersPage] = useAtom(usersPageDataAtom);
  const [page, setPage] = useAtom(usersPageNumAtom);
  const [pageSize] = useState(8);
  const [departments, setDepartments] = useAtom(departmentsAtom);

  const [allGroups, setAllGroups] = useState<Group[]>([]);
  const [allRepos, setAllRepos] = useState<Repository[]>([]);

  const [loading, setLoading] = useState(true);
  const showSkeleton = useDelayedLoading(loading);
  const [showModal, setShowModal] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const [deleteUserId, setDeleteUserId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [search, setSearch] = useAtom(usersSearchAtom);
  const [searchByMail, setSearchByMail] = useAtom(usersMailSearchAtom);
  const debouncedSearch = useDebounce(search);
  const debouncedSearchByMail = useDebounce(searchByMail);


  const [departmentFilter, setDepartmentFilter] = useAtom(usersDeptFilterAtom);

  const [syncingUsers, setSyncingUsers] = useState(false);
  const [syncingRepos, setSyncingRepos] = useState(false);
  const [syncMessage, setSyncMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const [formData, setFormData] = useState<CreateUserInput | UpdateUserInput>({
    uid: "",
    cn: "",
    sn: "",
    givenName: "",
    mail: "",
    department: "",
    password: "",
    repositories: [],
  });

const fetchUsers = async () => {
  try {
    setLoading(true);

    const filter: any = {};

    if (departmentFilter) {
      filter.department = departmentFilter;
    }

    if (debouncedSearch.trim()) {
      filter.cn = debouncedSearch.trim().replace(" ", ".");
    }

    if (debouncedSearchByMail.trim()) {
      filter.mail = debouncedSearchByMail.trim();
    }

    const offset = (page - 1) * pageSize;

    const data = await listUsers(
      filter,
      offset,
      pageSize
    );

    setUsersPage(data);
  } finally {
    setLoading(false);
  }
};



  const fetchAllDepartments = async () => {
    const data = await listAllDepartments();
    setDepartments(data);
  };

  const fetchAllGroups = async () => {
    try {
      const data = await listAllGroups();
      setAllGroups(data);
    } catch {
      // groups fetch is best-effort
    }
  };

  const fetchAllRepos = async () => {
    try {
      const data = await listRepositories({ limit: 1000, offset: 0 });
      setAllRepos(data.items || []);
    } catch {
      // repos fetch is best-effort
    }
  };

  useEffect(() => {
    fetchUsers();
    fetchAllDepartments();
    fetchAllGroups();
    fetchAllRepos();
  }, []);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch, debouncedSearchByMail, departmentFilter]);

  useEffect(() => {
    fetchUsers();
  }, [page, debouncedSearch, debouncedSearchByMail, departmentFilter]);

  // Compute groups for a given user uid
  const getGroupsForUser = (uid: string): Group[] => {
    return allGroups.filter((g) => g.members?.includes(uid));
  };

  const handleCreateClick = () => {
    setEditingUser(null);
    setFormData({
      uid: "",
      cn: "",
      sn: "",
      givenName: "",
      mail: "",
      department: "",
      password: "",
      repositories: [],
    });
    setSubmitError(null);
    setShowModal(true);
  };

const handleEditClick = (user: User) => {
  setEditingUser(user);

  const updateInput: UpdateUserInput = {
    uid: user.uid,
    cn: user.cn,
    sn: user.sn,
    givenName: user.givenName,
    mail: user.mail,
    department: user.department,
    repositories: user.repositories,
    password: "",
  };

  setFormData(updateInput);
  setSubmitError(null);
  setShowModal(true);
};


  const handleSubmit = async () => {
    setSubmitError(null);
    setSubmitting(true);
    try {
      if (editingUser) {
        const input = { ...formData } as UpdateUserInput;
        // Only send password if the user typed something
        if (!input.password) {
          delete input.password;
        }
        await updateUser(input);
      } else {
        const input = { ...formData } as CreateUserInput;
        // Default password to uid+123 if empty
        if (!input.password) {
          input.password = input.uid + "123";
        }
        await createUser(input);
      }
      setShowModal(false);
      fetchUsers();
    } catch (err) {
    setSubmitError(getGraphQLErrorMessage(err));
  } finally {
    setSubmitting(false);
  }
  };



  const handleDelete = (uid: string) => {
    setDeleteUserId(uid);
  };

  const confirmDelete = async () => {
    if (!deleteUserId) return;
    setDeleting(true);
    try {
      await deleteUser(deleteUserId);
      fetchUsers();
    } finally {
      setDeleting(false);
      setDeleteUserId(null);
    }
  };

  const handleSyncUsersToGitea = async () => {
    setSyncingUsers(true);
    setSyncMessage(null);
    try {
      const result = await syncAllLDAPUsersToGitea();
      setSyncMessage({ type: "success", text: `Synced ${result.length} LDAP users to Gitea` });
    } catch (err) {
      setSyncMessage({ type: "error", text: getGraphQLErrorMessage(err) });
    } finally {
      setSyncingUsers(false);
    }
  };

  const handleSyncReposToLDAP = async () => {
    setSyncingRepos(true);
    setSyncMessage(null);
    try {
      const results = await syncAllGiteaReposToLDAP();
      const totalRepos = results.reduce((sum, r) => sum + r.reposCount, 0);
      setSyncMessage({ type: "success", text: `Synced ${totalRepos} repos for ${results.length} users (Gitea → LDAP)` });
      fetchUsers();
    } catch (err) {
      setSyncMessage({ type: "error", text: getGraphQLErrorMessage(err) });
    } finally {
      setSyncingRepos(false);
    }
  };

  return (
    <div className="dashboard-page">

      <div className="dashboard-page-title">
        <h1>Users</h1>
        <p>Manage all user accounts across departments.</p>
      </div>

        <FilterBar
            filters={[
              {
                key: "search",
                type: "text",
                placeholder: "Search by name",
                value: search,
                onChange: setSearch,
              },
              {
                key: "searchMail",
                type: "text",
                placeholder: "Search by email...",
                value: searchByMail,
                onChange: setSearchByMail,
              },
              {
                key: "department",
                type: "select",
                placeholder: "All Departments",
                value: departmentFilter,
                options: departments.map((d) => ({
                  label: d.ou,
                  value: d.ou,
                })),
                onChange: setDepartmentFilter,
              },
            ]}
            actions={
              <div className="actions-group">
                <button
                  className="sync-btn"
                  onClick={handleSyncUsersToGitea}
                  disabled={syncingUsers}
                >
                  {syncingUsers ? "Syncing..." : "LDAP → Gitea"}
                </button>
                <button
                  className="sync-btn sync-btn-reverse"
                  onClick={handleSyncReposToLDAP}
                  disabled={syncingRepos}
                >
                  {syncingRepos ? "Syncing..." : "Gitea → LDAP"}
                </button>
                <button className="create-btn" onClick={handleCreateClick}>
                  + Add User
                </button>
              </div>
            }
          />

      {syncMessage && (
        <div className={`sync-banner sync-banner-${syncMessage.type}`}>
          <span>{syncMessage.text}</span>
          <button className="sync-banner-close" onClick={() => setSyncMessage(null)}>&times;</button>
        </div>
      )}

 <div className="table-wrapper">
<DataTable<User>
  data={usersPage?.items ?? []}
  loading={showSkeleton}
  page={page}
  pageSize={pageSize}
  total={usersPage?.total ?? 0}
  onPageChange={setPage}
  columns={[
    { key: "uid", header: "UID", sortable: true },
    {
      key: "givenName",
      header: "Name",
      sortable: true,
      render: (u) => `${u.givenName} ${u.sn}`,
    },
    { key: "mail", header: "Email", sortable: true },
    { key: "department", header: "Department", sortable: true },
    {
      key: "repositories",
      header: "Repos",
      render: (u) => (
        <span className="badge">{u.repositories?.length || 0} repos</span>
      ),
    },
    {
      key: "groups",
      header: "Groups",
      render: (u) => {
        const groups = getGroupsForUser(u.uid);
        return groups.length > 0
          ? groups.map((g) => g.cn).join(", ")
          : "—";
      },
    },
  ]}
  onEdit={handleEditClick}
  onDelete={(u) => handleDelete(u.uid)}
/>

</div>

{showModal && (
  <Modal
  title={editingUser ? "Update User" : "Create User"}
  subtitle={editingUser ? "Modify the user details below." : "Fill in the information below to add a new user."}
  onClose={() => setShowModal(false)}
  footer={
    <>
      <button className="cancel-btn" onClick={() => setShowModal(false)} disabled={submitting}>
        Cancel
      </button>
      <button className="submit-btn" onClick={handleSubmit} disabled={submitting}>
        {submitting ? "Saving..." : editingUser ? "Update" : "Create"}
      </button>
    </>
  }
>
  {submitError && <div className="form-error">{submitError}</div>}

  <UserForm
    editingUser={editingUser}
    formData={formData}
    setFormData={setFormData}
    departments={departments}
    availableRepos={allRepos}
    userGroups={editingUser ? getGroupsForUser(editingUser.uid) : []}
  />
</Modal>

)}

      {deleteUserId && (
        <ConfirmationModal
          message="Are you sure you want to delete this user? This action cannot be undone."
          onCancel={() => setDeleteUserId(null)}
          onConfirm={confirmDelete}
          loading={deleting}
        />
      )}

    </div>
  );
};
