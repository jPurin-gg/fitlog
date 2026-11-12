# Fitlog frontend

Next.js App Routerで構成したFitlogのWebクライアントです。アプリ全体の起動方法とAPI仕様はルートの [README](../README.md) と [技術仕様](../docs.md) を参照してください。

API呼び出しは `lib/api.ts` に集約しています。ブラウザは認証Cookieを送信し、各画面から `user_id` を渡しません。APIエラーはProblem Detailsから表示用メッセージへ変換します。日付・プラン名表示は `lib/fitlog.ts`、モーダルは `components/Modal.tsx` を共有します。名前付きの `dialog` と `aria-modal`、Escape、Tab の循環、開閉時のフォーカス移動、背景の `inert` とスクロール制御をまとめています。重ねて開いた場合は手前のモーダルだけが操作を受け付けます。

AI利用は `components/AISettings.tsx` の共有状態とトグルで管理し、ユーザー別に保存します。初期値はOFFです。月間プランがなくても記録画面から種目を選んで開始できます。月間案内の当月非表示は `features/dashboard/useMonthlyPromptDismissal.ts` で別に管理し、翌月には案内を再開します。

```sh
npm run dev
npm run lint
npx tsc --noEmit
npm test
npm run test:coverage
npm run build
npm run test:e2e
npm audit
```

ブラウザE2EはAPIをモックします。実APIとDBによる検証はルートREADMEの統合テストを参照してください。ローカルのChromeを使って逐次実行する場合は `E2E_PORT=3002 PLAYWRIGHT_CHANNEL=chrome npm run test:e2e -- --workers=1` を使えます。

単体テストは React Testing Library と MSW を使い、入力欄は label、操作は role とアクセシブルな名前で取得します。`test:coverage` は共通UI・機能画面・hooks・libをV8で計測し、テスト自身を除外して `coverage/` に HTML と LCOV を出力します。

axe-core によりログイン、履歴詳細、各モーダルの構造を単体テストで検査します。jsdomで計測できない色のコントラストは、Playwrightのaxe検査で確認します。ブラウザ検査は `serious`・`critical` の違反があれば失敗します。

E2EはデスクトップChromiumとPixel 7相当（画面サイズ・タッチ操作・モバイル設定）の両方で、手動記録・AI障害時の継続・月間案内・ログイン・設定モーダル・カレンダーの重ねたモーダル・履歴詳細・種目辞書を確認します。初回は `npx playwright install chromium` を実行してください。片方だけ確認するときは `npm run test:e2e -- --project=mobile-chromium` のように指定できます。任意のブラウザ実行ファイルを使う場合は `PLAYWRIGHT_EXECUTABLE_PATH` を指定できます。
