(function () {
  var url = new URL(window.location.href);
  var changed = false;

  ["panel", "success", "error"].forEach(function (key) {
    if (!url.searchParams.has(key)) {
      return;
    }
    url.searchParams.delete(key);
    changed = true;
  });

  if (!changed) {
    return;
  }

  var next = url.pathname + (url.search ? url.search : "") + url.hash;
  window.history.replaceState({}, document.title, next);
})();
