// filters.js - the filter panel on the home page.
//
// Every change asks /api/filter (debounced 250ms) and hides the cards that
// are not in the answer; the page itself never reloads. The current filters
// are kept in the URL, so the back button from an artist page restores them.
(function () {
  'use strict';

  var form = document.getElementById('filter-form');
  var grid = document.getElementById('artist-grid');
  if (!form || !grid) return;

  var DEBOUNCE_MS = 250;
  var cards = Array.prototype.slice.call(grid.querySelectorAll('.card'));
  var status = document.getElementById('status');
  var loading = document.getElementById('loading');
  var results = document.getElementById('results');
  var empty = document.getElementById('empty');
  var favOnly = document.getElementById('fav-only');
  var locSearch = document.getElementById('loc-search');
  var toggle = document.getElementById('filters-toggle');
  var panel = document.getElementById('filters');
  var total = cards.length;

  var timer = null;
  var requestId = 0;
  var matchingIds = null; // Set of artist ids from the server; null = no filter

  var ranges = {
    creation: { min: el('creation_min'), max: el('creation_max'), out: el('creation-out') },
    album: { min: el('first_album_min'), max: el('first_album_max'), out: el('album-out') }
  };
  var locationBoxes = Array.prototype.slice.call(form.querySelectorAll('input[name="location"]'));
  var countryBoxes = Array.prototype.slice.call(form.querySelectorAll('.country-box'));
  var stateBoxes = Array.prototype.slice.call(form.querySelectorAll('.state-box'));
  var memberBoxes = Array.prototype.slice.call(form.querySelectorAll('input[name="members"]'));

  function el(id) { return document.getElementById(id); }

  // ---- reading the form ----

  function isDefault(input) {
    return input.dataset.end === 'min' ? input.value === input.min : input.value === input.max;
  }

  // Builds the query string. Untouched controls are left out: the server
  // treats a missing parameter as "no filter".
  function buildParams() {
    var p = new URLSearchParams();
    [['creation_min', ranges.creation.min], ['creation_max', ranges.creation.max],
     ['first_album_min', ranges.album.min], ['first_album_max', ranges.album.max]
    ].forEach(function (pair) {
      if (!isDefault(pair[1])) p.set(pair[0], pair[1].value);
    });
    var members = memberBoxes.filter(function (b) { return b.checked; }).map(function (b) { return b.value; });
    if (members.length) p.set('members', members.join(','));
    locationBoxes.forEach(function (b) { if (b.checked) p.append('location', b.value); });
    return p;
  }

  function hasActiveFilters() {
    return buildParams().toString() !== '';
  }

  // ---- writing the form from the URL ----

  function restoreFromUrl() {
    var p = new URLSearchParams(location.search);
    var assign = function (input, key) {
      var v = parseInt(p.get(key), 10);
      if (!isNaN(v)) input.value = String(Math.min(Math.max(v, +input.min), +input.max));
    };
    assign(ranges.creation.min, 'creation_min');
    assign(ranges.creation.max, 'creation_max');
    assign(ranges.album.min, 'first_album_min');
    assign(ranges.album.max, 'first_album_max');

    var members = (p.get('members') || '').split(',');
    memberBoxes.forEach(function (b) { b.checked = members.indexOf(b.value) !== -1; });
    var locs = p.getAll('location');
    locationBoxes.forEach(function (b) { b.checked = locs.indexOf(b.value) !== -1; });
    syncCountryBoxes();
    // Open the countries that have a selected city, so the user can see why.
    countryBoxes.forEach(function (box) {
      var details = box.parentElement.querySelector('details');
      if (details.querySelector('input[name="location"]:checked')) details.open = true;
    });
  }

  function pushUrl() {
    var qs = buildParams().toString();
    history.replaceState(null, '', qs ? '?' + qs : location.pathname);
  }

  // ---- UI helpers ----

  function showRange(group) {
    var r = ranges[group];
    r.out.textContent = r.min.value + ' – ' + r.max.value;
  }

  // Keep "from" <= "to": the slider being moved pushes the other one along.
  function keepOrder(group, moved) {
    var r = ranges[group];
    if (+r.min.value > +r.max.value) {
      if (moved === r.min) r.max.value = r.min.value;
      else r.min.value = r.max.value;
    }
    showRange(group);
  }

  function syncCountryBoxes() {
    countryBoxes.forEach(function (box) {
      var cities = box.parentElement.querySelectorAll('input[name="location"]');
      var checked = Array.prototype.filter.call(cities, function (c) { return c.checked; }).length;
      box.checked = checked === cities.length && cities.length > 0;
      box.indeterminate = checked > 0 && checked < cities.length;
    });
  }

  function syncStateBoxes() {
    stateBoxes.forEach(function (box) {
      var cities = box.parentElement.querySelectorAll('input[name="location"]');
      var checked = Array.prototype.filter.call(cities, function (c) { return c.checked; }).length;
      box.checked = checked === cities.length && cities.length > 0;
      box.indeterminate = checked > 0 && checked < cities.length;
    });
  }

  function applyVisibility() {
    var onlyFavs = favOnly && favOnly.checked && window.Favorites;
    var shown = 0;
    cards.forEach(function (card) {
      var id = card.dataset.id;
      var visible = (matchingIds === null || matchingIds.has(id)) &&
                    (!onlyFavs || window.Favorites.has(id));
      card.hidden = !visible;
      if (visible) shown++;
    });
    status.textContent = shown === total
      ? total + ' artists'
      : 'Showing ' + shown + ' of ' + total + ' artists';
    empty.hidden = shown !== 0;
    document.dispatchEvent(new CustomEvent('filters:change', { detail: { visibleIds: visibleIds() } }));
  }

  function visibleIds() {
    return cards.filter(function (c) { return !c.hidden; }).map(function (c) { return c.dataset.id; });
  }

  function setLoading(on) {
    loading.hidden = !on;
    results.setAttribute('aria-busy', on ? 'true' : 'false');
  }

  // ---- talking to the server ----

  function refresh() {
    pushUrl();
    var qs = buildParams().toString();
    if (qs === '') {
      requestId++; // drop any answer still on its way
      matchingIds = null;
      setLoading(false);
      applyVisibility();
      return;
    }
    var mine = ++requestId;
    setLoading(true);
    fetch('/api/filter?' + qs)
      .then(function (res) {
        if (!res.ok) throw new Error('HTTP ' + res.status);
        return res.json();
      })
      .then(function (data) {
        if (mine !== requestId) return;
        matchingIds = new Set(data.artist_ids.map(String));
        setLoading(false);
        applyVisibility();
      })
      .catch(function () {
        if (mine !== requestId) return;
        setLoading(false);
        status.textContent = 'Could not update the results. Please try again.';
      });
  }

  function scheduleRefresh() {
    clearTimeout(timer);
    timer = setTimeout(refresh, DEBOUNCE_MS);
  }

  function clearAll() {
    form.reset(); // restores the default slider positions and unchecks everything
    ['creation', 'album'].forEach(showRange);
    syncCountryBoxes();
    if (locSearch) { locSearch.value = ''; filterLocationList(''); }
    clearTimeout(timer);
    refresh();
  }

  // ---- the location search box ----

  function filterLocationList(text) {
    var needle = text.trim().toLowerCase();
    form.querySelectorAll('.country').forEach(function (row) {
      var details = row.querySelector('details');
      var country = row.querySelector('.country-name').textContent.toLowerCase();
      var anyCity = false;
      row.querySelectorAll('li').forEach(function (li) {
        var match = needle === '' || country.indexOf(needle) !== -1 ||
                    li.querySelector('.city-name').textContent.toLowerCase().indexOf(needle) !== -1;
        li.hidden = !match;
        if (match) anyCity = true;
      });
      row.hidden = !anyCity;
      if (needle !== '' && anyCity) details.open = true;
    });
  }

  // ---- events ----

  form.addEventListener('input', function (e) {
    var t = e.target;
    if (t.type === 'range') keepOrder(t.dataset.group, t);
  });

  form.addEventListener('change', function (e) {
    var t = e.target;
    if (t === favOnly || t === locSearch) return;
    if (t.classList.contains('country-box') || t.classList.contains('state-box')) {
      t.parentElement.querySelectorAll('input[name="location"]').forEach(function (c) {
        if (!c.closest('li').hidden) c.checked = t.checked;
      });
    }
    syncCountryBoxes();
    syncStateBoxes();
    scheduleRefresh();
  });

  // Dragging a slider fires "input" many times; refresh on every one, debounced.
  form.addEventListener('input', function (e) {
    if (e.target.type === 'range') scheduleRefresh();
  });

  if (favOnly) favOnly.addEventListener('change', applyVisibility);
  document.addEventListener('favorites:change', applyVisibility);
  if (locSearch) locSearch.addEventListener('input', function () { filterLocationList(locSearch.value); });

  document.getElementById('clear-filters').addEventListener('click', clearAll);
  document.getElementById('empty-clear').addEventListener('click', function () {
    if (favOnly) favOnly.checked = false;
    clearAll();
  });

  if (toggle) {
    toggle.addEventListener('click', function () {
      var open = panel.classList.toggle('open');
      toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    });
  }

  // Pressing Enter in a field must not submit (and reload) the form.
  form.addEventListener('submit', function (e) { e.preventDefault(); });

  // ---- start ----
  restoreFromUrl();
  ['creation', 'album'].forEach(showRange);
  if (hasActiveFilters()) {
    if (panel && toggle) { panel.classList.add('open'); toggle.setAttribute('aria-expanded', 'true'); }
    refresh();
  } else {
    applyVisibility();
  }
})();