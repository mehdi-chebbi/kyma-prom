import { NavLink, useLocation, useNavigate } from "react-router-dom";
import "./header.css";
import { useEffect, useRef, useState } from "react";
import { logout } from "../../services/userService";

export function Header() {
  const navigate = useNavigate();
  const navRef = useRef<HTMLDivElement>(null);
  const location = useLocation();
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const managementRef = useRef<HTMLButtonElement>(null);
  const [mgmtOpen, setMgmtOpen] = useState(false);
  const [mgmtPos, setMgmtPos] = useState({ left: 0, top: 0 });
  const isManagementActive = location.pathname.startsWith("/management");

  const [isScrolled, setIsScrolled] = useState(false);

  const [showRightArrow, setShowRightArrow] = useState(false);
  const [showLeftArrow, setShowLeftArrow] = useState(false);

  const [underline, setUnderline] = useState({
    left: 0,
    width: 0,
    transition: ""
  });

  useEffect(() => {
    const handlePageScroll = () => {
      setIsScrolled(window.scrollY > 50);
    };

    window.addEventListener("scroll", handlePageScroll);
    return () => window.removeEventListener("scroll", handlePageScroll);
  }, []);

useEffect(() => {
  if (!navRef.current) return;

  const links = Array.from(
    navRef.current.querySelectorAll<HTMLElement>(
      "a, button.nav-parent"
    )
  );

  let activeEl: HTMLElement | undefined;

  // Management children routes
  if (location.pathname.startsWith("/management")) {
    activeEl = links.find(
      (el) => el.dataset.parent === "management"
    );
  } else {
    activeEl = links.find((el) =>
      el.classList.contains("active")
    );
  }

  if (!activeEl) return;

  setUnderline({
    left: activeEl.offsetLeft,
    width: activeEl.offsetWidth,
    transition: "all 0.25s cubic-bezier(.4,0,.2,1)"
  });
}, [location.pathname, isScrolled]);
const openManagement = () => {
  if (!managementRef.current) return;

  const rect = managementRef.current.getBoundingClientRect();

  setMgmtPos({
    left: rect.left,
    top: rect.bottom - 2
  });

  setMgmtOpen(true);
};

const closeManagement = () => {
  setMgmtOpen(false);
};


  const scrollToEnd = () => {
    if (!navRef.current) return;
    navRef.current.scrollTo({
      left: navRef.current.scrollWidth,
      behavior: "smooth"
    });
  };

  const scrollToStart = () => {
    if (!navRef.current) return;
    navRef.current.scrollTo({
      left: 0,
      behavior: "smooth"
    });
  };

  /* Underline animation logic */
useEffect(() => {
  if (!navRef.current) return;

  const links = Array.from(
    navRef.current.querySelectorAll<HTMLAnchorElement>("a")
  );

  const active = links.find((link) =>
    location.pathname.startsWith(link.getAttribute("href") || "")
  );

  if (!active) return;

  setUnderline({
    left: active.offsetLeft,
    width: active.offsetWidth,
    transition: "all 0.3s ease"
  });
}, [location.pathname, isScrolled]);


  return (
    <header className={`topbar-container ${isScrolled ? "minified" : ""}`}>
      <div className="topbar-row">
        <div className="topbar-left">

          <div className="project">
            <div className="logo">▲</div>
            <span className="project-name">KYMA Flow</span>
          </div>
        </div>
        <div className="topbar-right" ref={menuRef}>
          <button
            type="button"
            className="topbar-avatar"
            aria-label="Open user menu"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((v) => !v)}
          />

          {menuOpen && (
            <div className="avatar-menu">
              <button className="menu-item">Account Settings</button>
              <button
                className="menu-item"
                onClick={() => logout(navigate)}
              >
                Logout
              </button>
            </div>
          )}
        </div>

              </div>

              <div className="topbar-menu-wrapper">

                {showLeftArrow && (
                  <button className="scroll-arrow left" onClick={scrollToStart}>
                    ◀
                  </button>
                )}

                <nav className="topbar-menu" ref={navRef}>
                  <NavLink to="/projects">Projects</NavLink>
                  <NavLink to="/workspaces">Workspaces</NavLink>
                  <NavLink to="/dashboard">Dashboard</NavLink>
        <button
          ref={managementRef}
          className={`nav-parent ${isManagementActive ? "active" : ""}`}
          data-parent="management"
          onMouseEnter={openManagement}
          onMouseLeave={closeManagement}
        >
          Management
        </button>



          <NavLink to="/deployments">Deployments</NavLink>
          <NavLink to="/activity">Activity</NavLink>
          <NavLink to="/domains">Domains</NavLink>
          <NavLink to="/usage">Usage</NavLink>
          <NavLink to="/support">Support</NavLink>
          <NavLink to="/settings">Settings</NavLink>

          <span
            className="magic-underline"
            style={{
              left: underline.left,
              width: underline.width,
              transition: underline.transition
            }}
          />
        </nav>
{mgmtOpen && (
  <div
    className="management-dropdown"
    style={{
      left: mgmtPos.left,
      top: mgmtPos.top
    }}
    onMouseEnter={() => setMgmtOpen(true)}
    onMouseLeave={closeManagement}
  >
    <NavLink to="/management/users">Users</NavLink>
    <NavLink to="/management/departments">Departments</NavLink>
    <NavLink to="/management/groups">Groups</NavLink>
    <NavLink to="/management/repositories">Repositories</NavLink>
  </div>
)}

        {showRightArrow && (
          <button className="scroll-arrow right" onClick={scrollToEnd}>
            ▶
          </button>
        )}
        <div className="topbar-right mini" ref={menuRef}>
          <button
            type="button"
            className="topbar-avatar"
            aria-label="Open user menu"
            aria-expanded={menuOpen}
            aria-controls="avatar-menu"
            onClick={() => setMenuOpen(!menuOpen)}
          />
          {menuOpen && (
            <div id="avatar-menu" className="avatar-menu">
              <button className="menu-item">Account Settings</button>
              <button className="menu-item" onClick={() => logout(navigate)}>Logout</button>
            </div>
          )}
        </div>

      </div>
    </header>
  );
}
