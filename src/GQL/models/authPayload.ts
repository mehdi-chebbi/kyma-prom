import type { User } from "./user"

export type AuthPayload = {
  token: string
  user: User
}