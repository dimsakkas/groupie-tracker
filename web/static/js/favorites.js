// favorites.js - the heart buttons. Favorites live in localStorage, so they
// stay in this browser only. If storage is blocked (private mode, disabled
// cookies) the hearts still work for the current page view.
//
// Other scripts use window.Favorites:
//   Favorites.has(id)  Favorites.count()  and the "favorites:change" event.
(function () {
  'use strict';

  var KEY = 'groupie-tracker:favorites';
  var memory = null; // fallback when localStorage is unavailable

  function load() {
    try {
      var raw = window.localStorage.getItem(KEY);
      var parsed = raw ? JSON.parse(raw) : [];
      return Array.isArray(parsed) ? parsed.map(String) : [];
    } catch (err) {
      return memory || [];
    }
  }

  function save(ids) {
    memory = ids;
    try {
      window.localStorage.setItem(KEY, JSON.stringify(ids));
    } catch (err) {
      /* storage blocked: keep it in memory only */
    }
  }

  var favorites = new Set(load());

  function persist() {
    save(Array.from(favorites));
    document.dispatchEvent(new CustomEvent('favorites:change'));
  }

  function paint(button) {
    var on = favorites.has(button.dataset.id);
    var name = button.dataset.name || 'this artist';
    button.setAttribute('aria-pressed', on ? 'true' : 'false');
    button.setAttribute('aria-label', (on ? 'Remove ' : 'Add ') + name + (on ? ' from favorites' : ' to favorites'));
    button.firstElementChild.textContent = on ? '♥' : '♡';
  }

  function paintAll() {
    document.querySelectorAll('.fav').forEach(paint);
    var counter = document.getElementById('fav-count');
    if (counter) counter.textContent = String(favorites.size);
  }

  document.addEventListener('click', function (e) {
    var button = e.target.closest('.fav');
    if (!button) return;
    var id = button.dataset.id;
    if (favorites.has(id)) favorites.delete(id);
    else favorites.add(id);
    persist();
    paintAll();
  });

  window.Favorites = {
    has: function (id) { return favorites.has(String(id)); },
    count: function () { return favorites.size; }
  };

  paintAll();
})();