import { expect, it, vi } from "vitest";
import { ApiError, apiErrorMessage, apiFetch } from "./api";

it("JSON送信にCookie・Content-Type・指定ヘッダーとsignalを引き継ぐ", async () => {
  const fetchMock = vi.fn().mockResolvedValue(Response.json({ id: 42 }));
  vi.stubGlobal("fetch", fetchMock);
  const signal = new AbortController().signal;
  await expect(apiFetch("/api/workouts", { method: "POST", body: "{}", headers: { "Idempotency-Key": "test-key" }, signal })).resolves.toEqual({ id: 42 });
  const [url, init] = fetchMock.mock.calls[0];
  expect(url).toBe("http://localhost:3000/api/workouts");
  expect(init).toMatchObject({ method: "POST", body: "{}", credentials: "include", signal });
  expect(init.headers.get("Content-Type")).toBe("application/json");
  expect(init.headers.get("Idempotency-Key")).toBe("test-key");
});

it("指定したContent-Typeを上書きせず、204はJSONとして読まない", async () => {
  const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
  vi.stubGlobal("fetch", fetchMock);
  await expect(apiFetch("/api/resource", { method: "DELETE", body: "test", headers: { "Content-Type": "text/plain" } })).resolves.toBeUndefined();
  expect(fetchMock.mock.calls[0][1].headers.get("Content-Type")).toBe("text/plain");
});

it.each([
  { body: JSON.stringify({ detail: "入力を確認", title: "Invalid", code: "VALIDATION_ERROR", errors: { reps: "positive" } }), type: "application/problem+json", message: "入力を確認" },
  { body: JSON.stringify({ title: "利用できません" }), type: "application/json", message: "利用できません" },
  { body: "サーバーエラー", type: "text/plain", message: "サーバーエラー" },
  { body: "invalid json", type: "application/json", message: "API request failed (422)" },
  { body: "", type: "text/plain", message: "API request failed (422)" },
])("エラー本文を変換する: $message", async ({ body, type, message }) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(body, { status: 422, headers: { "Content-Type": type } })));
  const error = await apiFetch("/api/test").catch(error => error);
  expect(error).toBeInstanceOf(ApiError);
  if (!(error instanceof ApiError)) throw error;
  expect(error.status).toBe(422);
  expect(apiErrorMessage(error, "fallback")).toBe(message);
  if (type === "application/problem+json") expect(error.problem).toMatchObject({ code: "VALIDATION_ERROR", errors: { reps: "positive" } });
});

it("401だけがセッション失効イベントを発行する", async () => {
  const unauthorized = vi.fn();
  window.addEventListener("fitlog:unauthorized", unauthorized);
  try {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    for (const status of [403, 500, 401]) {
      fetchMock.mockResolvedValueOnce(Response.json({ detail: "拒否" }, { status }));
      await expect(apiFetch("/api/test")).rejects.toMatchObject({ status });
    }
    expect(unauthorized).toHaveBeenCalledOnce();
  } finally {
    window.removeEventListener("fitlog:unauthorized", unauthorized);
  }
});

it("通信障害や中断はそのまま返し、表示には呼び出し側のフォールバックを使う", async () => {
  for (const error of [new TypeError("offline"), new DOMException("aborted", "AbortError")]) {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(error));
    await expect(apiFetch("/api/test")).rejects.toBe(error);
    expect(apiErrorMessage(error, "接続できません")).toBe("接続できません");
  }
  expect(apiErrorMessage(null, "不明なエラー")).toBe("不明なエラー");
});
