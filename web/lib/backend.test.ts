import { decodeJwt, decodeProtectedHeader } from "jose";
import { afterEach, describe, expect, it, vi } from "vitest";

process.env.BACKEND_JWT_SECRET = "test-secret-1234567890";
process.env.BACKEND_INTERNAL_SECRET = "internal-secret-123";
process.env.BACKEND_URL = "http://api.test";

const { backend, BackendError, userToken } = await import("./backend");
const { BackendAdapter } = await import("./auth-adapter");

afterEach(() => vi.unstubAllGlobals());

describe("userToken", () => {
  it("mints a short-lived HS256 token the Go API accepts", async () => {
    const t = await userToken("user-1");
    expect(decodeProtectedHeader(t).alg).toBe("HS256");
    const c = decodeJwt(t);
    expect(c.sub).toBe("user-1");
    expect(c.aud).toBe("modelrouter-api");
    expect(Number(c.exp) - Number(c.iat)).toBe(60);
  });
});

describe("backend", () => {
  it("sends auth headers and surfaces API errors", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ error: { message: "nope" } }), { status: 402 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(backend("/api/me", { userId: "u1" })).rejects.toMatchObject({ status: 402, message: "nope" });
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("http://api.test/api/me");
    expect((init.headers as Record<string, string>).Authorization).toMatch(/^Bearer /);
    expect(init.cache).toBe("no-store");
    expect(new BackendError(1, "x")).toBeInstanceOf(Error);
  });
});

describe("BackendAdapter", () => {
  it("maps users and passes the internal secret", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ id: "u1", email: "a@b.c", name: "", image: "", emailVerified: null, role: "admin" })),
    );
    vi.stubGlobal("fetch", fetchMock);
    const user = await BackendAdapter().getUserByEmail!("a@b.c");
    expect(user).toMatchObject({ id: "u1", email: "a@b.c", name: null, emailVerified: null, role: "admin" });
    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect((init.headers as Record<string, string>)["X-Internal-Secret"]).toBe("internal-secret-123");
  });

  it("returns null for unknown users", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("null")));
    expect(await BackendAdapter().getUser!("missing")).toBeNull();
  });
});
