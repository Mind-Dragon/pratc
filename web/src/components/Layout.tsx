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
          <h1>{title}</h1>
          <p className="page-description">{description}</p>
        </header>
        {children}
      </main>
    </div>
  );
}
