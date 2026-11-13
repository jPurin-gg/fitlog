"use client";

import React from "react";
import { Loader2, Lock, LogOut, Sparkles, UserPlus, WifiOff } from "lucide-react";
import { ApiError, apiErrorMessage, apiFetch } from "@/lib/api";
import { AISettingsProvider } from "./AISettings";

export interface AuthUser {
  id: number;
  nickname: string;
}

interface AuthContextValue {
  user: AuthUser;
  /** このセッションで作成したばかりのアカウントかどうか。 */
  isNewAccount: boolean;
  logout: () => Promise<void>;
}

type Session =
  | { status: "checking" }
  // セッション確認が通信失敗や5xxで終わった状態。ログイン済みかどうかは分からない。
  | { status: "unreachable" }
  | { status: "signedOut"; expired: boolean }
  | { status: "signedIn"; user: AuthUser; isNewAccount: boolean; acknowledged: boolean };

const AuthContext = React.createContext<AuthContextValue | null>(null);

export function useAuth() {
  const value = React.useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthGate");
  }
  return value;
}

export function AuthGate({ children }: { children: React.ReactNode }) {
  const [session, setSession] = React.useState<Session>({ status: "checking" });
  const [attempt, setAttempt] = React.useState(0);

  React.useEffect(() => {
    let active = true;
    apiFetch<AuthUser>("/api/auth/me")
      .then(currentUser => {
        if (!active) return;
        setSession({
          status: "signedIn",
          user: { id: currentUser.id, nickname: currentUser.nickname },
          isNewAccount: false,
          acknowledged: true,
        });
      })
      .catch(error => {
        if (!active) return;
        if (error instanceof ApiError && error.status === 401) {
          setSession({ status: "signedOut", expired: false });
          return;
        }
        // 確認に失敗しただけでは未ログインとは限らない。ここでログインフォームを出すと
        // 再入力を促してしまうので、接続の問題として再試行を案内する。
        console.error("Failed to restore session", error);
        setSession({ status: "unreachable" });
      });
    return () => {
      active = false;
    };
  }, [attempt]);

  React.useEffect(() => {
    // ログイン中にどれかのAPIが401を返した＝利用中にセッションが切れた。
    // 未ログイン時の401（パスワード違いなど）は期限切れとして扱わない。
    const expireSession = () => {
      setSession(current => (current.status === "signedIn" ? { status: "signedOut", expired: true } : current));
    };
    window.addEventListener("fitlog:unauthorized", expireSession);
    return () => window.removeEventListener("fitlog:unauthorized", expireSession);
  }, []);

  const retry = React.useCallback(() => {
    setSession({ status: "checking" });
    setAttempt(current => current + 1);
  }, []);

  const logout = React.useCallback(async () => {
    try {
      await apiFetch<void>("/api/auth/session", { method: "DELETE" });
    } catch (error) {
      console.error("Failed to invalidate session", error);
    } finally {
      // 自分でログアウトした場合は、サーバーが既にCookieを拒否していても期限切れとは表示しない。
      setSession({ status: "signedOut", expired: false });
    }
  }, []);

  if (session.status === "checking") {
    return (
      <div className="min-h-screen bg-[#0a0a0a] text-white flex items-center justify-center">
        <Loader2 className="w-7 h-7 animate-spin text-primary" />
      </div>
    );
  }

  if (session.status === "unreachable") {
    return <ConnectionErrorScreen onRetry={retry} />;
  }

  if (session.status === "signedOut") {
    return (
      <LoginScreen
        sessionExpired={session.expired}
        onSignedIn={(user, isNewAccount) => setSession({ status: "signedIn", user, isNewAccount, acknowledged: !isNewAccount })}
      />
    );
  }

  if (!session.acknowledged) {
    return (
      <AccountCreatedScreen
        nickname={session.user.nickname}
        onContinue={() => setSession({ ...session, acknowledged: true })}
        onBackToLogin={() => void logout()}
      />
    );
  }

  return (
    <AuthContext.Provider value={{ user: session.user, isNewAccount: session.isNewAccount, logout }}>
      <AISettingsProvider userID={session.user.id}>{children}</AISettingsProvider>
    </AuthContext.Provider>
  );
}

