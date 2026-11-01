// search.js - live suggestions for the header search box (all pages).
//
// The search form is a plain GET form to /search, so everything still works
// without this script. Here we only add: suggestions while typing (debounced
// 250ms), keyboard navigation, and the "/" shortcut that focuses the box.
(function () {
  'use strict';

  var input = document.getElementById('search');
  var list = document.getElementById('suggestions');
  if (!input || !list) return;

  var DEBOUNCE_MS = 250;
  var timer = null;
  var requestId = 0; // ignore answers of outdated requests
  var items = []; // the <a> elements currently shown
  var active = -1;

  // On the results page, show the query that produced them.
  if (location.pathname === '/search') {
    input.value = new URLSearchParams(location.search).get('q') || '';
  }

  var TYPE_LABELS = {
    artist: 'Artist',
    member: 'Member',
    location: 'Location',
    creation_date: 'Creation year',
    first_album_date: 'First album'
  };

  // "los_angeles-usa" -> "Los Angeles, USA"
  function prettyLocation(raw) {
    var i = raw.lastIndexOf('-');
    var title = function (s) {
      return s.replace(/_/g, ' ').replace(/\b\w/g, function (c) { return c.toUpperCase(); });
    };
    if (i < 0) return title(raw);
    var country = raw.slice(i + 1).replace(/_/g, ' ');
    country = country.length <= 3 ? country.toUpperCase() : title(country);
    return title(raw.slice(0, i)) + ', ' + country;
  }

  // Other scripts (maps) reuse the helper.
  window.GT = window.GT || {};
  window.GT.prettyLocation = prettyLocation;

  function hide() {
    list.hidden = true;
    list.textContent = '';
    items = [];
    active = -1;
    input.setAttribute('aria-expanded', 'false');
    input.removeAttribute('aria-activedescendant');
  }

  function show() {
    list.hidden = false;
    input.setAttribute('aria-expanded', 'true');
  }

  function note(text) {
    list.textContent = '';
    var li = document.createElement('li');
    li.className = 'sug-note';
    li.textContent = text;
    list.appendChild(li);
    items = [];
    active = -1;
    show();
  }

  // Artists and members open the artist page; locations and dates open the
  // results page, because they belong to many artists.
  function hrefFor(entry, query) {
    if (entry.type === 'artist' || entry.type === 'member') return '/artist/' + entry.artist_id;
    return '/search?q=' + encodeURIComponent(entry.value);
  }

  function render(entries, query) {
    list.textContent = '';
    items = [];
    active = -1;

    entries.forEach(function (entry, i) {
      var li = document.createElement('li');
      li.setAttribute('role', 'presentation');
      var a = document.createElement('a');
      a.id = 'sug-' + i;
      a.href = hrefFor(entry, query);
      a.setAttribute('role', 'option');

      var badge = document.createElement('span');
      badge.className = 'badge';
      badge.textContent = TYPE_LABELS[entry.type] || entry.type;

      var value = document.createElement('span');
      value.className = 'sug-value';
      value.textContent = entry.type === 'location' ? prettyLocation(entry.value) : entry.value;

      a.appendChild(badge);
      a.appendChild(value);
      if (entry.context) {
        var ctx = document.createElement('span');
        ctx.className = 'sug-context';
        ctx.textContent = entry.context;
        a.appendChild(ctx);
      }
      li.appendChild(a);
      list.appendChild(li);
      items.push(a);
    });

    var all = document.createElement('li');
    all.setAttribute('role', 'presentation');
    var link = document.createElement('a');
    link.id = 'sug-all';
    link.href = '/search?q=' + encodeURIComponent(query);
    link.setAttribute('role', 'option');
    link.textContent = 'See all results for “' + query + '”';
    all.appendChild(link);
    list.appendChild(all);
    items.push(link);

    show();
  }

  function setActive(index) {
    if (items[active]) items[active].classList.remove('active');
    active = index;
    if (items[active]) {
      items[active].classList.add('active');
      input.setAttribute('aria-activedescendant', items[active].id);
      items[active].scrollIntoView({ block: 'nearest' });
    } else {
      input.removeAttribute('aria-activedescendant');
    }
  }

  function fetchSuggestions() {
    var query = input.value.trim();
    if (query === '') {
      hide();
      return;
    }
    var mine = ++requestId;
    note('Searching…'); // loading indicator

    fetch('/api/suggest?q=' + encodeURIComponent(query))
      .then(function (res) {
        if (!res.ok) throw new Error('HTTP ' + res.status);
        return res.json();
      })
      .then(function (entries) {
        if (mine !== requestId) return; // a newer request is on its way
        if (entries.length === 0) note('No suggestions for “' + query + '”');
        else render(entries, query);
      })
      .catch(function () {
        if (mine === requestId) note('Suggestions are unavailable. Press Enter to search.');
      });
  }

  input.addEventListener('input', function () {
    clearTimeout(timer);
    timer = setTimeout(fetchSuggestions, DEBOUNCE_MS);
  });

  input.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
      hide();
      return;
    }
    if (list.hidden || items.length === 0) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((active + 1) % items.length);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive(active <= 0 ? items.length - 1 : active - 1);
    } else if (e.key === 'Enter' && active >= 0) {
      e.preventDefault();
      window.location.href = items[active].href;
    }
  });

  document.addEventListener('click', function (e) {
    if (!e.target.closest('.site-search')) hide();
  });

  // "/" focuses the search box, unless the user is already typing somewhere.
  document.addEventListener('keydown', function (e) {
    if (e.key !== '/' || e.ctrlKey || e.metaKey || e.altKey) return;
    var t = e.target;
    var typing = t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName));
    if (typing) return;
    e.preventDefault();
    input.focus();
    input.select();
  });
})();