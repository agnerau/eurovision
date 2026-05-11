const countriesEl = document.getElementById("countries");
const slotsEl = document.getElementById("slots");
const submitBtn = document.getElementById("submitBtn");
const messageEl = document.getElementById("message");

let countries = [];

init();

async function init() {
  countries = await fetchJSON("/api/countries");

  renderCountries(countries);
  renderSlots(countries.length);

  initSortable();

  if (window.EDIT_MODE) {
    const stats = await fetchJSON("/api/my-stats");
    prefill(stats.picks || []);
  }
}

/* -------------------- FETCH -------------------- */

async function fetchJSON(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

/* -------------------- RENDER -------------------- */

function renderCountries(items) {
  countriesEl.innerHTML = "";

  items.forEach(c => {
    const el = document.createElement("div");
    el.className = "country";
    el.dataset.id = c.id;
    el.textContent = c.name;
    countriesEl.appendChild(el);
  });
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
    slotsEl.appendChild(slot);
  }
}

/* -------------------- SORTABLE -------------------- */

function initSortable() {
  // Countries pool (source)
  new Sortable(countriesEl, {
    group: "countries",
    animation: 150,
    forceFallback: true,
    fallbackTolerance: 3,
    fallbackOnBody: true,
    ghostClass: "dragging"
  });

  // EACH SLOT becomes its own drop zone
  document.querySelectorAll(".slot").forEach(slot => {
    new Sortable(slot, {
      group: "countries",
      animation: 150,
      sort: false,

      forceFallback: true,
      fallbackTolerance: 3,
      fallbackOnBody: true
    });
  });
}

/* -------------------- PREFILL -------------------- */

function prefill(picks) {
  picks.forEach(pick => {
    const countryEl = countriesEl.querySelector(`[data-id="${pick.country_id}"]`);
    const slot = slotsEl.children[pick.place - 1];

    if (countryEl && slot) {
      slot.appendChild(countryEl);
    }
  });
}

/* -------------------- SUBMIT -------------------- */

submitBtn.addEventListener("click", async () => {
  const picks = [];

  [...slotsEl.children].forEach((slot, index) => {
    const country = slot.querySelector(".country");

    if (country) {
      picks.push({
        country_id: Number(country.dataset.id),
        place: index + 1
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
    messageEl.textContent = await res.text();
    messageEl.className = "error";
    return;
  }

  messageEl.textContent = "Prediction saved!";
  messageEl.className = "success";

  setTimeout(() => {
    window.location.href = "/home";
  }, 1000);
});