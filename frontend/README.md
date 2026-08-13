# Fitlog frontend

Next.js App Routerで構成したFitlogのWebクライアントです。アプリ全体の起動方法とAPI仕様はルートの [README](../README.md) と [技術仕様](../docs.md) を参照してください。

API呼び出しは `lib/api.ts` に集約しています。ブラウザは認証Cookieを送信し、各画面から `user_id` を渡しません。APIエラーはProblem Detailsから表示用メッセージへ変換します。日付・プラン名表示は `lib/fitlog.ts`、モーダルのスクロール制御は `hooks/useBodyScrollLock.ts` を共有します。

```sh
npm run dev
npm run lint
npx tsc --noEmit
npm audit
```
