/* FOUC-safe theme bootstrap — external so CSP script-src 'self' allows it. */
(function () {
  try {
    var stored = localStorage.getItem("relaycheck-theme");
    var theme =
      stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
    var dark =
      theme === "dark" ||
      (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    if (dark) document.documentElement.classList.add("dark");
    else document.documentElement.classList.remove("dark");
    document.documentElement.style.colorScheme = dark ? "dark" : "light";
  } catch (_) {}
})();
