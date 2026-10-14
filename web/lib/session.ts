import { redirect } from "next/navigation";
import { auth } from "@/auth";

export async function requireUser(next = "/dashboard") {
  const session = await auth();
  if (!session?.user?.id) redirect(`/login?next=${encodeURIComponent(next)}`);
  return session.user;
}

export async function requireAdmin() {
  const user = await requireUser("/admin");
  if (user.role !== "admin") redirect("/dashboard");
  return user;
}
