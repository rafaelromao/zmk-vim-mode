// Loaded blocking from <head>: apply a saved theme before the first paint.
try {
  var saved = window.localStorage.getItem("zvm-theme");
  if (saved === "light" || saved === "dark") document.documentElement.dataset.theme = saved;
} catch (e) {
  // Storage is off: the page follows the system theme.
}
