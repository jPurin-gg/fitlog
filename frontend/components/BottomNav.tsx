"use client";

import React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Home as HomeIcon, Calendar as CalendarIcon, PlusCircle, Library as LibraryIcon } from "lucide-react";

export function BottomNav() {
  const pathname = usePathname();

  const links = [
    { href: "/", label: "ホーム", icon: HomeIcon },
    { href: "/exercises", label: "辞書", icon: LibraryIcon },
    { href: "/workout", label: "記録する", icon: PlusCircle },
    { href: "/calendar", label: "カレンダー", icon: CalendarIcon },
  ];

  return (
    <>
      {/* Spacer so content isn't hidden behind the fixed nav */}
      <div className="h-20 md:h-24 w-full" />
      
      <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-40 w-[calc(100vw-2rem)] max-w-[400px] pb-safe pointer-events-none">
        <nav className="glass bg-black/80 backdrop-blur-xl border border-white/10 rounded-[32px] p-2 flex justify-between items-center pointer-events-auto shadow-2xl shadow-primary/10">
          {links.map((link) => {
            const Icon = link.icon;
            const isActive = pathname === link.href;

            return (
              <Link 
                key={link.href} 
                href={link.href}
                className={`relative flex-1 flex flex-col items-center justify-center py-3 px-2 rounded-[24px] transition-all duration-300 ${
                  isActive 
                    ? "text-primary bg-primary/10" 
                    : "text-white/40 hover:text-white/70 hover:bg-white/5"
                }`}
              >
                {isActive && (
                  <div className="absolute top-0 left-1/2 -translate-x-1/2 w-8 h-1 bg-primary rounded-b-full shadow-[0_0_10px_rgba(255,170,0,0.8)]" />
                )}
                <Icon className={`w-6 h-6 mb-1 ${isActive ? "drop-shadow-[0_0_8px_rgba(255,170,0,0.5)]" : ""}`} />
                <span className="text-[10px] font-bold tracking-wider">{link.label}</span>
              </Link>
            );
          })}
        </nav>
      </div>
    </>
  );
}
