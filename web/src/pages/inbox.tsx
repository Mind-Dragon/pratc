import type { GetServerSideProps } from "next";

import TriageView from "../components/TriageView";
import { fetchAnalysis } from "../lib/api";
import type { AnalysisResponse } from "../types/api";

const DEFAULT_REPO = "opencode-ai/opencode";

type InboxProps = {
  analysis: AnalysisResponse | null;
};

export const getServerSideProps: GetServerSideProps<InboxProps> = async (context) => {
  const rawRepo = context.query.repo;
  const repo = typeof rawRepo === "string" && rawRepo.length > 0 ? rawRepo : DEFAULT_REPO;
  const analysis = await fetchAnalysis(repo);
  return {
    props: {
      analysis
    }
  };
};

export default function InboxPage({ analysis }: InboxProps) {
  return (
    <TriageView
      analysis={analysis}
      title="Inbox"
      eyebrow="PR Triage"
      description="Outlook-style sequential triage using live analysis data (read-only actions)."
    />
  );
}
