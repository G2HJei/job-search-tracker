// Small helpers for Jedediah. Loaded as a file because the CSP
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

  // ---- Clicks: remove a repeatable row, edit a field, open a table row ----
  document.addEventListener("click", function (e) {
    var remove = e.target.closest("[data-remove-row]");
    if (remove) {
      var row = remove.closest("[data-row]");
      if (row) row.remove();
      return;
    }
    var spot = e.target.closest("[data-edit-field]");
    if (spot) {
      clickToEdit(e, spot);
      return;
    }
    var link = e.target.closest("tr[data-href]");
    if (link && !e.target.closest("a, button, input, select, textarea, label, form") && !hasSelection()) {
      if (e.ctrlKey || e.metaKey) window.open(link.dataset.href, "_blank", "noopener");
      else window.location.href = link.dataset.href;
    }
  });

  function hasSelection() { return String(window.getSelection()) !== ""; }

  // ---- Click a field on the application page to edit it ----
  // View cards mark rows with data-edit-field (the input to focus). Header
  // dates and part cards (e.g. Interviews) name the section they belong to
  // with data-edit-section.
  var pendingEdit = null;

  function clickToEdit(e, spot) {
    var inner = e.target.closest("a, button, input, select, textarea, summary, label");
    if (inner && inner !== spot) return; // links, buttons and toggles keep working
    var owner = spot.closest("[data-edit-section]");
    var id = owner ? "section-" + owner.dataset.editSection : (spot.closest("article.section") || {}).id;
    var card = id && document.getElementById(id);
    if (!card || typeof htmx === "undefined") return;
    e.preventDefault();
    var field = spot.dataset.editField;
    if (card.classList.contains("editing")) {
      focusField(card, field);
    } else if (spot.tagName === "A") {
      openEdit(card, field);
    } else {
      // Wait briefly so double-clicking to select a word doesn't open the form.
      clearTimeout(pendingEdit);
      pendingEdit = setTimeout(function () {
        if (!hasSelection()) openEdit(card, field);
      }, 250);
    }
  }

  document.addEventListener("dblclick", function () { clearTimeout(pendingEdit); });

  function openEdit(card, field) {
    var edit = card.querySelector(".section-head [hx-get$='/edit']");
    if (!edit) return;
    htmx.ajax("GET", edit.getAttribute("hx-get"), { target: "#" + card.id, swap: "outerHTML" }).then(function () {
      var fresh = document.getElementById(card.id);
      if (fresh) focusField(fresh, field);
    });
  }

  function focusField(card, field) {
    var named = '[name="' + CSS.escape(field) + '"]';
    var input = card.querySelector(named + ":checked") || card.querySelector(named) ||
      card.querySelector("form input:not([type=hidden]), form select, form textarea");
    if (!input) return;
    input.focus();
    if (/^(text|search|url|tel|textarea)$/.test(input.type)) {
      input.setSelectionRange(input.value.length, input.value.length);
    }
  }

  // ---- Edit forms: Ctrl/Cmd+Enter saves, Esc cancels ----
  function snapshot(form) { return new URLSearchParams(new FormData(form)).toString(); }

  if (typeof htmx !== "undefined") {
    htmx.onLoad(function (elt) {
      elt.querySelectorAll(".section.editing > form").forEach(function (form) {
        form.dataset.snapshot = snapshot(form);
      });
    });
  }

  function cancelEdit(card) {
    var form = card.querySelector(":scope > form");
    var cancel = card.querySelector(".cancel-edit");
    if (!form || !cancel) return;
    var changed = form.dataset.snapshot !== undefined && snapshot(form) !== form.dataset.snapshot;
    if (changed && !window.confirm("Discard your changes to this section?")) return;
    cancel.click();
  }

  // ---- Card columns: as many as fit, as evenly filled as possible ----
  // Cards keep their column while they are edited; they are only dealt again
  // when the window size changes the number of columns. Within a column the
  // cards keep page order. Without JavaScript the container is a CSS grid.
  var COLUMN_REM = 25;

  function columnCount(box) {
    var rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
    var gap = parseFloat(getComputedStyle(box).columnGap) || 0;
    return Math.max(1, Math.floor((box.clientWidth + gap) / (COLUMN_REM * rem + gap)));
  }

  function layoutColumns(box, force) {
    var n = columnCount(box);
    if (!force && box.dataset.count === String(n)) return;
    box.dataset.count = n;
    // Cards are found by id because htmx swaps replace the elements.
    if (!box.cardIDs) {
      box.cardIDs = Array.prototype.map.call(box.children, function (el) { return el.id; });
    }
    var cards = box.cardIDs.map(function (id) { return document.getElementById(id); }).filter(Boolean);
    box.classList.add("dealt");
    box.textContent = "";
    var cols = [];
    for (var i = 0; i < n; i++) {
      cols.push(box.appendChild(document.createElement("div")));
      cols[i].className = "card-column";
    }
    // Measure every card at the column width, then pick its column.
    cards.forEach(function (card) { cols[0].appendChild(card); });
    var heights = cards.map(function (card) {
      return card.offsetHeight + (parseFloat(getComputedStyle(card).marginBottom) || 0);
    });
    var where = balance(heights, n);
    // Columns go left to right by their first card in page order.
    var rank = [];
    for (var c = 0; c < n; c++) rank.push(c);
    var first = function (c) { var i = where.indexOf(c); return i < 0 ? Infinity : i; };
    rank.sort(function (a, b) { return first(a) - first(b); });
    cards.forEach(function (card, i) { cols[rank.indexOf(where[i])].appendChild(card); });
  }

  // balance splits heights into n columns, keeping the tallest column as
  // short as it can: longest first into the shortest column, then cards are
  // moved or swapped out of the tallest column while that helps.
  function balance(heights, n) {
    var where = [], sums = [];
    for (var c = 0; c < n; c++) sums.push(0);
    var argmin = function () { return sums.indexOf(Math.min.apply(null, sums)); };
    var byHeight = heights.map(function (_, i) { return i; }).sort(function (a, b) { return heights[b] - heights[a]; });
    byHeight.forEach(function (i) { var c = argmin(); where[i] = c; sums[c] += heights[i]; });
    var put = function (i, c) { sums[where[i]] -= heights[i]; sums[c] += heights[i]; where[i] = c; };
    for (var round = 0; round < 100; round++) {
      var top = sums.indexOf(Math.max.apply(null, sums));
      if (!improve(top)) break;
    }
    return where;

    function improve(top) {
      for (var i = 0; i < heights.length; i++) {
        if (where[i] !== top) continue;
        for (var c = 0; c < n; c++) {
          if (c === top) continue;
          if (sums[c] + heights[i] < sums[top]) { put(i, c); return true; }
          for (var j = 0; j < heights.length; j++) {
            var d = heights[i] - heights[j];
            if (where[j] === c && d > 0 && sums[c] + d < sums[top]) { put(i, c); put(j, top); return true; }
          }
        }
      }
      return false;
    }
  }

  function layoutAll(force) {
    document.querySelectorAll("[data-columns]").forEach(function (box) { layoutColumns(box, force); });
  }
  layoutAll();
  // Card heights change once the heading font has loaded; deal again then.
  if (document.fonts) document.fonts.ready.then(function () { layoutAll(true); });
  var resizeTimer = null;
  window.addEventListener("resize", function () {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(function () { layoutAll(false); }, 150);
  });

  // ---- Upload as soon as a file is picked ----
  document.addEventListener("change", function (e) {
    var form = e.target.type === "file" && e.target.closest("form[data-autosubmit]");
    if (form && e.target.files.length) form.requestSubmit();
  });

  // ---- Confirm before submitting forms marked data-confirm ----
  document.addEventListener("submit", function (e) {
    var msg = e.target.dataset && e.target.dataset.confirm;
    if (msg && !window.confirm(msg)) {
      e.preventDefault();
      e.stopImmediatePropagation();
    }
  }, true);

  // ---- Keyboard: Ctrl+Enter / Esc in edit forms, n = new application, / = search ----
  document.addEventListener("keydown", function (e) {
    var t = e.target;
    var card = t.closest && t.closest(".section.editing");
    if (card) {
      var form = t.closest("form");
      if (form && e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        form.requestSubmit();
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        cancelEdit(card);
        return;
      }
    }
    if (e.ctrlKey || e.metaKey || e.altKey) return;
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
