(function () {
  var clock = document.querySelector("[data-dashboard-clock]");
  if (!clock) {
    return;
  }

  var lang = clock.getAttribute("data-lang") || "en";
  var mainTimeNode = clock.querySelector(".hero-clock-main-time");
  var secondsNode = clock.querySelector(".hero-clock-seconds");
  var weekdayNode = clock.querySelector(".hero-clock-weekday");
  var calendarNode = clock.querySelector(".hero-clock-calendar");

  function tick() {
    var now = new Date();

    if (mainTimeNode || secondsNode) {
      var hours = String(now.getHours()).padStart(2, "0");
      var minutes = String(now.getMinutes()).padStart(2, "0");
      var seconds = String(now.getSeconds()).padStart(2, "0");

      if (mainTimeNode) {
        mainTimeNode.textContent = hours + ":" + minutes;
      }
      if (secondsNode) {
        secondsNode.textContent = seconds;
      }
    }

    if (weekdayNode) {
      weekdayNode.textContent = new Intl.DateTimeFormat(lang, { weekday: "long" }).format(now);
    }

    if (calendarNode) {
      calendarNode.textContent = new Intl.DateTimeFormat(lang, {
        year: "numeric",
        month: "2-digit",
        day: "2-digit"
      }).format(now);
    }
  }

  tick();
  window.setInterval(tick, 1000);
})();
