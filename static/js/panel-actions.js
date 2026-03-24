document.addEventListener("click", async function (event) {
  var translations = window.panelI18n || {};
  var csrfMeta = document.querySelector('meta[name="csrf-token"]');
  var csrfToken = (csrfMeta && csrfMeta.getAttribute("content")) || translations.csrfToken || "";
  var trigger = event.target.closest("[data-delete]");
  if (!trigger) {
    return;
  }

  var dashboardEditor = trigger.closest("[data-dashboard-editor]");
  if (dashboardEditor && !document.body.classList.contains("is-editing")) {
    event.preventDefault();
    return;
  }

  event.preventDefault();
  var target = trigger.getAttribute("data-delete");
  if (!target || !window.confirm(translations.confirmDelete || "Delete this item?")) {
    return;
  }

  var response = await fetch(target, {
    method: "DELETE",
    credentials: "same-origin",
    headers: {
      Accept: "application/json",
      "X-CSRF-Token": csrfToken
    }
  });

  if (!response.ok) {
    var payload = await response.json().catch(function () {
      return { error: translations.deleteFailed || "Delete failed" };
    });
    window.alert(payload.error || translations.deleteFailed || "Delete failed");
    return;
  }

  window.location.reload();
});
