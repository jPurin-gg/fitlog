"use client";

import React from "react";
import { ErrorMessage } from "@/components/ErrorMessage";

export default function ErrorPage({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main className="min-h-screen bg-[#0a0a0a] p-6 text-white flex items-center justify-center">
      <div className="w-full max-w-lg">
        <ErrorMessage message="画面の表示に失敗しました。もう一度お試しください。" onRetry={reset} />
      </div>
    </main>
  );
}
