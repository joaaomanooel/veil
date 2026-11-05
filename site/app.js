(function () {
  "use strict";

  var THEME_KEY = "veil-theme";
  var root = document.documentElement;
  var announcer = document.getElementById("announcer");

  function readTheme() {
    try { return localStorage.getItem(THEME_KEY); } catch (e) { return null; }
  }

  function saveTheme(value) {
    try { localStorage.setItem(THEME_KEY, value); } catch (e) {}
  }

  function announce(message) {
    announcer.textContent = "";
    window.setTimeout(function () { announcer.textContent = message; }, 50);
  }

  function setupTheme() {
    var stored = readTheme();
    if (stored === "light" || stored === "dark") root.setAttribute("data-theme", stored);
    var button = document.getElementById("theme-toggle");
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
    var nav = button.parentNode;
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
    nav.classList.add("is-collapsible");
    button.hidden = false;
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

  var SAFE_ARG = /^[A-Za-z0-9_.\/~:@%+=,-]+$/;

  function quote(value) {
    if (SAFE_ARG.test(value)) return value;
    if (value.indexOf("~/") === 0) return "~/" + quote(value.slice(2));
    return "'" + value.replace(/'/g, "'\\''") + "'";
  }

  function trimPath(value) {
    var path = value.trim();
    while (path.length > 1 && path.charAt(path.length - 1) === "/") path = path.slice(0, -1);
    return path;
  }

  function checkPath(value, isWorkspace) {
    var path = trimPath(value);
    if (!path) return null;
    if (path === "/") return { error: "veil refuses / as a workspace or home." };
    if (path === "~" || path === "$HOME" || path === "${HOME}") {
      return { error: "veil refuses your home directory as a workspace or home." };
    }
    var homeRest = path.match(/^(?:~|\$HOME|\$\{HOME\})\/(.+)$/);
    if (homeRest && isWorkspace && homeRest[1].indexOf("/") === -1 && homeRest[1].charAt(0) === ".") {
      return { error: "veil refuses a hidden directory directly under your home as a workspace." };
    }
    if (path.charAt(0) === "/") {
      var parts = path.split("/").filter(Boolean);
      var homeBase = parts[0] === "home" || parts[0] === "Users";
      if ((homeBase && parts.length <= 2) || (parts[0] === "root" && parts.length === 1)) {
        return { note: "veil refuses this path if it is your home directory or one of its parents." };
      }
      if (isWorkspace && homeBase && parts.length === 3 && parts[2].charAt(0) === ".") {
        return { note: "veil refuses this path if it is a hidden directory directly under your home." };
      }
    }
    return null;
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

  function setupBuilder() {
    var form = document.getElementById("builder-form");
    if (!form) return;
    var output = document.querySelector("#builder-output code");
    var status = document.getElementById("builder-status");
    var fields = {
      gateway: document.getElementById("f-gateway"),
      workspace: document.getElementById("f-workspace"),
      home: document.getElementById("f-home"),
      command: document.getElementById("f-command")
    };
    var lastStatus = "";

    function setField(name, result) {
      var input = fields[name];
      var msg = document.getElementById("e-" + name);
      msg.textContent = result ? (result.error || result.note) : "";
      msg.classList.toggle("is-note", Boolean(result && result.note));
      if (result && result.error) input.setAttribute("aria-invalid", "true");
      else input.removeAttribute("aria-invalid");
    }

    function update() {
      var profile = form.querySelector("input[name=profile]:checked").value;
      var gateway = fields.gateway.value.trim();
      var workspace = trimPath(fields.workspace.value);
      var home = trimPath(fields.home.value);
      var command = fields.command.value.trim();
      var results = {};

      if (profile === "gateway" && !gateway) {
        results.gateway = { error: "--gateway HOST:PORT is required with --profile gateway." };
      } else if (profile === "offline" && gateway) {
        results.gateway = { error: "--gateway is allowed only with --profile gateway. Clear it or choose the gateway profile." };
      } else if (gateway && !/^(?:\[[0-9A-Fa-f:.]+\]|[^\s:\[\]]+):\d{1,5}$/.test(gateway)) {
        results.gateway = { error: "Use the form HOST:PORT, for example gateway.local:8118." };
      }
      results.workspace = checkPath(fields.workspace.value, true);
      results.home = checkPath(fields.home.value, false);
      if (!command) results.command = { error: "The command is required. Enter the tool to run, for example opencode." };

      var errors = [];
      Object.keys(fields).forEach(function (name) {
        setField(name, results[name]);
        if (results[name] && results[name].error) errors.push(results[name].error);
      });

      var copyButton = document.getElementById("builder-output").parentNode.querySelector(".copy-btn");
      var message;
      if (errors.length) {
        output.textContent = "Fix the highlighted fields to build the command.";
        message = (errors.length === 1 ? "1 problem: " : errors.length + " problems: ") + errors.join(" ");
        status.classList.add("is-error");
      } else {
        var parts = ["./veil", "run", "--profile", profile];
        if (profile === "gateway") parts.push("--gateway", quote(gateway));
        if (workspace) parts.push("--workspace", quote(workspace));
        if (home) parts.push("--home", quote(home));
        parts.push("--", command);
        output.textContent = parts.join(" ");
        message = lastStatus ? "The command is valid." : "";
        status.classList.remove("is-error");
      }
      if (copyButton) copyButton.setAttribute("aria-disabled", String(errors.length > 0));
      if (message !== lastStatus) {
        status.textContent = message;
        lastStatus = message;
      }
    }

    form.addEventListener("input", update);
    form.addEventListener("change", update);
    form.addEventListener("submit", function (event) { event.preventDefault(); });
    form.hidden = false;
    update();
  }

  setupTheme();
  setupCopyButtons();
  setupNav();
  setupToc();
  setupReveal();
  setupBuilder();
})();
