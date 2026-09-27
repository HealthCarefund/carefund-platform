import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Avoid perpetually-regenerating, uncommitted AGENTS.md/CLAUDE.md files
  // on every `next dev`/`next build` run.
  agentRules: false,
};

export default nextConfig;
