import { request, setCSRFToken, clearCSRFToken, type APIError } from "../../lib/api/client";
import type { components } from "../../lib/api/schema";

export type PublicUser = components["schemas"]["PublicUser"];
export type SessionInfo = components["schemas"]["SessionInfo"];
export type Project = components["schemas"]["Project"];

export async function fetchSession(): Promise<SessionInfo> {
  const session = await request<SessionInfo>("/auth/session");
  setCSRFToken(session.csrfToken);
  return session;
}

export async function login(email: string, password: string): Promise<PublicUser> {
  const result = await request<{ user: PublicUser }>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  return result.user;
}

export async function register(
  displayName: string,
  email: string,
  password: string,
): Promise<PublicUser> {
  const result = await request<{ user: PublicUser }>("/auth/register", {
    method: "POST",
    body: JSON.stringify({ displayName, email, password }),
    idempotencyKey: crypto.randomUUID(),
  });
  return result.user;
}

export async function recover(recoveryCode: string, newPassword: string): Promise<void> {
  await request<{ completed: boolean }>("/auth/recover", {
    method: "POST",
    body: JSON.stringify({ recoveryCode, newPassword }),
  });
}

export async function logout(): Promise<void> {
  try {
    await request("/auth/logout", { method: "POST" });
  } finally {
    clearCSRFToken();
  }
}

export async function listProjects(): Promise<Project[]> {
  const page = await request<{ items: Project[] }>("/projects?limit=50");
  return page.items ?? [];
}

export type { APIError };
