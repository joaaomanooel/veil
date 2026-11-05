(function () {
  "use strict";

  var THEME_KEY = "veil-theme";
  var root = document.documentElement;
  var announcer = document.getElementById("announcer");

  function saveTheme(value) {
    try { localStorage.setItem(THEME_KEY, value); } catch (e) {}
  }

  function announce(message) {
    if (!announcer) return;
    announcer.textContent = "";
    window.setTimeout(function () { announcer.textContent = message; }, 50);
  }

  function setupTheme() {
    var button = document.getElementById("theme-toggle");
    if (!button) return;
    var media = window.matchMedia("(prefers-color-scheme: dark)");

    function current() {
      return root.getAttribute("data-theme") || (media.matches ? "dark" : "light");
    }

    function render() {
      button.setAttribute("aria-pressed", String(current() === "dark"));
    }

    button.addEventListener("click", function () {
      var next = current() === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      saveTheme(next);
      render();
      announce(next === "dark" ? "Dark theme on" : "Light theme on");
    });
    media.addEventListener("change", render);
    render();
    button.hidden = false;
  }

  function copyText(text) {
    if (navigator.clipboard && window.isSecureContext) return navigator.clipboard.writeText(text);
    return new Promise(function (resolve, reject) {
      var area = document.createElement("textarea");
      area.value = text;
      area.setAttribute("readonly", "");
      area.className = "visually-hidden";
      document.body.appendChild(area);
      area.select();
      var ok = false;
      try { ok = document.execCommand("copy"); } catch (e) {}
      document.body.removeChild(area);
      if (ok) resolve(); else reject(new Error("copy failed"));
    });
  }

  function setupCopyButtons() {
    document.querySelectorAll(".code > pre[data-copy]").forEach(function (pre) {
      var wrapper = pre.parentNode;
      var bar = document.createElement("div");
      var title = document.createElement("span");
      var button = document.createElement("button");
      var label = document.createElement("span");
      var context = document.createElement("span");
      button.type = "button";
      button.className = "btn copy-btn";
      label.textContent = "Copy";
      context.className = "visually-hidden";
      context.textContent = " " + pre.getAttribute("data-copy");
      button.appendChild(label);
      button.appendChild(context);
      var timer;
      button.addEventListener("click", function () {
        if (button.getAttribute("aria-disabled") === "true") {
          announce("Fix the errors before copying.");
          return;
        }
        copyText(pre.querySelector("code").textContent).then(function () {
          label.textContent = "Copied";
          button.classList.add("is-copied");
          announce("Copied");
        }, function () {
          label.textContent = "Copy failed";
          announce("Copy failed. Select the text and copy it manually.");
        }).then(function () {
          window.clearTimeout(timer);
          timer = window.setTimeout(function () {
            label.textContent = "Copy";
            button.classList.remove("is-copied");
          }, 2000);
        });
      });
      bar.className = "code-bar";
      title.className = "code-title";
      title.setAttribute("aria-hidden", "true");
      title.textContent = pre.getAttribute("data-copy");
      bar.appendChild(title);
      bar.appendChild(button);
      wrapper.classList.add("has-copy");
      wrapper.insertBefore(bar, pre);
    });
  }

  function setupNav() {
    var button = document.getElementById("nav-toggle");
    if (!button) return;
    var list = document.getElementById("nav-list");

    function setOpen(open) {
      button.setAttribute("aria-expanded", String(open));
      list.classList.toggle("is-open", open);
    }

    button.addEventListener("click", function () {
      setOpen(button.getAttribute("aria-expanded") !== "true");
    });
    list.addEventListener("click", function (event) {
      if (event.target.closest("a")) setOpen(false);
    });
  }

  function setupToc() {
    var links = Array.prototype.slice.call(document.querySelectorAll(".toc a"));
    if (!links.length || !("IntersectionObserver" in window)) return;
    var byId = {};
    var visible = {};
    links.forEach(function (link) { byId[link.hash.slice(1)] = link; });

    function mark(id) {
      links.forEach(function (link) {
        if (link === byId[id]) link.setAttribute("aria-current", "true");
        else link.removeAttribute("aria-current");
      });
    }

    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) { visible[entry.target.id] = entry.isIntersecting; });
      var first = links.map(function (link) { return link.hash.slice(1); }).filter(function (id) { return visible[id]; })[0];
      if (first) mark(first);
    }, { rootMargin: "-15% 0px -55% 0px" });

    Object.keys(byId).forEach(function (id) {
      var section = document.getElementById(id);
      if (section) observer.observe(section);
    });
  }

  function setupReveal() {
    var bands = document.querySelectorAll(".band:not(.hero)");
    if (!bands.length || !("IntersectionObserver" in window)) return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        entry.target.classList.add("is-visible");
        observer.unobserve(entry.target);
      });
    }, { rootMargin: "0px 0px -10% 0px" });
    bands.forEach(function (band) {
      observer.observe(band);
    });
  }

  setupTheme();
  setupCopyButtons();
  setupNav();
  setupToc();
  setupReveal();
})();
