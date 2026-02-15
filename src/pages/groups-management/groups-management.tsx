import { useEffect, useState, useCallback } from "react";
import { useAtom } from "jotai";
import type { DragEvent } from "react";
import "./groups-management.css";
import { ConfirmationModal } from "../../components/confirmation-modal/confirmation-modal";
import {
  listAllGroups,
  getGroup,
  createGroup,
  deleteGroup,
  addUserToGroup,
  removeUserFromGroup,
  assignRepoToGroup,
} from "../../services/groupService";
import { listUsers } from "../../services/userService";

import {
  groupsAtom,
  usersAtom,
  selectedGroupCnAtom,
  selectedGroupDetailAtom,
} from "../../store/atoms";

export const GroupsManagement = () => {
  const [groups, setGroups] = useAtom(groupsAtom);
  const [selectedCn, setSelectedCn] = useAtom(selectedGroupCnAtom);
  const [selectedGroup, setSelectedGroup] = useAtom(selectedGroupDetailAtom);
  const [users, setUsers] = useAtom(usersAtom);
  const [loading, setLoading] = useState(true);

  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [userDragOver, setUserDragOver] = useState(false);
  const [repoDragOver, setRepoDragOver] = useState(false);

  // ── Fetch data ──
  const fetchGroups = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listAllGroups();
      setGroups(data || []);
    } finally {
      setLoading(false);
    }
  }, [setGroups]);

  const fetchGroupDetail = useCallback(async (cn: string) => {
    const detail = await getGroup(cn);
    setSelectedGroup(detail);
  }, [setSelectedGroup]);

  const fetchUsers = useCallback(async () => {
    const page = await listUsers(undefined, 0, 200);
    setUsers(page.items || []);
  }, [setUsers]);

  useEffect(() => {
    fetchGroups();
    fetchUsers();
  }, [fetchGroups, fetchUsers]);

  useEffect(() => {
    if (selectedCn) fetchGroupDetail(selectedCn);
  }, [selectedCn, fetchGroupDetail]);

  // ── Helpers ──
  const memberUids = (selectedGroup?.members || []).map((dn) => {
    const match = dn.match(/^(?:uid|cn)=([^,]+)/);
    return match ? match[1] : dn;
  });

  const allRepos = users.flatMap((u) => u.repositories || []);
  const uniqueRepos = [...new Set(allRepos)];
  const groupRepos = selectedGroup?.repositories || [];

  // ── Actions ──
  const handleCreateGroup = async () => {
    const cn = window.prompt("Group name (e.g. qa-team)");
    if (!cn) return;
    const desc = window.prompt("Description (optional)") || undefined;
    await createGroup(cn, desc);
    await fetchGroups();
    setSelectedCn(cn);
  };

  const handleDeleteGroup = (cn: string) => {
    setDeleteTarget(cn);
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteGroup(deleteTarget);
      if (selectedCn === deleteTarget) {
        setSelectedCn(null);
        setSelectedGroup(null);
      }
      await fetchGroups();
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const handleRemoveUser = async (uid: string) => {
    if (!selectedCn) return;
    await removeUserFromGroup(uid, selectedCn);
    await fetchGroupDetail(selectedCn);
    await fetchGroups();
  };

  const handleRemoveRepo = async (repo: string) => {
    if (!selectedCn || !selectedGroup) return;
    const updated = groupRepos.filter((r) => r !== repo);
    await assignRepoToGroup(selectedCn, updated);
    await fetchGroupDetail(selectedCn);
  };

  // ── Drag & Drop ──
  const onDragStart = (e: DragEvent, type: "user" | "repo", value: string) => {
    e.dataTransfer.setData("type", type);
    e.dataTransfer.setData("value", value);
    e.dataTransfer.effectAllowed = "copy";
  };

  const onDropUser = async (e: DragEvent) => {
    e.preventDefault();
    setUserDragOver(false);
    if (!selectedCn) return;
    const type = e.dataTransfer.getData("type");
    const value = e.dataTransfer.getData("value");
    if (type !== "user") return;
    await addUserToGroup(value, selectedCn);
    await fetchGroupDetail(selectedCn);
    await fetchGroups();
  };

  const onDropRepo = async (e: DragEvent) => {
    e.preventDefault();
    setRepoDragOver(false);
    if (!selectedCn) return;
    const type = e.dataTransfer.getData("type");
    const value = e.dataTransfer.getData("value");
    if (type !== "repo") return;
    const updated = [...new Set([...groupRepos, value])];
    await assignRepoToGroup(selectedCn, updated);
    await fetchGroupDetail(selectedCn);
  };

  const onDragOver = (e: DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
  };

  return (
    <div className="groups-page">
      <div className="groups-page-title">
        <h1>Groups</h1>
        <p>Manage groups, members, and repository access. Drag users or repos into a group to assign them.</p>
      </div>

      <div className="groups-layout">
        {/* ── Left: Group List ── */}
        <div className="groups-panel">
          <div className="groups-panel-header">
            <h3>Groups</h3>
            <button className="groups-create-btn" onClick={handleCreateGroup}>+ New</button>
          </div>

          {loading ? (
            <div style={{ color: "var(--light-purple-500)" }}>Loading...</div>
          ) : groups.length === 0 ? (
            <div style={{ color: "var(--light-purple-500)" }}>No groups found</div>
          ) : (
            groups.map((g) => (
              <div
                key={g.cn}
                className={`group-card ${selectedCn === g.cn ? "selected" : ""}`}
                onClick={() => setSelectedCn(g.cn)}
              >
                <div className="group-card-name">{g.cn}</div>
                <div className="group-card-meta">
                  {(g.members || []).length} members
                  {(g.repositories || []).length > 0 && ` / ${g.repositories!.length} repos`}
                </div>
                <div className="group-card-actions">
                  <button
                    className="group-delete-btn"
                    onClick={(e) => { e.stopPropagation(); handleDeleteGroup(g.cn); }}
                  >
                    Delete
                  </button>
                </div>
              </div>
            ))
          )}
        </div>

        {/* ── Center: Group Detail ── */}
        {!selectedGroup ? (
          <div className="group-detail">
            <div className="group-detail-empty">Select a group to manage</div>
          </div>
        ) : (
          <div className="group-detail">
            <h2 className="group-detail-title">{selectedGroup.cn}</h2>

            {/* Members drop zone */}
            <div
              className={`drop-zone ${userDragOver ? "drag-over" : ""}`}
              onDragOver={(e) => { onDragOver(e); setUserDragOver(true); }}
              onDragLeave={() => setUserDragOver(false)}
              onDrop={onDropUser}
            >
              <h4>Members (drop users here)</h4>
              {memberUids.length === 0 ? (
                <div style={{ color: "var(--light-purple-500)", fontSize: "0.85rem" }}>No members yet</div>
              ) : (
                <div>
                  {memberUids.filter((uid) => uid !== "admin").map((uid) => (
                    <span key={uid} className="member-chip">
                      {uid}
                      <button className="chip-remove" onClick={() => handleRemoveUser(uid)} title="Remove">x</button>
                    </span>
                  ))}
                </div>
              )}
            </div>

            {/* Repos drop zone */}
            <div
              className={`drop-zone ${repoDragOver ? "drag-over" : ""}`}
              onDragOver={(e) => { onDragOver(e); setRepoDragOver(true); }}
              onDragLeave={() => setRepoDragOver(false)}
              onDrop={onDropRepo}
            >
              <h4>Repositories (drop repos here)</h4>
              {groupRepos.length === 0 ? (
                <div style={{ color: "var(--light-purple-500)", fontSize: "0.85rem" }}>No repositories assigned</div>
              ) : (
                <div>
                  {groupRepos.map((repo) => (
                    <span key={repo} className="repo-chip">
                      {repo}
                      <button className="chip-remove" onClick={() => handleRemoveRepo(repo)} title="Remove">x</button>
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {/* ── Right: Available Users & Repos ── */}
        <div className="available-panel">
          <div className="available-section">
            <h3>Available Users</h3>
            {users.map((u) => {
              const inGroup = memberUids.includes(u.uid);
              return (
                <div
                  key={u.uid}
                  className={`draggable-item ${inGroup ? "in-group" : ""}`}
                  draggable={!inGroup && !!selectedCn}
                  onDragStart={(e) => onDragStart(e, "user", u.uid)}
                >
                  <strong>{u.uid}</strong> — {u.cn}
                  <div style={{ fontSize: "0.75rem", color: "var(--light-purple-500)" }}>{u.department}</div>
                </div>
              );
            })}
          </div>

          <div className="available-section">
            <h3>Available Repos</h3>
            {uniqueRepos.map((repo) => {
              const inGroup = groupRepos.includes(repo);
              return (
                <div
                  key={repo}
                  className={`draggable-item ${inGroup ? "in-group" : ""}`}
                  draggable={!inGroup && !!selectedCn}
                  onDragStart={(e) => onDragStart(e, "repo", repo)}
                >
                  {repo}
                </div>
              );
            })}
          </div>
        </div>
      </div>

      {deleteTarget && (
        <ConfirmationModal
          message={`Delete group "${deleteTarget}"? This cannot be undone.`}
          onCancel={() => setDeleteTarget(null)}
          onConfirm={confirmDelete}
          loading={deleting}
        />
      )}
    </div>
  );
};
