import { useEffect, useState } from "react";
import { useAtom } from "jotai";
import {
  listDepartments,
  createDepartment,
  listAllDepartments,
  deleteDepartment,
} from "../../services/departmentService";

import type { Department, CreateDepartmentInput, UpdateDepartmentInput } from "../../GQL/models/department";
import { DepartmentForm } from "./department-form/department-form";
import "./departments-dashboard.css";
import { ConfirmationModal } from "../../components/confirmation-modal/confirmation-modal";
import { FilterBar } from "../../components/filter-bar/filter-bar";
import { DataTable } from "../../components/data-table/data-table";
import { useDebounce } from "../../utils/useDebounce";
import { useDelayedLoading } from "../../utils/useDelayedLoading";
import { Modal } from "../../components/modal/modal";
import { getGraphQLErrorMessage } from "../../utils/getGraphQLErrorMessage";

import {
  depsPageDataAtom,
  depsPageNumAtom,
  depsSearchAtom,
  depsDescSearchAtom,
  departmentsAtom,
} from "../../store/atoms";

export const DepartmentsDashboard = () => {
  const [departmentsPage, setDepartmentsPage] = useAtom(depsPageDataAtom);
  const [page, setPage] = useAtom(depsPageNumAtom);
  const [pageSize] = useState(8);
  const [departments, setDepartments] = useAtom(departmentsAtom);

  const [loading, setLoading] = useState(true);
  const showSkeleton = useDelayedLoading(loading);
  const [showModal, setShowModal] = useState(false);
  const [editingDepartment, setEditingDepartment] = useState<Department | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const [deleteDepartmentId, setDeleteDepartmentId] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [search, setSearch] = useAtom(depsSearchAtom);
  const [searchByDescription, setSearchByDescription] = useAtom(depsDescSearchAtom);
  const debouncedSearch = useDebounce(search);
  const debouncedSearchByMail = useDebounce(searchByDescription);



  const [formData, setFormData] = useState<CreateDepartmentInput | UpdateDepartmentInput>({
    ou: "",
    description: "",
    repositories: [],
  });

const fetchDepartments = async () => {
  try {
    setLoading(true);

    const offset = (page - 1) * pageSize;

    // Build filter for backend
    const filter: { ou?: string; description?: string } = {};
    if (debouncedSearch.trim()) {
      filter.ou = debouncedSearch.trim();
    }
    if (debouncedSearchByMail.trim()) {
      filter.description = debouncedSearchByMail.trim();
    }

    const data = await listDepartments(
      Object.keys(filter).length > 0 ? filter : undefined,
      offset,
      pageSize
    );

    setDepartmentsPage(data);
  } finally {
    setLoading(false);
  }
};




  const fetchAllDepartments = async () => {
    const data = await listAllDepartments();
    console.log(data)
    setDepartments(data);
  };

  useEffect(() => {
    fetchDepartments();
    fetchAllDepartments();
  }, []);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch, debouncedSearchByMail]);

  useEffect(() => {
    fetchDepartments();
  }, [page, debouncedSearch, debouncedSearchByMail]);




  const handleCreateClick = () => {
    setEditingDepartment(null);
    setFormData({
      ou: "",
      description: "",
      repositories: []
    });
    setShowModal(true);
  };

const handleEditClick = (department: Department) => {
  setEditingDepartment(department);

  const updateInput: UpdateDepartmentInput = {
    ou: department.ou,
    description: department.description,
    manager: department?.manager || "No manager",
    repositories: department.repositories,
  };

  setFormData(updateInput);
  setShowModal(true);
};


  const handleSubmit = async () => {
    setSubmitError(null);
    setSubmitting(true);
    try {
      if (!editingDepartment)
        await createDepartment(formData as CreateDepartmentInput);

      setShowModal(false);
      fetchDepartments();
    } catch (err) {
    setSubmitError(getGraphQLErrorMessage(err));
  } finally {
    setSubmitting(false);
  }
  };



  const handleDelete = (uid: string) => {
    setDeleteDepartmentId(uid);
  };

  const confirmDelete = async () => {
    if (!deleteDepartmentId) return;
    setDeleting(true);
    try {
      await deleteDepartment(deleteDepartmentId);
      fetchDepartments();
    } finally {
      setDeleting(false);
      setDeleteDepartmentId(null);
    }
  };

  return (
    <div className="dashboard-page">

      <div className="dashboard-page-title">
        <h1>Departments</h1>
        <p>Manage all department accounts across departments.</p>
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
                key: "searchDesc",
                type: "text",
                placeholder: "Search by description",
                value: searchByDescription,
                onChange: setSearchByDescription,
              },
            ]}
            actions={
              <button className="create-btn" onClick={handleCreateClick}>
                + Add Department
              </button>
            }
          />

 <div className="table-wrapper">
<DataTable<Department>
  data={departmentsPage?.items ?? []}
  loading={showSkeleton}
  page={page}
  pageSize={pageSize}
  total={departmentsPage?.total ?? 0}
  onPageChange={setPage}
  columns={[
    { key: "ou", header: "OU", sortable: true },
    {
      key: "description",
      header: "Name",
      sortable: true,
      render: (u) => `${u.description}`,
    },
    { key: "manager", header: "Manager", sortable: true },
    { key: "repositories", header: "Resositories", sortable: false },
  ]}
  onEdit={handleEditClick}
  onDelete={(u) => handleDelete(u.ou)}
/>

</div>

{showModal && (
  <Modal
  title={editingDepartment ? "Update Department" : "Create Department"}
  subtitle={editingDepartment ? "Modify the department details below." : "Fill in the information below to add a new department."}
  onClose={() => setShowModal(false)}
  footer={
    <>
      <button className="cancel-btn" onClick={() => setShowModal(false)} disabled={submitting}>
        Cancel
      </button>
      <button className="submit-btn" onClick={handleSubmit} disabled={submitting}>
        {submitting ? "Saving..." : editingDepartment ? "Update" : "Create"}
      </button>
    </>
  }
>

  <DepartmentForm
    editingDepartment={editingDepartment}
    formData={formData}
    setFormData={setFormData}
  />
</Modal>

)}

      {deleteDepartmentId && (
        <ConfirmationModal
          message="Are you sure you want to delete this department? This action cannot be undone."
          onCancel={() => setDeleteDepartmentId(null)}
          onConfirm={confirmDelete}
          loading={deleting}
        />
      )}

    </div>
  );
};
