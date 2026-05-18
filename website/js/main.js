// Mobile hamburger menu
var navHamburger = document.getElementById('nav-hamburger');
var navLinks = document.getElementById('nav-links');

function closeNav() {
  navLinks.classList.remove('open');
  navHamburger.setAttribute('aria-expanded', 'false');
}

navHamburger.addEventListener('click', function (e) {
  e.stopPropagation();
  var open = navLinks.classList.toggle('open');
  this.setAttribute('aria-expanded', open);
});

document.addEventListener('click', function (e) {
  if (!navLinks.classList.contains('open')) return;
  if (navLinks.contains(e.target) || navHamburger.contains(e.target)) return;
  closeNav();
});

navLinks.querySelectorAll('a').forEach(function (link) {
  link.addEventListener('click', closeNav);
});

document.getElementById('install-copy').addEventListener('click', function () {
  var text = 'brew install --cask mdelapenya/tap/biomelab';
  navigator.clipboard.writeText(text).then(function () {
    var box = document.getElementById('install-copy');
    box.classList.add('copied');
    var icon = box.querySelector('.copy-icon');
    var orig = icon.innerHTML;
    icon.innerHTML = '<path d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z"/>';
    setTimeout(function () { box.classList.remove('copied'); icon.innerHTML = orig; }, 2000);
  });
});

// Preload alternate theme images so swaps are instant (no flash).
var preloaded = {};
document.querySelectorAll('.themed-img').forEach(function (img) {
  ['srcDark', 'srcLight'].forEach(function (key) {
    var url = img.dataset[key];
    if (url && !preloaded[url]) {
      preloaded[url] = true;
      var link = document.createElement('link');
      link.rel = 'prefetch';
      link.as = 'image';
      link.href = url;
      document.head.appendChild(link);
    }
  });
});

function applyThemeImages(theme) {
  document.querySelectorAll('.themed-img').forEach(function (img) {
    var src = theme === 'light' ? img.dataset.srcLight : img.dataset.srcDark;
    if (src) img.setAttribute('src', src);
  });
}
applyThemeImages(document.documentElement.getAttribute('data-theme'));

document.getElementById('theme-toggle').addEventListener('click', function () {
  var html = document.documentElement;
  var next = html.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
  html.setAttribute('data-theme', next);
  localStorage.setItem('theme', next);
  applyThemeImages(next);
});

fetch('https://api.github.com/repos/mdelapenya/biomelab')
  .then(function (r) { return r.json(); })
  .then(function (data) {
    if (data.stargazers_count !== undefined) {
      document.getElementById('star-count').textContent = data.stargazers_count.toLocaleString();
    }
  })
  .catch(function () { });

// Copy buttons on install tab panels
document.querySelectorAll('.copy-btn').forEach(function (btn) {
  btn.addEventListener('click', function () {
    navigator.clipboard.writeText(btn.dataset.copy).then(function () {
      btn.classList.add('copied');
      var svg = btn.innerHTML;
      btn.innerHTML = '<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z"/></svg>';
      setTimeout(function () { btn.classList.remove('copied'); btn.innerHTML = svg; }, 2000);
    });
  });
});

document.querySelectorAll('.tab-bar .tab').forEach(function (tab) {
  tab.addEventListener('click', function () {
    document.querySelectorAll('.tab-bar .tab').forEach(function (t) { t.classList.remove('active'); t.setAttribute('aria-selected', 'false'); });
    document.querySelectorAll('.tab-panel').forEach(function (p) { p.classList.remove('active'); });
    tab.classList.add('active');
    tab.setAttribute('aria-selected', 'true');
    document.getElementById('tab-' + tab.dataset.tab).classList.add('active');
  });
});

// Sandbox diagram: hover a branch in a sandbox → highlight matching worktree in host
document.querySelectorAll('.sbx-wt[data-wt]').forEach(function (wt) {
  wt.addEventListener('mouseenter', function () {
    var name = wt.dataset.wt;
    var match = document.querySelector('.sbx-octo-wt[data-wt="' + name + '"]');
    if (match) {
      wt.classList.add('sbx-wt-active');
      match.classList.add('sbx-octo-active');
    }
  });
  wt.addEventListener('mouseleave', function () {
    var name = wt.dataset.wt;
    var match = document.querySelector('.sbx-octo-wt[data-wt="' + name + '"]');
    if (match) {
      wt.classList.remove('sbx-wt-active');
      match.classList.remove('sbx-octo-active');
    }
  });
});

// Kanban diagram: hover a card → subtle glow effect
document.querySelectorAll('.kb-card[data-kb]').forEach(function (card) {
  card.addEventListener('mouseenter', function () {
    card.classList.add('kb-card-hover');
  });
  card.addEventListener('mouseleave', function () {
    card.classList.remove('kb-card-hover');
  });
});

document.querySelectorAll('a[href^="#"]').forEach(function (anchor) {
  anchor.addEventListener('click', function (e) {
    e.preventDefault();
    var target = document.querySelector(this.getAttribute('href'));
    if (target) target.scrollIntoView({ behavior: 'smooth', block: 'start' });
  });
});

