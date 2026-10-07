// Small helpers for Job Search Tracker. Loaded as a file because the CSP
// forbids inline scripts.
(function () {
  "use strict";

  // ---- Toasts: the server sends HX-Trigger: {"toast": "Saved"} ----
  function toast(message, isError) {
    var box = document.getElementById("toasts");
    if (!box || !message) return;
    var el = document.createElement("div");
    el.className = "toast" + (isError ? " error" : "");
    el.setAttribute("role", "status");
    el.textContent = message;
    box.appendChild(el);
    setTimeout(function () { el.classList.add("hide"); }, isError ? 5000 : 2200);
    setTimeout(function () { el.remove(); }, isError ? 5500 : 2700);
  }

  document.body.addEventListener("toast", function (e) {
    toast(e.detail && e.detail.value);
  });

  document.body.addEventListener("htmx:responseError", function (e) {
    var xhr = e.detail.xhr;
    var type = xhr.getResponseHeader("Content-Type") || "";
    var text = type.indexOf("text/plain") === 0 ? xhr.responseText.trim() : "";
    toast(text || "Request failed (" + xhr.status + ")", true);
  });

  document.body.addEventListener("htmx:sendError", function () {
    toast("Could not reach the app. Is it still running?", true);
  });

  // ---- Clicks: remove a repeatable row, fill a date with today ----
  function localToday() {
    var d = new Date();
    var pad = function (n) { return String(n).padStart(2, "0"); };
    return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate());
  }

  document.addEventListener("click", function (e) {
    var remove = e.target.closest("[data-remove-row]");
    if (remove) {
      var row = remove.closest("[data-row]");
      if (row) row.remove();
      return;
    }
    var today = e.target.closest("[data-today]");
    if (today) {
      e.preventDefault();
      var form = today.closest("form");
      var input = form && form.querySelector('input[name="' + CSS.escape(today.dataset.today) + '"]');
      if (input) input.value = localToday();
    }
  });

  // ---- Confirm before submitting forms marked data-confirm ----
  document.addEventListener("submit", function (e) {
    var msg = e.target.dataset && e.target.dataset.confirm;
    if (msg && !window.confirm(msg)) {
      e.preventDefault();
      e.stopImmediatePropagation();
    }
  }, true);

  // ---- Keyboard shortcuts: n = new application, / = search ----
  document.addEventListener("keydown", function (e) {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    var t = e.target;
    if (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)) {
      if (e.key === "Escape") t.blur();
      return;
    }
    if (e.key === "/") {
      var search = document.querySelector("#filters input[type=search]") || document.getElementById("global-search");
      if (search) {
        e.preventDefault();
        search.focus();
        search.select();
      }
    } else if (e.key === "n") {
      e.preventDefault();
      window.location.href = "/applications/new";
    }
  });

  // ---- Kanban board: drag a card to change its status ----
  function updateCounts() {
    document.querySelectorAll(".board-column").forEach(function (col) {
      var count = col.querySelector("[data-count]");
      if (count) count.textContent = col.querySelectorAll(".board-card").length;
    });
  }

  function moveCard(evt) {
    if (evt.from === evt.to) return;
    var id = evt.item.dataset.id;
    var status = evt.to.dataset.status;
    updateCounts();
    fetch("/applications/" + encodeURIComponent(id) + "/status", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded", "HX-Request": "true" },
      body: new URLSearchParams({ status: status, ctx: "board" }),
    }).then(function (r) {
      if (!r.ok) throw new Error(String(r.status));
      var msg = "Status updated";
      try { msg = JSON.parse(r.headers.get("HX-Trigger")).toast || msg; } catch (_) { /* keep default */ }
      toast(msg);
    }).catch(function () {
      toast("Could not change the status. Reloading…", true);
      setTimeout(function () { window.location.reload(); }, 1000);
    });
  }

  if (typeof Sortable !== "undefined") {
    document.querySelectorAll(".board-cards[data-sortable]").forEach(function (el) {
      Sortable.create(el, {
        group: "board",
        sort: false, // order within a column isn't stored
        animation: 150,
        ghostClass: "drag-ghost",
        onEnd: moveCard,
      });
    });
  }
})();
