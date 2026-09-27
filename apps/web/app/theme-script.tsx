// Sets data-theme before hydration so a returning visitor's explicit
// choice never flashes the wrong theme. Falls back to the OS preference
// (handled by the CSS prefers-color-scheme media query) when the visitor
// has never chosen one.
const THEME_INIT_SCRIPT = `
(function () {
  try {
    var stored = window.localStorage.getItem("carefund-theme");
    if (stored === "light" || stored === "dark") {
      document.documentElement.setAttribute("data-theme", stored);
    }
  } catch (e) {}
})();
`;

export function ThemeScript() {
  return <script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT }} />;
}