// Regent activity modal — click any kanban card (works in both
// kanban and grid views since the DOM nodes are the same) or press
// 'l' to open a WhatsApp-style log mirroring the biomelab GUI.
(function () {
  var modal = document.getElementById('rgt-modal');
  if (!modal) return;
  var body = modal.querySelector('.rgt-modal-body');
  var title = modal.querySelector('#rgt-modal-title');
  var exportBtn = modal.querySelector('.rgt-modal-export');
  var lastBranch = 'feat/auth-flow';

  function open(branch) {
    lastBranch = branch || lastBranch;
    title.textContent = 'Regent activity — ' + lastBranch;
    body.innerHTML = renderSteps(SAMPLE);
    body.scrollTop = 0;
    modal.removeAttribute('hidden');
    document.body.classList.add('rgt-modal-open');
    requestAnimationFrame(function () { modal.classList.add('open'); });
  }

  function close() {
    modal.classList.remove('open');
    document.body.classList.remove('rgt-modal-open');
    setTimeout(function () { modal.setAttribute('hidden', ''); }, 200);
  }

  modal.querySelectorAll('[data-rgt-close]').forEach(function (el) {
    el.addEventListener('click', close);
  });

  // Only Esc closes the modal — opening is click-only on the web.
  // The 'l' shortcut exists in the desktop app (where every card
  // has a clear "selected" state); on the web a stray keypress on
  // the landing page would pop the modal unexpectedly.
  document.addEventListener('keydown', function (e) {
    if (modal.hasAttribute('hidden')) return;
    if (e.key === 'Escape') close();
  });

  document.querySelectorAll('.kb-card[data-kb]').forEach(function (card) {
    card.addEventListener('click', function (e) {
      e.preventDefault();
      open(card.dataset.kb);
    });
  });

  body.addEventListener('click', function (e) {
    var btn = e.target.closest('.rgt-tools-toggle');
    if (!btn) return;
    var target = document.getElementById(btn.dataset.target);
    if (!target) return;
    var caret = btn.querySelector('span');
    if (target.hasAttribute('hidden')) {
      target.removeAttribute('hidden');
      if (caret) caret.textContent = '▼';
    } else {
      target.setAttribute('hidden', '');
      if (caret) caret.textContent = '▶';
    }
  });

  exportBtn.addEventListener('click', function () {
    var payload = { session_id: 'demo-' + lastBranch, steps: SAMPLE };
    var blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' });
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'regent-log-' + lastBranch.replace(/\//g, '-') + '.json';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
  });

  function esc(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;' }[c];
    });
  }
  function nl2br(s) { return esc(s).replace(/\n/g, '<br>'); }

  function renderSteps(steps) {
    return steps.map(function (s, i) {
      return '<div class="rgt-step">'
        + '<div class="rgt-meta">● ' + esc(s.hash) + ' · ' + esc(s.timestamp) + ' · ' + esc(s.origin) + '</div>'
        + (s.human ? '<div class="rgt-row rgt-row-right"><div class="rgt-bubble rgt-bubble-human">' + nl2br(s.human) + '</div></div>' : '')
        + (s.agent || (s.tools && s.tools.length) ? renderAgent(s, i) : '')
        + '</div>';
    }).join('');
  }

  function renderAgent(s, i) {
    var hasTools = s.tools && s.tools.length;
    var id = 'rgt-tools-' + i;
    return '<div class="rgt-row rgt-row-left">'
      + '<div class="rgt-bubble rgt-bubble-agent">'
      + (s.agent ? '<div class="rgt-msg">' + nl2br(s.agent) + '</div>' : '')
      + (hasTools ? '<button class="rgt-tools-toggle" data-target="' + id + '"><span>▶</span> ' + s.tools.length + ' tool' + (s.tools.length === 1 ? '' : 's') + '</button>' : '')
      + '</div></div>'
      + (hasTools ? '<div class="rgt-bubble rgt-bubble-tools" id="' + id + '" hidden>' + renderTools(s.tools) + '</div>' : '');
  }

  function renderTools(tools) {
    return tools.map(function (t) {
      var args = Object.keys(t.args || {}).sort();
      return '<div class="rgt-tool">'
        + '<div class="rgt-tool-name">└─ ' + esc(t.name) + '</div>'
        + (args.length ? '<div class="rgt-tool-bubble">'
            + args.map(function (k) {
                return '<div class="rgt-tool-arg">   ' + esc(k) + ': ' + esc(String(t.args[k])) + '</div>';
              }).join('')
            + '</div>' : '')
        + '</div>';
    }).join('');
  }

  var SAMPLE = [
    {
      hash: 'a1b2c3d4',
      timestamp: '2026-05-19 14:02:11',
      origin: 'claude_code',
      human: 'Add an integration test for the user repository using testcontainers-go — Postgres 16, run the migration suite against it, then a happy-path Create/Get.',
      agent: 'Wired testcontainers-go: added a Postgres fixture under tests/fixtures/postgres.go that boots postgres:16-alpine, applies migrations, and returns a *pgxpool.Pool. Switched user_repo_integration_test.go to use it. 14 cases green with -race.',
      tools: [
        { name: 'Read', args: { file_path: '/workspace/go.mod' } },
        { name: 'Bash', args: {
            command: 'go get github.com/testcontainers/testcontainers-go/modules/postgres@latest',
            description: 'Add the Postgres testcontainers module'
          } },
        { name: 'Write', args: {
            file_path: '/workspace/tests/fixtures/postgres.go',
            content: 'package fixtures\n\nimport (\n    "context"\n    "testing"\n\n    "github.com/jackc/pgx/v5/pgxpool"\n    "github.com/testcontainers/testcontainers-go/modules/postgres"\n)\n\nfunc Postgres(t *testing.T) *pgxpool.Pool { /* ... */ }'
          } },
        { name: 'Edit', args: {
            file_path: '/workspace/internal/user/user_repo_integration_test.go',
            old_string: 'pool := mockPool(t)',
            new_string: 'pool := fixtures.Postgres(t)'
          } },
        { name: 'Bash', args: {
            command: 'go test -race -run TestUserRepo ./internal/user/...',
            description: 'Run user repo integration tests with the race detector'
          } }
      ]
    },
    {
      hash: 'e5f6g7h8',
      timestamp: '2026-05-19 14:07:43',
      origin: 'claude_code',
      human: 'Verify this Docker Sandbox can\'t reach the public internet beyond the allowlist — try curling api.example.com and github.com from inside, make sure only github resolves.',
      agent: 'Confirmed: api.example.com fails at DNS resolution (the sbx network policy filters it), github.com:443 returns 200. The sandbox spec already pins egress to {github.com, proxy.golang.org}; no change needed.',
      tools: [
        { name: 'Read', args: { file_path: '/workspace/sandbox.spec.yaml' } },
        { name: 'Bash', args: {
            command: 'sbx exec my-sandbox -- curl -sS -o /dev/null -w "%{http_code}\\n" https://api.example.com',
            description: 'Try to reach a non-allowlisted host from inside the sandbox'
          } },
        { name: 'Bash', args: {
            command: 'sbx exec my-sandbox -- curl -sS -o /dev/null -w "%{http_code}\\n" https://github.com',
            description: 'Confirm github.com resolves through the allowlist'
          } }
      ]
    },
    {
      hash: '9c0d1f2a',
      timestamp: '2026-05-19 14:14:08',
      origin: 'claude_code',
      human: 'Same testcontainers pattern for Redis — the cache layer mock has hidden a JSON-encoding bug twice already this quarter.',
      agent: 'Mirrored the Postgres fixture for Redis 7.2 and switched cache_test.go over. Tests went from 50ms (mock) to ≈1.4s (real Redis) but immediately caught the encoding bug — the mock was lenient on []byte vs string keys. Added a regression test for it.',
      tools: [
        { name: 'Read', args: { file_path: '/workspace/tests/fixtures/postgres.go' } },
        { name: 'Write', args: {
            file_path: '/workspace/tests/fixtures/redis.go',
            content: 'package fixtures\n\nimport (\n    "context"\n    "testing"\n\n    "github.com/redis/go-redis/v9"\n    "github.com/testcontainers/testcontainers-go/modules/redis"\n)\n\nfunc Redis(t *testing.T) *redis.Client { /* ... */ }'
          } },
        { name: 'Edit', args: {
            file_path: '/workspace/internal/cache/cache_test.go',
            old_string: 'client := mockRedis(t)',
            new_string: 'client := fixtures.Redis(t)'
          } },
        { name: 'Bash', args: {
            command: 'go test -race -run TestCache ./internal/cache/...',
            description: 'Run cache tests against the real Redis container'
          } }
      ]
    }
  ];
})();

