(function () {
  function qs(selector, root) {
    return (root || document).querySelector(selector);
  }

  function qsa(selector, root) {
    return Array.prototype.slice.call((root || document).querySelectorAll(selector));
  }

  function setBusy(button, busyText) {
    if (!button) return function () {};
    var original = button.textContent;
    button.disabled = true;
    button.textContent = busyText || "Working...";
    return function restore() {
      button.disabled = false;
      button.textContent = original;
    };
  }

  qsa("[data-open-modal]").forEach(function (trigger) {
    trigger.addEventListener("click", function () {
      var id = trigger.getAttribute("data-open-modal");
      var modal = qs("#" + id);
      if (modal) modal.classList.add("is-open");
    });
  });

  qsa("[data-close-modal]").forEach(function (trigger) {
    trigger.addEventListener("click", function () {
      var modal = trigger.closest(".modal-backdrop");
      if (modal) modal.classList.remove("is-open");
    });
  });

  qsa(".modal-backdrop").forEach(function (modal) {
    modal.addEventListener("click", function (event) {
      if (event.target === modal) modal.classList.remove("is-open");
    });
  });

  qsa("[data-filter]").forEach(function (button) {
    button.addEventListener("click", function () {
      var group = button.closest("[data-filter-group]") || document;
      var value = button.getAttribute("data-filter");
      qsa("[data-filter]", group).forEach(function (item) {
        item.classList.toggle("active", item === button);
      });
      qsa("[data-status]", group).forEach(function (card) {
        var status = card.getAttribute("data-status") || "";
        card.classList.toggle("is-hidden", value !== "all" && status.indexOf(value) === -1);
      });
    });
  });

  qsa("[data-search-input]").forEach(function (input) {
    var scopeSelector = input.getAttribute("data-search-input");
    var scope = scopeSelector ? qs(scopeSelector) : document;
    input.addEventListener("input", function () {
      var query = input.value.trim().toLowerCase();
      qsa("[data-searchable]", scope).forEach(function (item) {
        var text = item.textContent.toLowerCase();
        item.classList.toggle("is-hidden", query && text.indexOf(query) === -1);
      });
    });
  });

  qsa("[data-tab-target]").forEach(function (tab) {
    tab.addEventListener("click", function () {
      var target = tab.getAttribute("data-tab-target");
      var root = tab.closest("[data-tabs-root]") || document;
      qsa("[data-tab-target]", root).forEach(function (item) {
        item.classList.toggle("active", item === tab);
      });
      qsa("[data-tab-panel]", root).forEach(function (panel) {
        panel.hidden = panel.getAttribute("data-tab-panel") !== target;
      });
    });
  });

  qsa("[data-copy]").forEach(function (button) {
    button.addEventListener("click", function () {
      var targetSelector = button.getAttribute("data-copy");
      var target = targetSelector ? qs(targetSelector) : null;
      var value = target ? target.value || target.textContent : button.getAttribute("data-copy-value") || "";
      var done = function () {
        var original = button.textContent;
        button.textContent = "Copied";
        setTimeout(function () { button.textContent = original; }, 1400);
      };
      if (navigator.clipboard && value) {
        navigator.clipboard.writeText(value).then(done).catch(done);
      } else {
        done();
      }
    });
  });

  qsa("[data-toggle]").forEach(function (button) {
    button.addEventListener("click", function () {
      button.classList.toggle("on");
      button.setAttribute("aria-pressed", button.classList.contains("on") ? "true" : "false");
    });
  });

  qsa("[data-row-action]").forEach(function (button) {
    button.addEventListener("click", function () {
      var row = button.closest("tr");
      var action = button.getAttribute("data-row-action");
      var badge = row ? qs(".badge", row) : null;
      if (!badge) return;
      if (action === "pause") {
        badge.className = "badge paused";
        badge.textContent = "Paused";
      }
      if (action === "resume") {
        badge.className = "badge downloading";
        badge.textContent = "Downloading";
      }
      if (action === "retry") {
        badge.className = "badge processing";
        badge.textContent = "Retry queued";
      }
      if (action === "delete" && row) {
        row.style.opacity = "0.45";
        setTimeout(function () { row.remove(); }, 220);
      }
    });
  });

  var login = qs("[data-login-form]");
  if (login) {
    login.addEventListener("submit", function (event) {
      event.preventDefault();
      var button = qs("button[type='submit']", login);
      var alert = qs("[data-login-alert]");
      var user = qs("[name='username']", login).value.trim();
      var pass = qs("[name='password']", login).value.trim();
      var restore = setBusy(button, "Signing in...");
      if (alert) alert.classList.remove("is-visible");
      setTimeout(function () {
        restore();
        if (user === "admin" && pass === "streamize") {
          if (alert) {
            alert.className = "alert alert-success is-visible";
            alert.textContent = "Signed in. Redirecting to Library...";
          }
        } else if (alert) {
          alert.className = "alert alert-error is-visible";
          alert.textContent = "Invalid credentials. Try admin / streamize for this prototype.";
        }
      }, 700);
    });
  }

  var magnetForm = qs("[data-magnet-form]");
  if (magnetForm) {
    magnetForm.addEventListener("submit", function (event) {
      event.preventDefault();
      var input = qs("[name='magnet']", magnetForm);
      var alert = qs("[data-magnet-alert]");
      var button = qs("button[type='submit']", magnetForm);
      var restore = setBusy(button, "Adding...");
      setTimeout(function () {
        restore();
        if (!input.value.trim().match(/^magnet:\?xt=/)) {
          alert.className = "alert alert-error is-visible";
          alert.textContent = "Paste a valid magnet URL beginning with magnet:?xt=.";
          return;
        }
        alert.className = "alert alert-success is-visible";
        alert.textContent = "Torrent added. Metadata fetch and queue check have started.";
      }, 650);
    });
  }

  var shareForm = qs("[data-share-form]");
  if (shareForm) {
    shareForm.addEventListener("submit", function (event) {
      event.preventDefault();
      var button = qs("button[type='submit']", shareForm);
      var result = qs("[data-share-result]");
      var restore = setBusy(button, "Creating...");
      setTimeout(function () {
        restore();
        if (result) result.classList.add("is-visible");
      }, 600);
    });
  }

  var playerToggles = qsa("[data-player-toggle]");
  var playerStage = qs("[data-player-stage]");
  function setPlayerPlaying(isPlaying) {
    playerToggles.forEach(function (button) {
      button.setAttribute("aria-pressed", isPlaying ? "true" : "false");
      var label = qs("[data-play-label]", button);
      if (label) label.textContent = isPlaying ? "Pause" : "Play";
      else button.textContent = isPlaying ? "Pause" : "Play";
    });
    if (playerStage) playerStage.classList.toggle("is-playing", isPlaying);
  }

  playerToggles.forEach(function (button) {
    button.addEventListener("click", function () {
      setPlayerPlaying(button.getAttribute("aria-pressed") !== "true");
    });
  });

  qsa("[data-video-file]").forEach(function (button) {
    button.addEventListener("click", function () {
      qsa("[data-video-file]").forEach(function (item) {
        item.classList.toggle("active", item === button);
      });
      var title = qs("#player-file-title");
      var meta = qs("#player-file-meta");
      var codec = qs("#player-codec");
      var badge = qs("#player-file-badge");
      if (title) title.textContent = button.getAttribute("data-title") || title.textContent;
      if (meta) meta.textContent = button.getAttribute("data-meta") || meta.textContent;
      if (codec) codec.textContent = button.getAttribute("data-codec") || codec.textContent;
      if (badge) {
        var state = button.getAttribute("data-badge") || "Ready";
        badge.textContent = state;
        badge.className = state === "Buffering" ? "badge warn" : state === "Direct" ? "badge online" : "badge ready";
      }
    });
  });

  qsa("[data-time-range]").forEach(function (range) {
    var output = qs(range.getAttribute("data-time-range"));
    range.addEventListener("input", function () {
      if (output) output.textContent = range.value + "%";
    });
  });
})();
