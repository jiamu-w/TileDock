(function () {
  var editorRoot = document.querySelector("[data-dashboard-editor]");
  if (!editorRoot) {
    return;
  }

  var body = document.body;
  var translations = window.panelI18n || {};
  var csrfMeta = document.querySelector('meta[name="csrf-token"]');
  var csrfToken = (csrfMeta && csrfMeta.getAttribute("content")) || translations.csrfToken || "";
  var toggle = editorRoot.querySelector("[data-edit-toggle]");
  var board = editorRoot.querySelector("#dashboard-groups");
  var reorderEndpoint = editorRoot.getAttribute("data-reorder-endpoint");
  var editModeKey = "tiledock.editMode";
  var dragState = { type: "", element: null, placeholder: null };
  var resizeState = null;
  var resizeSettleTimer = null;

  editorRoot.querySelectorAll(".floating-panel").forEach(function (panel) {
    if (panel.parentElement !== document.body) {
      document.body.appendChild(panel);
    }
  });

  function syncCSRFInputs(scope) {
    if (!csrfToken) {
      return;
    }
    (scope || document).querySelectorAll('input[name="_csrf"]').forEach(function (input) {
      input.value = csrfToken;
    });
  }

  function clearDragHints() {
    editorRoot.querySelectorAll(".squeeze-before, .squeeze-after, .is-dragging").forEach(function (node) {
      node.classList.remove("squeeze-before", "squeeze-after", "is-dragging");
    });
    editorRoot.querySelectorAll(".is-drag-source").forEach(function (node) {
      node.classList.remove("is-drag-source");
    });
  }

  function clearDragPlaceholder() {
    if (dragState.placeholder && dragState.placeholder.parentNode) {
      dragState.placeholder.parentNode.removeChild(dragState.placeholder);
    }
    dragState.placeholder = null;
  }

  function triggerResizeSettle(group) {
    if (!group) {
      return;
    }

    group.classList.remove("is-resize-settle");
    window.clearTimeout(resizeSettleTimer);
    void group.offsetWidth;
    group.classList.add("is-resize-settle");
    resizeSettleTimer = window.setTimeout(function () {
      group.classList.remove("is-resize-settle");
    }, 240);
  }

  function ensureGroupPlaceholder(group) {
    if (!group) {
      return null;
    }

    clearDragPlaceholder();
    var placeholder = document.createElement("article");
    placeholder.className = "dashboard-group dashboard-group-placeholder";
    placeholder.setAttribute("aria-hidden", "true");
    placeholder.style.setProperty("--group-cols", group.getAttribute("data-grid-cols") || "10");
    placeholder.style.setProperty("--group-rows", group.getAttribute("data-grid-rows") || "10");
    group.parentNode.insertBefore(placeholder, group.nextSibling);
    dragState.placeholder = placeholder;
    return placeholder;
  }


  function closePanels() {
    document.querySelectorAll(".hidden-panel.is-open").forEach(function (panel) {
      panel.classList.remove("is-open");
    });
  }

  function persistEditMode() {
    try {
      if (body.classList.contains("is-editing")) {
        window.sessionStorage.setItem(editModeKey, "1");
      } else {
        window.sessionStorage.removeItem(editModeKey);
      }
    } catch (_) {}
  }

  function restoreEditMode() {
    try {
      if (window.sessionStorage.getItem(editModeKey) === "1") {
        body.classList.add("is-editing");
      }
    } catch (_) {}
  }

  function syncDraggableState() {
    var editing = body.classList.contains("is-editing");

    editorRoot.querySelectorAll(".dashboard-group").forEach(function (item) {
      item.draggable = editing;
    });

    editorRoot.querySelectorAll(".dashboard-link[data-link-id]").forEach(function (item) {
      item.draggable = editing;
    });

    if (toggle) {
      toggle.textContent = editing
        ? (toggle.getAttribute("data-edit-off") || "Exit Edit Mode")
        : (toggle.getAttribute("data-edit-on") || "Enter Edit Mode");
    }
  }

  function refreshEmptyState() {
    editorRoot.querySelectorAll(".dashboard-links").forEach(function (list) {
      var realLinks = list.querySelectorAll(".dashboard-link[data-link-id]");
      var placeholder = list.querySelector(".empty-placeholder");

      if (realLinks.length === 0 && !placeholder) {
        var item = document.createElement("li");
        item.className = "dashboard-link empty-placeholder muted";
        item.textContent = translations.emptyLink || "No links yet.";
        list.appendChild(item);
      }

      if (realLinks.length > 0 && placeholder) {
        placeholder.remove();
      }

      realLinks.forEach(function (link) {
        var hiddenGroupInput = link.querySelector('input[name="group_id"]');
        if (hiddenGroupInput) {
          hiddenGroupInput.value = list.getAttribute("data-group-id") || "";
        }
      });
    });
  }

  function buildPayload() {
    var groupIDs = Array.from(editorRoot.querySelectorAll(".dashboard-group")).map(function (group) {
      return group.getAttribute("data-group-id");
    });

    var links = [];
    editorRoot.querySelectorAll(".dashboard-links").forEach(function (list) {
      var groupID = list.getAttribute("data-group-id");
      list.querySelectorAll(".dashboard-link[data-link-id]").forEach(function (link) {
        links.push({
          id: link.getAttribute("data-link-id"),
          group_id: groupID
        });
      });
    });

    return { group_ids: groupIDs, links: links };
  }

  function getBoardMetrics() {
    if (!board) {
      return null;
    }

    var style = window.getComputedStyle(board);
    var columns = (style.gridTemplateColumns || "").split(" ").filter(Boolean);
    var columnCount = Math.max(columns.length, 1);
    var gap = parseFloat(style.columnGap || style.gap || "0") || 0;
    var boardWidth = board.clientWidth;
    var cellWidth = columnCount > 0 ? (boardWidth - gap * (columnCount - 1)) / columnCount : boardWidth;
    var rowHeight = parseFloat(style.gridAutoRows || "20") || 20;

    return {
      columnCount: columnCount,
      cellWidth: cellWidth,
      rowHeight: rowHeight,
      gap: gap
    };
  }

  function applyGroupSpan(group, cols, rows) {
    group.setAttribute("data-grid-cols", String(cols));
    group.setAttribute("data-grid-rows", String(rows));
    group.style.setProperty("--group-cols", String(cols));
    group.style.setProperty("--group-rows", String(rows));
  }

  function getMinimumGroupRows(group, metrics, cols) {
    if (!group || !metrics) {
      return 8;
    }

    var nextCols = Math.max(8, Math.min(metrics.columnCount, Math.min(36, cols || 10)));
    var previousCols = group.style.getPropertyValue("--group-cols");
    var rowUnit = Math.max(metrics.rowHeight + metrics.gap, 1);
    var computedStyle = window.getComputedStyle(group);
    var header = group.querySelector(".dashboard-group-header");
    var links = group.querySelector(".dashboard-links");
    var paddingTop = parseFloat(computedStyle.paddingTop || "0") || 0;
    var paddingBottom = parseFloat(computedStyle.paddingBottom || "0") || 0;
    var groupGap = parseFloat(computedStyle.rowGap || computedStyle.gap || "0") || 0;

    group.style.setProperty("--group-cols", String(nextCols));
    var contentHeight =
      paddingTop +
      paddingBottom +
      (header ? header.offsetHeight : 0) +
      (links ? links.scrollHeight : 0) +
      groupGap;

    if (previousCols) {
      group.style.setProperty("--group-cols", previousCols);
    } else {
      group.style.removeProperty("--group-cols");
    }

    var rows = Math.ceil(contentHeight / rowUnit);
    return Math.max(8, Math.min(28, rows));
  }

  function getMinimumGroupCols(group, metrics) {
    if (!group || !metrics) {
      return 8;
    }

    var computedStyle = window.getComputedStyle(group);
    var links = group.querySelector(".dashboard-links");
    var firstLinkItem = links ? links.querySelector(".dashboard-link") : null;
    var paddingLeft = parseFloat(computedStyle.paddingLeft || "0") || 0;
    var paddingRight = parseFloat(computedStyle.paddingRight || "0") || 0;
    var gap = parseFloat(computedStyle.columnGap || computedStyle.gap || "0") || 0;
    var minimumVisibleWidth = firstLinkItem ? firstLinkItem.getBoundingClientRect().width : 174;
    var rowUnit = Math.max(metrics.cellWidth + metrics.gap, 1);
    var cols = Math.ceil((paddingLeft + paddingRight + minimumVisibleWidth + gap) / rowUnit);

    return Math.max(8, Math.min(metrics.columnCount, Math.min(36, cols)));
  }

  function normalizeGroupSpan(group, metrics, cols, rows) {
    var normalizedCols = Math.max(getMinimumGroupCols(group, metrics), Math.min(metrics.columnCount, Math.min(36, cols)));
    var normalizedRows = Math.max(getMinimumGroupRows(group, metrics, normalizedCols), Math.min(28, rows));

    return {
      cols: normalizedCols,
      rows: normalizedRows
    };
  }

  async function persistGroupSize(group) {
    if (!group) {
      return;
    }

    var groupID = group.getAttribute("data-group-id");
    var cols = parseInt(group.getAttribute("data-grid-cols") || "1", 10);
    var rows = parseInt(group.getAttribute("data-grid-rows") || "10", 10);
    if (!groupID || !Number.isFinite(cols) || !Number.isFinite(rows)) {
      return;
    }

    var response = await fetch("/navigation/groups/" + encodeURIComponent(groupID) + "/resize", {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-CSRF-Token": csrfToken
      },
      body: JSON.stringify({ cols: cols, rows: rows })
    });

    if (!response.ok) {
      var payload = await response.json().catch(function () {
        return { error: translations.reorderFailed || "Failed to save layout" };
      });
      window.alert(payload.error || translations.reorderFailed || "Failed to save layout");
      window.location.reload();
    }
  }

  async function persistOrder() {
    if (!reorderEndpoint) {
      return;
    }

    var response = await fetch(reorderEndpoint, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-CSRF-Token": csrfToken
      },
      body: JSON.stringify(buildPayload())
    });

    if (!response.ok) {
      var payload = await response.json().catch(function () {
        return { error: translations.reorderFailed || "Failed to save order" };
      });
      window.alert(payload.error || translations.reorderFailed || "Failed to save order");
      window.location.reload();
    }
  }

  toggle && toggle.addEventListener("click", function () {
    body.classList.toggle("is-editing");
    persistEditMode();
    closePanels();
    syncDraggableState();
    refreshEmptyState();
  });

  document.addEventListener("click", function (event) {
    var panelToggle = event.target.closest("[data-panel-toggle]");
    if (!panelToggle) {
      return;
    }

    var isLinkTile = panelToggle.classList.contains("dashboard-link-tile");
    var targetID = panelToggle.getAttribute("data-panel-toggle");
    var isSettingsPanel = targetID === "dashboard-settings-panel";
    if (!body.classList.contains("is-editing") && !isSettingsPanel) {
      return;
    }

    event.preventDefault();

    var panel = targetID ? document.getElementById(targetID) : null;
    if (!panel) {
      return;
    }

    var isOpen = panel.classList.contains("is-open");
    closePanels();
    if (isOpen) {
      return;
    }

    panel.classList.add("is-open");
    syncCSRFInputs(panel);

    if (isLinkTile) {
      var focusInput = panel.querySelector("input[name='title']");
      if (focusInput) {
        focusInput.focus();
      }
    }
  });

  board && board.addEventListener("dragstart", function (event) {
    if (event.target.closest(".group-resize-handle")) {
      event.preventDefault();
      return;
    }

    var link = event.target.closest(".dashboard-link[data-link-id]");
    if (link && body.classList.contains("is-editing")) {
      dragState.type = "link";
      dragState.element = link;
      link.classList.add("is-dragging");
      event.dataTransfer.effectAllowed = "move";
      return;
    }

    var group = event.target.closest(".dashboard-group");
    if (group && body.classList.contains("is-editing")) {
      dragState.type = "group";
      dragState.element = group;
      group.classList.add("is-dragging");
      group.classList.add("is-drag-source");
      ensureGroupPlaceholder(group);
      event.dataTransfer.effectAllowed = "move";
    }
  });

  document.addEventListener("pointerdown", function (event) {
    var handle = event.target.closest(".group-resize-handle");
    if (!handle || !body.classList.contains("is-editing")) {
      return;
    }

    var group = handle.closest(".dashboard-group");
    var metrics = getBoardMetrics();
    if (!group || !metrics) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();

    resizeState = {
      group: group,
      handle: handle,
      startX: event.clientX,
      startY: event.clientY,
      startCols: parseInt(group.getAttribute("data-grid-cols") || "1", 10) || 1,
      startRows: parseInt(group.getAttribute("data-grid-rows") || "10", 10) || 10,
      metrics: metrics
    };

    group.classList.add("is-resizing");
    handle.setPointerCapture(event.pointerId);
  });

  document.addEventListener("pointermove", function (event) {
    if (!resizeState) {
      return;
    }

    var deltaX = event.clientX - resizeState.startX;
    var deltaY = event.clientY - resizeState.startY;
    var metrics = resizeState.metrics;
    var stepX = Math.max(metrics.cellWidth + metrics.gap, 1);
    var stepY = Math.max(metrics.rowHeight + metrics.gap, 1);
    var nextCols = resizeState.startCols + Math.round(deltaX / stepX);
    var nextRows = resizeState.startRows + Math.round(deltaY / stepY);

    nextCols = Math.max(4, Math.min(metrics.columnCount, Math.min(36, nextCols)));
    nextRows = Math.max(4, Math.min(28, nextRows));

    applyGroupSpan(resizeState.group, nextCols, nextRows);
  });

  document.addEventListener("pointerup", async function (event) {
    if (!resizeState) {
      return;
    }

    var handle = resizeState.handle;
    var group = resizeState.group;
    group.classList.remove("is-resizing");
    if (handle.hasPointerCapture && handle.hasPointerCapture(event.pointerId)) {
      handle.releasePointerCapture(event.pointerId);
    }

    var currentCols = parseInt(group.getAttribute("data-grid-cols") || "10", 10) || 10;
    var currentRows = parseInt(group.getAttribute("data-grid-rows") || "10", 10) || 10;
    var normalized = normalizeGroupSpan(group, resizeState.metrics, currentCols, currentRows);
    applyGroupSpan(group, normalized.cols, normalized.rows);
    triggerResizeSettle(group);

    resizeState = null;
    await persistGroupSize(group);
  });

  board && board.addEventListener("dragover", function (event) {
    if (!body.classList.contains("is-editing")) {
      return;
    }
    event.preventDefault();

    if (dragState.type === "group") {
      var targetGroup = event.target.closest(".dashboard-group");
      if (targetGroup && targetGroup !== dragState.element) {
        editorRoot.querySelectorAll(".dashboard-group").forEach(function (node) {
          if (node !== dragState.element) {
            node.classList.remove("squeeze-before", "squeeze-after");
          }
        });

        var groupRect = targetGroup.getBoundingClientRect();
        var groupInsertAfter = event.clientY > groupRect.top + groupRect.height / 2;
        targetGroup.classList.add(groupInsertAfter ? "squeeze-after" : "squeeze-before");
        targetGroup.classList.remove(groupInsertAfter ? "squeeze-before" : "squeeze-after");
        if (dragState.placeholder) {
          targetGroup.parentNode.insertBefore(dragState.placeholder, groupInsertAfter ? targetGroup.nextSibling : targetGroup);
        }
      }
      return;
    }

    if (dragState.type !== "link") {
      return;
    }

    var list = event.target.closest(".dashboard-links");
    if (!list) {
      return;
    }

    editorRoot.querySelectorAll(".dashboard-link[data-link-id]").forEach(function (node) {
      if (node !== dragState.element) {
        node.classList.remove("squeeze-before", "squeeze-after");
      }
    });

    var targetLink = event.target.closest(".dashboard-link[data-link-id]");
    if (targetLink && targetLink !== dragState.element) {
      var linkRect = targetLink.getBoundingClientRect();
      var linkInsertAfter = event.clientY > linkRect.top + linkRect.height / 2;
      targetLink.classList.add(linkInsertAfter ? "squeeze-after" : "squeeze-before");
      targetLink.classList.remove(linkInsertAfter ? "squeeze-before" : "squeeze-after");
      list.insertBefore(dragState.element, linkInsertAfter ? targetLink.nextSibling : targetLink);
      return;
    }

    list.appendChild(dragState.element);
  });

  board && board.addEventListener("dragend", async function () {
    if (!body.classList.contains("is-editing") || !dragState.element) {
      return;
    }

    if (dragState.type === "group" && dragState.placeholder && dragState.placeholder.parentNode) {
      dragState.placeholder.parentNode.insertBefore(dragState.element, dragState.placeholder);
    }

    refreshEmptyState();
    clearDragHints();
    clearDragPlaceholder();
    dragState.type = "";
    dragState.element = null;
    await persistOrder();
  });

  document.addEventListener("submit", function (event) {
    var form = event.target;
    if (!(form instanceof HTMLFormElement)) {
      return;
    }
    if (!form.action || form.action.indexOf("/navigation/") === -1) {
      return;
    }

    var tokenInput = form.querySelector('input[name="_csrf"]');
    if (!tokenInput) {
      tokenInput = document.createElement("input");
      tokenInput.type = "hidden";
      tokenInput.name = "_csrf";
      form.appendChild(tokenInput);
    }
    tokenInput.value = csrfToken;
  });

  restoreEditMode();
  syncCSRFInputs(document);
  syncDraggableState();
  refreshEmptyState();
})();
