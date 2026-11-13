# 実APIを通すスモークE2E（compose.test.yml の e2e-smoke サービス）。
# Next.js を本番ビルドし、Playwright がテスト開始時に `next start` で起動してブラウザから操作する。
# ブラウザ入りの公式イメージ。タグは frontend/package-lock.json の @playwright/test と同じバージョンにそろえる。
FROM mcr.microsoft.com/playwright:v1.62.1-noble

WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1

COPY ./frontend/package*.json ./
RUN npm ci

COPY ./frontend ./
# /api の中継先（rewrites）は `next build` の時点で確定するため、ビルド前に compose 内のバックエンドを指定する。
# NEXT_PUBLIC_API_URL は空にして、本番と同じくブラウザが同一オリジンの /api を呼ぶ経路にする。
ARG BACKEND_INTERNAL_URL=http://backend:8080
ENV BACKEND_INTERNAL_URL=${BACKEND_INTERNAL_URL}
ENV NEXT_PUBLIC_API_URL=""
RUN npm run build

CMD ["npm", "run", "test:e2e:smoke"]
