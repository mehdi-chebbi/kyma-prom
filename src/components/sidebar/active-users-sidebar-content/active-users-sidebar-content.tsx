import React, { useEffect, useState } from "react";
import type { User } from "../../../GQL/models/user";
import "./active-users-sidebar-content.css";

interface UserSession extends User {
  isActive: boolean;
  lastOnline: Date;
}

const MOCK_USERS: UserSession[] = [
  {
    uid: "u1",
    cn: "Adam Chibani",
    sn: "Chibani",
    givenName: "Adam",
    mail: "adam.chibani@example.com",
    department: "Engineering",
    uidNumber: 101,
    gidNumber: 101,
    homeDirectory: "/home/adam",
    repositories: ["repo1", "repo2"],
    dn: "uid=u1,ou=users,dc=example,dc=com",
    isActive: true,
    lastOnline: new Date(),
  },
  {
    uid: "u2",
    cn: "Jane Doe",
    sn: "Doe",
    givenName: "Jane",
    mail: "jane.doe@example.com",
    department: "Marketing",
    uidNumber: 102,
    gidNumber: 102,
    homeDirectory: "/home/jane",
    repositories: ["repo3"],
    dn: "uid=u2,ou=users,dc=example,dc=com",
    isActive: false,
    lastOnline: new Date(Date.now() - 1000 * 60 * 5),
  },
  {
    uid: "u3",
    cn: "John Smith",
    sn: "Smith",
    givenName: "John",
    mail: "john.smith@example.com",
    department: "Sales",
    uidNumber: 103,
    gidNumber: 103,
    homeDirectory: "/home/john",
    repositories: ["repo4", "repo5"],
    dn: "uid=u3,ou=users,dc=example,dc=com",
    isActive: false,
    lastOnline: new Date(Date.now() - 1000 * 60 * 30),
  },
];
interface ActiveUsersSidebarContentProps {
    readonly collapsed?: boolean;
}
export const ActiveUsersSidebarContent: React.FC<ActiveUsersSidebarContentProps> = ({ collapsed = false }) => {
  const [users, setUsers] = useState<UserSession[]>([]);

  useEffect(() => {
    setUsers(MOCK_USERS);
  }, []);

  const activeUsers = users.filter(u => u.isActive);
  const offlineUsers = users.filter(u => !u.isActive)
                            .sort((a, b) => b.lastOnline.getTime() - a.lastOnline.getTime());

  return (
    <div className="right-sidebar-users">
      <section>
        <h3>Active Users</h3>
        <ul className="users-list">
          {activeUsers.map(user => (
            <li key={user.uid}   className={`user-item ${collapsed ? "collapsed" : ""}`} data-tooltip={collapsed ? user.cn : undefined}>
              <span className="user-info">
                <span className="status-dot online"></span>
                <span className="user-name">{user.cn}</span>
                <span className="user-department">{user.department}</span>
              </span>
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h3>Recently Offline</h3>
        <ul className="users-list">
          {offlineUsers.map(user => (
            <li key={user.uid}   className={`user-item ${collapsed ? "collapsed" : ""}`} data-tooltip={collapsed ? user.cn : undefined}>
              <span className="user-info">
                <span className="status-dot offline"></span>
                <span className="user-name">{user.cn}</span>
                <span className="user-department">{user.department}</span>
              </span>
              <span className="last-online">
                {user.lastOnline.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
              </span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
};
