"use client";

import React from "react";
import { Loader2, Lock, LogOut, Sparkles } from "lucide-react";
import { ApiError, apiErrorMessage, apiFetch } from "@/lib/api";

export interface AuthUser {
  id: number;
  nickname: string;
}

interface AuthContextValue {
  user: AuthUser;
  logout: () => Promise<void>;
}

const AuthContext = React.createContext<AuthContextValue | null>(null);

export function useAuth() {
  const value = React.useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthGate");
  }
  return value;
}

export function AuthGate({ children }: { children: React.ReactNode }) {
  const [user, setUser] = React.useState<AuthUser | null>(null);
  const [ready, setReady] = React.useState(false);

  React.useEffect(() => {
    let active = true;
    apiFetch<AuthUser>("/api/auth/me")
      .then(currentUser => {
        if (active) setUser(currentUser);
      })
      .catch(error => {
        if (!(error instanceof ApiError && error.status === 401)) {
          console.error("Failed to restore session", error);
        }
      })
      .finally(() => {
        if (active) setReady(true);
      });

    const resetSession = () => setUser(null);
    window.addEventListener("fitlog:unauthorized", resetSession);
    return () => {
      active = false;
      window.removeEventListener("fitlog:unauthorized", resetSession);
    };
  }, []);

  const logout = React.useCallback(async () => {
    try {
      await apiFetch<void>("/api/auth/session", { method: "DELETE" });
    } catch (error) {
      console.error("Failed to invalidate session", error);
    } finally {
      setUser(null);
    }
  }, []);

  if (!ready) {
    return (
      <div className="min-h-screen bg-[#0a0a0a] text-white flex items-center justify-center">
        <Loader2 className="w-7 h-7 animate-spin text-primary" />
      </div>
    );
  }

  if (!user) {
    return <LoginScreen onLogin={setUser} />;
  }

  return (
    <AuthContext.Provider value={{ user, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

function LoginScreen({ onLogin }: { onLogin: (user: AuthUser) => void }) {
  const [nickname, setNickname] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const login = async (event: React.FormEvent) => {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      const data = await apiFetch<AuthUser & { created: boolean }>("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ nickname, password }),
      });
      const nextUser = { id: data.id, nickname: data.nickname };
      onLogin(nextUser);
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
          <h1 className="text-3xl font-black tracking-tight mb-2">Fitlog に入る</h1>
          <p className="text-white/50 text-sm">ニックネームとパスワードで始めます。未登録の組み合わせは新しいユーザーとして作成されます。</p>
        </div>

        <form onSubmit={login} className="glass rounded-[32px] p-6 md:p-8 border border-white/10 space-y-5">
          <div className="space-y-2">
            <label className="text-[10px] font-black text-white/40 ml-1">ニックネーム</label>
            <input
              value={nickname}
              onChange={e => setNickname(e.target.value)}
              className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-base"
              placeholder="例: 筋トレマン"
              autoComplete="username"
            />
          </div>

          <div className="space-y-2">
            <label className="text-[10px] font-black text-white/40 ml-1">パスワード</label>
            <input
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              className="w-full bg-black/40 border border-white/10 rounded-2xl px-5 py-4 focus:outline-none focus:border-primary/50 text-base"
              placeholder="パスワード"
              autoComplete="current-password"
            />
          </div>

          {error && (
            <div className="p-4 bg-red-500/10 border border-red-500/30 rounded-2xl">
              <p className="text-sm text-red-200 whitespace-pre-line">{error}</p>
            </div>
          )}

          <button
            type="submit"
            disabled={loading || !nickname.trim() || !password}
            className="w-full py-4 bg-primary text-black font-black rounded-2xl hover:scale-[1.02] active:scale-[0.98] transition-all disabled:opacity-40 disabled:pointer-events-none flex items-center justify-center gap-2"
          >
            {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <>ログイン <Sparkles className="w-5 h-5" /></>}
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
