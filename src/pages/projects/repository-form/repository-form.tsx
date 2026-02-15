import type {
  CreateRepositoryInput,
  Repository,
} from "../../../GQL/models/repository";
import type { Branch } from "../../../services/repositoryService";

interface Props {
  formData: Partial<Repository & CreateRepositoryInput>;
  setFormData: (data: Partial<Repository & CreateRepositoryInput>) => void;
  branches?: Branch[];
  loadingBranches?: boolean;
  edit: boolean;
}

export const RepositoryForm = ({
  formData,
  setFormData,
  branches,
  loadingBranches,
  edit,
}: Props) => {
  return (
    <form className="repository-form" onSubmit={(e) => e.preventDefault()}>
      <div className="form-group">
        <label htmlFor="name">Name</label>
        <input
          id="name"
          type="text"
          placeholder="e.g. my-app"
          value={formData.name || ""}
          onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          required
          disabled={edit}
        />
      </div>

      <div className="form-group">
        <label htmlFor="description">Description</label>
        <textarea
          id="description"
          value={formData.description || ""}
          onChange={(e) =>
            setFormData({ ...formData, description: e.target.value })
          }
        />
      </div>

      {edit ? (
        loadingBranches ? (
          <p className="widget-text">Loading branches...</p>
        ) : (
          <div className="form-group">
            <label htmlFor="branch-select">Default Branch</label>
            <select
              id="branch-select"
              value={formData.defaultBranch}
              onChange={(e) =>
                setFormData({ ...formData, defaultBranch: e.target.value })
              }
            >
              {branches?.map((b) => (
                <option key={b.name} value={b.name}>
                  {b.name}
                </option>
              ))}
            </select>
          </div>
        )
      ) : (
        <div className="form-group">
          <label htmlFor="defaultBranch">Initial Branch Name</label>
          <input
            id="defaultBranch"
            type="text"
            placeholder="main"
            value={formData.defaultBranch || ""}
            onChange={(e) =>
              setFormData({ ...formData, defaultBranch: e.target.value })
            }
          />
        </div>
      )}

      {!edit && (
        <>
          <div className="form-group checkbox-group">
            <label>
              <input
                type="checkbox"
                checked={formData.autoInit || false}
                onChange={(e) =>
                  setFormData({ ...formData, autoInit: e.target.checked })
                }
              />{" "}
              Initialize with a README
            </label>
          </div>

          <div className="form-group">
            <label htmlFor="gitignores">.gitignore Template</label>
            <select
              id="gitignores"
              value={formData.gitignores || ""}
              onChange={(e) =>
                setFormData({ ...formData, gitignores: e.target.value })
              }
            >
              <option value="">None</option>
              <option value="Go">Go</option>
              <option value="Node">Node</option>
              <option value="Python">Python</option>
              <option value="React">React</option>
            </select>
          </div>

          <div className="form-group">
            <label htmlFor="license">License</label>
            <select
              id="license"
              value={formData.license || ""}
              onChange={(e) =>
                setFormData({ ...formData, license: e.target.value })
              }
            >
              <option value="">None</option>
              <option value="MIT">MIT</option>
              <option value="Apache-2.0">Apache 2.0</option>
              <option value="GPL-3.0">GPLv3</option>
            </select>
          </div>
        </>
      )}

      <div className="form-group checkbox-group">
        <label>
          <input
            id="visibility"
            type="checkbox"
            checked={formData.private || false}
            onChange={(e) =>
              setFormData({ ...formData, private: e.target.checked })
            }
          />{" "}
          Make repository private
        </label>
      </div>
    </form>
  );
};
