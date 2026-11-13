import { describe, expect, it, vi } from "vitest";

import { ApiError, ApiTimeoutError, apiErrorMessage, apiFetch, apiTimeouts } from "./api";

// 応答を返さず、signalで中断されたときだけ失敗するfetch。
function hangingFetch() {
  return vi.fn((_url: string, init: RequestInit) => new Promise<Response>((_resolve, reject) => {
    init.signal?.addEventListener("abort", () => reject(init.signal?.reason));
  }));
}

function respondWith(body: string | null, status: number, contentType?: string) {
  return vi.fn(async () => new Response(body, {
    status,
    headers: contentType ? { "Content-Type": contentType } : undefined,
  }));
}

async function failure(path = "/api/test", init?: Parameters<typeof apiFetch>[1]) {
  return apiFetch(path, init).then(
    () => { throw new Error("apiFetch should have failed"); },
    (error: unknown) => error,
  );
}

describe("apiFetch の待ち時間", () => {
  it("応答が返らないリクエストを指定時間で打ち切り、専用のエラーにする", async () => {
    vi.stubGlobal("fetch", hangingFetch());

    const error = await failure("/api/test", { timeoutMs: 20 });

    expect(error).toBeInstanceOf(ApiTimeoutError);
    expect(error).not.toBeInstanceOf(ApiError);
    expect((error as ApiTimeoutError).timeoutMs).toBe(20);
    expect(apiErrorMessage(error, "fallback")).toBe("サーバーからの応答がありませんでした。通信環境を確認して、もう一度お試しください。");
  });

  it("待ち時間を省略すると既定の上限で打ち切る", async () => {
    vi.useFakeTimers();
    try {
      vi.stubGlobal("fetch", hangingFetch());
      let settled = false;
      const result = failure().finally(() => { settled = true; });

      await vi.advanceTimersByTimeAsync(apiTimeouts.standardMs - 1);
      expect(settled).toBe(false);

      await vi.advanceTimersByTimeAsync(1);
      expect(await result).toBeInstanceOf(ApiTimeoutError);
    } finally {
      vi.useRealTimers();
    }
  });

  it("応答本文の読み取りが止まった場合も打ち切る", async () => {
    vi.stubGlobal("fetch", vi.fn(async (_url: string, init: RequestInit) => new Response(
      new ReadableStream({
        start(controller) {
          init.signal?.addEventListener("abort", () => controller.error(init.signal?.reason));
        },
      }),
      { status: 200, headers: { "Content-Type": "application/json" } },
    )));

    expect(await failure("/api/test", { timeoutMs: 20 })).toBeInstanceOf(ApiTimeoutError);
  });

  it("呼び出し側の中断は時間切れと区別してそのまま返す", async () => {
    vi.stubGlobal("fetch", hangingFetch());
    const controller = new AbortController();

    const result = failure("/api/test", { signal: controller.signal, timeoutMs: 5_000 });
    controller.abort();
    const error = await result;

    expect(error).not.toBeInstanceOf(ApiTimeoutError);
    expect(error).toMatchObject({ name: "AbortError" });
    expect(apiErrorMessage(error, "中断しました")).toBe("中断しました");
  });

  it("AI系は中継とバックエンドの上限より長く待つ", () => {
    expect(apiTimeouts.standardMs).toBe(15_000);
    expect(apiTimeouts.aiOptionalMs).toBeGreaterThan(15_000);
    expect(apiTimeouts.aiLongMs).toBeGreaterThan(120_000);
  });
});