// Kanban / grid toggle — press 'g' to switch views
(function () {
  var diagram = document.querySelector('.kanban-diagram-html');
  var title   = document.querySelector('#kb-title');
  var hint    = document.querySelector('.kb-footer-hint .kb-footer-mode');
  var STORE_KEY = 'kb-view';

  function getHintHTML(mode) {
    if (mode === 'grid') {
      return 'Press <kbd class="kb-g-glow">g</kbd> to return to kanban board';
    } else {
      return 'Try <kbd class="kb-g-glow">g</kbd> for grid view';
    }
  }

  function setView(mode) {
    if (mode === 'grid') {
      diagram.classList.add('kb-view-grid');
      title.textContent = 'Card Grid View';
      hint.innerHTML = getHintHTML('grid');
    } else {
      diagram.classList.remove('kb-view-grid');
      title.textContent = 'PR Lifecycle Board';
      hint.innerHTML = getHintHTML('kanban');
    }
    localStorage.setItem(STORE_KEY, mode);
  }

  // Restore saved preference
  var saved = localStorage.getItem(STORE_KEY);
  if (saved === 'grid') setView('grid');

  document.addEventListener('keydown', function (e) {
    // Ignore when focus is in an input / textarea
    var tag = document.activeElement && document.activeElement.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA') return;
    if (e.key === 'g' || e.key === 'G') {
      setView(diagram.classList.contains('kb-view-grid') ? 'kanban' : 'grid');
    }
  });
})();
