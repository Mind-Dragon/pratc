import React from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/router", () => ({
  useRouter: () => ({
    pathname: "/"
  })
}));

vi.mock("next/link", () => ({
  default: ({ href, className, children }: { href: string; className?: string; children: React.ReactNode }) => (
    <a className={className} href={href}>
      {children}
    </a>
  )
}));

import Layout from "./Layout";

describe("Layout", () => {
  it("renders the shell heading and children", () => {
    render(
      <Layout description="Scaffold description" eyebrow="Air Traffic Control" title="prATC Dashboard">
        <div>child content</div>
      </Layout>
    );

    expect(screen.getByRole("heading", { name: "prATC Dashboard" })).toBeTruthy();
    expect(screen.getByText("Scaffold description")).toBeTruthy();
    expect(screen.getByText("child content")).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "Primary" })).toBeTruthy();
  });
});
