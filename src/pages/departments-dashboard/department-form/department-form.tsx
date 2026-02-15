import React, { useEffect, useRef } from "react";
import type { Department, CreateDepartmentInput } from "../../../GQL/models/department";
import "./department-form.css";

interface Props {
  editingDepartment: Department | null;
  formData: CreateDepartmentInput;
  setFormData: (d: CreateDepartmentInput) => void;
}

export const DepartmentForm = ({
  editingDepartment,
  formData,
  setFormData,
}: Props) => {





  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  return (
        <>

          <div className="form-group">
            <label htmlFor="ou">OU</label>
            <input
              id="ou"
              name="ou"
              value={formData.ou}
              onChange={handleChange}
              disabled={!!editingDepartment}
              placeholder="department-ou"
            />
          </div>

          <div className="form-group">
            <label htmlFor="description">Description</label>
            <input
              id="description"
              name="description"
              value={formData.description || ""}
              onChange={handleChange}
              placeholder="Description"
            />
          </div>

          {/* <div className="form-group">
            <label htmlFor="manager">Manager UID</label>
            <input
              id="manager"
              name="manager"
              value={formData.manager || ""}
              onChange={handleChange}
              placeholder="manager uid"
            />
          </div> */}

        </>
  );
};