describe("apiFetch のエラー文言", () => {
  it.each([
    { status: 502, body: "<html><head><title>502 Bad Gateway</title></head><body><h1>Bad Gateway</h1></body></html>", contentType: "text/html" },
    { status: 504, body: "<!DOCTYPE html><html><body>Gateway Time-out</body></html>", contentType: "text/html; charset=utf-8" },
    { status: 500, body: "Internal Server Error", contentType: "text/plain" },
    { status: 503, body: "Service Unavailable", contentType: undefined },
  ])("$status のJSONでない本文を画面用の文言に使わない", async ({ status, body, contentType }) => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    vi.stubGlobal("fetch", respondWith(body, status, contentType));

    const error = await failure();

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(status);
    expect((error as ApiError).problem).toBeUndefined();
    const message = apiErrorMessage(error, "fallback");
    expect(message).not.toBe("fallback");
    // HTMLタグや英語の生文言を含まない日本語の文になっている。
    expect(message).not.toMatch(/[A-Za-z<>]/);
    expect(message).toMatch(/[ぁ-ん]/);
    // 本文は調査用にコンソールへ残す。
    expect(consoleError).toHaveBeenCalledTimes(1);
    expect(consoleError.mock.calls[0].join(" ")).toContain(body);
  });

  it.each([
    { status: 422, body: "invalid json", contentType: "application/json" },
    { status: 404, body: "", contentType: "text/plain" },
    { status: 429, body: "{}", contentType: "application/json" },
  ])("$status で詳細を読み取れないときも英語のフォールバックを出さない", async ({ status, body, contentType }) => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    vi.stubGlobal("fetch", respondWith(body, status, contentType));

    const message = apiErrorMessage(await failure(), "fallback");

    expect(message).not.toBe("fallback");
    expect(message).not.toMatch(/[A-Za-z<>]/);
  });

  it("Problem Details の detail と title は従来どおり表示に使う", async () => {
    vi.stubGlobal("fetch", respondWith(
      JSON.stringify({ status: 400, detail: "セット記録の入力を確認してください。", code: "VALIDATION_ERROR" }),
      400,
      "application/problem+json",
    ));
    const withDetail = await failure();
    expect(apiErrorMessage(withDetail, "fallback")).toBe("セット記録の入力を確認してください。");
    expect((withDetail as ApiError).problem).toMatchObject({ code: "VALIDATION_ERROR" });

    vi.stubGlobal("fetch", respondWith(JSON.stringify({ title: "利用できません" }), 503, "application/json"));
    expect(apiErrorMessage(await failure(), "fallback")).toBe("利用できません");
  });

  it("通信障害はそのまま返し、表示には呼び出し側の文言を使う", async () => {
    const networkError = new TypeError("Failed to fetch");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(networkError));

    const error = await failure();

    expect(error).toBe(networkError);
    expect(apiErrorMessage(error, "接続できません")).toBe("接続できません");
  });
});

describe("apiFetch のリクエスト", () => {
  it("Cookie・Content-Type・指定ヘッダーを送り、待ち時間の指定はfetchへ渡さない", async () => {
    const fetchMock = vi.fn<(url: string, init: RequestInit) => Promise<Response>>(async () => Response.json({ id: 42 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(apiFetch("/api/workouts/1/sets", {
      method: "POST",
      body: "{}",
      headers: { "Idempotency-Key": "test-key" },
      timeoutMs: 1_000,
    })).resolves.toEqual({ id: 42 });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:3000/api/workouts/1/sets");
    expect(init).toMatchObject({ method: "POST", body: "{}", credentials: "include" });
    expect(init).not.toHaveProperty("timeoutMs");
    expect(init.signal?.aborted).toBe(false);
    const headers = init.headers as Headers;
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(headers.get("Idempotency-Key")).toBe("test-key");
  });

  it("204は本文を読まず、401だけがセッション失効イベントを発行する", async () => {
    vi.stubGlobal("fetch", respondWith(null, 204));
    await expect(apiFetch("/api/auth/session", { method: "DELETE" })).resolves.toBeUndefined();

    const unauthorized = vi.fn();
    window.addEventListener("fitlog:unauthorized", unauthorized);
    try {
      for (const status of [403, 500, 401]) {
        vi.stubGlobal("fetch", respondWith(JSON.stringify({ status, detail: "拒否" }), status, "application/problem+json"));
        expect(await failure()).toMatchObject({ status });
      }
      expect(unauthorized).toHaveBeenCalledTimes(1);
    } finally {
      window.removeEventListener("fitlog:unauthorized", unauthorized);
    }
  });
});
