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
      // eslint-plugin-react@7.37.5 (pulled in transitively by
      // eslint-config-next 16.3.3) declares peer support only up to
      // ESLint ^9.7. At 10.11.0 its auto-detection of the React version —
      // used by most of its component-tracking rules, not just one or
      // two — calls an ESLint 9 `context.getFilename()` API that ESLint
      // 10 removed, crashing the lint run entirely. There is no newer
      // eslint-plugin-react release yet. Setting the version explicitly
      // here is the plugin's own documented way to skip that detection
      // step altogether, which avoids the crash across every affected
      // rule at once rather than disabling them one at a time as each
      // surfaces.
      react: { version: "19.3.0" },
    },
  },
];

export default eslintConfig;
