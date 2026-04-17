import "./filter-bar.css";
import { CustomSelect } from "../custom-select/custom-select";

export type FilterField =
  | {
      key: string;
      type: "text";
      placeholder?: string;
      value: string;
      onChange: (v: string) => void;
    }
  | {
      key: string;
      type: "select";
      placeholder?: string;
      value: string;
      options: { label: string; value: string }[];
      onChange: (v: string) => void;
    };

interface FilterBarProps {
  filters: FilterField[];
  actions?: React.ReactNode[];
}

export const FilterBar = ({ filters, actions }: FilterBarProps) => {
  return (
    <div className="filter-bar">
      <div className="filter-fields">
        {filters.map((f) => {
          if (f.type === "text") {
            return (
              <input
                key={f.key}
                id={f.key}
                type="text"
                placeholder={f.placeholder}
                value={f.value}
                onChange={(e) => f.onChange(e.target.value)}
                className="filter-input"
              />
            );
          }

          return (
            <CustomSelect
              key={f.key}
              value={f.value}
              options={f.options}
              placeholder={f.placeholder}
              onChange={f.onChange}
            />
          );
        })}
      </div>

      {actions && <div className="filter-actions">{actions}</div>}
    </div>
  );
};
