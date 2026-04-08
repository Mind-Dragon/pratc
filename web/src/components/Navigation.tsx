"use client";

import React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";

const navItems = [
  { href: "/", label: "Dashboard" },
  { href: "/inbox", label: "Inbox" },
  { href: "/graph", label: "Graph" },
  { href: "/plan", label: "Plan" },
  { href: "/monitor", label: "Monitor" }
];

export default function Navigation() {
  const pathname = usePathname();

  return (
    <nav className="sidebar" aria-label="Primary">
      <div className="sidebar-brand">
        <p>prATC</p>
        <span>PR air traffic control</span>
      </div>
      <ul>
        {navItems.map((item) => {
          const isActive = pathname === item.href;

          return (
            <li key={item.href}>
              <Link className={isActive ? "nav-link nav-link--active" : "nav-link"} href={item.href}>
                {item.label}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
