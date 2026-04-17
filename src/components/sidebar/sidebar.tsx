import React, { useState, useRef, useEffect } from "react";
import type { ReactNode } from "react";
import "./sidebar.css";

type SidebarProps = {
  side?: "left" | "right";
  children: ReactNode;
};

interface CollapsibleChildProps {
  collapsed?: boolean;
}

export function Sidebar({ side = "left", children }: Readonly<SidebarProps>) {
  const [isOpen, setIsOpen] = useState(false);
  const sidebarRef = useRef<HTMLDivElement>(null);

  const toggle = () => setIsOpen(prev => !prev);

  useEffect(() => {
    if (sidebarRef.current) {
      sidebarRef.current.style.width = isOpen
        ? `${sidebarRef.current.scrollWidth}px`
        : "2.25rem";
    }
  }, [isOpen, children]);

  const childrenWithProps = React.Children.map(children, child => {
    if (React.isValidElement<CollapsibleChildProps>(child)) {
      return React.cloneElement(child, { collapsed: !isOpen });
    }
    return child;
  });

  return (
    <aside ref={sidebarRef} className={`sidebar ${side} ${isOpen? "open" : ""}`}>
      <button
        type="button"
        className="sidebar-toggle"
        onClick={toggle}
        aria-pressed={isOpen}
        aria-label="Toggle sidebar"
      >
        <span className={`arrow ${isOpen ? "open" : ""}`}>➤</span>
      </button>
      {childrenWithProps}
    </aside>
  );
}
