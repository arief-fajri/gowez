export type Theme = "light" | "dark";
export type Role = "Admin" | "Editor" | "Viewer";
export type Status = "Active" | "Suspended";
export type Kind = "add" | "remove" | "toggle" | "system";
export type Slug = "dashboard" | "users" | "analytics" | "orders" | "reports" | "settings";

export interface User {
  id: number;
  name: string;
  email: string;
  role: Role;
  status: Status;
}

export interface Activity {
  id: number;
  text: string;
  time: string;
  kind: Kind;
}

export interface NewUser {
  name: string;
  email: string;
  role: Role;
  status: Status;
}
