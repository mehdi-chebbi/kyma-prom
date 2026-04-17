import GiteaIcon from "../../icons/gitea-icon";
import GraphanaIcon from "../../icons/graphana-icon";
const GITEA_ENDPOINT = import.meta.env.VITE_GITEA_ACCESS_ENDPOINT!;

const ITEMS = [
  { id: "gitea", label: "Gitea", subtitle: "Source Code Hosting", icon: <GiteaIcon />, link: GITEA_ENDPOINT },
  { id: "sonarqube", label: "SonarQube", subtitle: "Code Quality & Security", icon: <GiteaIcon />, link: "" },
  { id: "graphana", label: "Graphana", subtitle: "Analytics & Monitoring", icon: <GraphanaIcon />, link: ""},
];

export function LeftSidebarContent() {
  console.log(GITEA_ENDPOINT)
  return (
    <ul className="sidebar__list">
      {ITEMS.map(item => (
        <li key={item.id} className="sidebar__item">
          <a
            className="sidebar__link"
            href={item.link}
            target="_blank"
            rel="noopener noreferrer"
            data-tooltip={`${item.label} - ${item.subtitle}`}
          >
            <span className="icon">{item.icon}</span>
            <span className="text">
              <span className="label">{item.label}</span>
              <span className="subtitle">{item.subtitle}</span>
            </span>
          </a>
        </li>
      ))}
    </ul>
  );
}

