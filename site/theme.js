try {
  var savedTheme = localStorage.getItem("veil-theme");
  if (savedTheme === "light" || savedTheme === "dark") document.documentElement.setAttribute("data-theme", savedTheme);
} catch (e) {}