function ConnectionErrorScreen({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 flex items-center justify-center">
      <main className="w-full max-w-md text-center">
        <div className="w-16 h-16 bg-red-500/10 rounded-2xl flex items-center justify-center mx-auto mb-5">
          <WifiOff className="w-8 h-8 text-red-200" />
        </div>
        <h1 className="text-2xl font-black tracking-tight mb-2">サーバーに接続できません</h1>
        <p role="alert" className="text-white/60 text-sm mb-6">
          ログイン状態を確認できませんでした。通信環境を確認して、もう一度お試しください。
        </p>
        <button
          type="button"
          onClick={onRetry}
          className="w-full py-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all"
        >
          再試行
        </button>
      </main>
    </div>
  );
}

function AccountCreatedScreen({ nickname, onContinue, onBackToLogin }: { nickname: string; onContinue: () => void; onBackToLogin: () => void }) {
  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 flex items-center justify-center">
      <main className="w-full max-w-md text-center">
        <div className="w-16 h-16 bg-primary/20 rounded-2xl flex items-center justify-center mx-auto mb-5">
          <UserPlus className="w-8 h-8 text-primary" />
        </div>
        <h1 className="text-2xl font-black tracking-tight mb-2">アカウントを作成しました</h1>
        <p role="status" className="text-white/80 text-sm mb-3">
          新しいアカウント「{nickname}」を作成しました。記録はまだありません。
        </p>
        <p className="text-white/50 text-sm mb-6">
          以前から使っているのに新しく作成された場合は、ニックネームの入力違いの可能性があります。ログイン画面に戻り、登録済みのニックネームでログインしてください。
        </p>
        <div className="space-y-3">
          <button
            type="button"
            onClick={onContinue}
            className="w-full py-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all"
          >
            はじめる
          </button>
          <button
            type="button"
            onClick={onBackToLogin}
            className="w-full py-3 glass rounded-2xl text-sm font-bold text-white/70 hover:bg-white/10 transition-colors"
          >
            ログイン画面に戻る
          </button>
        </div>
      </main>
    </div>
  );
}

type AuthMode = "login" | "register";

const AUTH_MODES: { value: AuthMode; label: string }[] = [
  { value: "login", label: "ログイン" },
  { value: "register", label: "新規登録" },
];

