import { useState, useRef, useEffect } from "react";
import { Tooltip } from "../tooltip/tooltip";
import "./project-card.css";
import GithubIcon from "../../assets/github-icon.svg";
import BranchIcon from "../../assets/branch-icon.svg";

interface ProjectCardProps {
  readonly icon?: string;
  readonly title: string;
  readonly repoName?: string;
  readonly repoLink?: string;
  readonly dateRange?: string;
  readonly branch?: string;
  readonly status?: string;
  readonly stars?: number;
  readonly forks?: number;
  readonly onEdit?: () => void;
  readonly onDelete?: () => void;
  readonly onOpenCode?: () => void;
  readonly openingCode?: boolean;
  readonly codeServerStatus?: "RUNNING" | "STOPPED" | "PENDING" | "STARTING" | null;
  readonly codeServerUrl?: string;
}

export function ProjectCard({
  icon = "⚛️",
  title,
  repoName,
  repoLink,
  dateRange,
  branch = "master",
  status = "◉",
  stars,
  forks,
  onEdit,
  onDelete,
  onOpenCode,
  openingCode,
  codeServerStatus,
  codeServerUrl,
}: ProjectCardProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const projectMenuRef = useRef<HTMLDivElement | null>(null);

  // Close menu if clicked outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (projectMenuRef.current && !projectMenuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className="project-card">
      <div className="project-left">
        <div className="project-icon">{icon}</div>
        <div className="project-meta">
          <h3 className="project-title">{title}</h3>

          {repoName && (
            <Tooltip text={repoName} color="var(--light-blue-600)" position="top">
              <a
                href={repoLink}
                target="_blank"
                className="project-subtext flex min-w-0 flex-row items-center gap-0.5 rounded-full p-0.5 pr-1.5 max-w-48"
              >
                <img src={GithubIcon} alt="Github Icon" />
                <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">
                  {repoName}
                </span>
              </a>
            </Tooltip>
          )}

          {dateRange && (
            <span className="project-date">
              {dateRange} —
              <img src={BranchIcon} alt="branch icon" />
              &nbsp;
              {branch}
            </span>
          )}
        </div>
      </div>

      <div className="project-right" ref={projectMenuRef}>
        {(stars !== undefined || forks !== undefined) && (
          <div className="project-stats">
            {stars !== undefined && <span>⭐ {stars}</span>}
            {forks !== undefined && <span>🍴 {forks}</span>}
          </div>
        )}

        <div className="project-status">{status}</div>

        {onOpenCode && (
          <div className="open-code-wrapper">
            {codeServerStatus && (
              <span className={`code-status-dot dot-${codeServerStatus.toLowerCase()}`} />
            )}
            {codeServerStatus === "RUNNING" && codeServerUrl ? (
              <a
                href={codeServerUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="open-code-btn open-code-running"
              >
                Open IDE
              </a>
            ) : (
              <button
                className={`open-code-btn${codeServerStatus === "STOPPED" ? " open-code-resume" : ""}`}
                onClick={onOpenCode}
                disabled={openingCode || codeServerStatus === "PENDING" || codeServerStatus === "STARTING"}
              >
                {openingCode
                  ? "Starting..."
                  : codeServerStatus === "PENDING" || codeServerStatus === "STARTING"
                    ? "Starting..."
                    : codeServerStatus === "STOPPED"
                      ? "Resume IDE"
                      : "Open in Code"}
              </button>
            )}
          </div>
        )}

        {(onEdit || onDelete) && (
          <div className="project-menu" onClick={() => setMenuOpen(prev => !prev)}>
            ⋮
            {menuOpen && (
              <div className="project-dropdown">
                {onEdit && <div className="project-dropdown-item" onClick={() => { setMenuOpen(false); onEdit(); }}>Edit</div>}
                {onDelete && <div className="project-dropdown-item" onClick={() => { setMenuOpen(false); onDelete(); }}>Delete</div>}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
