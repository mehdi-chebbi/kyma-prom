import { useEffect, useState } from "react";
import type { Repository } from "../../../GQL/models/repository";
import { listBranches, type Branch } from "../../../services/repositoryService";
import { CustomSelect } from "../../../components/custom-select/custom-select";
import "./repo-form.css";

export interface RepoFormData {
  name: string;
  description: string;
  private: boolean;
  defaultBranch: string;
  autoInit: boolean;
  gitignores: string;
  license: string;
}

export const EMPTY_REPO_FORM: RepoFormData = {
  name: "",
  description: "",
  private: false,
  defaultBranch: "main",
  autoInit: true,
  gitignores: "",
  license: "",
};

const GITIGNORE_OPTIONS = [
  { label: "None", value: "" },
  { label: "Go", value: "Go" },
  { label: "Node", value: "Node" },
  { label: "Python", value: "Python" },
  { label: "React", value: "React" },
];

const LICENSE_OPTIONS = [
  { label: "None", value: "" },
  { label: "MIT", value: "MIT" },
  { label: "Apache-2.0", value: "Apache-2.0" },
  { label: "GPL-3.0", value: "GPL-3.0" },
];

interface Props {
  editingRepo: Repository | null;
  formData: RepoFormData;
  setFormData: (d: RepoFormData) => void;
}

export const RepoForm = ({ editingRepo, formData, setFormData }: Props) => {
  const [branches, setBranches] = useState<Branch[]>([]);

  useEffect(() => {
    if (editingRepo) {
      listBranches(editingRepo.owner.login, editingRepo.name)
        .then(setBranches)
        .catch(() => setBranches([]));
    }
  }, [editingRepo]);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) => {
    const { name, type, value } = e.target;
    const checked = (e.target as HTMLInputElement).checked;
    setFormData({
      ...formData,
      [name]: type === "checkbox" ? checked : value,
    });
  };

  return (
    <>
      <div className="form-group">
        <label htmlFor="name">Name</label>
        <input
          name="name"
          value={formData.name}
          onChange={handleChange}
          disabled={!!editingRepo}
          placeholder="my-repository"
        />
      </div>

      <div className="form-group">
        <label htmlFor="description">Description</label>
        <textarea
          name="description"
          value={formData.description}
          onChange={handleChange}
          rows={3}
          placeholder="A short description of this repository"
        />
      </div>

      <div className="form-group checkbox-group">
        <label>
          <input
            type="checkbox"
            name="private"
            checked={formData.private}
            onChange={handleChange}
          />
          Private repository
        </label>
      </div>

      <div className="form-group">
        <label htmlFor="defaultBranch">Default Branch</label>
        {editingRepo && branches.length > 0 ? (
          <CustomSelect
            value={formData.defaultBranch}
            options={branches.map((b) => ({ label: b.name, value: b.name }))}
            onChange={(v) => setFormData({ ...formData, defaultBranch: v })}
          />
        ) : (
          <input
            name="defaultBranch"
            value={formData.defaultBranch}
            onChange={handleChange}
            placeholder="main"
            disabled={!!editingRepo}
          />
        )}
      </div>

      {!editingRepo && (
        <>
          <div className="form-group checkbox-group">
            <label>
              <input
                type="checkbox"
                name="autoInit"
                checked={formData.autoInit}
                onChange={handleChange}
              />
              Initialize with README
            </label>
          </div>

          <div className="form-group">
            <label>.gitignore template</label>
            <CustomSelect
              value={formData.gitignores}
              options={GITIGNORE_OPTIONS}
              placeholder="Select a template"
              onChange={(v) => setFormData({ ...formData, gitignores: v })}
            />
          </div>

          <div className="form-group">
            <label>License</label>
            <CustomSelect
              value={formData.license}
              options={LICENSE_OPTIONS}
              placeholder="Select a license"
              onChange={(v) => setFormData({ ...formData, license: v })}
            />
          </div>
        </>
      )}
    </>
  );
};
