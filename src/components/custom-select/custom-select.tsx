import React, { useState, useRef, useEffect } from "react";
import "./custom-select.css";

interface Option {
  label: string;
  value: string;
}

interface CustomSelectProps {
  value: string;
  options: Option[];
  placeholder?: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  loading?: boolean;
  className?: string;
  arrowIcon?: React.ReactNode;
}

export const CustomSelect = ({
  value,
  options,
  placeholder = "Select...",
  onChange,
  disabled = false,
  loading = false,
  className = "",
  arrowIcon,
}: CustomSelectProps) => {
  const [open, setOpen] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const handleSelect = (val: string) => {
    onChange(val);
    setOpen(false);
  };

  return (
    <div
      ref={wrapperRef}
      className={`select-wrapper ${disabled ? "disabled" : ""} ${className}`}
    >
      <div
        className={`select-display ${open ? "open" : ""}`}
        onClick={() => !disabled && !loading && setOpen((prev) => !prev)}
      >
        <span>{value ? options.find((o) => o.value === value)?.label : placeholder}</span>
        <div className={`select-arrow ${open ? "rotate" : ""}`}>
          {loading ? "…" : arrowIcon ?? "▼"}
        </div>
      </div>

      <ul className={`options ${open ? "show" : ""}`}>
        <li onClick={()=> handleSelect(null)}>{placeholder}</li>
        {options.map((opt) => (
          <li key={opt.value} onClick={() => handleSelect(opt.value)}>
            {opt.label}
          </li>
        ))}
      </ul>
    </div>
  );
};
