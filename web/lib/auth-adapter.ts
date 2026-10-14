import type { Adapter, AdapterAccount, AdapterUser, VerificationToken } from "next-auth/adapters";
import { backend } from "./backend";

type BackendUser = { id: string; email: string; name: string; image: string; emailVerified: string | null; role: string };

function toUser(u: BackendUser | null): AdapterUser | null {
  if (!u) return null;
  return {
    id: u.id,
    email: u.email,
    name: u.name || null,
    image: u.image || null,
    emailVerified: u.emailVerified ? new Date(u.emailVerified) : null,
    role: u.role,
  } as AdapterUser;
}

const internal = { internal: true } as const;

/**
 * Auth.js adapter backed by the Go API, so there is a single source of truth
 * for users. Sessions use JWTs, so session methods are not needed.
 */
export function BackendAdapter(): Adapter {
  return {
    async createUser(user) {
      const u = await backend<BackendUser>("/internal/auth/users", {
        ...internal,
        method: "POST",
        body: { email: user.email, name: user.name ?? "", image: user.image ?? "", emailVerified: user.emailVerified },
      });
      return toUser(u)!;
    },
    async getUser(id) {
      return toUser(await backend<BackendUser | null>(`/internal/auth/users/${encodeURIComponent(id)}`, internal));
    },
    async getUserByEmail(email) {
      return toUser(await backend<BackendUser | null>(`/internal/auth/user-by-email?email=${encodeURIComponent(email)}`, internal));
    },
    async getUserByAccount({ provider, providerAccountId }) {
      const q = new URLSearchParams({ provider, providerAccountId });
      return toUser(await backend<BackendUser | null>(`/internal/auth/user-by-account?${q}`, internal));
    },
    async updateUser(user) {
      const u = await backend<BackendUser>(`/internal/auth/users/${encodeURIComponent(user.id)}`, {
        ...internal,
        method: "PATCH",
        body: { name: user.name ?? undefined, image: user.image ?? undefined, email: user.email, emailVerified: user.emailVerified },
      });
      return toUser(u)!;
    },
    async linkAccount(account: AdapterAccount) {
      await backend("/internal/auth/accounts", {
        ...internal,
        method: "POST",
        body: { userId: account.userId, provider: account.provider, providerAccountId: account.providerAccountId, type: account.type },
      });
      return account;
    },
    async createVerificationToken(token: VerificationToken) {
      await backend("/internal/auth/verification-tokens", { ...internal, method: "POST", body: token });
      return token;
    },
    async useVerificationToken({ identifier, token }) {
      const t = await backend<{ identifier: string; token: string; expires: string } | null>(
        "/internal/auth/verification-tokens/use",
        { ...internal, method: "POST", body: { identifier, token } },
      );
      return t ? { identifier: t.identifier, token: t.token, expires: new Date(t.expires) } : null;
    },
  };
}
