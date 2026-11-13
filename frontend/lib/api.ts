export interface ApiProblem {
  type?: string;
  title?: string;
  status: number;
  detail?: string;
  code?: string;
  request_id?: string;
  errors?: Record<string, string>;
}

export class ApiError extends Error {
  readonly status: number;
  readonly problem?: ApiProblem;

  constructor(status: number, message: string, problem?: ApiProblem) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
  }
}

// 制限時間内に応答が返らなかったことを表す。サーバーが処理を終えたかどうかは分からない。
export class ApiTimeoutError extends Error {
  readonly timeoutMs: number;

  constructor(timeoutMs: number) {
    super("サーバーからの応答がありませんでした。通信環境を確認して、もう一度お試しください。");
    this.name = "ApiTimeoutError";
    this.timeoutMs = timeoutMs;
  }
}

// ブラウザが応答を待つ上限。バックエンドと /api 中継（next.config.ts の proxyTimeout）の
// 上限より少し長くし、サーバー側が返すエラーを先に受け取れるようにする。
export const apiTimeouts = {
  // 通常のAPI。
  standardMs: 15_000,
  // 記録に付随するAI（日次調整・次セット提案・AI総評）。AI_OPTIONAL_TIMEOUT_SECONDS（既定15秒）に余裕を足す。
  aiOptionalMs: 25_000,
  // 月間プラン生成・代替種目相談。AIの再試行を含めバックエンドの WriteTimeout（2分）まで応答しうる。
  aiLongMs: 125_000,
};

export interface ApiRequestInit extends RequestInit {
  // 応答を待つ上限（ミリ秒）。省略時は apiTimeouts.standardMs。
  timeoutMs?: number;
}

const apiBase = (process.env.NEXT_PUBLIC_API_URL || "").replace(/\/$/, "");

export async function apiFetch<T>(path: string, init: ApiRequestInit = {}): Promise<T> {
  const { timeoutMs = apiTimeouts.standardMs, signal: callerSignal, ...requestInit } = init;
  const headers = new Headers(requestInit.headers);
  if (requestInit.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  // 呼び出し側の中断と時間切れを1つのsignalにまとめる。
  const controller = new AbortController();
  const abortFromCaller = () => controller.abort(callerSignal?.reason);
  if (callerSignal?.aborted) {
    abortFromCaller();
  } else {
    callerSignal?.addEventListener("abort", abortFromCaller, { once: true });
  }

  // 応答ヘッダーだけでなく本文の読み取りまでを待ち時間に含める。
  // 通信の中断を待たずに打ち切れるよう、時間切れは送信と競わせる。
  let cancelTimeout = () => {};
  const timeout = new Promise<never>((_resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new ApiTimeoutError(timeoutMs));
      controller.abort();
    }, timeoutMs);
    cancelTimeout = () => clearTimeout(timer);
  });

  try {
    const result = await Promise.race([
      send(`${apiBase}${path}`, { ...requestInit, headers, credentials: "include", signal: controller.signal }),
      timeout,
    ]);
    return result as T;
  } finally {
    cancelTimeout();
    callerSignal?.removeEventListener("abort", abortFromCaller);
  }
}

async function send(url: string, init: RequestInit): Promise<unknown> {
  const response = await fetch(url, init);

  if (!response.ok) {
    const problem = await readProblem(response);
    if (response.status === 401 && typeof window !== "undefined") {
      window.dispatchEvent(new Event("fitlog:unauthorized"));
    }
    throw new ApiError(
      response.status,
      problem?.detail || problem?.title || statusMessage(response.status),
      problem,
    );
  }

  if (response.status === 204) {
    return undefined;
  }
  return response.json();
}

export function apiErrorMessage(error: unknown, fallback: string): string {
  if ((error instanceof ApiError || error instanceof ApiTimeoutError) && error.message) {
    return error.message;
  }
  return fallback;
}

async function readProblem(response: Response): Promise<ApiProblem | undefined> {
  const contentType = response.headers.get("Content-Type") || "";
  if (!contentType.includes("json")) {
    // 中継やプロキシが返すHTML・プレーンテキストは画面に出さず、調査用にコンソールへ残す。
    const text = await response.text();
    if (text) {
      console.error(`API error response (${response.status}) was not JSON:`, text.slice(0, 500));
    }
    return undefined;
  }
  try {
    return await response.json() as ApiProblem;
  } catch {
    return undefined;
  }
}

// Problem Detailsを読み取れなかったときに表示する文言。
function statusMessage(status: number): string {
  switch (status) {
    case 401:
      return "ログインの有効期限が切れました。もう一度ログインしてください。";
    case 403:
      return "この操作は許可されていません。";
    case 404:
      return "対象のデータが見つかりませんでした。";
    case 408:
    case 504:
      return "サーバーの応答に時間がかかっています。少し待ってからもう一度お試しください。";
    case 409:
      return "ほかの操作と重なりました。画面を読み込み直してからもう一度お試しください。";
    case 429:
      return "リクエストが集中しています。少し待ってからもう一度お試しください。";
    case 502:
    case 503:
      return "サーバーに接続できませんでした。少し待ってからもう一度お試しください。";
  }
  if (status >= 500) {
    return "サーバーでエラーが発生しました。少し待ってからもう一度お試しください。";
  }
  return "リクエストを処理できませんでした。入力内容を確認して、もう一度お試しください。";
}
