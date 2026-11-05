(function () {
  "use strict";

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
    var copyButton = document.getElementById("builder-output").parentNode.querySelector(".copy-btn");

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

  setupBuilder();
})();
