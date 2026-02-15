import { useEffect } from "react";
import type { CreateUserInput, UpdateUserInput, User } from "../../../GQL/models/user";
import type { Department } from "../../../GQL/models/department";
import type { Group } from "../../../GQL/models/group";
import type { Repository } from "../../../GQL/models/repository";
import { CustomSelect } from "../../../components/custom-select/custom-select";
import "./user-form.css";

interface Props {
  editingUser: User | null;
  formData: CreateUserInput | UpdateUserInput;
  setFormData: (d: any) => void;
  departments: Department[];
  availableRepos: Repository[];
  userGroups: Group[];
}

export const UserForm = ({
  editingUser,
  formData,
  setFormData,
  departments,
  availableRepos,
  userGroups,
}: Props) => {
  useEffect(() => {
    if (!editingUser) {
      const first = formData.givenName?.trim().toLowerCase();
      const last = formData.sn?.trim().toLowerCase();
      if (first && last) {
        setFormData({ ...formData, cn: `${first}.${last}` });
      }
    }
  }, [formData.givenName, formData.sn]);

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const currentRepos: string[] = formData.repositories || [];

  const handleAddRepo = (repoFullName: string) => {
    if (repoFullName && !currentRepos.includes(repoFullName)) {
      setFormData({ ...formData, repositories: [...currentRepos, repoFullName] });
    }
  };

  const handleRemoveRepo = (repo: string) => {
    setFormData({
      ...formData,
      repositories: currentRepos.filter((r) => r !== repo),
    });
  };

  // Filter out repos already assigned
  const selectableRepos = availableRepos.filter(
    (r) => !currentRepos.includes(r.fullName)
  );

  return (
    <>
      <div className="form-group">
        <label htmlFor="uid">UID</label>
        <input
          name="uid"
          value={formData.uid || ""}
          onChange={handleChange}
          disabled={!!editingUser}
        />
      </div>

      <div className="form-group">
        <label htmlFor="password">
          Password
        </label>
        <input
          type="password"
          name="password"
          value={formData.password || ""}
          onChange={handleChange}
          placeholder={
            editingUser
              ? "Leave blank to keep current"
              : "Will default to uid+123 if empty"
          }
        />
      </div>

      <div className="form-group">
        <label htmlFor="givenName">First Name</label>
        <input
          name="givenName"
          value={formData.givenName || ""}
          onChange={handleChange}
        />
      </div>

      <div className="form-group">
        <label htmlFor="sn">Last Name</label>
        <input
          name="sn"
          value={formData.sn || ""}
          onChange={handleChange}
        />
      </div>

      <div className="form-group">
        <label htmlFor="mail">Email</label>
        <input
          type="email"
          name="mail"
          value={formData.mail || ""}
          onChange={handleChange}
        />
      </div>

      <div className="form-group">
        <label htmlFor="department">Department</label>
        <CustomSelect
          value={formData.department || ""}
          options={departments.map(d => ({
            label: d.ou,
            value: d.ou,
          }))}
          onChange={(v) =>
            setFormData({ ...formData, department: v })
          }
        />
      </div>

      <div className="form-group">
        <label>Repositories</label>
        <div className="chips-container">
          {currentRepos.map((repo) => (
            <span key={repo} className="repo-chip">
              {repo}
              <button
                type="button"
                className="chip-remove"
                onClick={() => handleRemoveRepo(repo)}
              >
                &times;
              </button>
            </span>
          ))}
          {currentRepos.length === 0 && (
            <span className="chips-empty">No repositories assigned</span>
          )}
        </div>
        {selectableRepos.length > 0 && (
          <CustomSelect
            value=""
            placeholder="Add a repository..."
            options={selectableRepos.map((r) => ({
              label: r.fullName,
              value: r.fullName,
            }))}
            onChange={handleAddRepo}
          />
        )}
      </div>

      {editingUser && (
        <div className="form-group">
          <label>Group Memberships</label>
          <div className="chips-container">
            {userGroups.length > 0 ? (
              userGroups.map((g) => (
                <span key={g.cn} className="member-chip">
                  {g.cn}
                </span>
              ))
            ) : (
              <span className="chips-empty">No group memberships</span>
            )}
          </div>
        </div>
      )}
    </>
  );
};
