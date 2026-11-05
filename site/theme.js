try {
  var savedTheme = localStorage.getItem("veil-theme");
  if (savedTheme === "light" || savedTheme === "dark") document.documentElement.setAttribute("data-theme", savedTheme);
} catch (e) {}
document.documentElement.classList.add("nav-pending");
window.addEventListener("load", function () {
  if (!document.documentElement.classList.contains("nav-ready")) document.documentElement.classList.remove("nav-pending");
});
