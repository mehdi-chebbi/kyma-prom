import { useState } from "react";
import "./tools-dock.css";
import { LeftSidebarContent } from "./left-sidebar-content/left-sidebar-content";


export function ToolsDock() {
  const [open, setOpen] = useState(false);

  return (
    <aside className={`vertical-sidebar ${open ? "open" : "closed"}`}>
      <nav>
        <button
            className={`hamburger ${open ? "open" : ""} `}
            aria-label="Toggle sidebar"
            onClick={() => setOpen(o => !o)}
          >
            <span></span>
            <span></span>
            <span></span>
          </button>

        <section className="sidebar__wrapper">
          <LeftSidebarContent />
        </section>
      </nav>
    </aside>
  );
}
