import { atom } from "jotai";
import type { User, UserPage } from "../GQL/models/user";
import type { Department, DepartmentPage } from "../GQL/models/department";
import type { Group } from "../GQL/models/group";

// ── Shared data ──
export const usersAtom = atom<User[]>([]);
export const departmentsAtom = atom<Department[]>([]);
export const groupsAtom = atom<Group[]>([]);

// ── Users page ──
export const usersPageDataAtom = atom<UserPage | null>(null);
export const usersPageNumAtom = atom(1);
export const usersSearchAtom = atom("");
export const usersMailSearchAtom = atom("");
export const usersDeptFilterAtom = atom("");

// ── Departments page ──
export const depsPageDataAtom = atom<DepartmentPage | null>(null);
export const depsPageNumAtom = atom(1);
export const depsSearchAtom = atom("");
export const depsDescSearchAtom = atom("");

// ── Groups page ──
export const selectedGroupCnAtom = atom<string | null>(null);
export const selectedGroupDetailAtom = atom<Group | null>(null);
