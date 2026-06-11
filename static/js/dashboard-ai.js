(function () {
  var csrfMeta = document.querySelector('meta[name="csrf-token"]');
  var csrfToken = csrfMeta ? csrfMeta.getAttribute("content") || "" : "";
  var searchInput = document.querySelector("[data-dashboard-search]");
  var aiSearchButton = document.querySelector("[data-ai-search]");

  async function postJSON(url, payload) {
    var response = await fetch(url, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
        "X-CSRF-Token": csrfToken
      },
      body: JSON.stringify(payload)
    });
    var data = await response.json().catch(function () {
      return {};
    });
    if (!response.ok) {
      throw new Error(data.error || "AI request failed");
    }
    return data;
  }

  function setButtonBusy(button, busy) {
    if (!button) {
      return;
    }
    button.disabled = busy;
    button.setAttribute("aria-busy", busy ? "true" : "false");
  }

  function aiConfigFromSettings(button) {
    var form = button ? button.closest("form") : document.querySelector("#dashboard-settings-form");
    if (!form) {
      return null;
    }
    var enabledInput = form.querySelector("input[name='ai_enabled']");
    return {
      enabled: enabledInput ? enabledInput.checked : false,
      provider: valueOf(form, "ai_provider"),
      baseURL: valueOf(form, "ai_base_url"),
      apiKey: valueOf(form, "ai_api_key"),
      model: valueOf(form, "ai_model")
    };
  }

  function valueOf(form, name) {
    var input = form.querySelector("[name='" + name + "']");
    return input ? input.value.trim() : "";
  }

  document.addEventListener("click", async function (event) {
    var button = event.target.closest("[data-ai-test]");
    if (!button) {
      return;
    }
    var status = document.querySelector("[data-ai-test-status]");
    var config = aiConfigFromSettings(button);
    if (!config) {
      return;
    }
    event.preventDefault();
    setButtonBusy(button, true);
    if (status) {
      status.textContent = "Testing...";
    }
    try {
      var result = await postJSON("/api/ai/test", config);
      if (status) {
        status.textContent = "OK: " + (result.provider || "AI") + " / " + (result.model || "");
      }
    } catch (error) {
      if (status) {
        status.textContent = error.message || "AI test failed";
      } else {
        window.alert(error.message || "AI test failed");
      }
    } finally {
      setButtonBusy(button, false);
    }
  });

  document.addEventListener("click", async function (event) {
    var button = event.target.closest("[data-ai-enrich]");
    if (!button) {
      return;
    }
    var form = button.closest("form");
    if (!form) {
      return;
    }
    var urlInput = form.querySelector("input[name='url']");
    var titleInput = form.querySelector("input[name='title']");
    var descInput = form.querySelector("input[name='description']");
    var groupInput = form.querySelector("input[name='group_id']");
    var url = urlInput ? urlInput.value.trim() : "";
    if (!url) {
      urlInput && urlInput.focus();
      return;
    }

    event.preventDefault();
    setButtonBusy(button, true);
    try {
      var result = await postJSON("/api/ai/links/enrich", {
        url: url,
        title: titleInput ? titleInput.value : "",
        description: descInput ? descInput.value : "",
        group_id: groupInput ? groupInput.value : ""
      });
      if (titleInput && result.title) {
        titleInput.value = result.title;
      }
      if (descInput && result.description) {
        descInput.value = result.description;
      }
      if (groupInput && result.group_id) {
        groupInput.value = result.group_id;
      }
    } catch (error) {
      window.alert(error.message || "AI request failed");
    } finally {
      setButtonBusy(button, false);
    }
  });

  aiSearchButton && aiSearchButton.addEventListener("click", async function () {
    var query = searchInput ? searchInput.value.trim() : "";
    if (!query) {
      searchInput && searchInput.focus();
      return;
    }
    setButtonBusy(aiSearchButton, true);
    try {
      var result = await postJSON("/api/ai/search", { query: query });
      var ids = new Set(result.ids || []);
      document.querySelectorAll(".dashboard-link[data-link-id]").forEach(function (link) {
        var id = link.getAttribute("data-link-id") || "";
        link.hidden = ids.size > 0 && !ids.has(id);
      });
      document.querySelectorAll(".dashboard-group[data-group-id]").forEach(function (group) {
        var visible = Array.prototype.some.call(group.querySelectorAll(".dashboard-link[data-link-id]"), function (link) {
          return !link.hidden;
        });
        group.hidden = !visible;
      });
    } catch (error) {
      window.alert(error.message || "AI search failed");
    } finally {
      setButtonBusy(aiSearchButton, false);
    }
  });

  searchInput && searchInput.addEventListener("input", function () {
    document.querySelectorAll(".dashboard-link[data-link-id], .dashboard-group[data-group-id]").forEach(function (node) {
      node.hidden = false;
    });
  });
})();
