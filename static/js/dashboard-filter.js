(function () {
  var root = document.querySelector("[data-dashboard-editor]");
  if (!root) {
    return;
  }

  var searchInput = document.querySelector("[data-dashboard-search]");
  var categoryButtons = Array.from(document.querySelectorAll("[data-category-filter]"));
  var groups = Array.from(root.querySelectorAll(".dashboard-group[data-group-id]"));
  var sourceGroups = groups.filter(function (group) {
    return !group.hasAttribute("data-favorites-section");
  });
  var favoritesSection = root.querySelector("[data-favorites-section]");
  var favoritesList = root.querySelector("[data-favorites-list]");
  var favoritesDesc = root.querySelector("[data-favorites-desc]");
  var emptyState = document.querySelector("[data-dashboard-empty]");
  var favoriteCountNode = document.querySelector("[data-favorite-count]");
  var canManageFavorites = root.getAttribute("data-can-manage") === "true";
  var favoriteKey = "tiledock.favoriteLinks";
  var favorites = loadFavorites();
  var lang = document.documentElement.getAttribute("lang") || "en";
  var ticking = false;

  function normalize(value) {
    return String(value || "").toLowerCase().trim();
  }

  function loadFavorites() {
    try {
      var parsed = JSON.parse(window.localStorage.getItem(favoriteKey) || "[]");
      return Array.isArray(parsed) ? parsed : [];
    } catch (_) {
      return [];
    }
  }

  function saveFavorites() {
    try {
      window.localStorage.setItem(favoriteKey, JSON.stringify(favorites));
    } catch (_) {}
  }

  function isFavorite(linkID) {
    return favorites.indexOf(linkID) !== -1;
  }

  function setFavorite(linkID, nextValue) {
    if (!canManageFavorites) {
      return;
    }
    if (!linkID) {
      return;
    }

    if (nextValue && !isFavorite(linkID)) {
      favorites.push(linkID);
    }
    if (!nextValue) {
      favorites = favorites.filter(function (id) {
        return id !== linkID;
      });
    }
    saveFavorites();
    syncFavoriteUI();
    applyFilters();
  }

  function syncFavoriteUI() {
    var count = 0;
    sourceGroups.forEach(function (group) {
      group.querySelectorAll(".dashboard-link[data-link-id]").forEach(function (link) {
        var linkID = link.getAttribute("data-link-id");
        var favorite = isFavorite(linkID);
        var button = link.querySelector("[data-favorite-toggle]");
        link.classList.toggle("is-favorite", favorite);
        if (button) {
          button.setAttribute("aria-pressed", favorite ? "true" : "false");
          button.disabled = !canManageFavorites;
          button.setAttribute("aria-disabled", !canManageFavorites ? "true" : "false");
        }
        if (favorite) {
          count += 1;
        }
      });
    });
    buildFavoritesSection();
    root.querySelectorAll("[data-favorite-proxy-id]").forEach(function (link) {
      var linkID = link.getAttribute("data-link-id");
      var favorite = isFavorite(linkID);
      var button = link.querySelector("[data-favorite-toggle]");
      link.classList.toggle("is-favorite", favorite);
      if (button) {
        button.setAttribute("aria-pressed", favorite ? "true" : "false");
        button.disabled = !canManageFavorites;
        button.setAttribute("aria-disabled", !canManageFavorites ? "true" : "false");
      }
    });
    if (favoriteCountNode) {
      favoriteCountNode.textContent = String(count);
    }
    if (favoritesDesc) {
      favoritesDesc.textContent = count + " " + labelFor("site_count");
    }
  }

  function buildFavoriteItem(sourceLink) {
    var sourceTile = sourceLink.querySelector(".dashboard-link-tile");
    var sourceIcon = sourceLink.querySelector(".link-icon");
    var sourceTitle = sourceLink.querySelector(".link-labels strong");
    var sourceDesc = sourceLink.querySelector(".link-labels .muted");
    var sourceGroup = sourceLink.closest(".dashboard-group[data-group-id]");
    var categoryName = sourceGroup ? sourceGroup.getAttribute("data-category-name") || "" : "";
    var item = document.createElement("li");
    var shell = document.createElement("div");
    var tile = document.createElement("a");
    var icon = sourceIcon ? sourceIcon.cloneNode(true) : document.createElement("span");
    var labels = document.createElement("span");
    var title = document.createElement("strong");
    var detail = document.createElement("div");
    var button = document.createElement("button");
    var linkID = sourceLink.getAttribute("data-link-id") || "";
    var description = sourceDesc ? sourceDesc.textContent.trim() : "";

    item.className = "dashboard-link is-favorite";
    item.setAttribute("data-link-id", linkID);
    item.setAttribute("data-favorite-proxy-id", linkID);
    item.setAttribute("data-search-text", [
      sourceLink.getAttribute("data-search-text") || "",
      categoryName
    ].join(" "));

    shell.className = "link-shell";
    tile.className = "dashboard-link-tile";
    tile.href = sourceTile ? sourceTile.href : "#";
    tile.target = "_blank";
    tile.rel = "noreferrer";
    labels.className = "link-labels";
    title.textContent = sourceTitle ? sourceTitle.textContent.trim() : "";
    button.className = "favorite-toggle";
    button.type = "button";
    button.setAttribute("data-favorite-toggle", "");
    button.setAttribute("aria-label", "Toggle favorite");
    button.setAttribute("aria-pressed", "true");
    button.disabled = !canManageFavorites;
    button.setAttribute("aria-disabled", !canManageFavorites ? "true" : "false");
    button.title = "Favorite";

    labels.appendChild(title);
    if (description) {
      var desc = document.createElement("span");
      desc.className = "muted";
      desc.textContent = description;
      labels.appendChild(desc);
    }
    tile.appendChild(icon);
    tile.appendChild(labels);
    shell.appendChild(tile);
    shell.appendChild(button);
    item.appendChild(shell);
    if (description) {
      detail.className = "link-hover-detail";
      detail.textContent = description;
      item.appendChild(detail);
    }
    return item;
  }

  function buildFavoritesSection() {
    if (!favoritesSection || !favoritesList) {
      return;
    }

    favoritesList.textContent = "";
    var favoriteLinks = [];
    sourceGroups.forEach(function (group) {
      group.querySelectorAll(".dashboard-link[data-link-id]").forEach(function (link) {
        if (isFavorite(link.getAttribute("data-link-id"))) {
          favoriteLinks.push(link);
        }
      });
    });

    favoriteLinks.forEach(function (link) {
      favoritesList.appendChild(buildFavoriteItem(link));
    });
    favoritesSection.hidden = favoriteLinks.length === 0;
  }

  function labelFor(key) {
    var zh = lang === "zh";
    var ja = lang === "ja";
    if (key === "favorites") {
      return zh ? "当前浏览器标记为常用的网站。" : ja ? "This browser's pinned sites." : "Pinned sites from this browser.";
    }
    if (key === "category") {
      return zh ? "当前分类下的网站。" : ja ? "Sites in this category." : "Filtered dashboard links.";
    }
    if (key === "site_count") {
      return zh ? "个网站" : ja ? "サイト" : "sites";
    }
    return "";
  }

  function getButtonLabel(button) {
    var label = button ? button.querySelector("span") : null;
    return label ? label.textContent.trim() : "";
  }

  function setActiveCategory(categoryID) {
    var activeButton = null;
    categoryButtons.forEach(function (button) {
      var active = button.getAttribute("data-category-filter") === categoryID;
      button.classList.toggle("is-active", active);
      button.setAttribute("aria-current", active ? "true" : "false");
      if (active) {
        activeButton = button;
      }
    });

    if (activeButton) {
      activeButton.scrollIntoView({ block: "nearest", inline: "nearest" });
    }
  }

  function applyFilters() {
    var query = normalize(searchInput ? searchInput.value : "");
    var visibleGroups = 0;

    groups.forEach(function (group) {
      var categoryName = group.getAttribute("data-category-name") || "";
      var visibleLinks = 0;
      var isFavoritesSection = group.hasAttribute("data-favorites-section");

      group.querySelectorAll(".dashboard-link[data-link-id]").forEach(function (link) {
        var haystack = normalize([
          link.getAttribute("data-search-text"),
          group.getAttribute("data-search-text"),
          categoryName
        ].join(" "));
        var matchesSearch = !query || haystack.indexOf(query) !== -1;

        link.hidden = !matchesSearch;
        if (matchesSearch) {
          visibleLinks += 1;
        }
      });

      group.hidden = visibleLinks === 0 || (isFavoritesSection && favorites.length === 0);
      if (visibleLinks > 0) {
        visibleGroups += 1;
      }
    });

    if (emptyState) {
      emptyState.hidden = groups.length === 0 || visibleGroups > 0;
    }
    updateActiveFromScroll();
  }

  function scrollWithOffset(target) {
    if (!target) {
      return;
    }

    target.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  function scrollToFavorite() {
    if (favoritesSection && !favoritesSection.hidden) {
      scrollWithOffset(favoritesSection);
      return true;
    }
    return false;
  }

  function updateActiveFromScroll() {
    var visibleGroups = groups.filter(function (group) {
      return !group.hidden;
    });
    if (visibleGroups.length === 0) {
      setActiveCategory("all");
      return;
    }

    var topbar = document.querySelector(".dashboard-topbar");
    var offset = (topbar ? topbar.offsetHeight : 0) + 36;
    var activeGroup = null;

    visibleGroups.forEach(function (group) {
      var rect = group.getBoundingClientRect();
      if (rect.top <= offset && rect.bottom > offset) {
        activeGroup = group;
      } else if (!activeGroup && rect.top > offset) {
        activeGroup = group;
      }
    });

    if (window.scrollY < 24) {
      setActiveCategory("all");
      return;
    }

    if (activeGroup) {
      setActiveCategory(activeGroup.getAttribute("data-group-id") || "all");
    }
  }

  function scheduleScrollSpy() {
    if (ticking) {
      return;
    }
    ticking = true;
    window.requestAnimationFrame(function () {
      ticking = false;
      updateActiveFromScroll();
    });
  }

  categoryButtons.forEach(function (button) {
    button.addEventListener("click", function () {
      var categoryID = button.getAttribute("data-category-filter") || "all";
      setActiveCategory(categoryID);

      if (categoryID === "all") {
        window.scrollTo({ top: 0, behavior: "smooth" });
        return;
      }

      if (categoryID === "favorites") {
        if (!scrollToFavorite()) {
          window.scrollTo({ top: 0, behavior: "smooth" });
        }
        return;
      }

      scrollWithOffset(groups.find(function (group) {
        return group.getAttribute("data-group-id") === categoryID;
      }));
    });
  });

  root.addEventListener("click", function (event) {
    var button = event.target.closest("[data-favorite-toggle]");
    if (!button) {
      return;
    }
    if (!canManageFavorites) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    var link = button.closest(".dashboard-link[data-link-id]");
    var linkID = link ? link.getAttribute("data-link-id") : "";
    setFavorite(linkID, !isFavorite(linkID));
  });

  searchInput && searchInput.addEventListener("input", applyFilters);
  window.addEventListener("scroll", scheduleScrollSpy, { passive: true });
  window.addEventListener("resize", scheduleScrollSpy);

  syncFavoriteUI();
  applyFilters();
  updateActiveFromScroll();
})();
