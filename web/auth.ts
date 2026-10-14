import NextAuth, { type NextAuthConfig } from "next-auth";
import GitHub from "next-auth/providers/github";
import Google from "next-auth/providers/google";
import Resend from "next-auth/providers/resend";
import { BackendAdapter } from "@/lib/auth-adapter";
import { backend } from "@/lib/backend";

const providers: NextAuthConfig["providers"] = [];
if (process.env.AUTH_GOOGLE_ID) providers.push(Google({ allowDangerousEmailAccountLinking: true }));
if (process.env.AUTH_GITHUB_ID) providers.push(GitHub({ allowDangerousEmailAccountLinking: true }));
if (process.env.AUTH_RESEND_KEY) {
  providers.push(Resend({ from: process.env.AUTH_EMAIL_FROM || "ModelRouter <login@modelrouter.app>" }));
}

/** Which sign-in methods are configured (used by the login page). */
export const enabledProviders = {
  google: Boolean(process.env.AUTH_GOOGLE_ID),
  github: Boolean(process.env.AUTH_GITHUB_ID),
  email: Boolean(process.env.AUTH_RESEND_KEY),
};

const ROLE_REFRESH_SECONDS = 300;

export const { handlers, auth, signIn, signOut } = NextAuth({
  adapter: BackendAdapter(),
  providers,
  session: { strategy: "jwt", maxAge: 30 * 24 * 60 * 60 },
  pages: { signIn: "/login", verifyRequest: "/login/check-email", error: "/login" },
  trustHost: true,
  callbacks: {
    async jwt({ token, user }) {
      const now = Math.floor(Date.now() / 1000);
      if (user?.id) {
        token.uid = user.id;
        token.role = user.role ?? "user";
        token.roleAt = now;
      } else if (token.uid && now - Number(token.roleAt ?? 0) > ROLE_REFRESH_SECONDS) {
        // Keep the role fresh so promotions/demotions show up without re-login.
        // The API never trusts this value; it re-checks the role itself.
        try {
          const u = await backend<{ role: string } | null>(`/internal/auth/users/${token.uid}`, { internal: true });
          if (!u) return null;
          token.role = u.role;
          token.roleAt = now;
        } catch {
          /* keep the previous role on transient errors */
        }
      }
      return token;
    },
    session({ session, token }) {
      if (token.uid) {
        session.user.id = String(token.uid);
        session.user.role = token.role === "admin" ? "admin" : "user";
      }
      return session;
    },
  },
});
