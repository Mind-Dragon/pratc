import React from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/inbox"
}));

vi.mock("next/link", () => ({
  default: ({ href, className, children }: { href: string; className?: string; children: React.ReactNode }) => (
    <a className={className} href={href}>
      {children}
    </a>
  )
}));

import Navigation from "./Navigation";

describe("Navigation", () => {
  it("marks the current route active", () => {
    render(<Navigation />);

    expect(screen.getByRole("link", { name: "Inbox" }).className).toContain("nav-link--active");
    expect(screen.getByRole("link", { name: "Dashboard" }).className).not.toContain("nav-link--active");
  });
});
