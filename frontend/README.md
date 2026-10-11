# Fitlog frontend

Next.js App Routerで構成したFitlogのWebクライアントです。アプリ全体の起動方法とAPI仕様はルートの [README](../README.md) と [技術仕様](../docs.md) を参照してください。

API呼び出しは `lib/api.ts` に集約しています。ブラウザは認証Cookieを送信し、各画面から `user_id` を渡しません。APIエラーはProblem Detailsから表示用メッセージへ変換します。日付・プラン名表示は `lib/fitlog.ts`、モーダルのスクロール制御は `hooks/useBodyScrollLock.ts` を共有します。

AI利用は `components/AISettings.tsx` の共有状態とトグルで管理し、ユーザー別に保存します。初期値はOFFです。月間プランがなくても記録画面から種目を選んで開始できます。月間案内の当月非表示は `features/dashboard/useMonthlyPromptDismissal.ts` で別に管理し、翌月には案内を再開します。

```sh
npm run dev
npm run lint
npx tsc --noEmit
npm test
npm run build
npm run test:e2e
npm audit
```

ブラウザE2EはAPIをモックします。実APIとDBによる検証はルートREADMEの統合テストを参照してください。ローカルのChromeを使って逐次実行する場合は `E2E_PORT=3002 PLAYWRIGHT_CHANNEL=chrome npm run test:e2e -- --workers=1` を使えます。
