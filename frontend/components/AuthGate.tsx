"use client";

import React from "react";
import { Loader2, Lock, LogOut, Sparkles } from "lucide-react";

export interface AuthUser {
  id: number;
  nickname: string;
}

interface AuthContextValue {
  user: AuthUser;
  logout: () => void;
}

const AuthContext = React.createContext<AuthContextValue | null>(null);
const STORAGE_KEY = "fitlog_user";

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
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (saved) {
      try {
        const parsed = JSON.parse(saved);
        if (parsed?.id && parsed?.nickname) {
          setUser({ id: parsed.id, nickname: parsed.nickname });
        }
      } catch {
        window.localStorage.removeItem(STORAGE_KEY);
      }
    }
    setReady(true);
  }, []);

  const logout = React.useCallback(() => {
    window.localStorage.removeItem(STORAGE_KEY);
    setUser(null);
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
  const apiUrl = process.env.NEXT_PUBLIC_API_URL || "";

  const login = async (event: React.FormEvent) => {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      const res = await fetch(`${apiUrl}/api/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ nickname, password }),
      });
      if (!res.ok) {
        setError(await res.text() || "ログインに失敗しました。");
        return;
      }
      const data = await res.json();
      const nextUser = { id: data.user_id, nickname: data.nickname };
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(nextUser));
      onLogin(nextUser);
    } catch (e) {
      console.error(e);
      setError("バックエンドに接続できません。");
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
      onClick={logout}
      className="flex-none p-3 glass rounded-2xl hover:bg-white/10 transition-colors"
      aria-label="ログアウト"
    >
      <LogOut className="w-5 h-5 text-white/70" />
    </button>
  );
}
