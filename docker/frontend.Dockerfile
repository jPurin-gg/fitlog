# docker/frontend.Dockerfile
FROM node:22

# Next.js を /app に展開
WORKDIR /app

# 開発サーバを 0.0.0.0 で待ち受け（コンテナ外からアクセス可能にするため）
# ※ package.json の dev スクリプトが `next dev` の場合、
#   可能なら `next dev -p 3000 -H 0.0.0.0` にしておくと確実
EXPOSE 3000

# docker-compose 側で `npm run dev` を実行する（command 指定）
# CMD ["npm", "run", "dev"]
