import type { MigrateRepositoryInput } from "../../../GQL/models/repository";

interface Props {
  formData: Partial<MigrateRepositoryInput>;
  setFormData: (data: Partial<MigrateRepositoryInput>) => void;
}

export const MigrateForm = ({ formData, setFormData }: Props) => {
  const updateField = (field: keyof MigrateRepositoryInput, value: any) => {
    setFormData({ ...formData, [field]: value });
  };

  return (
    <form className="repository-form" onSubmit={(e) => e.preventDefault()}>
      <div className="form-group">
        <label htmlFor="cloneAddr">Clone URL</label>
        <input
          id="cloneAddr"
          type="url"
          placeholder="https://github.com/user/repo"
          value={formData.cloneAddr || ""}
          onChange={(e) => updateField("cloneAddr", e.target.value)}
          required
        />
      </div>

      <div className="form-group">
        <label htmlFor="repoName">New Repository Name</label>
        <input
          id="repoName"
          type="text"
          placeholder="e.g. my-migrated-repo"
          value={formData.repoName || ""}
          onChange={(e) => updateField("repoName", e.target.value)}
          required
        />
      </div>

      <div className="form-group">
        <label htmlFor="service">Source Service</label>
        <select
          id="service"
          value={formData.service || "github"}
          onChange={(e) => updateField("service", e.target.value)}
        >
          <option value="github">GitHub</option>
          <option value="gitlab">GitLab</option>
          <option value="gitea">Gitea</option>
        </select>
      </div>

      <div className="form-group">
        <label htmlFor="authToken">Authorization Token (Optional)</label>
        <input
          id="authToken"
          type="password"
          placeholder="Personal Access Token"
          value={formData.authToken || ""}
          onChange={(e) => updateField("authToken", e.target.value)}
        />
        <small className="form-hint">Required for private repositories</small>
      </div>

      <div className="form-group checkbox-group">
        <label>
          <input
            type="checkbox"
            checked={formData.private || false}
            onChange={(e) => updateField("private", e.target.checked)}
          />{" "}
          Make repository private
        </label>
      </div>

      <div className="form-group checkbox-group">
        <label>
          <input
            type="checkbox"
            checked={formData.mirror || false}
            onChange={(e) => updateField("mirror", e.target.checked)}
          />{" "}
          Mirror (periodic sync with source)
        </label>
      </div>

      {!formData.mirror && (
        <div className="metadata-options">
          <p className="section-label">Migrate Metadata:</p>
          <div className="checkbox-grid">
            <label>
              <input
                type="checkbox"
                checked={formData.wiki ?? true}
                onChange={(e) => updateField("wiki", e.target.checked)}
              />{" "}
              Wiki
            </label>
            <label>
              <input
                type="checkbox"
                checked={formData.issues ?? true}
                onChange={(e) => updateField("issues", e.target.checked)}
              />{" "}
              Issues
            </label>
            <label>
              <input
                type="checkbox"
                checked={formData.pullRequests ?? true}
                onChange={(e) => updateField("pullRequests", e.target.checked)}
              />{" "}
              PRs
            </label>
            <label>
              <input
                type="checkbox"
                checked={formData.labels ?? true}
                onChange={(e) => updateField("labels", e.target.checked)}
              />{" "}
              Labels
            </label>
          </div>
        </div>
      )}
    </form>
  );
};
