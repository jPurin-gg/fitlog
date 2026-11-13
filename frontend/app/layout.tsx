import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { BottomNav } from "@/components/BottomNav";
import { AuthGate } from "@/components/AuthGate";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Fitlog",
  description: "筋トレの計画と記録を管理するアプリ",
};

// 画面はダーク固定。ブラウザの UI 色と部品の配色を背景（globals.css の --background）にそろえる。
export const viewport: Viewport = {
  themeColor: "#0a0a0a",
  colorScheme: "dark",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ja">
      {/* 下部ナビ用の余白は BottomNav のスペーサーだけで持つ。各ページのラッパーは flex-1 で
          残りの高さを埋め、内容が1画面に収まるときは縦スクロールが出ないようにする。 */}
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased relative flex min-h-dvh flex-col`}
      >
        <AuthGate>
          {children}
          <BottomNav />
        </AuthGate>
      </body>
    </html>
  );
}
