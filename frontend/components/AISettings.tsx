"use client";

import React from "react";

interface AISettings {
  enabled: boolean;
  ready: boolean;
  setEnabled: (enabled: boolean) => void;
  isEnabled: () => boolean;
}

const AISettingsContext = React.createContext<AISettings | null>(null);

export function AISettingsProvider({ userID, children }: { userID: number; children: React.ReactNode }) {
  return <UserAISettings key={userID} userID={userID}>{children}</UserAISettings>;
}

function UserAISettings({ userID, children }: { userID: number; children: React.ReactNode }) {
  const storageKey = `fitlog:ai-enabled:${userID}`;
  const [enabled, updateEnabled] = React.useState(false);
  const [ready, setReady] = React.useState(false);
  const enabledRef = React.useRef(false);

  React.useEffect(() => {
    let saved = false;
    try { saved = window.localStorage.getItem(storageKey) === "true"; } catch { /* OFFで利用を継続 */ }
    enabledRef.current = saved;
    updateEnabled(saved);
    setReady(true);

    const sync = (event: StorageEvent) => {
      if (event.key !== storageKey && event.key !== null) return;
      const next = event.key !== null && event.newValue === "true";
      enabledRef.current = next;
      updateEnabled(next);
    };
    window.addEventListener("storage", sync);
    return () => {
      enabledRef.current = false;
      window.removeEventListener("storage", sync);
    };
  }, [storageKey]);

  const setEnabled = React.useCallback((next: boolean) => {
    enabledRef.current = next;
    updateEnabled(next);
    try { window.localStorage.setItem(storageKey, String(next)); } catch { /* この画面では選択を維持 */ }
  }, [storageKey]);
  const isEnabled = React.useCallback(() => enabledRef.current, []);

  return (
    <AISettingsContext.Provider value={{ enabled, ready, setEnabled, isEnabled }}>
      {children}
    </AISettingsContext.Provider>
  );
}

export function useAISettings() {
  const settings = React.useContext(AISettingsContext);
  if (!settings) throw new Error("useAISettings must be used inside AISettingsProvider");
  return settings;
}

export function AIToggle() {
  const { enabled, ready, setEnabled } = useAISettings();
  return (
    <button
      type="button"
      role="switch"
      aria-label="AIを使う"
      aria-checked={enabled}
      disabled={!ready}
      onClick={() => setEnabled(!enabled)}
      className={`inline-flex items-center gap-3 rounded-full border px-4 py-2 text-sm font-bold transition-colors disabled:opacity-50 ${enabled ? "border-primary/40 bg-primary/10 text-primary" : "border-white/20 bg-white/5 text-white/70"}`}
    >
      AI {enabled ? "ON" : "OFF"}
      <span aria-hidden="true" className={`flex h-5 w-9 items-center rounded-full p-0.5 ${enabled ? "justify-end bg-primary" : "justify-start bg-white/25"}`}>
        <span className="h-4 w-4 rounded-full bg-white" />
      </span>
    </button>
  );
}
