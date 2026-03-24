(function () {
  var weather = document.querySelector("[data-dashboard-weather]");
  if (!weather) {
    return;
  }

  var tempNode = weather.querySelector(".hero-weather-temp");
  var iconNode = weather.querySelector(".hero-weather-icon");
  var textNode = weather.querySelector(".hero-weather-text");
  var locationNode = weather.querySelector(".hero-weather-location");
  var loadingText = weather.getAttribute("data-loading-text") || "Loading...";
  var unavailableText = weather.getAttribute("data-unavailable-text") || "Unavailable";

  function renderFallback(message) {
    if (tempNode) {
      tempNode.textContent = "--°C";
    }
    if (iconNode) {
      iconNode.textContent = "--";
    }
    if (textNode) {
      textNode.textContent = message;
    }
  }

  function renderWeather(data) {
    if (tempNode) {
      tempNode.textContent = data.temperature || "--°C";
    }
    if (iconNode) {
      iconNode.textContent = data.icon || "--";
    }
    if (textNode) {
      textNode.textContent = data.condition || unavailableText;
    }
    if (locationNode && data.location) {
      locationNode.textContent = data.location;
    }
  }

  function fetchWeather() {
    if (textNode) {
      textNode.textContent = loadingText;
    }

    window.fetch("/api/weather/current", {
      headers: {
        "Accept": "application/json"
      }
    }).then(function (response) {
      if (!response.ok) {
        throw new Error("weather request failed");
      }
      return response.json();
    }).then(function (data) {
      if (!data || data.enabled === false) {
        renderFallback(unavailableText);
        return;
      }
      renderWeather(data);
    }).catch(function () {
      renderFallback(unavailableText);
    });
  }

  fetchWeather();
  window.setInterval(fetchWeather, 10 * 60 * 1000);
})();
