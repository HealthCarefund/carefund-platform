import nextCoreWebVitals from "eslint-config-next/core-web-vitals";
import nextTypescript from "eslint-config-next/typescript";

const eslintConfig = [
  ...nextCoreWebVitals,
  ...nextTypescript,
  {
    ignores: [".next/**", "node_modules/**", "dist/**", "next-env.d.ts"],
  },
  {
    settings: {
      // Keep React version detection explicit. This avoids relying on
      // eslint-plugin-react auto-detection APIs that can vary with the
      // installed ESLint version without disabling the lint rules.
      react: { version: "19.3.0" },
    },
  },
];

export default eslintConfig;