function LoginScreen({ sessionExpired, onSignedIn }: { sessionExpired: boolean; onSignedIn: (user: AuthUser, isNewAccount: boolean) => void }) {
  const [mode, setMode] = React.useState<AuthMode>("login");
  const [nickname, setNickname] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [passwordConfirmation, setPasswordConfirmation] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");

  const registering = mode === "register";
  const passwordMismatch = registering && passwordConfirmation !== "" && password !== passwordConfirmation;
  const canSubmit = !loading && nickname.trim() !== "" && password !== "" && (!registering || password === passwordConfirmation);

  const switchMode = (nextMode: AuthMode) => {
    setMode(nextMode);
    setPasswordConfirmation("");
    setError("");
  };

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    setLoading(true);
    setError("");
    try {
      // ログインは既存アカウントだけを対象にし、アカウントの作成は新規登録を選んだときだけ行う。
      const data = await apiFetch<AuthUser & { created?: boolean }>(registering ? "/api/auth/register" : "/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ nickname, password }),
      });
      onSignedIn({ id: data.id, nickname: data.nickname }, data.created === true);
    } catch (e) {
      console.error(e);
      setError(apiErrorMessage(e, "バックエンドに接続できません。"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white p-4 flex items-center justify-center relative overflow-hidden">
      <div className="fixed top-[-10%] left-[-10%] w-[40%] h-[40%] bg-primary/10 blur-[120px] rounded-full pointer-events-none" />
      <div className="fixed bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-secondary/10 blur-[120px] rounded-full pointer-events-none" />

      <main className="w-full max-w-md relative z-10">
        <div className="mb-8 text-center">
          <div className="w-16 h-16 bg-primary/20 rounded-2xl flex items-center justify-center mx-auto mb-5">
            <Lock className="w-8 h-8 text-primary" />
          </div>
          <h1 className="text-3xl font-black tracking-tight mb-2">{registering ? "Fitlog に新規登録" : "Fitlog に入る"}</h1>
          <p className="text-white/50 text-sm">
            {registering
              ? "新しいアカウントを作成します。すでに登録済みの方は「ログイン」を選んでください。"
              : "登録済みのニックネームとパスワードでログインします。はじめての方は「新規登録」を選んでください。"}
          </p>
        </div>

        {sessionExpired && (
          <div role="status" className="mb-5 p-4 bg-amber-500/10 border border-amber-500/30 rounded-2xl">
            <p className="text-sm text-amber-100">セッションの有効期限が切れました。もう一度ログインしてください。</p>
          </div>
        )}

        <form onSubmit={submit} className="glass rounded-[32px] p-6 md:p-8 border border-white/10 space-y-5">
          <fieldset>
            <legend className="sr-only">ログインまたは新規登録</legend>
            <div className="grid grid-cols-2 gap-2 rounded-2xl bg-black/40 border border-white/10 p-1">
              {AUTH_MODES.map(option => (
                <label
                  key={option.value}
                  className={`cursor-pointer rounded-xl py-3 text-center text-sm font-black transition-colors focus-within:ring-2 focus-within:ring-primary/60 ${
                    mode === option.value ? "bg-primary text-black" : "text-white/60 hover:text-white"
                  }`}
                >
                  <input
                    type="radio"
                    name="auth-mode"
                    value={option.value}
                    checked={mode === option.value}
                    onChange={() => switchMode(option.value)}
                    className="sr-only"
                  />
                  {option.label}
                </label>
              ))}
            </div>
          </fieldset>

          <div className="space-y-2">
            <label htmlFor="login-nickname" className="text-[10px] font-black text-white/40 ml-1">ニックネーム</label>
            <input
              id="login-nickname"
              value={nickname}
              onChange={e => setNickname(e.target.value)}
              className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-base"
              placeholder="例: 筋トレマン"
              autoComplete="username"
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              aria-describedby={registering ? "login-nickname-hint" : undefined}
            />
            {registering && (
              <p id="login-nickname-hint" className="text-xs text-white/40 ml-1">大文字小文字・全角半角の違いは区別されません。</p>
            )}
          </div>

          <div className="space-y-2">
            <label htmlFor="login-password" className="text-[10px] font-black text-white/40 ml-1">パスワード</label>
            <input
              id="login-password"
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-base"
              placeholder="パスワード"
              autoComplete={registering ? "new-password" : "current-password"}
            />
          </div>

          {registering && (
            <div className="space-y-2">
              <label htmlFor="login-password-confirmation" className="text-[10px] font-black text-white/40 ml-1">パスワード（確認）</label>
              <input
                id="login-password-confirmation"
                type="password"
                value={passwordConfirmation}
                onChange={e => setPasswordConfirmation(e.target.value)}
                className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-base"
                placeholder="もう一度入力"
                autoComplete="new-password"
                aria-invalid={passwordMismatch}
                aria-describedby={passwordMismatch ? "login-password-mismatch" : undefined}
              />
              {passwordMismatch && (
                <p id="login-password-mismatch" className="text-xs text-red-200 ml-1">パスワードが一致しません。</p>
              )}
            </div>
          )}

          {error && (
            <div role="alert" className="p-4 bg-red-500/10 border border-red-500/30 rounded-2xl">
              <p className="text-sm text-red-200 whitespace-pre-line">{error}</p>
            </div>
          )}

          <button
            type="submit"
            disabled={!canSubmit}
            className="w-full py-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-40 disabled:pointer-events-none flex items-center justify-center gap-2"
          >
            {loading ? (
              <Loader2 className="w-5 h-5 animate-spin" />
            ) : registering ? (
              <>アカウントを作成 <UserPlus className="w-5 h-5" /></>
            ) : (
              <>ログイン <Sparkles className="w-5 h-5" /></>
            )}
          </button>
        </form>
      </main>
    </div>
  );
}

export function LogoutButton() {
  const { logout } = useAuth();
  return (
    <button
      onClick={() => void logout()}
      className="flex-none p-3 glass rounded-2xl hover:bg-white/10 transition-colors"
      aria-label="ログアウト"
    >
      <LogOut className="w-5 h-5 text-white/70" />
    </button>
  );
}
