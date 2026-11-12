import React from "react";

export function ErrorMessage({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div role="alert" className="rounded-2xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200">
      <p className="whitespace-pre-line">{message}</p>
      {onRetry && (
        <button type="button" onClick={onRetry} className="mt-3 rounded-xl bg-white/10 px-4 py-2 font-bold hover:bg-white/20">
          再試行
        </button>
      )}
    </div>
  );
}
