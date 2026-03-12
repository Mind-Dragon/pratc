import React from "react";
import Link from "next/link";
import { useRouter } from "next/router";

const navItems = [
  { href: "/", label: "Dashboard" },
  { href: "/triage", label: "Triage" }
];

export default function Navigation() {
  const router = useRouter();

  return (
    <nav className="sidebar" aria-label="Primary">
      <div className="sidebar-brand">
        <p>prATC</p>
        <span>PR air traffic control</span>
      </div>
      <ul>
        {navItems.map((item) => {
          const isActive = router.pathname === item.href;

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
