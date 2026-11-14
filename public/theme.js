// Runs before the bundle so the first paint already has the right theme.
// It lives in its own file because the Content-Security-Policy forbids inline scripts.
(function () {
  try {
    var t = localStorage.getItem("theme");
    if (t === "dark" || (t !== "light" && window.matchMedia("(prefers-color-scheme: dark)").matches)) {
      document.documentElement.classList.add("dark");
    }
  } catch (e) {}
})();
