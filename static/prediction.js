const countriesEl = document.getElementById("countries");
const slotsEl = document.getElementById("slots");
const submitBtn = document.getElementById("submitBtn");
const messageEl = document.getElementById("message");

let countries = [];
let draggedId = null;

init();

async function init() {
  countries = await fetchJSON("/api/countries");

  renderCountries(countries);
  renderSlots(countries.length);

  if (window.EDIT_MODE) {
    const stats = await fetchJSON("/api/my-stats");
    prefill(stats.picks || []);
  }
}

async function fetchJSON(url) {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(await res.text());
  }
  return res.json();
}

function renderCountries(items) {
  countriesEl.innerHTML = "";

  items.forEach(country => {
    countriesEl.appendChild(createCountryEl(country));
  });
}

function createCountryEl(country) {
  const el = document.createElement("div");
  el.className = "country";
  el.draggable = true;
  el.dataset.id = country.id;
  el.textContent = country.name;

  el.addEventListener("dragstart", () => {
    draggedId = country.id;
  });

  el.addEventListener("click", () => {
    const parentSlot = el.closest(".slot");

    if (parentSlot) {
      countriesEl.appendChild(el);
      return;
    }

    const emptySlot = [...document.querySelectorAll(".slot")]
      .find(slot => !slot.querySelector(".country"));

    if (emptySlot) {
      emptySlot.appendChild(el);
    }
  });

  return el;
}

function renderSlots(count) {
  slotsEl.innerHTML = "";

  for (let i = 1; i <= count; i++) {
    const slot = document.createElement("div");
    slot.className = "slot";
    slot.dataset.place = i;

    const label = document.createElement("div");
    label.className = "place-label";
    label.textContent = `#${i}`;

    slot.appendChild(label);

    slot.addEventListener("dragover", e => {
      e.preventDefault();
      slot.classList.add("over");
    });

    slot.addEventListener("dragleave", () => {
      slot.classList.remove("over");
    });

    slot.addEventListener("drop", e => {
      e.preventDefault();
      slot.classList.remove("over");

      if (!draggedId) return;

      const draggedEl = document.querySelector(`.country[data-id="${draggedId}"]`);
      if (!draggedEl) return;

      const existing = slot.querySelector(".country");
      if (existing) {
        countriesEl.appendChild(existing);
      }

      slot.appendChild(draggedEl);
      draggedId = null;
    });

    slotsEl.appendChild(slot);
  }
}

function prefill(picks) {
  picks.forEach(pick => {
    const countryEl = document.querySelector(`.country[data-id="${pick.country_id}"]`);
    const slot = document.querySelector(`.slot[data-place="${pick.place}"]`);

    if (countryEl && slot) {
      const existing = slot.querySelector(".country");
      if (existing) {
        countriesEl.appendChild(existing);
      }

      slot.appendChild(countryEl);
    }
  });
}

submitBtn.addEventListener("click", async () => {
  const picks = [];

  document.querySelectorAll(".slot").forEach(slot => {
    const countryEl = slot.querySelector(".country");

    if (countryEl) {
      picks.push({
        country_id: Number(countryEl.dataset.id),
        place: Number(slot.dataset.place)
      });
    }
  });

  const res = await fetch("/api/my-stats", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({ picks })
  });

  if (!res.ok) {
    messageEl.className = "error";
    messageEl.textContent = await res.text();
    return;
  }

  messageEl.className = "success";
  messageEl.textContent = "Prediction saved!";
  setTimeout(() => {
    window.location.href = "/home";
  }, 1000);
});