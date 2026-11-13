// スモークE2E用のAIプロバイダー（OpenAI互換Chat Completions）のスタブ。
// スモークはAI OFFで通る経路だけを使うため、呼ばれたことだけを記録して503を返す。
// プロンプトなどの本文は保存しない。
//   GET    /__stub/health  ヘルスチェック
//   GET    /__stub/calls   呼び出し記録（テストが0件であることを確認する）
//   DELETE /__stub/calls   呼び出し記録の初期化（globalSetup が実行前に呼ぶ）
//   それ以外               AIの呼び出しとして記録し、503を返す
import { createServer } from "node:http";

const port = Number.parseInt(process.env.AI_STUB_PORT ?? "8090", 10);
const calls = [];

function sendJSON(response, status, body) {
  response.writeHead(status, { "Content-Type": "application/json" });
  response.end(JSON.stringify(body));
}

const server = createServer((request, response) => {
  const { pathname } = new URL(request.url ?? "/", "http://ai-stub");
  request.resume();

  if (request.method === "GET" && pathname === "/__stub/health") {
    sendJSON(response, 200, { status: "ok" });
    return;
  }
  if (pathname === "/__stub/calls" && request.method === "GET") {
    sendJSON(response, 200, { calls });
    return;
  }
  if (pathname === "/__stub/calls" && request.method === "DELETE") {
    calls.length = 0;
    response.writeHead(204).end();
    return;
  }

  calls.push({ method: request.method, path: pathname, at: new Date().toISOString() });
  console.log(`AI stub was called: ${request.method} ${pathname}`);
  sendJSON(response, 503, { error: { code: "ai_stub_unavailable", message: "The smoke E2E stack does not provide AI responses." } });
});

server.listen(port, "0.0.0.0", () => {
  console.log(`AI stub listening on port ${port}`);
});

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => {
    server.closeAllConnections();
    server.close(() => process.exit(0));
  });
}
