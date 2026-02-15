import { useState } from "react";
import { MigrateForm } from "./migrate-form";
import type { MigrateRepositoryInput } from "../../../GQL/models/repository";
import { Modal } from "../../../components/modal/modal";
import { migrateRepository } from "../../../services/repositoryService";

interface MigrateModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
}

export const MigrateRepositoryModal = ({
  isOpen,
  onClose,
  onSuccess,
}: MigrateModalProps) => {
  const [formData, setFormData] = useState<Partial<MigrateRepositoryInput>>({
    service: "github",
    wiki: true,
    issues: true,
    pullRequests: true,
    labels: true,
    mirror: false,
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleConfirm = async () => {
    if (!formData.cloneAddr || !formData.repoName) {
      setError("Clone Address and Repository Name are required.");
      return;
    }

    setLoading(true);
    setError(null);

    try {
      await migrateRepository(formData as MigrateRepositoryInput);

      if (onSuccess) onSuccess();
      onClose();

      setFormData({
        service: "github",
        wiki: true,
        issues: true,
        pullRequests: true,
        labels: true,
        mirror: false,
      });
    } catch (err: any) {
      setError(err.message || "Migration failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title="Migrate Repository"
      subtitle="Clone an existing repository from another service"
      onClose={onClose}
      width={500}
      footer={
        <>
          <button className="cancel-btn" onClick={onClose} disabled={loading}>
            Cancel
          </button>
          <button
            className="submit-btn"
            onClick={handleConfirm}
            disabled={loading}
          >
            {loading ? "Migrating..." : "Start Migration"}
          </button>
        </>
      }
    >
      {error && <p className="error-text">{error}</p>}
      <MigrateForm formData={formData} setFormData={setFormData} />
    </Modal>
  );
};
