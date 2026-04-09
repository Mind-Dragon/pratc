"use client";

import React from "react";
import type { PropsWithChildren } from "react";

import Navigation from "./Navigation";

type LayoutProps = PropsWithChildren<{
  title: string;
  eyebrow: string;
  description: string;
}>;

export default function Layout({ title, eyebrow, description, children }: LayoutProps) {
  return (
    <div className="app-shell">
      <Navigation />
      <main className="app-main">
        <header className="page-header">
          <p className="hero-kicker">{eyebrow}</p>
          <h1>
            {title}
            {title.includes("Monitor") && (
              <span className="live-indicator" style={{
                marginLeft: 12,
                padding: "4px 10px",
                borderRadius: 999,
                background: "rgba(0, 255, 157, 0.15)",
                color: "var(--green)",
                fontSize: "0.72rem",
                fontWeight: 700,
                textTransform: "uppercase",
                letterSpacing: "0.08em",
              }}>
                ● Live
              </span>
            )}
          </h1>
          <p className="page-description">{description}</p>
        </header>
        {children}
      </main>
    </div>
  );
}
