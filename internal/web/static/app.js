// Backlog Web UI

const $ = (sel, ctx = document) => ctx.querySelector(sel);
const $$ = (sel, ctx = document) => [...ctx.querySelectorAll(sel)];
const html = String.raw;

// ─── Markdown ────────────────────────────────────────────────────────────────
function renderMarkdown(text) {
  if (!text) return '';
  let s = text
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

  s = s.replace(/```([^\n]*)\n?([\s\S]*?)```/g, (_, lang, code) =>
    `<pre><code>${code.trimEnd()}</code></pre>`);
  s = s.replace(/`([^`]+)`/g, '<code>$1</code>');
  s = s.replace(/^### (.+)$/gm, '<h3>$1</h3>');
  s = s.replace(/^## (.+)$/gm, '<h2>$1</h2>');
  s = s.replace(/^# (.+)$/gm, '<h1>$1</h1>');
  s = s.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
  s = s.replace(/\*(.+?)\*/g, '<em>$1</em>');
  s = s.replace(/^[ \t]*[-*] (.+)$/gm, '<li>$1</li>');
  s = s.replace(/^[ \t]*\d+\. (.+)$/gm, '<li>$1</li>');
  s = s.replace(/(<li>[\s\S]*?<\/li>)(\n<li>[\s\S]*?<\/li>)*/g, m => `<ul>${m}</ul>`);
  // GFM table: detect block of lines where every line starts and ends with |
  s = s.replace(/(?:^|\n)((?:\|[^\n]+\|\n?)+)/g, (_, tableBlock) => {
    const rows = tableBlock.trim().split('\n').filter(r => r.trim().startsWith('|'));
    if (rows.length < 2) return _;
    // Second row is separator (|---|---|) — validate and remove
    const sepRow = rows[1];
    if (!/^\|[\s\-:|]+\|$/.test(sepRow.replace(/\s/g, ''))) return _;
    const headerCells = rows[0].split('|').slice(1, -1).map(c => `<th>${c.trim()}</th>`).join('');
    const bodyRows = rows.slice(2).map(r => {
      const cells = r.split('|').slice(1, -1).map(c => `<td>${c.trim()}</td>`).join('');
      return `<tr>${cells}</tr>`;
    }).join('');
    return `<table><thead><tr>${headerCells}</tr></thead><tbody>${bodyRows}</tbody></table>`;
  });
  const blocks = s.split(/\n{2,}/);
  s = blocks.map(b => {
    b = b.trim();
    if (!b) return '';
    if (/^<(h[1-6]|ul|ol|pre|blockquote|table)/.test(b)) return b;
    return `<p>${b.replace(/\n/g, '<br>')}</p>`;
  }).join('\n');
  return s;
}

// ─── EasyMDE helper ──────────────────────────────────────────────────────────
// Token estimate uses the OpenAI rule of thumb of ~4 chars per token for English.
// Char-based handles code, punctuation, and non-English better than word-based;
// the trailing `~` in the UI label conveys this is an approximation.
function estimateTokens(text) {
  if (!text) return 0;
  return Math.ceil(text.length / 4);
}

function createMDE(textareaEl, opts = {}) {
  return new EasyMDE({
    element: textareaEl,
    spellChecker: false,
    autofocus: opts.autofocus ?? false,
    minHeight: opts.minHeight ?? '160px',
    toolbar: ['bold','italic','code','|','heading-2','heading-3','|',
              'unordered-list','ordered-list','|','preview','side-by-side','|','guide'],
    status: [
      'lines',
      'words',
      {
        className: 'tokens',
        defaultValue: (el) => { el.textContent = '0'; },
        // EasyMDE only passes `el` to onUpdate (the second arg shown in some
        // docs does not exist in the released build). Walk the DOM to find the
        // sibling CodeMirror instance, which exposes the live text.
        onUpdate: (el) => {
          const container = el.closest('.EasyMDEContainer');
          const cmEl = container && container.querySelector('.CodeMirror');
          const cm = cmEl && cmEl.CodeMirror;
          el.textContent = estimateTokens(cm ? cm.getValue() : '');
        },
      },
    ],
    ...opts,
  });
}

// ─── State ───────────────────────────────────────────────────────────────────
const state = {
  page: 'tasks',
  project: '',
  projects: [],
  tasks: [],
  taskTotal: 0,
  taskOffset: 0,
  filters: {
    search: '',
    status: [],
    type: [],
    priority: [],
    label: [],
  },
  sorts: [
    { field: 'priority', dir: 'asc' },
  ],
  currentTaskId: null,
  currentTaskSeq: null,
  taskView: 'list',
  // 'active' (default) shows non-archived tasks. 'archived' shows ONLY archived.
  archivedView: 'active',
  kanbanGroupBy: 'status',
  activityKindFilter: '',
  activityActorFilter: '',
  activityRefreshTimer: null,
  docsSearch: '',
  currentDocId: null,
  currentDoc: null,
  projectsSearch: '',
  projectsShowArchived: false,
  projectLabels: {},
};
const PAGE_SIZE = 50;

// ─── Terminal Command Bar ─────────────────────────────────────────────────────
function updateCmdBar() {
  const bar = document.getElementById('cmd-bar');
  if (!bar) return;
  let cmd = 'backlog';
  if (state.page === 'tasks') {
    cmd += ' task list';
    if (state.project) cmd += ` --project ${state.project}`;
    if (state.filters.status.length) cmd += ` --status ${state.filters.status[0]}`;
    if (state.filters.priority.length) cmd += ` --priority P${state.filters.priority[0]}`;
    if (state.filters.type.length) cmd += ` --type ${state.filters.type[0]}`;
    if (state.filters.label.length) cmd += ` --label ${state.filters.label[0]}`;
    if (state.filters.search) cmd += ` --search "${state.filters.search}"`;
    if (state.sorts.length) cmd += ` --sort ${state.sorts[0].field}`;
  } else if (state.page === 'task-detail') {
    cmd += ` task show TASK-${state.currentTaskSeq || '?'}`;
  } else if (state.page === 'docs') {
    cmd += ` doc list --project ${state.project || '<project>'}`;
  } else if (state.page === 'memory') {
    cmd += ` memory list --project ${state.project || '<project>'}`;
  } else if (state.page === 'projects') {
    cmd += ' project list';
    if (state.projectsShowArchived) cmd += ' --include-archived';
  } else if (state.page === 'activity') {
    cmd += ' activity';
    if (state.project) cmd += ` --project ${state.project}`;
    if (state.activityKindFilter) cmd += ` --kind ${state.activityKindFilter}`;
    if (state.activityActorFilter) cmd += ` --actor-name ${state.activityActorFilter}`;
  }
  cmd += ' --json';
  bar.textContent = '$ ' + cmd;
}

// ─── API ─────────────────────────────────────────────────────────────────────
const api = {
  async get(path) {
    const r = await fetch('/api' + path);
    if (!r.ok) throw new Error((await r.json().catch(() => ({error: r.statusText}))).error);
    return r.json();
  },
  async post(path, body) {
    const r = await fetch('/api' + path, {
      method: 'POST', headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
    });
    if (!r.ok) throw new Error((await r.json().catch(() => ({error: r.statusText}))).error);
    return r.json();
  },
  async patch(path, body) {
    const r = await fetch('/api' + path, {
      method: 'PATCH', headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(body),
    });
    if (!r.ok) throw new Error((await r.json().catch(() => ({error: r.statusText}))).error);
    return r.json();
  },
  async del(path) {
    const r = await fetch('/api' + path, {method: 'DELETE'});
    if (r.status === 204) return true;
    if (!r.ok) throw new Error((await r.json().catch(() => ({error: r.statusText}))).error);
    return true;
  },
};

// ─── Router ──────────────────────────────────────────────────────────────────
//
// URL scheme:
//   /                  → tasks list (legacy alias)
//   /tasks             → tasks list
//   /tasks/<seq>       → task detail (also accepts ULID fallback)
//   /plans             → plans list
//   /docs              → docs list (Notion-style reader, empty right pane)
//   /docs/<id>         → docs list + the doc with that id loaded in the reader
//   /memory            → memory list
//   /attachments       → attachments list
//   /projects          → projects list
//   /activity          → activity feed
//
// Query string:
//   ?project=<alias>   → restores the global project filter on load
//
// Internal calls during popstate set { _replace: true } so we update state
// without pushing a new history entry.
const PAGES_WITHOUT_PARAMS = new Set(['tasks','plans','memory','attachments','projects','activity']);

function parseRoute(pathname, search) {
  const params = {};
  const q = new URLSearchParams(search || '');
  if (q.has('project')) params.project = q.get('project');
  if (q.has('view')) params.view = q.get('view');
  if (q.has('archived')) params.archived = q.get('archived');

  const path = (pathname || '/').replace(/\/+$/, '') || '/';
  if (path === '/' || path === '/tasks') {
    return { page: 'tasks', params };
  }
  if (path.startsWith('/tasks/')) {
    const ref = decodeURIComponent(path.slice('/tasks/'.length).split('/')[0]);
    if (ref) {
      params.taskRef = ref;
      // numeric ref is the human seq (TASK-N); anything else is treated as ULID.
      if (/^\d+$/.test(ref)) params.taskSeq = ref;
      return { page: 'task-detail', params };
    }
    return { page: 'tasks', params };
  }
  if (path === '/docs') {
    return { page: 'docs', params };
  }
  if (path.startsWith('/docs/')) {
    const id = decodeURIComponent(path.slice('/docs/'.length).split('/')[0]);
    if (id) params.docId = id;
    return { page: 'docs', params };
  }
  const segment = path.slice(1).split('/')[0];
  if (PAGES_WITHOUT_PARAMS.has(segment)) {
    return { page: segment, params };
  }
  // Unknown path → fall back to tasks list (the SPA owns the URL space).
  return { page: 'tasks', params };
}

function buildRoute(page, params = {}) {
  let path = '/';
  switch (page) {
    case 'tasks':       path = '/tasks'; break;
    case 'task-detail': {
      const ref = params.taskRef || params.taskSeq || params.currentTaskSeq || params.currentTaskId || '';
      path = ref ? '/tasks/' + encodeURIComponent(ref) : '/tasks';
      break;
    }
    case 'plans':       path = '/plans'; break;
    case 'docs': {
      const docId = params.docId !== undefined ? params.docId : state.currentDocId;
      path = docId ? '/docs/' + encodeURIComponent(docId) : '/docs';
      break;
    }
    case 'memory':      path = '/memory'; break;
    case 'attachments': path = '/attachments'; break;
    case 'projects':    path = '/projects'; break;
    case 'activity':    path = '/activity'; break;
    default:            path = '/';
  }
  const q = new URLSearchParams();
  const project = params.project !== undefined ? params.project : state.project;
  if (project) q.set('project', project);
  if (page === 'tasks' || page === 'task-detail') {
    const view = params.view !== undefined ? params.view : state.taskView;
    if (view && view !== 'list') q.set('view', view);
  }
  if (page === 'tasks') {
    const av = params.archived !== undefined ? params.archived : (state.archivedView === 'archived' ? '1' : '');
    if (av === '1' || av === 'true') q.set('archived', '1');
  }
  const qs = q.toString();
  return qs ? path + '?' + qs : path;
}

function navigate(page, opts = {}) {
  const replace = opts._replace === true;
  const skipHistory = opts._skipHistory === true;
  // Strip router-internal flags so they don't land in state.
  const stateOverrides = { ...opts };
  delete stateOverrides._replace;
  delete stateOverrides._skipHistory;

  state.page = page;
  if (stateOverrides.resetFilters) {
    state.filters = { search: '', status: [], type: [], priority: [], label: [] };
    state.sorts = [{ field: 'priority', dir: 'asc' }];
    state.taskOffset = 0;
    delete stateOverrides.resetFilters;
  }
  // Map URL-style `docId` onto the canonical `currentDocId` state key so
  // navigate('docs', { docId: '<id>' }) does the right thing. When the id
  // changes, drop the cached doc payload so the reader pane re-fetches.
  // navigate('docs') with no docId is the sidebar/list-view intent: clear
  // any selected doc so the URL collapses back to /docs.
  if (page === 'docs') {
    const hadDocId = Object.prototype.hasOwnProperty.call(stateOverrides, 'docId');
    const nextId = hadDocId ? (stateOverrides.docId || null) : null;
    if (nextId !== state.currentDocId) state.currentDoc = null;
    state.currentDocId = nextId;
    if (hadDocId) delete stateOverrides.docId;
  }
  Object.assign(state, stateOverrides);

  if (!skipHistory) {
    const url = buildRoute(page, stateOverrides);
    const histState = { page, stateOverrides };
    try {
      if (replace) history.replaceState(histState, '', url);
      else         history.pushState(histState, '', url);
    } catch (_) { /* ignore (e.g. file://) */ }
  }

  updateCmdBar();
  render();
}

// Sync the current URL's ?project= without pushing a history entry. Used by
// the project select onchange and other in-page filter changes.
function syncUrlProject() {
  try {
    const url = buildRoute(state.page, {
      taskRef: state.currentTaskSeq || state.currentTaskId,
    });
    history.replaceState({ page: state.page, stateOverrides: {} }, '', url);
  } catch (_) { /* ignore */ }
}

function applyRoute(route) {
  // Apply URL-derived state without re-pushing history.
  if (route.params.project !== undefined) state.project = route.params.project;
  if (route.params.view !== undefined && VALID_TASK_VIEWS.includes(route.params.view)) {
    state.taskView = route.params.view;
  }
  if (route.params.archived !== undefined) {
    state.archivedView = (route.params.archived === '1' || route.params.archived === 'true') ? 'archived' : 'active';
  }
  if (route.page === 'task-detail') {
    state.currentTaskId  = route.params.taskRef || null;
    state.currentTaskSeq = route.params.taskSeq || null;
  }
  if (route.page === 'docs') {
    const next = route.params.docId || null;
    // Drop the cached doc payload when the id changes (or clears), so the
    // reader pane re-fetches instead of flashing stale content.
    if (next !== state.currentDocId) state.currentDoc = null;
    state.currentDocId = next;
  }
  state.page = route.page;
}

// ─── Helpers ─────────────────────────────────────────────────────────────────
function escapeHtml(s) {
  return (s || '')
    .replace(/&/g,'&amp;')
    .replace(/</g,'&lt;')
    .replace(/>/g,'&gt;')
    .replace(/"/g,'&quot;')
    .replace(/'/g,'&#39;');
}
function statusBadge(s) {
  return `<span class="badge badge-${s}">${s}</span>`;
}
function priorityBadge(p) {
  const labels = {1:'P1 Critical',2:'P2 High',3:'P3 Normal',4:'P4 Low',5:'P5 Backlog'};
  return `<span class="badge badge-p${p}">P${p}</span>`;
}
// Compact pill that identifies the owning project of a row when the current
// view is aggregating across all projects. Clickable: scopes the global
// project filter to that alias and re-renders the page.
function projectChipHTML(alias, name) {
  const a = (alias || '').trim();
  if (!a) return '';
  const label = (name || a).trim();
  return `<button type="button" class="project-chip" data-project-alias="${escapeHtml(a)}" title="Filter to ${escapeHtml(label)}">${escapeHtml(label)}</button>`;
}
// Wires every .project-chip in the current page to scope the global project
// filter. Call after rendering an aggregated list.
function bindProjectChips() {
  document.querySelectorAll('.project-chip').forEach(chip => {
    chip.onclick = e => {
      e.preventDefault();
      e.stopPropagation();
      const alias = chip.dataset.projectAlias;
      if (!alias || alias === state.project) return;
      state.project = alias;
      const psel = document.getElementById('project-select');
      if (psel) psel.value = alias;
      syncUrlProject();
      updateCmdBar();
      render();
    };
  });
}
async function getLabelsForProject(alias) {
  const key = alias || '';
  if (!key) return [];
  if (state.projectLabels[key]) return state.projectLabels[key];
  try {
    const data = await api.get('/labels?project=' + encodeURIComponent(key));
    state.projectLabels[key] = data.labels || [];
  } catch (_) {
    state.projectLabels[key] = [];
  }
  return state.projectLabels[key];
}
function caretIcon() {
  return `<span class="prop-caret" aria-hidden="true"><svg xmlns="http://www.w3.org/2000/svg" width="9" height="6" viewBox="0 0 9 6" fill="none"><path d="M1 1l3.5 3.5L8 1" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg></span>`;
}
function timeAgo(ns) {
  if (!ns) return '—';
  const ms = ns / 1e6;
  const diff = Date.now() - ms;
  if (diff < 60000) return 'just now';
  if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago';
  if (diff < 86400000) return Math.floor(diff / 3600000) + 'h ago';
  if (diff < 7 * 86400000) return Math.floor(diff / 86400000) + 'd ago';
  return new Date(ms).toISOString().slice(0, 10);
}
function formatDate(ns) {
  if (!ns) return '—';
  return new Date(ns / 1e6).toISOString().slice(0, 16).replace('T', ' ');
}
function formatLocalDateTime(ns) {
  if (!ns) return '';
  const d = new Date(ns / 1e6);
  const date = d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  const time = d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  return `${date}, ${time}`;
}
function nsToDateInput(ns) {
  if (!ns) return '';
  return new Date(ns / 1e6).toISOString().slice(0, 10);
}
function getCookie(name) {
  const m = document.cookie.match(new RegExp('(?:^|; )' + name.replace(/[.$?*|{}()[\]\\\/+^]/g, '\\$&') + '=([^;]*)'));
  return m ? decodeURIComponent(m[1]) : null;
}
function setCookie(name, value, days = 365) {
  const expires = new Date(Date.now() + days * 864e5).toUTCString();
  document.cookie = `${name}=${encodeURIComponent(value)}; expires=${expires}; path=/; SameSite=Lax`;
}
const VALID_TASK_VIEWS = ['list', 'board', 'grid', 'timeline'];
// Centralises view-toggle changes so cookie persistence + re-render stay in sync.
function setTaskView(view) {
  if (!VALID_TASK_VIEWS.includes(view)) return;
  state.taskView = view;
  setCookie('backlog.taskView', view);
  render();
}
function toast(msg, type = 'success') {
  let tc = $('#toast-container');
  if (!tc) {
    tc = document.createElement('div');
    tc.id = 'toast-container';
    tc.className = 'toast-container';
    document.body.appendChild(tc);
  }
  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.textContent = msg;
  tc.appendChild(el);
  setTimeout(() => el.remove(), 3500);
}
function showModal(content) {
  const overlay = document.createElement('div');
  overlay.className = 'modal-overlay';
  overlay.innerHTML = content;
  document.body.appendChild(overlay);
  const close = () => overlay.remove();
  overlay.querySelectorAll('.modal-close').forEach(el => el.addEventListener('click', close));
  overlay.addEventListener('click', e => { if (e.target === overlay) close(); });
  setTimeout(() => overlay.querySelector('input,textarea')?.focus(), 50);
  return overlay;
}
// confirm(msg, opts?) — opts: { confirmLabel?: string, confirmClass?: string, cancelLabel?: string }
// Defaults preserve the historical destructive styling (red "Delete" button) so
// existing call sites for delete actions keep working. Pass confirmClass:'btn-primary'
// for reversible actions like Archive.
function confirm(msg, opts = {}) {
  const confirmLabel = escapeHtml(opts.confirmLabel || 'Delete');
  const confirmClass = opts.confirmClass || 'btn-danger';
  const cancelLabel  = escapeHtml(opts.cancelLabel  || 'Cancel');
  return new Promise(resolve => {
    const overlay = showModal(html`
      <div class="modal modal-sm">
        <div class="modal-body" style="padding:24px 20px">
          <p style="margin-bottom:20px">${escapeHtml(msg)}</p>
          <div class="modal-footer" style="padding:0;border:none">
            <button class="btn btn-ghost modal-close" id="btn-cancel">${cancelLabel}</button>
            <button class="btn ${confirmClass}" id="btn-confirm">${confirmLabel}</button>
          </div>
        </div>
      </div>`);
    overlay.querySelector('#btn-confirm').onclick = () => { overlay.remove(); resolve(true); };
    overlay.querySelector('#btn-cancel').onclick = () => { overlay.remove(); resolve(false); };
  });
}

// ─── Shared page header / toolbar ────────────────────────────────────────────
// Composes the Tasks-style header markup (large title, count subtitle, optional
// view toggle, optional primary action) and an optional toolbar row below it.
// Caller wires button onclicks after innerHTML by id.
//
//   pageHeader({
//     title:      'Docs',
//     subtitle:   '12 docs',
//     viewToggle: '<div class="view-toggle">…</div>',  // optional, raw HTML
//     primary:    { id:'btn-new-doc', label:'+ New Doc', shortcut:'N' },
//     toolbar:    '<input class="search-input">…',     // optional, raw HTML
//   })
function pageHeader(opts) {
  const { title = '', subtitle = '', viewToggle = '', primary = null, toolbar = '' } = opts || {};
  let actions = '';
  if (viewToggle || primary) {
    let primaryBtn = '';
    if (primary) {
      const sc = primary.shortcut ? ` <kbd>${escapeHtml(primary.shortcut)}</kbd>` : '';
      primaryBtn = '<button class="btn btn-primary" id="' + primary.id + '">' +
        escapeHtml(primary.label) + sc + '</button>';
    }
    actions =
      '<div class="flex items-center gap-2">' +
      (viewToggle || '') +
      primaryBtn +
      '</div>';
  }
  const subtitleHTML = subtitle ? '<div class="page-subtitle">' + escapeHtml(subtitle) + '</div>' : '';
  const header =
    '<header class="page-header">' +
      '<div>' +
        '<div class="page-title">' + escapeHtml(title) + '</div>' +
        subtitleHTML +
      '</div>' +
      actions +
    '</header>';
  const toolbarHTML = toolbar ? '<div class="toolbar">' + toolbar + '</div>' : '';
  return header + toolbarHTML;
}

// ─── Render ──────────────────────────────────────────────────────────────────
async function render() {
  const app = $('#app');

  if (state.page !== 'activity' && state.activityRefreshTimer) {
    clearTimeout(state.activityRefreshTimer);
    state.activityRefreshTimer = null;
  }

  $$('.sidebar-item[data-page]').forEach(el =>
    el.classList.toggle('active', el.dataset.page === state.page));

  const psel = $('#project-select');
  if (psel) psel.value = state.project;

  updateCmdBar();

  switch (state.page) {
    case 'tasks':       return renderTasks(app);
    case 'task-detail': return renderTaskDetail(app);
    case 'docs':        return renderDocs(app);
    case 'plans':       return renderPlans(app);
    case 'memory':      return renderMemory(app);
    case 'attachments': return renderAttachments(app);
    case 'projects':    return renderProjects(app);
    case 'activity':    return renderActivity(app);
  }
}

// ─── Tasks ───────────────────────────────────────────────────────────────────

const SORT_FIELDS = [
  { value: 'priority', label: 'Priority' },
  { value: 'created',  label: 'Created' },
  { value: 'updated',  label: 'Updated' },
  { value: 'title',    label: 'Title' },
  { value: 'seq',      label: 'Ref #' },
];

const FILTER_PROPS = [
  { key: 'status',   label: 'Status',   values: ['todo','doing','done'] },
  { key: 'type',     label: 'Type',     values: ['task','bug','issue','improvement','feature','vulnerability','chore','spike'] },
  { key: 'priority', label: 'Priority', values: ['1','2','3','4','5'], renderLabel: v => 'P' + v },
  { key: 'label',    label: 'Label',    values: () => taskLabelValues() },
];

function taskLabelValues() {
  const byName = new Set();
  if (state.project && state.projectLabels[state.project]) {
    state.projectLabels[state.project].forEach(l => byName.add(l.name));
  }
  (state.tasks || []).forEach(t => (t.labels || []).forEach(l => byName.add(l.name)));
  return [...byName].sort((a, b) => a.localeCompare(b));
}

function filterPropValues(prop) {
  return typeof prop.values === 'function' ? prop.values() : prop.values;
}

// --- Popover helper ---
function showPopover(anchorEl, contentEl) {
  document.querySelectorAll('.popover').forEach(p => p.remove());

  const pop = document.createElement('div');
  pop.className = 'popover';
  pop.appendChild(contentEl);
  document.body.appendChild(pop);

  const rect = anchorEl.getBoundingClientRect();
  const scrollY = window.scrollY || 0;
  const scrollX = window.scrollX || 0;
  pop.style.top = (rect.bottom + scrollY + 4) + 'px';
  pop.style.left = (rect.left + scrollX) + 'px';

  requestAnimationFrame(() => {
    const popRect = pop.getBoundingClientRect();
    if (popRect.right > window.innerWidth - 8) {
      pop.style.left = Math.max(8, rect.right + scrollX - popRect.width) + 'px';
    }
  });

  const close = (e) => {
    if (e && pop.contains(e.target)) return;
    pop.remove();
    document.removeEventListener('mousedown', close);
    document.removeEventListener('keydown', onKey);
  };
  const onKey = (e) => { if (e.key === 'Escape') close(); };

  setTimeout(() => {
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', onKey);
  }, 0);

  return { pop, close };
}

// --- Sort popover ---
function showSortPopover(anchorEl) {
  const container = document.createElement('div');

  const renderRows = () => {
    container.innerHTML = '';

    if (state.sorts.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'popover-empty';
      empty.textContent = 'No sort applied';
      container.appendChild(empty);
    }

    state.sorts.forEach((s, i) => {
      const row = document.createElement('div');
      row.className = 'sort-row';

      const fieldSel = document.createElement('select');
      fieldSel.className = 'popover-select';
      SORT_FIELDS.forEach(f => {
        const opt = document.createElement('option');
        opt.value = f.value;
        opt.textContent = f.label;
        if (f.value === s.field) opt.selected = true;
        fieldSel.appendChild(opt);
      });
      fieldSel.onchange = () => {
        state.sorts[i].field = fieldSel.value;
        renderRows(); updateCmdBar(); render();
      };

      const dirBtn = document.createElement('button');
      dirBtn.className = 'btn btn-ghost btn-sm sort-dir-btn';
      dirBtn.textContent = s.dir === 'asc' ? '↑ Asc' : '↓ Desc';
      dirBtn.onclick = () => {
        state.sorts[i].dir = s.dir === 'asc' ? 'desc' : 'asc';
        dirBtn.textContent = state.sorts[i].dir === 'asc' ? '↑ Asc' : '↓ Desc';
        updateCmdBar(); render();
      };

      const removeBtn = document.createElement('button');
      removeBtn.className = 'btn btn-ghost btn-sm sort-remove-btn';
      removeBtn.textContent = '\xd7';
      removeBtn.onclick = () => { state.sorts.splice(i, 1); renderRows(); updateCmdBar(); render(); };

      row.appendChild(fieldSel);
      row.appendChild(dirBtn);
      row.appendChild(removeBtn);
      container.appendChild(row);
    });

    if (state.sorts.length < SORT_FIELDS.length) {
      const addBtn = document.createElement('button');
      addBtn.className = 'btn btn-ghost btn-sm popover-add-btn';
      addBtn.textContent = '+ Add sort';
      addBtn.onclick = () => {
        const used = new Set(state.sorts.map(s => s.field));
        const next = SORT_FIELDS.find(f => !used.has(f.value));
        if (next) {
          state.sorts.push({ field: next.value, dir: 'asc' });
          renderRows(); updateCmdBar(); render();
        }
      };
      container.appendChild(addBtn);
    }
  };

  renderRows();
  showPopover(anchorEl, container);
}

// --- Filter property popover ---
function showFilterPropPopover(anchorEl) {
  const container = document.createElement('div');

  FILTER_PROPS.forEach(prop => {
    const values = filterPropValues(prop);
    if (values.length === 0) return;
    const item = document.createElement('div');
    item.className = 'popover-item';
    item.textContent = prop.label;
    item.onclick = () => {
      document.querySelectorAll('.popover').forEach(p => p.remove());
      showFilterValuePopover(anchorEl, prop);
    };
    container.appendChild(item);
  });

  showPopover(anchorEl, container);
}

// --- Filter value popover ---
function showFilterValuePopover(anchorEl, prop) {
  const container = document.createElement('div');

  const header = document.createElement('div');
  header.className = 'popover-header';
  header.textContent = prop.label;
  container.appendChild(header);

  const values = filterPropValues(prop);
  if (values.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'popover-empty';
    empty.textContent = 'No values available';
    container.appendChild(empty);
  }

  values.forEach(val => {
    const row = document.createElement('label');
    row.className = 'popover-item popover-check-item';

    const cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.value = val;
    cb.checked = (state.filters[prop.key] || []).includes(val);
    cb.onchange = () => {
      const arr = state.filters[prop.key] || (state.filters[prop.key] = []);
      if (cb.checked) {
        if (!arr.includes(val)) arr.push(val);
      } else {
        const idx = arr.indexOf(val);
        if (idx !== -1) arr.splice(idx, 1);
      }
      state.taskOffset = 0;
      updateCmdBar();
      render();
    };

    const lbl = document.createElement('span');
    lbl.textContent = prop.renderLabel ? prop.renderLabel(val) : val;

    row.appendChild(cb);
    row.appendChild(lbl);
    container.appendChild(row);
  });

  showPopover(anchorEl, container);
}

// --- Filter chips ---
function buildFilterChipsHTML() {
  const chips = [];
  FILTER_PROPS.forEach(prop => {
    (state.filters[prop.key] || []).forEach(val => {
      const label = prop.renderLabel ? prop.renderLabel(val) : val;
      chips.push(
        '<span class="filter-chip" data-prop="' + prop.key + '" data-val="' + escapeHtml(val) + '">' +
        escapeHtml(prop.label) + ': ' + escapeHtml(label) +
        '<button class="filter-chip-remove" data-prop="' + prop.key + '" data-val="' + escapeHtml(val) + '">\xd7</button>' +
        '</span>'
      );
    });
  });
  return chips.join('');
}

function hasActiveFilters() {
  return state.filters.status.length > 0 ||
    state.filters.type.length > 0 ||
    state.filters.priority.length > 0 ||
    state.filters.label.length > 0 ||
    !!state.filters.search;
}

function bindChipRemoveHandlers() {
  document.querySelectorAll('.filter-chip-remove').forEach(btn => {
    btn.onclick = e => {
      e.stopPropagation();
      const prop = btn.dataset.prop;
      const val = btn.dataset.val;
      const arr = state.filters[prop];
      const idx = arr.indexOf(val);
      if (idx !== -1) arr.splice(idx, 1);
      state.taskOffset = 0;
      updateCmdBar();
      render();
    };
  });
}

function buildSortLabel() {
  if (!state.sorts.length) return '⇅ Sort';
  return '⇅ ' + state.sorts.map(s => {
    const f = SORT_FIELDS.find(x => x.value === s.field);
    return (f ? f.label : s.field) + (s.dir === 'asc' ? ' ↑' : ' ↓');
  }).join(', ');
}

function thSort(label, sortField) {
  const active = state.sorts.length > 0 && state.sorts[0].field === sortField;
  const dir = active ? state.sorts[0].dir : null;
  const icon = dir === 'asc' ? ' ↑' : dir === 'desc' ? ' ↓' : '';
  const extraClass = sortField === 'seq' ? ' task-ref' : '';
  return '<th class="th-sort' + (active ? ' th-sort-active' : '') + extraClass + '" data-sort-field="' + sortField + '">' + label + icon + '</th>';
}

function buildTaskQuery(limit = PAGE_SIZE, offset = state.taskOffset || 0) {
  const archivedOnly = state.archivedView === 'archived';
  const q = new URLSearchParams();
  if (state.project) q.set('project', state.project);
  if (state.filters.status.length === 1)   q.set('status', state.filters.status[0]);
  if (state.filters.type.length === 1)     q.set('type', state.filters.type[0]);
  if (state.filters.priority.length === 1) q.set('priority', state.filters.priority[0]);
  state.filters.label.forEach(label => q.append('label', label));
  if (state.filters.search) q.set('search', state.filters.search);
  if (state.sorts.length) {
    const primaryField = state.sorts[0].field === 'priority' ? '' : state.sorts[0].field;
    if (primaryField) q.set('sort', primaryField);
    if (state.sorts[0].dir) q.set('order', state.sorts[0].dir);
  }
  if (archivedOnly) q.set('include_archived', 'true');
  if (limit !== null) q.set('limit', limit);
  if (offset !== null) q.set('offset', offset);
  return q;
}

async function renderTasks(el) {
  const archivedOnly = state.archivedView === 'archived';
  const q = buildTaskQuery(0, 0);

  try {
    const data = await api.get('/tasks?' + q);
    let tasks = data.tasks || [];
    let total = data.page?.total || 0;
    if (archivedOnly) {
      // The API include_archived flag returns active + archived; narrow to only
      // archived rows on the client (and reflect that in the total/count).
      const filtered = tasks.filter(t => !!t.archived_at);
      total = filtered.length === tasks.length ? total : filtered.length;
      tasks = filtered;
    }
    state.tasks = tasks;
    state.taskTotal = total;
    if (state.project) await getLabelsForProject(state.project);
  } catch (e) { console.error('renderTasks error:', e); toast(e.message, 'error'); return; }

  const activeFilters = hasActiveFilters();
  const showing = (state.taskOffset || 0) + state.tasks.length;
  let subtitle;
  if (archivedOnly) {
    subtitle = state.taskTotal + ' archived task' + (state.taskTotal !== 1 ? 's' : '');
  } else if (activeFilters) {
    subtitle = state.tasks.length + ' result' + (state.tasks.length !== 1 ? 's' : '') + ' (' + state.taskTotal + ' total)';
  } else {
    subtitle = state.taskTotal + ' task' + (state.taskTotal !== 1 ? 's' : '');
  }

  const sortBtnClass = 'btn btn-ghost btn-sm' + (state.sorts.length ? ' sort-btn-active' : '');
  const clearStyle = activeFilters ? '' : ' style="display:none"';

  const groupByDropdown = state.taskView === 'board'
    ? '<label class="kanban-groupby-label">Group by:' +
        '<select class="filter-select" id="kanban-groupby">' +
          '<option value="status"'   + (state.kanbanGroupBy === 'status'   ? ' selected' : '') + '>Status</option>' +
          '<option value="priority"' + (state.kanbanGroupBy === 'priority' ? ' selected' : '') + '>Priority</option>' +
          '<option value="type"'     + (state.kanbanGroupBy === 'type'     ? ' selected' : '') + '>Type</option>' +
          '<option value="assignee"' + (state.kanbanGroupBy === 'assignee' ? ' selected' : '') + '>Assignee</option>' +
        '</select>' +
      '</label>'
    : '';

  const viewToggleHTML =
    '<div class="view-toggle">' +
      '<button class="btn btn-sm view-toggle-btn ' + (state.taskView === 'list' ? 'view-toggle-active' : 'btn-ghost') + '" id="btn-view-list" title="List view">&#9776; List</button>' +
      '<button class="btn btn-sm view-toggle-btn ' + (state.taskView === 'board' ? 'view-toggle-active' : 'btn-ghost') + '" id="btn-view-board" title="Board view">&#11035; Board</button>' +
      '<button class="btn btn-sm view-toggle-btn ' + (state.taskView === 'grid' ? 'view-toggle-active' : 'btn-ghost') + '" id="btn-view-grid" title="Grid view">&#8862; Grid</button>' +
      '<button class="btn btn-sm view-toggle-btn ' + (state.taskView === 'timeline' ? 'view-toggle-active' : 'btn-ghost') + '" id="btn-view-timeline" title="Timeline view">&#8645; Timeline</button>' +
    '</div>' + groupByDropdown +
    '<button class="btn btn-ghost" id="btn-tasks-download">Download</button>';

  const archivedBtnClass = 'btn btn-ghost btn-sm archived-toggle-btn' + (archivedOnly ? ' archived-toggle-active' : '');
  const archivedBtnLabel = archivedOnly ? '\u2714 Archived' : 'Archived';

  const toolbarHTML =
    '<input class="search-input" id="filter-search" placeholder="Search…" value="' + escapeHtml(state.filters.search) + '">' +
    '<button class="btn btn-ghost btn-sm" id="btn-filter-open">+ Filter &#9662;</button>' +
    '<button class="' + sortBtnClass + '" id="btn-sort-open">' + buildSortLabel() + '</button>' +
    '<button class="' + archivedBtnClass + '" id="btn-toggle-archived" title="Show only archived tasks">' + archivedBtnLabel + '</button>' +
    '<span id="filter-chips">' + buildFilterChipsHTML() + '</span>' +
    '<button class="btn btn-ghost btn-sm" id="btn-clear-filters"' + clearStyle + '>Clear all</button>';

  el.innerHTML =
    pageHeader({
      title: 'Tasks',
      subtitle,
      viewToggle: viewToggleHTML,
      primary: { id: 'btn-new-task', label: '+ New Task', shortcut: 'N' },
      toolbar: toolbarHTML,
    }) +
    '<div id="task-content"></div>';

  bindChipRemoveHandlers();

  const contentEl = document.querySelector('#task-content');
  if (state.taskView === 'board') {
    renderKanban(state.tasks, contentEl, activeFilters, showing);
  } else if (state.taskView === 'grid') {
    renderGrid(state.tasks, contentEl, activeFilters, showing);
  } else if (state.taskView === 'timeline') {
    renderTimeline(state.tasks, contentEl, activeFilters, showing);
  } else {
    contentEl.innerHTML =
      '<div class="card">' +
        '<div class="table-wrap">' +
          '<table>' +
            '<thead><tr>' +
              thSort('Ref', 'seq') +
              thSort('Title', 'title') +
              '<th>Status</th>' +
              thSort('Priority', 'priority') +
              '<th>Type</th>' +
              '<th>Project</th>' +
              thSort('Updated', 'updated') +
            '</tr></thead>' +
            '<tbody id="task-tbody">' +
              renderTaskRows(state.tasks, activeFilters) +
            '</tbody>' +
          '</table>' +
        '</div>' +
        (state.taskTotal > showing ?
          '<div class="load-more-bar"><button class="btn btn-ghost" id="btn-load-more">Load more (' + (state.taskTotal - showing) + ' remaining)</button></div>'
          : '') +
      '</div>';
  }

  bindTaskListHandlers(el, activeFilters);
}

function renderTaskRows(tasks, activeFilters) {
  if (tasks.length === 0) {
    const emptyMsg = state.archivedView === 'archived'
      ? 'No archived tasks'
      : (activeFilters ? 'No tasks match the current filters' : 'No tasks yet — create one to get started');
    return (
      '<tr><td colspan="7">' +
      '<div class="empty">' +
      '<div class="empty-icon"></div>' +
      '<div class="empty-text">' + emptyMsg + '</div>' +
      '</div></td></tr>');
  }
  return tasks.map(t => {
    const archived = !!t.archived_at;
    const rowClass = 'task-row' + (archived ? ' task-row-archived' : '');
    const titleCell = archived
      ? '<td><span class="task-title-archived">' + escapeHtml(t.title) + '</span> <span class="badge badge-archived">Archived</span></td>'
      : '<td>' + escapeHtml(t.title) + '</td>';
    const updatedCell = archived
      ? '<td class="text-muted text-sm" title="' + formatDate(t.updated_at) + '">' +
          timeAgo(t.updated_at) +
          ' <button class="btn btn-ghost btn-sm task-row-restore" data-id="' + t.id + '" title="Restore task">Restore</button>' +
        '</td>'
      : '<td class="text-muted text-sm" title="' + formatDate(t.updated_at) + '">' + timeAgo(t.updated_at) + '</td>';
    return (
      '<tr class="' + rowClass + '" data-id="' + t.id + '" data-seq="' + t.seq + '" data-status="' + t.status + '" style="cursor:pointer">' +
      '<td class="text-muted text-sm mono task-ref">TASK-' + t.seq + '</td>' +
      titleCell +
      '<td>' + statusBadge(t.status) + '</td>' +
      '<td>' + priorityBadge(t.priority) + '</td>' +
      '<td class="text-muted text-sm">' + escapeHtml(t.type) + '</td>' +
      '<td class="text-muted text-sm">' + escapeHtml(t.project?.name || t.project?.alias || '') + '</td>' +
      updatedCell +
      '</tr>'
    );
  }).join('');
}

async function handleRestoreTask(id) {
  try {
    await api.patch('/tasks/' + id + '/unarchive', {});
    toast('Task restored');
    render();
  } catch (err) {
    toast(err.message, 'error');
  }
}

function bindTaskListHandlers(el, activeFilters) {
  document.getElementById('btn-new-task').onclick = () => showNewTaskModal();
  document.getElementById('btn-view-list').onclick = () => setTaskView('list');
  document.getElementById('btn-view-board').onclick = () => setTaskView('board');
  document.getElementById('btn-view-grid').onclick = () => setTaskView('grid');
  document.getElementById('btn-view-timeline').onclick = () => setTaskView('timeline');
  document.getElementById('btn-tasks-download').onclick = e => showTasksDownloadPopover(e.currentTarget);
  const groupByEl = document.getElementById('kanban-groupby');
  if (groupByEl) groupByEl.onchange = () => { state.kanbanGroupBy = groupByEl.value; setCookie('backlog.kanbanGroupBy', state.kanbanGroupBy); render(); };
  const clearBtn = document.getElementById('btn-clear-filters');
  if (clearBtn) clearBtn.addEventListener('click', () => {
    state.filters = { search: '', status: [], type: [], priority: [], label: [] };
    state.taskOffset = 0;
    render();
  });
  const filterOpenBtn = document.getElementById('btn-filter-open');
  if (filterOpenBtn) filterOpenBtn.addEventListener('click', e => {
    e.stopPropagation();
    showFilterPropPopover(e.currentTarget);
  });
  const sortOpenBtn = document.getElementById('btn-sort-open');
  if (sortOpenBtn) sortOpenBtn.addEventListener('click', e => {
    e.stopPropagation();
    showSortPopover(e.currentTarget);
  });
  const archivedToggleBtn = document.getElementById('btn-toggle-archived');
  if (archivedToggleBtn) archivedToggleBtn.addEventListener('click', () => {
    state.archivedView = state.archivedView === 'archived' ? 'active' : 'archived';
    setCookie('backlog.archivedView', state.archivedView);
    state.taskOffset = 0;
    syncUrlProject();
    updateCmdBar();
    render();
  });
  bindLoadMore(el);
  document.querySelectorAll('.task-row').forEach(row =>
    row.onclick = e => {
      if (e.target.closest('.task-row-restore')) return;
      navigate('task-detail', { currentTaskId: row.dataset.id, currentTaskSeq: row.dataset.seq });
    });
  document.querySelectorAll('.task-row-restore').forEach(btn => btn.onclick = e => {
    e.stopPropagation();
    handleRestoreTask(btn.dataset.id);
  });
  document.querySelectorAll('th[data-sort-field]').forEach(th =>
    th.onclick = () => {
      const field = th.dataset.sortField;
      if (state.sorts.length > 0 && state.sorts[0].field === field) {
        state.sorts[0].dir = state.sorts[0].dir === 'asc' ? 'desc' : 'asc';
      } else {
        state.sorts = [{ field, dir: 'asc' }];
      }
      state.taskOffset = 0;
      updateCmdBar();
      render();
    });

  let searchTimer;
  document.getElementById('filter-search').oninput = e => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      state.filters.search = e.target.value;
      state.taskOffset = 0;
      updateCmdBar();
      render();
    }, 300);
  };
}

function buildLoadMoreQuery() {
  const q = new URLSearchParams();
  if (state.project) q.set('project', state.project);
  if (state.filters.status.length === 1)   q.set('status', state.filters.status[0]);
  if (state.filters.type.length === 1)     q.set('type', state.filters.type[0]);
  if (state.filters.priority.length === 1) q.set('priority', state.filters.priority[0]);
  state.filters.label.forEach(label => q.append('label', label));
  if (state.filters.search) q.set('search', state.filters.search);
  if (state.sorts.length) {
    const pf = state.sorts[0].field === 'priority' ? '' : state.sorts[0].field;
    if (pf) q.set('sort', pf);
    if (state.sorts[0].dir) q.set('order', state.sorts[0].dir);
  }
  if (state.archivedView === 'archived') q.set('include_archived', 'true');
  return q;
}

function bindLoadMore(el) {
  const btn = document.getElementById('btn-load-more');
  if (!btn) return;
  btn.addEventListener('click', async () => {
    state.taskOffset = (state.taskOffset || 0) + PAGE_SIZE;
    const q = buildLoadMoreQuery();
    q.set('limit', PAGE_SIZE);
    q.set('offset', state.taskOffset);
    try {
      const data = await api.get('/tasks?' + q);
      let newTasks = data.tasks || [];
      if (state.archivedView === 'archived') {
        newTasks = newTasks.filter(t => !!t.archived_at);
      }
      state.tasks = [...state.tasks, ...newTasks];
      const showing = state.taskOffset + state.tasks.length;
      const bar = el.querySelector('.load-more-bar');

      if (state.taskView === 'grid') {
        const tbody = document.getElementById('grid-tbody');
        newTasks.forEach(t => tbody.insertAdjacentHTML('beforeend', renderGridRow(t)));
        bindGridRowHandlers(el);
      } else {
        const tbody = document.getElementById('task-tbody');
        tbody.insertAdjacentHTML('beforeend', renderTaskRows(newTasks, false));
        document.querySelectorAll('.task-row').forEach(row =>
          row.onclick = e => {
            if (e.target.closest('.task-row-restore')) return;
            navigate('task-detail', { currentTaskId: row.dataset.id, currentTaskSeq: row.dataset.seq });
          });
        document.querySelectorAll('.task-row-restore').forEach(btn => btn.onclick = e => {
          e.stopPropagation();
          handleRestoreTask(btn.dataset.id);
        });
      }

      if (state.taskTotal > showing) {
        bar.innerHTML = '<button class="btn btn-ghost" id="btn-load-more">Load more (' + (state.taskTotal - showing) + ' remaining)</button>';
        bindLoadMore(el);
      } else {
        if (bar) bar.remove();
      }
    } catch (e) { toast(e.message, 'error'); }
  });
}

// ─── Grid (Notion-style inline-editable table) ────────────────────────────────

const TASK_TYPES = ['task','bug','issue','improvement','feature','vulnerability','chore','spike'];

function renderGridRow(t) {
  const projectName = escapeHtml(t.project?.name || t.project?.alias || '');
  const archived = !!t.archived_at;
  const rowClass = 'grid-row' + (archived ? ' task-row-archived' : '');
  const titleHtml = archived
    ? '<span class="grid-title-span"><span class="task-title-archived">' + escapeHtml(t.title) + '</span> <span class="badge badge-archived">Archived</span></span>'
    : '<span class="grid-title-span">' + escapeHtml(t.title) + '</span>';
  return `<tr class="${rowClass}" data-id="${t.id}" data-seq="${t.seq}" data-status="${t.status}">
    <td class="text-muted text-sm mono grid-ref-cell" style="width:80px;cursor:pointer" data-id="${t.id}" data-seq="${t.seq}">TASK-${t.seq}</td>
    <td class="grid-title-cell" data-id="${t.id}" data-title="${escapeHtml(t.title)}" style="min-width:200px;cursor:text">
      ${titleHtml}
    </td>
    <td class="grid-status-cell" data-id="${t.id}" data-status="${escapeHtml(t.status)}" style="width:90px;cursor:pointer">
      <span class="grid-status-span">${statusBadge(t.status)}</span>
    </td>
    <td class="grid-priority-cell" data-id="${t.id}" data-priority="${t.priority}" style="width:90px;cursor:pointer">
      <span class="grid-priority-span">${priorityBadge(t.priority)}</span>
    </td>
    <td class="grid-type-cell" data-id="${t.id}" data-type="${escapeHtml(t.type)}" style="width:110px;cursor:pointer">
      <span class="grid-type-span text-muted text-sm">${escapeHtml(t.type)}</span>
    </td>
    <td class="text-muted text-sm" style="width:110px">${projectName}</td>
  </tr>`;
}

function renderGrid(tasks, el, hasFilters, showing) {
  el.innerHTML = html`
    <div class="card">
      <div class="table-wrap">
        <table class="grid-table">
          <thead><tr>
            <th style="width:80px">Ref</th>
            <th>Title</th>
            <th style="width:90px">Status</th>
            <th style="width:90px">Priority</th>
            <th style="width:110px">Type</th>
            <th style="width:110px">Project</th>
          </tr></thead>
          <tbody id="grid-tbody">
            ${tasks.length === 0
              ? `<tr><td colspan="6"><div class="empty"><div class="empty-icon"></div><div class="empty-text">${hasFilters ? 'No tasks match the current filters.' : 'The ledger is empty. Create the first task to begin.'}</div></div></td></tr>`
              : tasks.map(renderGridRow).join('')}
          </tbody>
        </table>
      </div>
      ${state.taskTotal > showing ? `
        <div class="load-more-bar">
          <button class="btn btn-ghost" id="btn-load-more">Load more (${state.taskTotal - showing} remaining)</button>
        </div>` : ''}
    </div>`;

  bindGridRowHandlers(el);
}

function bindGridRowHandlers(el) {
  // Ref cell → navigate to task detail
  $$('.grid-ref-cell', el).forEach(cell => {
    cell.onclick = () => navigate('task-detail', { currentTaskId: cell.dataset.id, currentTaskSeq: cell.dataset.seq });
  });

  // Title cell → inline input
  $$('.grid-title-cell', el).forEach(cell => {
    cell.onclick = e => {
      if (cell.querySelector('.grid-cell-input')) return; // already editing
      const span = cell.querySelector('.grid-title-span');
      const currentTitle = cell.dataset.title;
      const input = document.createElement('input');
      input.className = 'grid-cell-input';
      input.value = currentTitle;
      cell.replaceChildren(input);
      input.focus();
      input.select();

      const save = async () => {
        const newVal = input.value.trim();
        if (!newVal || newVal === currentTitle) {
          cell.replaceChildren(span);
          return;
        }
        try {
          const updated = await api.patch('/tasks/' + cell.dataset.id, { title: newVal });
          const tr = cell.closest('tr');
          tr.outerHTML = renderGridRow(updated);
          bindGridRowHandlers(el);
          toast('Title updated');
        } catch (err) {
          toast(err.message, 'error');
          cell.replaceChildren(span);
        }
      };

      input.onblur = save;
      input.onkeydown = e => {
        if (e.key === 'Enter') { input.blur(); }
        if (e.key === 'Escape') { cell.replaceChildren(span); }
      };
    };
  });

  // Status cell → inline select
  $$('.grid-status-cell', el).forEach(cell => {
    cell.onclick = e => {
      if (cell.querySelector('.grid-cell-select')) return;
      const span = cell.querySelector('.grid-status-span');
      const current = cell.dataset.status;
      const sel = document.createElement('select');
      sel.className = 'grid-cell-select';
      ['todo','doing','done'].forEach(s => {
        const opt = document.createElement('option');
        opt.value = s; opt.textContent = s;
        if (s === current) opt.selected = true;
        sel.appendChild(opt);
      });
      cell.replaceChildren(sel);
      sel.focus();

      const save = async () => {
        const newVal = sel.value;
        if (newVal === current) { cell.replaceChildren(span); return; }
        try {
          const updated = await api.patch('/tasks/' + cell.dataset.id + '/status', { status: newVal });
          const tr = cell.closest('tr');
          tr.outerHTML = renderGridRow(updated);
          bindGridRowHandlers(el);
          toast('Status updated');
        } catch (err) {
          toast(err.message, 'error');
          cell.replaceChildren(span);
        }
      };

      sel.onchange = save;
      sel.onblur = () => { cell.replaceChildren(span); };
      sel.onkeydown = e => { if (e.key === 'Escape') cell.replaceChildren(span); };
    };
  });

  // Priority cell → inline select
  $$('.grid-priority-cell', el).forEach(cell => {
    cell.onclick = e => {
      if (cell.querySelector('.grid-cell-select')) return;
      const span = cell.querySelector('.grid-priority-span');
      const current = String(cell.dataset.priority);
      const sel = document.createElement('select');
      sel.className = 'grid-cell-select';
      [1,2,3,4,5].forEach(p => {
        const opt = document.createElement('option');
        opt.value = String(p); opt.textContent = 'P' + p;
        if (String(p) === current) opt.selected = true;
        sel.appendChild(opt);
      });
      cell.replaceChildren(sel);
      sel.focus();

      const save = async () => {
        const newVal = sel.value;
        if (newVal === current) { cell.replaceChildren(span); return; }
        try {
          const updated = await api.patch('/tasks/' + cell.dataset.id, { priority: parseInt(newVal) });
          const tr = cell.closest('tr');
          tr.outerHTML = renderGridRow(updated);
          bindGridRowHandlers(el);
          toast('Priority updated');
        } catch (err) {
          toast(err.message, 'error');
          cell.replaceChildren(span);
        }
      };

      sel.onchange = save;
      sel.onblur = () => { cell.replaceChildren(span); };
      sel.onkeydown = e => { if (e.key === 'Escape') cell.replaceChildren(span); };
    };
  });

  // Type cell → inline select
  $$('.grid-type-cell', el).forEach(cell => {
    cell.onclick = e => {
      if (cell.querySelector('.grid-cell-select')) return;
      const span = cell.querySelector('.grid-type-span');
      const current = cell.dataset.type;
      const sel = document.createElement('select');
      sel.className = 'grid-cell-select';
      TASK_TYPES.forEach(t => {
        const opt = document.createElement('option');
        opt.value = t; opt.textContent = t;
        if (t === current) opt.selected = true;
        sel.appendChild(opt);
      });
      cell.replaceChildren(sel);
      sel.focus();

      const save = async () => {
        const newVal = sel.value;
        if (newVal === current) { cell.replaceChildren(span); return; }
        try {
          const updated = await api.patch('/tasks/' + cell.dataset.id, { type: newVal });
          const tr = cell.closest('tr');
          tr.outerHTML = renderGridRow(updated);
          bindGridRowHandlers(el);
          toast('Type updated');
        } catch (err) {
          toast(err.message, 'error');
          cell.replaceChildren(span);
        }
      };

      sel.onchange = save;
      sel.onblur = () => { cell.replaceChildren(span); };
      sel.onkeydown = e => { if (e.key === 'Escape') cell.replaceChildren(span); };
    };
  });
}

// ─── Kanban ───────────────────────────────────────────────────────────────────

function getKanbanColumns(tasks, groupBy) {
  if (groupBy === 'priority') {
    const defs = [
      { key: 1, label: 'P1 Critical' },
      { key: 2, label: 'P2 High' },
      { key: 3, label: 'P3 Normal' },
      { key: 4, label: 'P4 Low' },
      { key: 5, label: 'P5 Backlog' },
    ];
    const byPri = {};
    defs.forEach(d => { byPri[d.key] = []; });
    tasks.forEach(t => {
      const k = t.priority;
      if (byPri[k]) byPri[k].push(t);
      else byPri[3].push(t);
    });
    return defs.map(d => ({ key: d.key, label: d.label, tasks: byPri[d.key] }));
  }

  if (groupBy === 'type') {
    const allTypes = ['task','bug','issue','improvement','feature','vulnerability','chore','spike'];
    const byType = {};
    allTypes.forEach(t => { byType[t] = []; });
    tasks.forEach(t => {
      const k = t.type || 'task';
      if (byType[k]) byType[k].push(t);
      else byType['task'].push(t);
    });
    // Only include types that have at least 1 task
    return allTypes
      .filter(t => byType[t].length > 0)
      .map(t => ({ key: t, label: t.charAt(0).toUpperCase() + t.slice(1), tasks: byType[t] }));
  }

  if (groupBy === 'assignee') {
    const seen = new Set();
    const assignees = [];
    tasks.forEach(t => {
      const a = t.assignee || '';
      if (a && !seen.has(a)) { seen.add(a); assignees.push(a); }
    });
    assignees.sort();
    const byAssignee = {};
    assignees.forEach(a => { byAssignee[a] = []; });
    byAssignee[''] = [];
    tasks.forEach(t => {
      const a = t.assignee || '';
      if (byAssignee[a] !== undefined) byAssignee[a].push(t);
      else byAssignee[''].push(t);
    });
    const cols = assignees.map(a => ({ key: a, label: a, tasks: byAssignee[a] }));
    cols.push({ key: '', label: 'Unassigned', tasks: byAssignee[''] });
    return cols;
  }

  // default: status
  const defs = [
    { key: 'todo',  label: 'Todo' },
    { key: 'doing', label: 'Doing' },
    { key: 'done',  label: 'Done' },
  ];
  const byStatus = { todo: [], doing: [], done: [] };
  tasks.forEach(t => {
    if (byStatus[t.status]) byStatus[t.status].push(t);
    else byStatus['todo'].push(t);
  });
  return defs.map(d => ({ key: d.key, label: d.label, tasks: byStatus[d.key] }));
}

function buildKanbanMoveBtns(task, currentColKey, groupBy, columns) {
  return columns.map(col => {
    const isCurrent = String(col.key) === String(currentColKey);
    return `<button class="btn btn-sm kanban-move-btn ${isCurrent ? 'kanban-move-current' : 'btn-ghost'}"` +
      ` data-id="${task.id}"` +
      ` data-groupby="${escapeHtml(groupBy)}"` +
      ` data-colkey="${escapeHtml(String(col.key))}"` +
      (isCurrent ? ' disabled' : '') +
      `>${escapeHtml(col.label)}</button>`;
  }).join('');
}

// ─── Timeline view ───────────────────────────────────────────────────────────
// Reverse-chronological feed of tasks bucketed by local-day (Today, Yesterday,
// then full weekday + date). Sorted newest-first by created_at.

function timelineDayLabel(d) {
  const today = new Date();
  const yesterday = new Date(today.getTime() - 86400000);
  const sameDay = (a, b) => a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  if (sameDay(d, today)) {
    return 'Today (' + d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }) + ')';
  }
  if (sameDay(d, yesterday)) {
    return 'Yesterday (' + d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' }) + ')';
  }
  return d.toLocaleDateString(undefined, { weekday: 'long', month: 'short', day: 'numeric', year: 'numeric' });
}

function renderTimeline(tasks, el, hasFilters, showing) {
  if (tasks.length === 0) {
    el.innerHTML = '<div class="card"><div class="empty"><div class="empty-icon"></div><div class="empty-text">' +
      (hasFilters ? 'No tasks match the current filters.' : 'No tasks yet — create one to get started.') +
      '</div></div></div>';
    return;
  }
  const sorted = [...tasks].sort((a, b) => (b.created_at || 0) - (a.created_at || 0));
  const groups = [];
  let currentKey = '';
  let currentGroup = null;
  for (const t of sorted) {
    const d = new Date((t.created_at || 0) / 1e6);
    const key = d.toDateString();
    if (key !== currentKey) {
      currentKey = key;
      currentGroup = { day: d, items: [] };
      groups.push(currentGroup);
    }
    currentGroup.items.push(t);
  }
  const html = groups.map(g => {
    const items = g.items.map(t => {
      const time = new Date((t.created_at || 0) / 1e6).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
      const archived = !!t.archived_at;
      const rowClass = 'timeline-row task-row' + (archived ? ' task-row-archived' : '');
      const titleHtml = archived
        ? '<span><span class="task-title-archived">' + escapeHtml(t.title) + '</span> <span class="badge badge-archived">Archived</span></span>'
        : '<span>' + escapeHtml(t.title) + '</span>';
      return (
        '<div class="' + rowClass + '" data-id="' + t.id + '" data-seq="' + t.seq + '" data-status="' + t.status + '" style="cursor:pointer;display:grid;grid-template-columns:80px 88px 1fr auto auto;gap:12px;align-items:center;padding:8px 12px;border-bottom:1px solid var(--border-2)">' +
          '<span class="text-muted text-sm mono">' + escapeHtml(time) + '</span>' +
          '<span class="text-muted text-sm mono">TASK-' + t.seq + '</span>' +
          titleHtml +
          '<span class="text-muted text-sm">' + escapeHtml(t.type || '') + '</span>' +
          '<span>' + statusBadge(t.status) + ' ' + priorityBadge(t.priority) + '</span>' +
        '</div>'
      );
    }).join('');
    return (
      '<div class="timeline-group" style="margin-bottom:24px">' +
        '<div class="timeline-day" style="font-weight:600;color:var(--fg-muted);padding:8px 12px;border-bottom:1px solid var(--border)">' + escapeHtml(timelineDayLabel(g.day)) + '</div>' +
        items +
      '</div>'
    );
  }).join('');
  const loadMore = state.taskTotal > showing
    ? '<div class="load-more-bar"><button class="btn btn-ghost" id="btn-load-more">Load older (' + (state.taskTotal - showing) + ' remaining)</button></div>'
    : '';
  el.innerHTML = '<div class="card">' + html + loadMore + '</div>';
}

function renderKanban(tasks, el, hasFilters, showing) {
  const groupBy = state.kanbanGroupBy || 'status';
  const columns = getKanbanColumns(tasks, groupBy);

  if (columns.length === 0) {
    el.innerHTML = '<div class="kanban-empty" style="padding:32px;text-align:center">No tasks</div>';
    return;
  }

  el.innerHTML = `<div class="kanban">${columns.map(col => {
    const colTasks = col.tasks;
    return `
      <div class="kanban-col">
        <div class="kanban-col-header">
          <span class="kanban-col-name">${escapeHtml(col.label)}</span>
          <span class="count-badge">${colTasks.length}</span>
        </div>
        <div class="kanban-cards">
          ${colTasks.length === 0
            ? `<div class="kanban-empty">No tasks</div>`
            : colTasks.map(t => `
              <div class="kanban-card${t.archived_at ? ' task-row-archived' : ''}" data-id="${t.id}" data-seq="${t.seq}">
                <div class="kanban-card-ref mono">TASK-${t.seq}${t.archived_at ? ' <span class="badge badge-archived">Archived</span>' : ''}</div>
                <div class="kanban-card-title">${t.archived_at ? `<span class="task-title-archived">${escapeHtml(t.title)}</span>` : escapeHtml(t.title)}</div>
                <div class="kanban-card-meta">
                  ${priorityBadge(t.priority)}
                  <span class="text-muted text-sm">${escapeHtml(t.type)}</span>
                  ${t.project?.name || t.project?.alias ? `<span class="text-muted text-sm">${escapeHtml(t.project.name || t.project.alias)}</span>` : ''}
                </div>
                <div class="kanban-move-btns">
                  ${buildKanbanMoveBtns(t, col.key, groupBy, columns)}
                </div>
              </div>`).join('')}
        </div>
      </div>`;
  }).join('')}</div>`;

  // Card click → task detail
  $$('.kanban-card', el).forEach(card => {
    card.onclick = e => {
      if (e.target.closest('.kanban-move-btn')) return;
      navigate('task-detail', { currentTaskId: card.dataset.id, currentTaskSeq: card.dataset.seq });
    };
  });

  // Move buttons
  $$('.kanban-move-btn:not([disabled])', el).forEach(btn => {
    btn.onclick = async e => {
      e.stopPropagation();
      try {
        const gb = btn.dataset.groupby;
        const colKey = btn.dataset.colkey;
        if (gb === 'status') {
          await api.patch('/tasks/' + btn.dataset.id + '/status', { status: colKey });
        } else if (gb === 'priority') {
          await api.patch('/tasks/' + btn.dataset.id, { priority: parseInt(colKey) });
        } else if (gb === 'type') {
          await api.patch('/tasks/' + btn.dataset.id, { type: colKey });
        } else if (gb === 'assignee') {
          await api.patch('/tasks/' + btn.dataset.id, { assignee: colKey });
        }
        render();
      } catch (err) { toast(err.message, 'error'); }
    };
  });
}

function showNewTaskModal() {
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header">
        <span class="modal-title">New Task</span>
        <button class="modal-close">×</button>
      </div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Project *</label>
          <select class="form-control form-select" id="new-task-project">
            ${state.projects.map(p =>
              `<option value="${p.alias}" ${p.alias===state.project?'selected':''}>${escapeHtml(p.name || p.alias)}</option>`
            ).join('')}
          </select>
        </div>
        <div class="form-group">
          <label class="form-label">Title *</label>
          <input class="form-control" id="new-task-title" placeholder="Task title">
        </div>
        <div class="form-group">
          <label class="form-label">Description</label>
          <textarea class="form-control prose-area" id="new-task-desc" placeholder="Optional — supports markdown"></textarea>
        </div>
        <div class="form-group">
          <label class="form-label">Path <span class="text-muted">(optional)</span></label>
          <input class="form-control" id="new-task-path" placeholder="e.g. internal/handlers/search.go:84 or https://…">
        </div>
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
          <div class="form-group">
            <label class="form-label">Assignee <span class="text-muted">(optional)</span></label>
            <input class="form-control" id="new-task-assignee" placeholder="Name or handle">
          </div>
          <div class="form-group">
            <label class="form-label">Due date <span class="text-muted">(optional)</span></label>
            <input class="form-control" id="new-task-due" type="date">
          </div>
        </div>
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
          <div class="form-group">
            <label class="form-label">Source <span class="text-muted">(optional)</span></label>
            <input class="form-control" id="new-task-source" placeholder="e.g. security-review">
          </div>
          <div class="form-group">
            <label class="form-label">External ref <span class="text-muted">(optional)</span></label>
            <input class="form-control" id="new-task-external" placeholder="URL, ticket, or scan ID">
          </div>
        </div>
        <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:12px">
          <div class="form-group">
            <label class="form-label">Type</label>
            <select class="form-control form-select" id="new-task-type">
              ${['task','bug','issue','improvement','feature','vulnerability','chore','spike']
                .map(t => `<option value="${t}">${t}</option>`).join('')}
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Priority</label>
            <select class="form-control form-select" id="new-task-priority">
              ${[1,2,3,4,5].map(p => `<option value="${p}" ${p===3?'selected':''}>P${p}</option>`).join('')}
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Status</label>
            <select class="form-control form-select" id="new-task-status">
              <option value="todo">todo</option>
              <option value="doing">doing</option>
              <option value="done">done</option>
            </select>
          </div>
        </div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-create-task">Create Task</button>
      </div>
    </div>`);

  const mde = createMDE(overlay.querySelector('#new-task-desc'), { minHeight: '420px' });

  const doCreate = async () => {
    const title = overlay.querySelector('#new-task-title').value.trim();
    const project = overlay.querySelector('#new-task-project').value;
    if (!title) { toast('Title is required', 'error'); return; }
    if (!project) { toast('Select a project', 'error'); return; }
    try {
      const body = {
        project, title,
        description: mde.value(),
        type: overlay.querySelector('#new-task-type').value,
        priority: parseInt(overlay.querySelector('#new-task-priority').value),
        status: overlay.querySelector('#new-task-status').value,
      };
      const newPath = overlay.querySelector('#new-task-path').value.trim();
      const assignee = overlay.querySelector('#new-task-assignee').value.trim();
      const dueDate = overlay.querySelector('#new-task-due').value;
      const source = overlay.querySelector('#new-task-source').value.trim();
      const externalRef = overlay.querySelector('#new-task-external').value.trim();
      if (newPath) body.project_path = newPath;
      if (assignee) body.assignee = assignee;
      if (dueDate) body.due_date = dueDate;
      if (source) body.source = source;
      if (externalRef) body.external_ref = externalRef;
      await api.post('/tasks', body);
      overlay.remove();
      toast('Task created');
      render();
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-create-task').onclick = doCreate;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doCreate(); });
}

// ─── Inline editing helpers ───────────────────────────────────────────────────
function showMetaDropdown(anchor, options, onSelect) {
  document.querySelectorAll('.meta-dropdown').forEach(d => d.remove());
  const rect = anchor.getBoundingClientRect();
  const dd = document.createElement('div');
  dd.className = 'meta-dropdown';
  dd.style.cssText = `position:fixed;top:${rect.bottom + 3}px;left:${rect.left}px;z-index:400;min-width:${Math.max(rect.width, 120)}px`;
  options.forEach(([val, label, extra]) => {
    const item = document.createElement('div');
    item.className = 'meta-dropdown-item' + (extra === 'current' ? ' meta-dropdown-current' : '');
    item.innerHTML = label;
    item.addEventListener('mousedown', e => {
      e.preventDefault();
      dd.remove();
      onSelect(val);
    });
    dd.appendChild(item);
  });
  document.body.appendChild(dd);
  const close = e => { if (!dd.contains(e.target)) { dd.remove(); document.removeEventListener('mousedown', close); } };
  setTimeout(() => document.addEventListener('mousedown', close), 0);
  return dd;
}

function makeMetaEditable(task) {
  const wire = (id, options, saveFn) => {
    const el = document.getElementById(id);
    if (!el) return;
    // Anchor the dropdown next to the value pill, but accept clicks anywhere
    // on the parent .prop-editable row so the affordance band is generous.
    const row = el.closest('.prop-editable') || el;
    const handler = e => {
      e.stopPropagation();
      if (document.querySelector('.meta-dropdown')) { document.querySelectorAll('.meta-dropdown').forEach(d => d.remove()); return; }
      showMetaDropdown(el, options, async val => {
        try {
          await saveFn(val);
          navigate('task-detail', { currentTaskId: task.id, currentTaskSeq: task.seq, _replace: true });
        } catch (err) { toast(err.message, 'error'); }
      });
    };
    row.addEventListener('click', handler);
  };

  wire('meta-status',
    [['todo', statusBadge('todo'), task.status==='todo'?'current':null],
     ['doing', statusBadge('doing'), task.status==='doing'?'current':null],
     ['done', statusBadge('done'), task.status==='done'?'current':null]],
    val => api.patch('/tasks/' + task.id + '/status', { status: val })
  );

  wire('meta-priority',
    [[1,priorityBadge(1)],[2,priorityBadge(2)],[3,priorityBadge(3)],[4,priorityBadge(4)],[5,priorityBadge(5)]],
    val => api.patch('/tasks/' + task.id, { priority: val })
  );

  wire('meta-type',
    ['task','bug','issue','improvement','feature','vulnerability','chore','spike','bucket-list']
      .map(v => [v, `<span class="text-sm">${v}</span>`]),
    val => api.patch('/tasks/' + task.id, { type: val })
  );

  // Title inline edit
  const titleEl = document.getElementById('detail-title');
  if (titleEl) {
    titleEl.addEventListener('click', () => {
      if (titleEl.querySelector('input')) return;
      const orig = titleEl.innerHTML;
      const input = document.createElement('input');
      input.className = 'detail-title-input';
      input.value = task.title;
      titleEl.innerHTML = '';
      titleEl.appendChild(input);
      input.focus(); input.select();
      let done = false;
      const save = async () => {
        if (done) return; done = true;
        const v = input.value.trim();
        if (!v || v === task.title) { titleEl.innerHTML = orig; return; }
        try {
          await api.patch('/tasks/' + task.id, { title: v });
          navigate('task-detail', { currentTaskId: task.id, currentTaskSeq: task.seq, _replace: true });
        } catch (err) { toast(err.message, 'error'); titleEl.innerHTML = orig; done = false; }
      };
      setTimeout(() => input.addEventListener('blur', save), 0);
      input.addEventListener('keydown', e => {
        if (e.key === 'Enter') { e.preventDefault(); save(); }
        if (e.key === 'Escape') { e.preventDefault(); done = true; titleEl.innerHTML = orig; }
      });
    });
  }
}

function showTaskLabelPopover(anchorEl, task, suggestions = []) {
  const container = document.createElement('div');

  const header = document.createElement('div');
  header.className = 'popover-header';
  header.textContent = 'Add label';
  container.appendChild(header);

  const form = document.createElement('div');
  form.className = 'task-label-popover-form';

  const input = document.createElement('input');
  input.className = 'task-label-popover-input';
  input.placeholder = 'Label name';
  input.autocomplete = 'off';

  const button = document.createElement('button');
  button.className = 'task-label-popover-submit';
  button.type = 'button';
  button.textContent = '+';
  button.title = 'Add label';

  form.appendChild(input);
  form.appendChild(button);
  container.appendChild(form);

  const attach = async name => {
    const label = (name || '').trim();
    if (!label) return;
    try {
      await api.post('/tasks/' + task.id + '/labels', { name: label });
      if (task.project?.alias) delete state.projectLabels[task.project.alias];
      close();
      toast('Label attached');
      navigate('task-detail', { currentTaskId: task.id, currentTaskSeq: task.seq, _replace: true });
    } catch (err) {
      toast(err.message, 'error');
    }
  };

  const existing = (suggestions || []).filter(Boolean);
  if (existing.length) {
    const list = document.createElement('div');
    list.className = 'task-label-popover-suggestions';
    existing.slice(0, 8).forEach(label => {
      const row = document.createElement('button');
      row.className = 'popover-item task-label-suggestion';
      row.type = 'button';
      row.textContent = label.name || label;
      row.onclick = () => attach(row.textContent);
      list.appendChild(row);
    });
    container.appendChild(list);
  }

  const { close } = showPopover(anchorEl, container);
  setTimeout(() => input.focus(), 0);
  button.onclick = () => attach(input.value);
  input.addEventListener('keydown', e => {
    if (e.key === 'Enter') {
      e.preventDefault();
      attach(input.value);
    }
  });
}

// ─── Task Detail ─────────────────────────────────────────────────────────────
async function renderTaskDetail(el) {
  let task;
  try {
    task = await api.get('/tasks/' + state.currentTaskId + '?with_plans=1&with_comments=1');
    if (task.project?.alias) await getLabelsForProject(task.project.alias);
  } catch (e) { toast(e.message, 'error'); navigate('tasks'); return; }

  const projectLabels = task.project?.alias ? (state.projectLabels[task.project.alias] || []) : [];
  const attachedLabelNames = new Set((task.labels || []).map(l => l.name));
  const unattachedLabels = projectLabels.filter(l => !attachedLabelNames.has(l.name));

  el.innerHTML = html`
    <div class="page-header">
      <div style="flex:1;min-width:0">
        <a href="#" id="back-to-tasks" class="back-crumb">Tasks</a>
        <div class="task-id-label mono">TASK-${task.seq}</div>
        <div class="page-title">
          <span id="detail-title" class="detail-title-editable" title="Click to edit">${escapeHtml(task.title)}</span>
        </div>
      </div>
      <div class="flex gap-2 items-center" style="flex-shrink:0">
        ${task.archived_at ? '<span class="badge badge-archived" title="Archived ' + escapeHtml(formatDate(task.archived_at)) + '">Archived</span>' : ''}
        <button class="btn btn-ghost btn-sm" id="btn-download-task">Download</button>
        <button class="btn btn-ghost btn-sm" id="btn-edit-task">Edit</button>
        ${task.archived_at
          ? '<button class="btn btn-ghost btn-sm" id="btn-archive-task" title="Restore task">Restore</button>'
          : '<button class="btn btn-ghost btn-sm btn-danger-ghost" id="btn-archive-task" title="Archive task">Archive</button>'}
      </div>
    </div>
    <div class="detail-grid">
      <div class="detail-body">
        ${task.description ? `
          <section class="task-section">
            <div class="markdown">${renderMarkdown(task.description)}</div>
          </section>` : ''}

        <section class="task-section">
          <div class="section-head">
            <h3 class="section-title">Plans <span class="count-badge">${(task.plans || []).length}</span></h3>
            <button class="btn btn-ghost btn-sm" id="btn-add-plan">+ Add plan</button>
          </div>
          ${(task.plans || []).length === 0
            ? '<p class="section-empty">No plans yet.</p>'
            : (task.plans || []).map(p => `
              <div class="plan-item" data-plan-id="${p.id}">
                <div class="flex justify-between items-center" style="margin-bottom:6px">
                  <strong>${escapeHtml(p.version?.title || 'Plan')}</strong>
                  <div class="flex gap-2 items-center">
                    <span class="text-muted text-sm">v${p.current_version}</span>
                    <button class="btn btn-ghost btn-sm plan-edit-btn" data-plan-id="${p.id}">Edit</button>
                    <button class="btn btn-ghost btn-sm plan-history-btn" data-plan-id="${p.id}">History</button>
                  </div>
                </div>
                <div class="markdown text-sm">${renderMarkdown(p.version?.body || '')}</div>
              </div>`).join('')}
        </section>

        <section class="task-section">
          <div class="section-head">
            <h3 class="section-title">Comments <span class="count-badge">${(task.comments || []).length}</span></h3>
          </div>
          ${(task.comments || []).length === 0
            ? '<p class="section-empty">No comments yet.</p>'
            : (task.comments || []).map(c => `
              <div class="comment-item">
                <div class="comment-meta">
                  <span class="comment-actor mono">${escapeHtml(c.actor?.kind)}:${escapeHtml(c.actor?.name)}</span>
                  <span class="comment-time" title="${formatDate(c.created_at)}">${timeAgo(c.created_at)}</span>
                </div>
                <div class="comment-body">${escapeHtml(c.body)}</div>
              </div>`).join('')}
          <div class="comment-composer">
            <textarea class="form-control prose-area" id="new-comment" placeholder="Add a comment…"></textarea>
            <button class="btn btn-primary btn-sm" id="btn-add-comment">Comment</button>
          </div>
        </section>

        <section class="task-section">
          <div class="section-head">
            <h3 class="section-title">Attachments <span class="count-badge" id="attach-count">…</span></h3>
          </div>
          <div id="task-attachments-list"><div class="text-muted text-sm">Loading…</div></div>
          <div class="attach-upload" style="margin-top:10px">
            <label class="btn btn-ghost btn-sm" style="cursor:pointer">
              + Attach file
              <input type="file" id="task-attach-file" style="display:none" multiple>
            </label>
            <span id="attach-upload-status" class="text-muted text-sm" style="margin-left:8px"></span>
          </div>
        </section>

        <section class="task-section">
          <div class="section-head">
            <h3 class="section-title">Activity</h3>
          </div>
          <div id="task-activity-list"><div class="text-muted text-sm">Loading…</div></div>
        </section>
      </div>
      <aside class="detail-aside">
        <div class="properties">
          <div class="prop prop-editable" data-meta-row="status" title="Click to change status"><div class="prop-label">Status</div><div class="prop-value"><span class="meta-editable" id="meta-status">${statusBadge(task.status)}</span>${caretIcon()}</div></div>
          <div class="prop prop-editable" data-meta-row="priority" title="Click to change priority"><div class="prop-label">Priority</div><div class="prop-value"><span class="meta-editable" id="meta-priority">${priorityBadge(task.priority)}</span>${caretIcon()}</div></div>
          <div class="prop prop-editable" data-meta-row="type" title="Click to change type"><div class="prop-label">Type</div><div class="prop-value"><span class="meta-editable" id="meta-type">${escapeHtml(task.type)}</span>${caretIcon()}</div></div>
          <div class="prop prop-readonly"><div class="prop-label">Project</div><div class="prop-value">${escapeHtml(task.project?.name || task.project?.alias || '')}</div></div>
          <div class="prop prop-readonly"><div class="prop-label">Actor</div><div class="prop-value mono text-sm">${escapeHtml(task.actor?.kind)}:${escapeHtml(task.actor?.name)}</div></div>
          <div class="prop prop-readonly"><div class="prop-label">Created</div><div class="prop-value text-muted" title="${formatDate(task.created_at)} UTC">${timeAgo(task.created_at)}<div class="prop-subtext">${formatLocalDateTime(task.created_at)}</div></div></div>
          ${task.assignee ? `<div class="prop prop-readonly"><div class="prop-label">Assignee</div><div class="prop-value">${escapeHtml(task.assignee)}</div></div>` : ''}
          ${task.due_at ? `<div class="prop prop-readonly"><div class="prop-label">Due</div><div class="prop-value" title="${formatDate(task.due_at)} UTC">${timeAgo(task.due_at)}<div class="prop-subtext">${formatLocalDateTime(task.due_at)}</div></div></div>` : ''}
          ${task.source ? `<div class="prop prop-readonly"><div class="prop-label">Source</div><div class="prop-value">${escapeHtml(task.source)}</div></div>` : ''}
          ${task.external_ref ? `<div class="prop prop-readonly"><div class="prop-label">External</div><div class="prop-value">${/^https?:\/\//.test(task.external_ref) ? `<a href="${escapeHtml(task.external_ref)}" target="_blank">${escapeHtml(task.external_ref)}</a>` : escapeHtml(task.external_ref)}</div></div>` : ''}
          ${task.project_path ? `<div class="prop prop-readonly"><div class="prop-label">Path</div><div class="prop-value">${/^https?:\/\//.test(task.project_path) ? `<a href="${escapeHtml(task.project_path)}" target="_blank">${escapeHtml(task.project_path)}</a>` : `<code>${escapeHtml(task.project_path)}</code>`}</div></div>` : ''}
          <div class="prop prop-readonly prop-label-manager">
            <div class="prop-label">Labels</div>
            <div class="prop-value">
              <div class="task-label-list">
                ${(task.labels || []).length === 0
                  ? '<span class="task-label-empty">-</span>'
                  : (task.labels || []).map(l => `<span class="badge label-badge task-label-pill">${escapeHtml(l.name)} <button class="label-detach" data-label="${escapeHtml(l.name)}" title="Detach label">×</button></span>`).join('')}
                <button class="task-label-trigger" id="btn-open-label-add" title="Add label">+ Label</button>
              </div>
            </div>
          </div>
        </div>
      </aside>
    </div>`;

  $('#back-to-tasks').onclick = e => { e.preventDefault(); navigate('tasks'); };

  makeMetaEditable(task);

  loadTaskActivity(task.id);

  loadTaskAttachments(task.id);

  const attachInput = document.getElementById('task-attach-file');
  const uploadStatus = document.getElementById('attach-upload-status');
  if (attachInput) {
    attachInput.onchange = async () => {
      const files = [...attachInput.files];
      if (!files.length) return;
      uploadStatus.textContent = 'Uploading…';
      let failed = 0;
      for (const f of files) {
        const fd = new FormData();
        fd.append('file', f);
        fd.append('linked_type', 'task');
        fd.append('linked_id', task.id);
        try {
          const res = await fetch('/api/attachments', { method: 'POST', body: fd });
          if (!res.ok) throw new Error(await res.text());
        } catch(e) {
          failed++;
          toast('Upload failed: ' + e.message, 'error');
        }
      }
      attachInput.value = '';
      uploadStatus.textContent = '';
      if (!failed) toast(files.length === 1 ? 'File attached' : files.length + ' files attached');
      loadTaskAttachments(task.id);
    };
  }

  $('#btn-download-task').onclick = e => showTaskDownloadPopover(e.currentTarget, task);

  $('#btn-edit-task').onclick = () => showEditTaskModal(task);

  $('#btn-open-label-add').onclick = e => showTaskLabelPopover(e.currentTarget, task, unattachedLabels);

  $$('.label-detach').forEach(btn => btn.onclick = async e => {
    e.preventDefault();
    e.stopPropagation();
    try {
      await api.del('/tasks/' + task.id + '/labels/' + encodeURIComponent(btn.dataset.label));
      toast('Label detached');
      navigate('task-detail', { currentTaskId: task.id, currentTaskSeq: task.seq, _replace: true });
    } catch (err) { toast(err.message, 'error'); }
  });

  $('#btn-archive-task').onclick = async () => {
    if (task.archived_at) {
      try {
        await api.patch('/tasks/' + task.id + '/unarchive', {});
        toast('Task restored');
        navigate('task-detail', { currentTaskId: task.id, currentTaskSeq: task.seq, _replace: true });
      } catch (err) {
        toast(err.message, 'error');
      }
      return;
    }
    const ok = await confirm(
      'Archive this task? It will be hidden from active lists but recoverable from the Archived view.',
      { confirmLabel: 'Archive task', confirmClass: 'btn-primary' }
    );
    if (!ok) return;
    try {
      await api.patch('/tasks/' + task.id + '/archive', {});
      toast('Task archived');
      navigate('tasks');
    } catch (err) {
      toast(err.message, 'error');
    }
  };

  $('#btn-add-comment').onclick = async () => {
    const body = $('#new-comment').value.trim();
    if (!body) return;
    try {
      await api.post('/tasks/' + task.id + '/comments', { body });
      toast('Comment added');
      navigate('task-detail', { currentTaskId: task.id, currentTaskSeq: task.seq, _replace: true });
    } catch (err) { toast(err.message, 'error'); }
  };

  // Allow Ctrl+Enter in comment box
  $('#new-comment').addEventListener('keydown', e => {
    if (e.key === 'Enter' && e.ctrlKey) $('#btn-add-comment').click();
  });

  $('#btn-add-plan').onclick = () => {
    const overlay = showModal(html`
      <div class="modal modal-lg">
        <div class="modal-header"><span class="modal-title">Add Plan</span><button class="modal-close">×</button></div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Title *</label>
            <input class="form-control" id="plan-title" placeholder="Plan title">
          </div>
          <div class="form-group">
            <label class="form-label">Body (Markdown)</label>
            <textarea class="form-control" id="plan-body"></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
          <button class="btn btn-ghost modal-close">Cancel</button>
          <button class="btn btn-primary" id="btn-save-plan">Save Plan</button>
        </div>
      </div>`);
    const planMde = createMDE(overlay.querySelector('#plan-body'), { minHeight: '420px' });
    const doSave = async () => {
      const title = overlay.querySelector('#plan-title').value.trim();
      if (!title) { toast('Title required', 'error'); return; }
      try {
        await api.post('/tasks/' + task.id + '/plans', {
          title, body: planMde.value(),
        });
        overlay.remove();
        toast('Plan added');
        navigate('task-detail', { currentTaskId: task.id, currentTaskSeq: task.seq, _replace: true });
      } catch (err) { toast(err.message, 'error'); }
    };
    overlay.querySelector('#btn-save-plan').onclick = doSave;
    overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doSave(); });
  };

  $$('.plan-edit-btn').forEach(btn => btn.onclick = async () => {
    try {
      const plan = await api.get('/plans/' + btn.dataset.planId);
      showEditPlanModal(plan, task.id);
    } catch (err) { toast(err.message, 'error'); }
  });

  $$('.plan-history-btn').forEach(btn => btn.onclick = () => {
    showPlanHistoryModal(btn.dataset.planId);
  });
}

async function loadTaskActivity(taskId) {
  const el = document.getElementById('task-activity-list');
  if (!el) return;
  try {
    const data = await api.get('/activity?entity=task&entity_id=' + taskId);
    const events = data.events || [];
    if (!events.length) {
      el.innerHTML = '<div class="text-muted text-sm">No activity yet.</div>';
      return;
    }
    el.innerHTML = events.map(e => {
      const actor = escapeHtml((e.actor?.kind || e.actor_kind || '') + ':' + (e.actor?.name || e.actor_name || ''));
      const ago = timeAgo(e.created_at);
      return `<div class="activity-row"><span class="activity-actor">${actor}</span> <span class="activity-action">${escapeHtml(e.action)}</span> <span class="text-muted text-sm">${ago}</span>${e.summary ? '<div class="activity-summary text-sm">' + escapeHtml(e.summary) + '</div>' : ''}</div>`;
    }).join('');
  } catch(e) {
    el.innerHTML = '<div class="text-muted text-sm">Could not load activity.</div>';
  }
}

async function loadTaskAttachments(taskId) {
  const listEl = document.getElementById('task-attachments-list');
  const countEl = document.getElementById('attach-count');
  if (!listEl) return;
  try {
    const data = await api.get('/attachments?task_id=' + taskId);
    const attachments = data.attachments || [];
    if (countEl) countEl.textContent = attachments.length;
    if (!attachments.length) {
      listEl.innerHTML = '<p class="section-empty">No attachments.</p>';
      return;
    }
    listEl.innerHTML = '<table class="attach-table"><tbody>' +
      attachments.map(a => {
        const size = formatBytes(a.size);
        const ago = timeAgo(a.created_at);
        const url = '/api/attachments/' + encodeURIComponent(a.id);
        const isImage = (a.mime_type || '').startsWith('image/');
        return `<tr class="attach-row" data-attach-id="${a.id}">
          <td class="attach-name"><a href="#" class="attach-name-link" data-id="${a.id}" data-name="${escapeHtml(a.name)}" data-mime="${escapeHtml(a.mime_type || '')}" data-is-image="${isImage}" title="${escapeHtml(a.name)}">${escapeHtml(a.name)}</a></td>
          <td class="text-muted text-sm">${size}</td>
          <td class="text-muted text-sm">${ago}</td>
          <td class="attach-actions">
            <a class="btn btn-ghost btn-sm" href="${url}" download="${escapeHtml(a.name)}">Download</a>
            <button class="btn btn-ghost btn-sm btn-danger-ghost attach-delete-btn" data-id="${a.id}">Delete</button>
          </td>
        </tr>`;
      }).join('') +
    '</tbody></table>';

    // Wire delete buttons
    // Click filename → preview modal (images) or open in new tab (others)
    listEl.querySelectorAll('.attach-name-link').forEach(link => {
      link.onclick = e => {
        e.preventDefault();
        if (link.dataset.isImage === 'true') {
          showAttachmentPreview(link.dataset.id, link.dataset.name, link.dataset.mime);
        } else {
          window.open('/api/attachments/' + link.dataset.id, '_blank');
        }
      };
    });

    listEl.querySelectorAll('.attach-delete-btn').forEach(btn => {
      btn.onclick = async () => {
        if (!await confirm('Delete this attachment? This cannot be undone.')) return;
        btn.disabled = true;
        try {
          await fetch('/api/attachments/' + btn.dataset.id, { method: 'DELETE' });
          btn.closest('tr').remove();
          const remaining = listEl.querySelectorAll('.attach-row').length;
          if (countEl) countEl.textContent = remaining;
          if (!remaining) listEl.innerHTML = '<p class="section-empty">No attachments.</p>';
          toast('Attachment deleted');
        } catch(e) {
          toast('Delete failed: ' + e.message, 'error');
          btn.disabled = false;
        }
      };
    });
  } catch(e) {
    if (listEl) listEl.innerHTML = '<p class="section-empty text-muted">Could not load attachments.</p>';
  }
}

function showEditTaskModal(task) {
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header"><span class="modal-title">Edit Task</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Title *</label>
          <input class="form-control" id="edit-title">
        </div>
        <div class="form-group">
          <label class="form-label">Description</label>
          <textarea class="form-control prose-area" id="edit-desc"></textarea>
        </div>
        <div class="form-group">
          <label class="form-label">Path <span class="text-muted">(optional)</span></label>
          <input class="form-control" id="edit-path" placeholder="e.g. internal/handlers/search.go:84 or https://…">
        </div>
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
          <div class="form-group">
            <label class="form-label">Assignee <span class="text-muted">(optional)</span></label>
            <input class="form-control" id="edit-assignee" placeholder="Name or handle">
          </div>
          <div class="form-group">
            <label class="form-label">Due date <span class="text-muted">(optional)</span></label>
            <input class="form-control" id="edit-due" type="date">
          </div>
        </div>
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
          <div class="form-group">
            <label class="form-label">Source <span class="text-muted">(optional)</span></label>
            <input class="form-control" id="edit-source" placeholder="e.g. security-review">
          </div>
          <div class="form-group">
            <label class="form-label">External ref <span class="text-muted">(optional)</span></label>
            <input class="form-control" id="edit-external" placeholder="URL, ticket, or scan ID">
          </div>
        </div>
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px">
          <div class="form-group">
            <label class="form-label">Type</label>
            <select class="form-control form-select" id="edit-type">
              ${['task','bug','issue','improvement','feature','vulnerability','chore','spike']
                .map(t => `<option value="${t}" ${t===task.type?'selected':''}>${t}</option>`).join('')}
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Priority</label>
            <select class="form-control form-select" id="edit-priority">
              ${[1,2,3,4,5].map(p => `<option value="${p}" ${p===task.priority?'selected':''}>P${p}</option>`).join('')}
            </select>
          </div>
        </div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-save-edit">Save</button>
      </div>
    </div>`);

  // Set values safely via JS (avoids attribute injection issues with special chars)
  overlay.querySelector('#edit-title').value = task.title;
  overlay.querySelector('#edit-path').value = task.project_path || '';
  overlay.querySelector('#edit-assignee').value = task.assignee || '';
  overlay.querySelector('#edit-due').value = nsToDateInput(task.due_at);
  overlay.querySelector('#edit-source').value = task.source || '';
  overlay.querySelector('#edit-external').value = task.external_ref || '';

  const mde = createMDE(overlay.querySelector('#edit-desc'), { minHeight: '420px' });
  mde.value(task.description || '');

  const doSave = async () => {
    const title = overlay.querySelector('#edit-title').value.trim();
    if (!title) { toast('Title required', 'error'); return; }
    try {
      const dueDate = overlay.querySelector('#edit-due').value;
      await api.patch('/tasks/' + task.id, {
        title,
        description: mde.value(),
        type: overlay.querySelector('#edit-type').value,
        priority: parseInt(overlay.querySelector('#edit-priority').value),
        assignee: overlay.querySelector('#edit-assignee').value.trim(),
        due_date: dueDate || undefined,
        clear_due_at: !dueDate,
        source: overlay.querySelector('#edit-source').value.trim(),
        external_ref: overlay.querySelector('#edit-external').value.trim(),
        project_path: overlay.querySelector('#edit-path').value,
      });
      overlay.remove();
      toast('Task updated');
      navigate('task-detail', { currentTaskId: task.id, currentTaskSeq: task.seq, _replace: true });
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-save-edit').onclick = doSave;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doSave(); });
}

function showEditPlanModal(plan, taskId) {
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header"><span class="modal-title">Edit Plan</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Title *</label>
          <input class="form-control" id="edit-plan-title">
        </div>
        <div class="form-group">
          <label class="form-label">Body (Markdown)</label>
          <textarea class="form-control" id="edit-plan-body"></textarea>
        </div>
        <div class="form-group">
          <label class="form-label">Change note <span class="text-muted">(optional)</span></label>
          <input class="form-control" id="edit-plan-note" placeholder="What changed?">
        </div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-save-plan-edit">Save</button>
      </div>
    </div>`);

  overlay.querySelector('#edit-plan-title').value = plan.version?.title || '';

  const mde = createMDE(overlay.querySelector('#edit-plan-body'), { minHeight: '420px' });
  mde.value(plan.version?.body || '');

  const doSave = async () => {
    const title = overlay.querySelector('#edit-plan-title').value.trim();
    if (!title) { toast('Title required', 'error'); return; }
    try {
      await api.patch('/plans/' + plan.id, {
        title,
        body: mde.value(),
        change_note: overlay.querySelector('#edit-plan-note').value,
      });
      overlay.remove();
      toast('Plan updated');
      navigate('task-detail', { currentTaskId: taskId, currentTaskSeq: state.currentTaskSeq, _replace: true });
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-save-plan-edit').onclick = doSave;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doSave(); });
}

async function showPlanHistoryModal(planId) {
  let versions;
  try {
    const data = await api.get('/plans/' + planId + '/history');
    versions = data.versions || [];
  } catch (e) { toast(e.message, 'error'); return; }

  const overlay = showModal(html`
    <div class="modal" style="width:680px">
      <div class="modal-header">
        <span class="modal-title">Plan History</span>
        <button class="modal-close">×</button>
      </div>
      <div class="modal-body" style="max-height:520px;overflow-y:auto">
        ${versions.length === 0
          ? '<p class="text-muted text-sm">No history available.</p>'
          : versions.map(v => `
            <div class="plan-history-item" style="margin-bottom:16px;border-bottom:1px solid var(--border);padding-bottom:16px">
              <div class="flex justify-between items-center" style="margin-bottom:6px">
                <div class="flex gap-2 items-center">
                  <strong>v${v.version}</strong>
                  <span class="text-muted text-sm">${escapeHtml(v.title)}</span>
                </div>
                <div class="text-muted text-sm">
                  <span class="mono">${escapeHtml(v.actor?.kind)}:${escapeHtml(v.actor?.name)}</span>
                  · <span title="${formatDate(v.created_at)}">${timeAgo(v.created_at)}</span>
                </div>
              </div>
              ${v.change_note ? `<p class="text-muted text-sm" style="margin-bottom:6px;font-style:italic">${escapeHtml(v.change_note)}</p>` : ''}
              <details>
                <summary class="btn btn-ghost btn-sm" style="display:inline-block;cursor:pointer">Show body</summary>
                <div class="markdown text-sm" style="margin-top:8px">${renderMarkdown(v.body)}</div>
              </details>
            </div>`).join('')}
      </div>
    </div>`);
}

// ─── Plans list ──────────────────────────────────────────────────────────────

async function renderPlans(el) {
  // When no project is selected the API aggregates plans across every
  // (non-archived) project and tags each row with project_alias / project_name
  // so we can render a project chip in the Project column.
  const aggregated = !state.project;
  const url = aggregated ? '/plans' : '/plans?project=' + encodeURIComponent(state.project);

  try {
    const data = await api.get(url);
    const allPlans = data.plans || [];
    const search = (state.plansSearch || '').toLowerCase();
    const plans = search
      ? allPlans.filter(p => {
          const title = (p.version?.title || '').toLowerCase();
          const ref = ('task-' + (p.task_seq ?? '')).toLowerCase();
          const proj = (p.project_name || p.project_alias || '').toLowerCase();
          return title.includes(search) || ref.includes(search) || proj.includes(search);
        })
      : allPlans;
    const subtitle = search
      ? `${plans.length} of ${allPlans.length} plan${allPlans.length !== 1 ? 's' : ''}`
      : `${allPlans.length} plan${allPlans.length !== 1 ? 's' : ''}`;

    const placeholder = aggregated
      ? 'Search plans by title, TASK-N, or project…'
      : 'Search plans by title or TASK-N…';
    const toolbarHTML =
      `<input class="search-input" id="plans-search" placeholder="${placeholder}" value="${escapeHtml(state.plansSearch || '')}">`;

    el.innerHTML =
      pageHeader({
        title: 'Plans',
        subtitle,
        viewToggle: '<button class="btn btn-ghost" id="btn-plans-download">Download</button>',
        toolbar: toolbarHTML,
      }) + html`
      <div class="card">
        <div class="table-wrap">
          <table>
            <thead><tr>
              <th>Task</th>
              <th>Title</th>
              ${aggregated ? '<th>Project</th>' : ''}
              <th>Version</th>
              <th>Actor</th>
              <th>Updated</th>
              <th></th>
            </tr></thead>
            <tbody>
              ${plans.length === 0
                ? `<tr><td colspan="${aggregated ? 7 : 6}"><div class="empty"><div class="empty-icon"></div><div class="empty-text">${search ? 'No plans match the search.' : 'No plans recorded yet. Open a task to attach one.'}</div></div></td></tr>`
                : plans.map(p => `
                  <tr>
                    <td class="text-muted text-sm mono">
                      <a href="#" class="plan-task-link" data-task-id="${p.task_id}" data-task-seq="${p.task_seq}">TASK-${p.task_seq}</a>
                    </td>
                    <td>${escapeHtml(p.version?.title || '')}</td>
                    ${aggregated ? `<td>${projectChipHTML(p.project_alias, p.project_name)}</td>` : ''}
                    <td class="text-muted text-sm">v${p.current_version}</td>
                    <td class="text-muted text-sm mono">${escapeHtml(p.version?.actor?.kind || '')}:${escapeHtml(p.version?.actor?.name || '')}</td>
                    <td class="text-muted text-sm" title="${formatDate(p.updated_at)}">${timeAgo(p.updated_at)}</td>
                    <td>
                      <button class="btn btn-ghost btn-sm plan-list-edit" data-plan-id="${p.id}" data-task-id="${p.task_id}">Edit</button>
                      <button class="btn btn-ghost btn-sm plan-list-history" data-plan-id="${p.id}">History</button>
                    </td>
                  </tr>`).join('')}
            </tbody>
          </table>
        </div>
      </div>`;

    let plansSearchTimer;
    $('#plans-search').oninput = e => {
      clearTimeout(plansSearchTimer);
      plansSearchTimer = setTimeout(() => { state.plansSearch = e.target.value; renderPlans(el); }, 250);
    };
    $('#btn-plans-download').onclick = e => {
      showPlansDownloadPopover(e.currentTarget, plans, { aggregated, search });
    };

    $$('.plan-task-link').forEach(a => {
      a.onclick = e => {
        e.preventDefault();
        navigate('task-detail', { currentTaskId: a.dataset.taskId, currentTaskSeq: a.dataset.taskSeq });
      };
    });

    $$('.plan-list-edit').forEach(btn => btn.onclick = async () => {
      try {
        const plan = await api.get('/plans/' + btn.dataset.planId);
        showEditPlanModal(plan, btn.dataset.taskId);
      } catch (e) { toast(e.message, 'error'); }
    });

    $$('.plan-list-history').forEach(btn => btn.onclick = () => {
      showPlanHistoryModal(btn.dataset.planId);
    });

    bindProjectChips();

  } catch (e) { toast(e.message, 'error'); }
}

// ─── Docs ────────────────────────────────────────────────────────────────────
//
// Notion-style layout:
//   page header (title + count + + New Doc)
//   ┌──────────────┬──────────────────────────────┐
//   │ docs list    │ docs reader (full-page view) │
//   │ (sidebar)    │   - empty state, or          │
//   │              │   - h1 + meta + body         │
//   └──────────────┴──────────────────────────────┘
//
// Selecting a doc updates state.currentDocId and pushes /docs/<id> via
// navigate('docs', { docId }). The list and reader both re-render on every
// navigation; the reader fetches the full doc (with body) on demand.

async function renderDocs(el) {
  // When no project is selected the API aggregates docs across every
  // (non-archived) project and sets `project` on each row so we can show a
  // project chip in the list pane (under the title meta line).
  const aggregated = !state.project;
  const url = aggregated ? '/docs' : '/docs?project=' + encodeURIComponent(state.project);

  try {
    const data = await api.get(url);
    const allDocs = data.docs || [];
    const search = (state.docsSearch || '').toLowerCase();
    const docs = search
      ? allDocs.filter(d => {
          const title = (d.title || '').toLowerCase();
          const proj = (d.project?.name || d.project?.alias || '').toLowerCase();
          return title.includes(search) || (aggregated && proj.includes(search));
        })
      : allDocs;
    const subtitle = search
      ? `${docs.length} of ${allDocs.length} doc${allDocs.length !== 1 ? 's' : ''}`
      : `${allDocs.length} doc${allDocs.length !== 1 ? 's' : ''}`;

    const listHTML = renderDocsListPane(docs, allDocs.length, search, aggregated);
    const readerHTML = renderDocsReaderPane(allDocs);

    el.innerHTML =
      pageHeader({
        title: 'Docs',
        subtitle,
        primary: { id: 'btn-new-doc', label: '+ New Doc', shortcut: 'N' },
      }) + html`
      <div class="docs-layout" data-selected="${state.currentDocId ? 'true' : 'false'}">
        ${listHTML}
        ${readerHTML}
      </div>`;

    $('#btn-new-doc').onclick = () => showNewDocModal();

    let docsSearchTimer;
    const searchInput = $('#docs-search');
    if (searchInput) {
      searchInput.oninput = e => {
        clearTimeout(docsSearchTimer);
        docsSearchTimer = setTimeout(() => { state.docsSearch = e.target.value; renderDocs(el); }, 250);
      };
    }

    $$('.docs-list-item').forEach(btn => {
      btn.onclick = () => navigate('docs', { docId: btn.dataset.docId });
    });
    bindProjectChips();
    const backLink = $('#docs-reader-back');
    if (backLink) {
      backLink.onclick = e => { e.preventDefault(); navigate('docs', { docId: null }); };
    }

    // Reader pane: if we have a cached doc payload, the body has already been
    // rendered inline; just wire the action buttons. Otherwise the pane is a
    // "Loading…" placeholder — fetch the full doc and swap it in.
    if (state.currentDocId) {
      // If the selected id is no longer in the list (deleted in another tab,
      // bad deep link, project switch), bail back to the list view.
      if (!allDocs.some(d => d.id === state.currentDocId)) {
        state.currentDocId = null;
        state.currentDoc = null;
        navigate('docs', { docId: null, _replace: true });
        return;
      }
      if (state.currentDoc && state.currentDoc.id === state.currentDocId) {
        wireDocReaderActions(state.currentDoc);
      } else {
        await loadAndRenderDocReader(state.currentDocId);
      }
    }
  } catch (e) { toast(e.message, 'error'); }
}

// Left pane: search input + doc rows. Active row carries data-active="true".
// When `aggregated` is true the rows include a project chip under the title.
function renderDocsListPane(docs, totalCount, search, aggregated) {
  const items = docs.length === 0
    ? `<div class="docs-list-empty">${search ? 'No docs match the search.' : 'No documents yet. Click + New Doc to start.'}</div>`
    : docs.map(d => {
        const isActive = d.id === state.currentDocId ? 'true' : 'false';
        const projectAlias = d.project?.alias || '';
        const projectName  = d.project?.name  || projectAlias;
        // The chip lives outside the <button> so its own click handler can
        // re-scope the project filter without also activating the doc row.
        const chip = aggregated && projectAlias
          ? `<span class="docs-list-item-project">${projectChipHTML(projectAlias, projectName)}</span>`
          : '';
        return `
          <div class="docs-list-row" data-active="${isActive}">
            <button type="button" class="docs-list-item" data-doc-id="${d.id}" data-active="${isActive}">
              <span class="docs-list-item-title">${escapeHtml(d.title)}</span>
              <span class="docs-list-item-meta">v${d.current_version} · ${timeAgo(d.updated_at)}</span>
            </button>
            ${chip}
          </div>`;
      }).join('');

  const placeholder = aggregated ? 'Search docs by title or project…' : 'Search docs…';
  return html`
    <aside class="docs-list" aria-label="Docs list">
      <div class="docs-list-search">
        <input class="search-input" id="docs-search" placeholder="${placeholder}" value="${escapeHtml(state.docsSearch || '')}">
      </div>
      <div class="docs-list-items" role="list">
        ${items}
      </div>
    </aside>`;
}

// Right pane: empty state, loading placeholder for a selected id, or the
// rendered doc body if state.currentDoc is already populated for that id.
function renderDocsReaderPane(allDocs) {
  if (!state.currentDocId) {
    return html`
      <main class="docs-reader docs-reader-empty">
        <div class="docs-reader-empty-text">
          ${allDocs.length === 0 ? 'No docs yet. Create one to get started.' : 'Select a doc to read.'}
        </div>
      </main>`;
  }
  if (state.currentDoc && state.currentDoc.id === state.currentDocId) {
    return html`<main class="docs-reader" id="docs-reader-main">${docReaderHTML(state.currentDoc)}</main>`;
  }
  // Placeholder shown while the full body is loading.
  const stub = allDocs.find(d => d.id === state.currentDocId);
  const title = stub ? escapeHtml(stub.title) : 'Loading…';
  return html`
    <main class="docs-reader" id="docs-reader-main">
      <a href="#" class="back-crumb docs-reader-back" id="docs-reader-back">Docs</a>
      <h1 class="docs-reader-title">${title}</h1>
      <div class="docs-reader-loading text-muted">Loading…</div>
    </main>`;
}

async function loadAndRenderDocReader(id) {
  try {
    const doc = await api.get('/docs/' + id);
    // Race guard: another navigation may have happened while we were
    // fetching; only paint if our id is still the selected one.
    if (state.currentDocId !== id) return;
    state.currentDoc = doc;
    const mount = $('#docs-reader-main');
    if (mount) mount.innerHTML = docReaderHTML(doc);
    wireDocReaderActions(doc);
  } catch (e) {
    toast(e.message, 'error');
    const mount = $('#docs-reader-main');
    if (mount) {
      mount.innerHTML = html`
        <a href="#" class="back-crumb docs-reader-back" id="docs-reader-back">Docs</a>
        <div class="docs-reader-error text-muted">Couldn't load this doc.</div>`;
      const back = $('#docs-reader-back');
      if (back) back.onclick = ev => { ev.preventDefault(); navigate('docs', { docId: null }); };
    }
  }
}

// Long-form view: back link (mobile/narrow), title, single-row meta with
// inline action buttons, then the body. Wraps the body in .markdown-body so
// the Notion-style typography rules in style.css apply.
function docReaderHTML(doc) {
  const v = doc.version || {};
  const actor = v.actor || doc.actor || {};
  const actorStr = (actor.kind || actor.name)
    ? `${escapeHtml(actor.kind || '')}:${escapeHtml(actor.name || '')}`
    : '';
  const body = v.body && v.body.trim()
    ? renderMarkdown(v.body)
    : '<p class="text-muted">No content yet. Click Edit to add some.</p>';

  return html`
    <a href="#" class="back-crumb docs-reader-back" id="docs-reader-back">Docs</a>
    <h1 class="docs-reader-title">${escapeHtml(doc.title)}</h1>
    <div class="docs-reader-meta">
      <span class="docs-reader-meta-item">v${doc.current_version}</span>
      <span class="docs-reader-meta-sep">·</span>
      <span class="docs-reader-meta-item" title="${formatDate(doc.updated_at)}">Edited ${timeAgo(doc.updated_at)}</span>
      ${actorStr ? `<span class="docs-reader-meta-sep">·</span><span class="docs-reader-meta-item mono">${actorStr}</span>` : ''}
      <span class="docs-reader-actions">
        <button class="btn btn-ghost btn-sm" id="docs-reader-download">Download</button>
        <button class="btn btn-ghost btn-sm" id="docs-reader-edit">Edit</button>
        <button class="btn btn-ghost btn-sm" id="docs-reader-history">History</button>
        <button class="btn btn-ghost btn-sm btn-danger-ghost" id="docs-reader-delete">Delete</button>
      </span>
    </div>
    <article class="docs-reader-body markdown markdown-body">
      ${body}
    </article>
    <section class="docs-reader-history" id="docs-reader-history-section" hidden>
      <div class="docs-reader-history-header text-muted text-sm">Loading version history…</div>
    </section>`;
}

function wireDocReaderActions(doc) {
  const back = $('#docs-reader-back');
  if (back) back.onclick = e => { e.preventDefault(); navigate('docs', { docId: null }); };

  const editBtn = $('#docs-reader-edit');
  if (editBtn) editBtn.onclick = () => showEditDocModal(doc);

  const downloadBtn = $('#docs-reader-download');
  if (downloadBtn) downloadBtn.onclick = e => showDocDownloadPopover(e.currentTarget, doc);

  const histBtn = $('#docs-reader-history');
  if (histBtn) histBtn.onclick = () => toggleDocReaderHistory(doc.id);

  const delBtn = $('#docs-reader-delete');
  if (delBtn) delBtn.onclick = async () => {
    const ok = await confirm(`Delete "${doc.title}"? This cannot be undone.`, {
      confirmLabel: 'Delete', confirmClass: 'btn-danger',
    });
    if (!ok) return;
    try {
      await api.del('/docs/' + doc.id);
      toast('Document deleted');
      state.currentDoc = null;
      navigate('docs', { docId: null, _replace: true });
    } catch (e) { toast(e.message, 'error'); }
  };
}

// Inline version history. Rendered as a collapsible section *below* the body
// rather than a modal — it's a natural fit for the long-form layout and
// avoids stacking modals on top of the page-level reader. Falls back to no-op
// if the section was already populated.
async function toggleDocReaderHistory(id) {
  const section = $('#docs-reader-history-section');
  if (!section) return;
  if (!section.hasAttribute('hidden')) {
    section.setAttribute('hidden', '');
    return;
  }
  section.removeAttribute('hidden');
  if (section.dataset.loaded === 'true') return;

  try {
    const data = await api.get('/docs/' + id + '/history');
    const versions = (data.versions || []).slice().sort((a, b) => b.version - a.version);
    if (versions.length === 0) {
      section.innerHTML = `<div class="docs-reader-history-header">Version history</div>
        <div class="text-muted text-sm">No history available.</div>`;
    } else {
      section.innerHTML =
        `<div class="docs-reader-history-header">Version history</div>` +
        versions.map(v => `
          <div class="docs-reader-history-item">
            <div class="docs-reader-history-row">
              <strong>v${v.version}</strong>
              <span class="text-muted text-sm">${escapeHtml(v.title)}</span>
              <span class="docs-reader-history-spacer"></span>
              <span class="text-muted text-sm mono">${escapeHtml(v.actor?.kind || '')}:${escapeHtml(v.actor?.name || '')}</span>
              <span class="text-muted text-sm" title="${formatDate(v.created_at)}">${timeAgo(v.created_at)}</span>
            </div>
            ${v.change_note ? `<p class="docs-reader-history-note text-muted text-sm">${escapeHtml(v.change_note)}</p>` : ''}
            <details class="docs-reader-history-details">
              <summary class="btn btn-ghost btn-sm">Show body</summary>
              <div class="markdown markdown-body text-sm">${renderMarkdown(v.body)}</div>
            </details>
          </div>`).join('');
    }
    section.dataset.loaded = 'true';
  } catch (e) {
    section.innerHTML = `<div class="text-muted text-sm">Couldn't load history: ${escapeHtml(e.message)}</div>`;
  }
}

function showNewDocModal() {
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header"><span class="modal-title">New Doc</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Project *</label>
          <select class="form-control form-select" id="doc-project">
            ${state.projects.map(p =>
              `<option value="${p.alias}" ${p.alias===state.project?'selected':''}>${escapeHtml(p.name || p.alias)}</option>`
            ).join('')}
          </select>
        </div>
        <div class="form-group"><label class="form-label">Title *</label><input class="form-control" id="doc-title"></div>
        <div class="form-group">
          <label class="form-label">Body (Markdown)</label>
          <textarea class="form-control" id="doc-body"></textarea>
        </div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-save-doc">Create</button>
      </div>
    </div>`);

  const mde = createMDE(overlay.querySelector('#doc-body'), { minHeight: '420px' });

  const doCreate = async () => {
    const title = overlay.querySelector('#doc-title').value.trim();
    const project = overlay.querySelector('#doc-project').value;
    if (!title) { toast('Title required', 'error'); return; }
    if (!project) { toast('Select a project', 'error'); return; }
    try {
      await api.post('/docs', { project, title, body: mde.value() });
      overlay.remove(); toast('Doc created'); render();
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-save-doc').onclick = doCreate;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doCreate(); });
}

function showEditDocModal(doc) {
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header"><span class="modal-title">Edit: ${escapeHtml(doc.title)}</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group"><label class="form-label">Title</label><input class="form-control" id="edit-doc-title"></div>
        <div class="form-group">
          <label class="form-label">Body (Markdown)</label>
          <textarea class="form-control" id="edit-doc-body"></textarea>
        </div>
        <div class="form-group"><label class="form-label">Change note <span class="text-muted">(optional)</span></label><input class="form-control" id="edit-doc-note" placeholder="What changed?"></div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-update-doc">Save</button>
      </div>
    </div>`);

  overlay.querySelector('#edit-doc-title').value = doc.title;

  const mde = createMDE(overlay.querySelector('#edit-doc-body'), { minHeight: '420px' });
  mde.value(doc.version?.body || '');

  const doSave = async () => {
    try {
      await api.patch('/docs/' + doc.id, {
        title: overlay.querySelector('#edit-doc-title').value,
        body: mde.value(),
        change_note: overlay.querySelector('#edit-doc-note').value,
      });
      overlay.remove(); toast('Doc updated');
      // Drop the cached doc so the reader pane re-fetches the new version.
      if (state.currentDocId === doc.id) state.currentDoc = null;
      render();
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-update-doc').onclick = doSave;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doSave(); });
}

// ─── Memory ──────────────────────────────────────────────────────────────────
async function renderMemory(el) {
  // When no project is selected the API aggregates memory across every
  // (non-archived) project and sets `project` on each entry so we can render
  // a project chip in the entry's meta row.
  const aggregated = !state.project;
  const tagFilter = state.memoryTagFilter || '';
  const search = (state.memorySearch || '').toLowerCase();
  const q = new URLSearchParams();
  if (state.project) q.set('project', state.project);
  if (tagFilter) q.set('tag', tagFilter);
  const qs = q.toString();

  try {
    const data = await api.get('/memory' + (qs ? '?' + qs : ''));
    const allEntries = data.entries || [];
    const entries = search
      ? allEntries.filter(m => {
          const body = (m.body || '').toLowerCase();
          const proj = (m.project?.name || m.project?.alias || '').toLowerCase();
          return body.includes(search) || (aggregated && proj.includes(search));
        })
      : allEntries;
    const subtitle = (search || tagFilter)
      ? `${entries.length} of ${allEntries.length} entr${allEntries.length !== 1 ? 'ies' : 'y'}`
      : `${allEntries.length} entr${allEntries.length !== 1 ? 'ies' : 'y'}`;

    const placeholder = aggregated ? 'Search memory by body or project…' : 'Search memory…';
    const toolbarHTML =
      `<input class="search-input" id="mem-search" placeholder="${placeholder}" value="${escapeHtml(state.memorySearch || '')}">` +
      `<input class="search-input" id="mem-tag-filter" placeholder="Filter by tag…" value="${escapeHtml(tagFilter)}" style="width:180px">` +
      (tagFilter ? `<button class="btn btn-ghost btn-sm" id="btn-clear-tag">Clear tag</button>` : '');

    el.innerHTML =
      pageHeader({
        title: 'Memory',
        subtitle,
        viewToggle: '<button class="btn btn-ghost" id="btn-memory-download">Download</button>',
        primary: { id: 'btn-new-memory', label: '+ New Memory', shortcut: 'N' },
        toolbar: toolbarHTML,
      }) + html`
      <div class="memory-list">
        ${entries.length === 0
          ? `<div class="empty"><div class="empty-icon"></div><div class="empty-text">${tagFilter ? 'No entries with tag "' + escapeHtml(tagFilter) + '".' : (search ? 'No entries match the search.' : 'No memory recorded yet.')}</div></div>`
          : entries.map(m => `
            <div class="memory-card card" data-id="${m.id}">
              <div class="memory-card-header">
                <div class="memory-meta">
                  <span class="text-muted text-sm" title="${formatDate(m.created_at)}">${timeAgo(m.created_at)}</span>
                  ${aggregated && m.project ? projectChipHTML(m.project.alias, m.project.name) : ''}
                  ${m.tags ? m.tags.split(',').filter(Boolean).map(t =>
                    `<span class="tag-chip" data-tag="${escapeHtml(t.trim())}">${escapeHtml(t.trim())}</span>`
                  ).join('') : ''}
                  <span class="text-muted text-sm mono">${escapeHtml(m.actor?.kind)}:${escapeHtml(m.actor?.name)}</span>
                </div>
                <div class="memory-actions">
                  <button class="btn btn-ghost btn-sm mem-edit" data-id="${m.id}" title="Edit entry">Edit</button>
                  <button class="btn btn-ghost btn-sm mem-append" data-id="${m.id}" title="Append text">+ Append</button>
                  <button class="btn btn-ghost btn-sm btn-danger-ghost mem-del" data-id="${m.id}" title="Delete">×</button>
                </div>
              </div>
              <div class="memory-body">${m.body && m.body.length > 300
                  ? `<span class="mem-body-short">${escapeHtml(m.body.slice(0, 300))}&hellip;</span><span class="mem-body-full" style="display:none">${escapeHtml(m.body)}</span><br><button class="btn btn-ghost btn-sm mem-toggle-body" style="margin-top:4px">Show more</button>`
                  : escapeHtml(m.body)
                }</div>
            </div>`).join('')}
      </div>`;

    let tagTimer;
    $('#mem-tag-filter').oninput = e => {
      clearTimeout(tagTimer);
      tagTimer = setTimeout(() => { state.memoryTagFilter = e.target.value.trim(); render(); }, 300);
    };
    let memSearchTimer;
    $('#mem-search').oninput = e => {
      clearTimeout(memSearchTimer);
      memSearchTimer = setTimeout(() => { state.memorySearch = e.target.value; renderMemory(el); }, 250);
    };
    const clearTagBtn = $('#btn-clear-tag');
    if (clearTagBtn) clearTagBtn.onclick = () => { state.memoryTagFilter = ''; render(); };
    $$('.tag-chip').forEach(chip => chip.onclick = () => { state.memoryTagFilter = chip.dataset.tag; render(); });

    $('#btn-new-memory').onclick = () => {
      showNewMemoryModal();
    };
    $('#btn-memory-download').onclick = e => {
      showMemoryDownloadPopover(e.currentTarget, allEntries, { aggregated, tagFilter });
    };
    bindProjectChips();
    $$('.mem-toggle-body').forEach(btn => btn.onclick = () => {
      const card = btn.closest('.memory-body');
      const short = card.querySelector('.mem-body-short');
      const full = card.querySelector('.mem-body-full');
      const expanded = full.style.display !== 'none';
      short.style.display = expanded ? '' : 'none';
      full.style.display = expanded ? 'none' : '';
      btn.textContent = expanded ? 'Show more' : 'Show less';
    });
    $$('.mem-edit').forEach(btn => btn.onclick = () => showEditMemoryModal(entries.find(e => e.id === btn.dataset.id)));
    $$('.mem-append').forEach(btn => btn.onclick = () => showAppendMemoryModal(btn.dataset.id, entries.find(e => e.id === btn.dataset.id)));
    $$('.mem-del').forEach(btn => btn.onclick = async () => {
      const ok = await confirm('Delete this memory entry? This cannot be undone.');
      if (!ok) return;
      try { await api.del('/memory/' + btn.dataset.id); toast('Deleted'); render(); }
      catch (e) { toast(e.message, 'error'); }
    });
  } catch (e) { toast(e.message, 'error'); }
}

function showNewMemoryModal() {
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header"><span class="modal-title">Add Memory Entry</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Project *</label>
          <select class="form-control form-select" id="mem-project">
            ${state.projects.map(p =>
              `<option value="${p.alias}" ${p.alias===state.project?'selected':''}>${escapeHtml(p.name || p.alias)}</option>`
            ).join('')}
          </select>
        </div>
        <div class="form-group">
          <label class="form-label">Body *</label>
          <textarea class="form-control" id="mem-body" placeholder="Free-form text or markdown"></textarea>
        </div>
        <div class="form-group"><label class="form-label">Tags <span class="text-muted">(comma-separated, optional)</span></label><input class="form-control" id="mem-tags" placeholder="decision,arch,context"></div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-save-memory">Save</button>
      </div>
    </div>`);

  const mde = createMDE(overlay.querySelector('#mem-body'), { autofocus: true, minHeight: '420px' });

  const doSave = async () => {
    const body = mde.value().trim();
    const project = overlay.querySelector('#mem-project').value;
    if (!body) { toast('Body is required', 'error'); return; }
    if (!project) { toast('Select a project', 'error'); return; }
    try {
      await api.post('/memory', { project, body, tags: overlay.querySelector('#mem-tags').value.trim() });
      overlay.remove(); toast('Memory added'); render();
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-save-memory').onclick = doSave;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doSave(); });
}

function showAppendMemoryModal(id, entry) {
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header"><span class="modal-title">Append to Memory</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Current</label>
          <div class="memory-preview">${escapeHtml((entry?.body || '').slice(0, 400))}${(entry?.body || '').length > 400 ? '…' : ''}</div>
        </div>
        <div class="form-group">
          <label class="form-label">Append *</label>
          <textarea class="form-control" id="mem-append-text" placeholder="Text to append"></textarea>
        </div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-do-append">Append</button>
      </div>
    </div>`);

  const mde = createMDE(overlay.querySelector('#mem-append-text'), { autofocus: true, minHeight: '420px' });

  const doSave = async () => {
    const text = mde.value();
    if (!text.trim()) { toast('Nothing to append', 'error'); return; }
    try {
      await api.patch('/memory/' + id + '/append', { value: text });
      overlay.remove(); toast('Appended'); render();
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-do-append').onclick = doSave;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doSave(); });
}

function showEditMemoryModal(entry) {
  if (!entry) { toast('Entry not found', 'error'); return; }
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header"><span class="modal-title">Edit Memory Entry</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Body *</label>
          <textarea class="form-control" id="mem-edit-body" placeholder="Free-form text or markdown"></textarea>
        </div>
        <div class="form-group"><label class="form-label">Tags <span class="text-muted">(comma-separated, optional)</span></label><input class="form-control" id="mem-edit-tags" placeholder="decision,arch,context"></div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-save-mem-edit">Save</button>
      </div>
    </div>`);

  overlay.querySelector('#mem-edit-tags').value = entry.tags || '';

  const mde = createMDE(overlay.querySelector('#mem-edit-body'), { autofocus: true, minHeight: '420px' });
  mde.value(entry.body || '');

  const doSave = async () => {
    const body = mde.value().trim();
    if (!body) { toast('Body is required', 'error'); return; }
    try {
      await api.patch('/memory/' + entry.id, { body, tags: overlay.querySelector('#mem-edit-tags').value.trim() });
      overlay.remove(); toast('Memory updated'); render();
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-save-mem-edit').onclick = doSave;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doSave(); });
}

function showDownloadMenu(anchorEl, title, items, note = '') {
  const container = document.createElement('div');
  const header = document.createElement('div');
  header.className = 'popover-header';
  header.textContent = title;
  container.appendChild(header);

  if (note) {
    const noteEl = document.createElement('div');
    noteEl.className = 'popover-note';
    noteEl.textContent = note;
    container.appendChild(noteEl);
  }

  items.forEach(item => {
    const row = document.createElement('div');
    row.className = 'popover-item';
    row.textContent = item.label;
    row.onclick = async () => {
      try {
        await item.run();
        close();
      } catch (e) {
        toast(e.message || String(e), 'error');
      }
    };
    container.appendChild(row);
  });

  const { close } = showPopover(anchorEl, container);
}

function exportScope(opts = {}) {
  return {
    project: state.project || null,
    project_name: selectedProjectName(),
    search: opts.search || null,
    tag: opts.tagFilter || null,
    all_projects: !!opts.aggregated,
  };
}

function exportScopeName(opts = {}) {
  return opts.aggregated ? 'all-projects' : (state.project || 'project');
}

function exportHeaderLines(title, opts = {}) {
  const scope = opts.aggregated
    ? 'All Projects'
    : (selectedProjectName() || state.project || 'Project');
  const lines = [
    `# ${markdownLine(`${title} - ${scope}`)}`,
    '',
    `Exported: ${new Date().toLocaleString()}`,
  ];
  if (opts.search) lines.push(`Search: ${opts.search}`);
  if (opts.tagFilter) lines.push(`Tag filter: ${opts.tagFilter}`);
  lines.push('');
  return lines;
}

function collectionExportFilename(kind, opts = {}, ext = 'md') {
  const date = new Date().toISOString().slice(0, 10);
  const scope = exportScopeName(opts);
  const search = opts.search ? '-search-' + opts.search : '';
  return `backlog-${kind}-${safeFilenamePart(scope + search)}-${date}.${ext}`;
}

function groupByProject(items, projectFn) {
  const grouped = new Map();
  (items || []).forEach(item => {
    const project = projectFn(item) || {};
    const alias = project.alias || 'unknown';
    const name = project.name || alias;
    if (!grouped.has(alias)) grouped.set(alias, { alias, name, items: [] });
    grouped.get(alias).items.push(item);
  });
  return grouped;
}

async function tasksForDownload() {
  const q = buildTaskQuery(0, 0);
  const data = await api.get('/tasks?' + q);
  let tasks = data.tasks || [];
  if (state.archivedView === 'archived') {
    tasks = tasks.filter(t => !!t.archived_at);
  }
  return tasks;
}

function showTaskDownloadPopover(anchorEl, task) {
  showDownloadMenu(anchorEl, 'Download task', [
    {
      label: 'Markdown (.md)',
      run: async () => {
        downloadTextFile(taskExportFilename(task, 'md'), buildSingleTaskMarkdown(task), 'text/markdown;charset=utf-8');
        toast('Task downloaded');
      },
    },
    {
      label: 'CSV (.csv)',
      run: async () => {
        downloadTextFile(taskExportFilename(task, 'csv'), buildTasksCSV([task]), 'text/csv;charset=utf-8');
        toast('Task downloaded');
      },
    },
    {
      label: 'JSON (.json)',
      run: async () => {
        const payload = {
          exported_at: new Date().toISOString(),
          scope: {
            project: task.project?.alias || state.project || null,
            project_name: task.project?.name || selectedProjectName() || null,
          },
          task,
        };
        downloadTextFile(taskExportFilename(task, 'json'), JSON.stringify(payload, null, 2) + '\n', 'application/json;charset=utf-8');
        toast('Task downloaded');
      },
    },
  ]);
}

function showTasksDownloadPopover(anchorEl) {
  showDownloadMenu(anchorEl, 'Download tasks', [
    {
      label: 'Markdown (.md)',
      run: async () => {
        const tasks = await tasksForDownload();
        downloadTextFile(tasksExportFilename('md'), buildTasksMarkdown(tasks), 'text/markdown;charset=utf-8');
        toast('Tasks downloaded');
      },
    },
    {
      label: 'CSV (.csv)',
      run: async () => {
        const tasks = await tasksForDownload();
        downloadTextFile(tasksExportFilename('csv'), buildTasksCSV(tasks), 'text/csv;charset=utf-8');
        toast('Tasks downloaded');
      },
    },
    {
      label: 'JSON (.json)',
      run: async () => {
        const tasks = await tasksForDownload();
        const payload = {
          exported_at: new Date().toISOString(),
          scope: {
            ...exportScope({ aggregated: !state.project, search: state.filters.search || '' }),
            archived_view: state.archivedView,
            filters: state.filters,
            sort: state.sorts,
          },
          tasks,
        };
        downloadTextFile(tasksExportFilename('json'), JSON.stringify(payload, null, 2) + '\n', 'application/json;charset=utf-8');
        toast('Tasks downloaded');
      },
    },
  ]);
}

function taskExportFilename(task, ext) {
  const date = new Date().toISOString().slice(0, 10);
  const ref = task.seq ? `TASK-${task.seq}` : (task.id || 'task');
  return `backlog-task-${safeFilenamePart(ref + '-' + (task.title || 'task'))}-${date}.${ext}`;
}

function tasksExportFilename(ext) {
  const date = new Date().toISOString().slice(0, 10);
  return `backlog-tasks-${safeFilenamePart(exportScopeName({ aggregated: !state.project }))}-${date}.${ext}`;
}

function buildSingleTaskMarkdown(task) {
  const lines = exportHeaderLines('Backlog Task', { aggregated: false });
  appendTaskMarkdown(lines, task, 2);
  return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

function buildTasksMarkdown(tasks) {
  const lines = exportHeaderLines('Backlog Tasks', { aggregated: !state.project, search: state.filters.search || '' });
  if (!tasks.length) {
    lines.push('No tasks found.');
    return lines.join('\n') + '\n';
  }
  tasks.forEach(task => {
    appendTaskMarkdown(lines, task, 2);
  });
  return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

function appendTaskMarkdown(lines, task, headingLevel) {
  lines.push(`${'#'.repeat(headingLevel)} TASK-${task.seq || '?'} - ${markdownLine(task.title)}`, '');
  lines.push(`- Project: ${task.project?.name || task.project?.alias || ''}`);
  lines.push(`- Status: ${task.status}`);
  lines.push(`- Priority: P${task.priority}`);
  lines.push(`- Type: ${task.type}`);
  if (task.assignee) lines.push(`- Assignee: ${task.assignee}`);
  if (task.due_at) lines.push(`- Due: ${formatDate(task.due_at)}`);
  if (task.labels?.length) lines.push(`- Labels: ${task.labels.map(l => l.name).join(', ')}`);
  if (task.source) lines.push(`- Source: ${task.source}`);
  if (task.external_ref) lines.push(`- External ref: ${task.external_ref}`);
  if (task.project_path) lines.push(`- Path: ${task.project_path}`);
  lines.push(`- Created: ${formatDate(task.created_at)}`);
  lines.push(`- Updated: ${formatDate(task.updated_at)}`);
  lines.push('');
  if (task.description) lines.push(task.description.trim(), '');

  if (task.plans?.length) {
    lines.push(`${'#'.repeat(headingLevel + 1)} Plans`, '');
    task.plans.forEach(plan => {
      const v = plan.version || {};
      lines.push(`${'#'.repeat(headingLevel + 2)} ${markdownLine(v.title || 'Plan')}`, '');
      lines.push(`- Version: v${plan.current_version || v.version || ''}`);
      lines.push(`- Updated: ${formatDate(plan.updated_at || v.created_at)}`, '');
      lines.push((v.body || '').trim() || '_No body_', '');
    });
  }

  if (task.comments?.length) {
    lines.push(`${'#'.repeat(headingLevel + 1)} Comments`, '');
    task.comments.forEach(comment => {
      const actor = `${comment.actor?.kind || ''}:${comment.actor?.name || ''}`.replace(/^:|:$/g, '');
      lines.push(`- ${formatDate(comment.created_at)}${actor ? ' - ' + actor : ''}: ${comment.body || ''}`);
    });
    lines.push('');
  }

  lines.push('---', '');
}

function buildTasksCSV(tasks) {
  const rows = [[
    'ref', 'title', 'project', 'status', 'priority', 'type', 'assignee', 'due',
    'labels', 'source', 'external_ref', 'path', 'created', 'updated',
  ]];
  tasks.forEach(task => rows.push([
    `TASK-${task.seq}`,
    task.title || '',
    task.project?.name || task.project?.alias || '',
    task.status || '',
    `P${task.priority || ''}`,
    task.type || '',
    task.assignee || '',
    task.due_at ? formatDate(task.due_at) : '',
    task.labels?.length ? task.labels.map(l => l.name).join(', ') : '',
    task.source || '',
    task.external_ref || '',
    task.project_path || '',
    formatDate(task.created_at),
    formatDate(task.updated_at),
  ]));
  return rows.map(row => row.map(csvCell).join(',')).join('\n') + '\n';
}

function csvCell(value) {
  const s = String(value ?? '');
  return /[",\n\r]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s;
}

function showPlansDownloadPopover(anchorEl, plans, opts = {}) {
  showDownloadMenu(anchorEl, 'Download plans', [
    {
      label: 'Markdown (.md)',
      run: async () => {
        downloadTextFile(collectionExportFilename('plans', opts, 'md'), buildPlansMarkdown(plans, opts), 'text/markdown;charset=utf-8');
        toast('Plans downloaded');
      },
    },
    {
      label: 'JSON (.json)',
      run: async () => {
        const payload = { exported_at: new Date().toISOString(), scope: exportScope(opts), plans };
        downloadTextFile(collectionExportFilename('plans', opts, 'json'), JSON.stringify(payload, null, 2) + '\n', 'application/json;charset=utf-8');
        toast('Plans downloaded');
      },
    },
  ]);
}

function buildPlansMarkdown(plans, opts = {}) {
  const lines = exportHeaderLines('Backlog Plans', opts);
  if (!plans.length) {
    lines.push('No plans found.');
    return lines.join('\n') + '\n';
  }
  if (opts.aggregated) {
    const grouped = groupByProject(plans, plan => ({
      alias: plan.project_alias || 'unknown',
      name: plan.project_name || plan.project_alias || 'Unknown',
    }));
    grouped.forEach(group => {
      lines.push(`## ${markdownLine(group.name)} (${group.alias})`, '');
      group.items.forEach(plan => appendPlanMarkdown(lines, plan, 3));
    });
  } else {
    plans.forEach(plan => appendPlanMarkdown(lines, plan, 2));
  }
  return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

function appendPlanMarkdown(lines, plan, headingLevel) {
  const v = plan.version || {};
  lines.push(`${'#'.repeat(headingLevel)} TASK-${plan.task_seq || '?'} - ${markdownLine(v.title || 'Plan')}`);
  lines.push('');
  lines.push(`- Version: v${plan.current_version || v.version || ''}`);
  lines.push(`- Updated: ${formatDate(plan.updated_at || v.created_at)}`);
  const actor = `${v.actor?.kind || ''}:${v.actor?.name || ''}`.replace(/^:|:$/g, '');
  if (actor) lines.push(`- Actor: ${actor}`);
  if (plan.source) lines.push(`- Source: ${plan.source}`);
  lines.push('');
  lines.push((v.body || '').trim() || '_No body_');
  lines.push('', '---', '');
}

function showDocDownloadPopover(anchorEl, doc) {
  showDownloadMenu(anchorEl, 'Download doc', [
    {
      label: 'Markdown (.md)',
      run: async () => {
        downloadTextFile(docExportFilename(doc, 'md'), buildSingleDocMarkdown(doc), 'text/markdown;charset=utf-8');
        toast('Doc downloaded');
      },
    },
    {
      label: 'JSON (.json)',
      run: async () => {
        const payload = {
          exported_at: new Date().toISOString(),
          scope: {
            project: doc.project?.alias || state.project || null,
            project_name: doc.project?.name || selectedProjectName() || null,
          },
          doc,
        };
        downloadTextFile(docExportFilename(doc, 'json'), JSON.stringify(payload, null, 2) + '\n', 'application/json;charset=utf-8');
        toast('Doc downloaded');
      },
    },
  ]);
}

function showDocsDownloadPopover(anchorEl, docs, opts = {}) {
  showDownloadMenu(anchorEl, 'Download docs', [
    {
      label: 'Markdown (.md)',
      run: async () => {
        const fullDocs = await fullDocsForDownload(docs);
        downloadTextFile(collectionExportFilename('docs', opts, 'md'), buildDocsMarkdown(fullDocs, opts), 'text/markdown;charset=utf-8');
        toast('Docs downloaded');
      },
    },
    {
      label: 'JSON (.json)',
      run: async () => {
        const fullDocs = await fullDocsForDownload(docs);
        const payload = { exported_at: new Date().toISOString(), scope: exportScope(opts), docs: fullDocs };
        downloadTextFile(collectionExportFilename('docs', opts, 'json'), JSON.stringify(payload, null, 2) + '\n', 'application/json;charset=utf-8');
        toast('Docs downloaded');
      },
    },
  ]);
}

async function fullDocsForDownload(docs) {
  return Promise.all((docs || []).map(async stub => {
    const doc = await api.get('/docs/' + stub.id);
    return { ...doc, project: doc.project || stub.project };
  }));
}

function docExportFilename(doc, ext) {
  const date = new Date().toISOString().slice(0, 10);
  return `backlog-doc-${safeFilenamePart(doc.title || doc.version?.title || doc.id || 'doc')}-${date}.${ext}`;
}

function buildSingleDocMarkdown(doc) {
  const v = doc.version || {};
  const actor = `${v.actor?.kind || doc.actor?.kind || ''}:${v.actor?.name || doc.actor?.name || ''}`.replace(/^:|:$/g, '');
  const lines = [
    `# ${markdownLine(doc.title || v.title || 'Untitled doc')}`,
    '',
    `Exported: ${new Date().toLocaleString()}`,
  ];
  if (doc.project?.name || doc.project?.alias) {
    lines.push(`Project: ${doc.project?.name || doc.project?.alias}`);
  }
  lines.push(`Version: v${doc.current_version || v.version || ''}`);
  lines.push(`Updated: ${formatDate(doc.updated_at || v.created_at)}`);
  if (actor) lines.push(`Actor: ${actor}`);
  lines.push('');
  lines.push((v.body || '').trim() || '_No body_');
  return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

function buildDocsMarkdown(docs, opts = {}) {
  const lines = exportHeaderLines('Backlog Docs', opts);
  if (!docs.length) {
    lines.push('No docs found.');
    return lines.join('\n') + '\n';
  }
  if (opts.aggregated) {
    const grouped = groupByProject(docs, doc => ({
      alias: doc.project?.alias || 'unknown',
      name: doc.project?.name || doc.project?.alias || 'Unknown',
    }));
    grouped.forEach(group => {
      lines.push(`## ${markdownLine(group.name)} (${group.alias})`, '');
      group.items.forEach(doc => appendDocMarkdown(lines, doc, 3));
    });
  } else {
    docs.forEach(doc => appendDocMarkdown(lines, doc, 2));
  }
  return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

function appendDocMarkdown(lines, doc, headingLevel) {
  const v = doc.version || {};
  lines.push(`${'#'.repeat(headingLevel)} ${markdownLine(doc.title || v.title || 'Untitled doc')}`);
  lines.push('');
  lines.push(`- Version: v${doc.current_version || v.version || ''}`);
  lines.push(`- Updated: ${formatDate(doc.updated_at || v.created_at)}`);
  const actor = `${v.actor?.kind || doc.actor?.kind || ''}:${v.actor?.name || doc.actor?.name || ''}`.replace(/^:|:$/g, '');
  if (actor) lines.push(`- Actor: ${actor}`);
  lines.push('');
  lines.push((v.body || '').trim() || '_No body_');
  lines.push('', '---', '');
}

function showMemoryDownloadPopover(anchorEl, entries, opts = {}) {
  const container = document.createElement('div');
  const header = document.createElement('div');
  header.className = 'popover-header';
  header.textContent = 'Download memory';
  container.appendChild(header);

  const md = document.createElement('div');
  md.className = 'popover-item';
  md.textContent = 'Markdown (.md)';
  container.appendChild(md);

  const json = document.createElement('div');
  json.className = 'popover-item';
  json.textContent = 'JSON (.json)';
  container.appendChild(json);

  const { close } = showPopover(anchorEl, container);
  md.onclick = () => {
    const filename = memoryExportFilename(opts, 'md');
    downloadTextFile(filename, buildMemoryMarkdown(entries, opts), 'text/markdown;charset=utf-8');
    close();
    toast('Memory downloaded');
  };
  json.onclick = () => {
    const filename = memoryExportFilename(opts, 'json');
    const payload = {
      exported_at: new Date().toISOString(),
      scope: {
        project: state.project || null,
        project_name: selectedProjectName(),
        tag: opts.tagFilter || null,
        all_projects: !!opts.aggregated,
      },
      entries,
    };
    downloadTextFile(filename, JSON.stringify(payload, null, 2) + '\n', 'application/json;charset=utf-8');
    close();
    toast('Memory downloaded');
  };
}

function selectedProjectName() {
  const p = state.projects.find(project => project.alias === state.project);
  return p ? (p.name || p.alias) : '';
}

function memoryExportFilename(opts = {}, ext = 'md') {
  const date = new Date().toISOString().slice(0, 10);
  const scope = opts.aggregated ? 'all-projects' : (state.project || 'project');
  const tag = opts.tagFilter ? '-tag-' + opts.tagFilter : '';
  return `backlog-memory-${safeFilenamePart(scope + tag)}-${date}.${ext}`;
}

function safeFilenamePart(s) {
  return (s || 'memory')
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'memory';
}

function buildMemoryMarkdown(entries, opts = {}) {
  const generated = new Date().toLocaleString();
  const title = opts.aggregated
    ? 'Backlog Memory - All Projects'
    : `Backlog Memory - ${selectedProjectName() || state.project || 'Project'}`;
  const lines = [
    `# ${markdownLine(title)}`,
    '',
    `Exported: ${generated}`,
    opts.tagFilter ? `Tag filter: ${opts.tagFilter}` : '',
    '',
  ].filter(line => line !== '');

  if (!entries || entries.length === 0) {
    lines.push('No memory entries.');
    return lines.join('\n') + '\n';
  }

  if (opts.aggregated) {
    const grouped = new Map();
    entries.forEach(entry => {
      const alias = entry.project?.alias || 'unknown';
      const name = entry.project?.name || alias;
      if (!grouped.has(alias)) grouped.set(alias, { alias, name, entries: [] });
      grouped.get(alias).entries.push(entry);
    });
    grouped.forEach(group => {
      lines.push(`## ${markdownLine(group.name)} (${group.alias})`, '');
      group.entries.forEach((entry, index) => appendMemoryEntryMarkdown(lines, entry, index + 1, 3));
    });
  } else {
    entries.forEach((entry, index) => appendMemoryEntryMarkdown(lines, entry, index + 1, 2));
  }

  return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

function appendMemoryEntryMarkdown(lines, entry, index, headingLevel) {
  lines.push(`${'#'.repeat(headingLevel)} Memory ${index}`);
  lines.push('');
  lines.push(`- Created: ${formatDate(entry.created_at)}`);
  const actor = `${entry.actor?.kind || ''}:${entry.actor?.name || ''}`.replace(/^:|:$/g, '');
  if (actor) lines.push(`- Actor: ${actor}`);
  if (entry.tags) lines.push(`- Tags: ${entry.tags}`);
  lines.push('');
  lines.push((entry.body || '').trim() || '_No body_');
  lines.push('', '---', '');
}

function markdownLine(s) {
  return String(s || '').replace(/\s+/g, ' ').trim();
}

function downloadTextFile(filename, content, mimeType) {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

// ─── Attachments ─────────────────────────────────────────────────────────────

async function handleDeleteAttachment(id, btn) {
  if (!await confirm('Delete this attachment? This cannot be undone.')) return;
  btn.disabled = true;
  try {
    await api.del('/attachments/' + id);
    btn.closest('tr, li, .attachment-row').remove();
    toast('Attachment deleted');
  } catch (e) {
    toast('Failed to delete: ' + e.message, 'error');
    btn.disabled = false;
  }
}

function formatBytes(n) {
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
  return (n / (1024 * 1024)).toFixed(1) + ' MB';
}

function showAttachmentPreview(id, name, mimeType) {
  showModal(html`
    <div class="modal" style="width:800px">
      <div class="modal-header"><span class="modal-title">${escapeHtml(name)}</span><button class="modal-close">×</button></div>
      <div class="modal-body" style="text-align:center">
        <img src="/api/attachments/${id}" style="max-width:100%;max-height:70vh">
      </div>
    </div>`);
}

async function renderAttachments(el) {
  // When no project is selected the API aggregates attachments across every
  // (non-archived) project and tags each row with project_alias / project_name
  // so we can render a project chip in the Project column.
  const aggregated = !state.project;
  const url = aggregated ? '/attachments' : '/attachments?project=' + encodeURIComponent(state.project);

  try {
    const data = await api.get(url);
    const allAttachments = data.attachments || [];
    let attachments = allAttachments;

    const sortBy = state.attachmentSort || 'date';
    const searchQ = (state.attachmentSearch || '').toLowerCase();

    if (searchQ) {
      attachments = attachments.filter(a => {
        const proj = (a.project_name || a.project_alias || '').toLowerCase();
        return a.name.toLowerCase().includes(searchQ) || (aggregated && proj.includes(searchQ));
      });
    }

    if (sortBy === 'name') {
      attachments = [...attachments].sort((a, b) => a.name.localeCompare(b.name));
    } else if (sortBy === 'size') {
      attachments = [...attachments].sort((a, b) => b.size - a.size);
    }
    // default 'date': already sorted DESC by server

    const subtitle = searchQ
      ? `${attachments.length} of ${allAttachments.length} file${allAttachments.length !== 1 ? 's' : ''}`
      : `${allAttachments.length} file${allAttachments.length !== 1 ? 's' : ''}`;

    const placeholder = aggregated ? 'Search by filename or project…' : 'Search by filename…';
    const toolbarHTML =
      `<input class="search-input" id="att-search" placeholder="${placeholder}" value="${escapeHtml(state.attachmentSearch || '')}" style="width:220px">` +
      `<select class="filter-select" id="att-sort">` +
        `<option value="date" ${sortBy === 'date' ? 'selected' : ''}>Sort: Date</option>` +
        `<option value="name" ${sortBy === 'name' ? 'selected' : ''}>Sort: Name</option>` +
        `<option value="size" ${sortBy === 'size' ? 'selected' : ''}>Sort: Size</option>` +
      `</select>`;

    const colspan = aggregated ? 8 : 7;
    el.innerHTML =
      pageHeader({
        title: 'Attachments',
        subtitle,
        primary: { id: 'btn-upload-attachment', label: '+ Upload', shortcut: 'N' },
        toolbar: toolbarHTML,
      }) + html`
      <div class="card">
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Size</th>
                <th>Type</th>
                <th>Attached to</th>
                ${aggregated ? '<th>Project</th>' : ''}
                <th>Actor</th>
                <th>Uploaded</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              ${attachments.length === 0
                ? `<tr><td colspan="${colspan}"><div class="empty"><div class="empty-icon"></div><div class="empty-text">No attachments on file.</div></div></td></tr>`
                : attachments.map(a => {
                    const isImage = a.mime_type && a.mime_type.startsWith('image/');
                    let linkedCell = '—';
                    if (a.linked_type === 'task' && a.task_seq) {
                      linkedCell = `<a href="#" class="att-task-link" data-id="${escapeHtml(a.linked_id)}" data-seq="${a.task_seq}">TASK-${a.task_seq}</a>${a.task_title ? ': ' + escapeHtml(a.task_title) : ''}`;
                    } else if (a.linked_type === 'doc' && a.doc_title) {
                      linkedCell = escapeHtml(a.doc_title);
                    }
                    return `
                      <tr>
                        <td>
                          <a href="#" class="att-name-link" data-id="${escapeHtml(a.id)}" data-name="${escapeHtml(a.name)}" data-mime="${escapeHtml(a.mime_type)}" data-is-image="${isImage}">
                            ${escapeHtml(a.name)}
                          </a>
                        </td>
                        <td class="text-muted text-sm">${formatBytes(a.size)}</td>
                        <td class="text-muted text-sm mono">${escapeHtml(a.mime_type)}</td>
                        <td class="text-sm">${linkedCell}</td>
                        ${aggregated ? `<td>${projectChipHTML(a.project_alias, a.project_name)}</td>` : ''}
                        <td class="text-muted text-sm mono">${escapeHtml(a.actor?.kind)}:${escapeHtml(a.actor?.name)}</td>
                        <td class="text-muted text-sm" title="${formatDate(a.created_at)}">${timeAgo(a.created_at)}</td>
                        <td>
                          <a class="btn btn-ghost btn-sm" href="/api/attachments/${encodeURIComponent(a.id)}" download="${escapeHtml(a.name)}">Download</a>
                          <button class="btn btn-ghost btn-sm" onclick="handleDeleteAttachment('${escapeHtml(a.id)}', this)">Delete</button>
                        </td>
                      </tr>`;
                  }).join('')}
            </tbody>
          </table>
        </div>
      </div>`;

    let searchTimer;
    $('#att-search').oninput = e => {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(() => { state.attachmentSearch = e.target.value; renderAttachments(el); }, 250);
    };

    $('#att-sort').onchange = e => { state.attachmentSort = e.target.value; renderAttachments(el); };

    $('#btn-upload-attachment').onclick = () => {
      showUploadAttachmentModal();
    };
    bindProjectChips();

    $$('.att-name-link').forEach(link => link.onclick = e => {
      e.preventDefault();
      const isImage = link.dataset.isImage === 'true';
      if (isImage) {
        showAttachmentPreview(link.dataset.id, link.dataset.name, link.dataset.mime);
      } else {
        window.location.href = '/api/attachments/' + link.dataset.id;
      }
    });

    $$('.att-task-link').forEach(link => link.onclick = e => {
      e.preventDefault();
      navigate('task-detail', { currentTaskId: link.dataset.id, currentTaskSeq: parseInt(link.dataset.seq, 10) });
    });

  } catch (e) { toast(e.message, 'error'); }
}

async function showUploadAttachmentModal(preselectFile) {
  if (!state.projects || state.projects.length === 0) {
    toast('Create a project first', 'error');
    return;
  }

  const overlay = showModal(html`
    <div class="modal">
      <div class="modal-header"><span class="modal-title">Upload Attachment</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Project *</label>
          <select class="form-control form-select" id="att-project">
            ${state.projects.map(p =>
              `<option value="${p.alias}" ${p.alias===state.project?'selected':''}>${escapeHtml(p.name || p.alias)}</option>`
            ).join('')}
          </select>
        </div>
        <div class="form-group">
          <label class="form-label">File *</label>
          <input class="form-control" type="file" id="att-file">
          <div class="text-muted text-sm" style="margin-top:4px">Max 10 MB.</div>
        </div>
        <div class="form-group">
          <label class="form-label">Attach to</label>
          <select class="form-control form-select" id="att-linked-type"></select>
        </div>
        <div class="form-group" id="att-target-task-group">
          <label class="form-label">Task *</label>
          <select class="form-control form-select" id="att-target-task"></select>
        </div>
        <div class="form-group" id="att-target-doc-group" style="display:none">
          <label class="form-label">Doc *</label>
          <select class="form-control form-select" id="att-target-doc"></select>
        </div>
        <div class="form-group">
          <label class="form-label">Name override <span class="text-muted">(optional)</span></label>
          <input class="form-control" id="att-name-override" placeholder="Leave blank to use original filename">
        </div>
        <div id="att-error" class="text-sm" style="color:var(--red-fg);display:none;margin-top:8px"></div>
      </div>
      <div class="modal-footer">
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-do-upload">Upload</button>
      </div>
    </div>`);

  const fileInput = overlay.querySelector('#att-file');
  if (preselectFile) {
    const dt = new DataTransfer();
    dt.items.add(preselectFile);
    fileInput.files = dt.files;
  }

  const projectSel = overlay.querySelector('#att-project');
  const linkedTypeSel = overlay.querySelector('#att-linked-type');
  const taskGroup = overlay.querySelector('#att-target-task-group');
  const docGroup = overlay.querySelector('#att-target-doc-group');
  const taskSel = overlay.querySelector('#att-target-task');
  const docSel = overlay.querySelector('#att-target-doc');
  const errEl = overlay.querySelector('#att-error');

  const loadTargets = async () => {
    const project = projectSel.value;
    if (!project) return;
    linkedTypeSel.innerHTML = '<option disabled selected>Loading…</option>';
    taskSel.innerHTML = '';
    docSel.innerHTML = '';
    try {
      const [tData, dData] = await Promise.all([
        api.get('/tasks?project=' + encodeURIComponent(project) + '&limit=200&sort=created&order=desc'),
        api.get('/docs?project=' + encodeURIComponent(project)),
      ]);
      const tasks = tData.tasks || [];
      const docs = dData.docs || [];
      linkedTypeSel.innerHTML =
        (tasks.length > 0 ? '<option value="task">Task</option>' : '') +
        (docs.length > 0 ? '<option value="doc">Doc</option>' : '');
      taskSel.innerHTML = tasks.map(t => {
        const priority = t.priority ? `P${t.priority}` : '';
        const status = t.status || '';
        const meta = [priority, status].filter(Boolean).join(' · ');
        const titleTrunc = t.title.length > 60 ? t.title.slice(0, 60) + '…' : t.title;
        return `<option value="${escapeHtml(t.id)}">[TASK-${t.seq}] ${escapeHtml(titleTrunc)}${meta ? ' · ' + escapeHtml(meta) : ''}</option>`;
      }).join('');
      docSel.innerHTML = docs.map(d =>
        `<option value="${escapeHtml(d.id)}">${escapeHtml(d.title)}</option>`
      ).join('');
      const v = linkedTypeSel.value;
      taskGroup.style.display = v === 'task' ? '' : 'none';
      docGroup.style.display = v === 'doc' ? '' : 'none';
      if (tasks.length === 0 && docs.length === 0) {
        errEl.textContent = 'This project has no tasks or docs yet. Create one first to attach files to.';
        errEl.style.display = '';
      } else {
        errEl.style.display = 'none';
      }
    } catch (e) {
      errEl.textContent = e.message;
      errEl.style.display = '';
    }
  };

  projectSel.onchange = loadTargets;
  linkedTypeSel.onchange = e => {
    const v = e.target.value;
    taskGroup.style.display = v === 'task' ? '' : 'none';
    docGroup.style.display = v === 'doc' ? '' : 'none';
  };
  loadTargets();

  overlay.querySelector('#btn-do-upload').onclick = async () => {
    const errEl = overlay.querySelector('#att-error');
    errEl.style.display = 'none';
    const file = fileInput.files[0];
    if (!file) { errEl.textContent = 'Choose a file'; errEl.style.display = ''; return; }
    const linkedType = overlay.querySelector('#att-linked-type').value;
    const linkedID = linkedType === 'task'
      ? overlay.querySelector('#att-target-task').value
      : overlay.querySelector('#att-target-doc').value;
    if (!linkedID) { errEl.textContent = 'Choose a task or doc'; errEl.style.display = ''; return; }
    const nameOverride = overlay.querySelector('#att-name-override').value.trim();

    const fd = new FormData();
    fd.append('file', file);
    fd.append('linked_type', linkedType);
    fd.append('linked_id', linkedID);
    if (nameOverride) fd.append('name', nameOverride);
    try {
      const r = await fetch('/api/attachments', { method: 'POST', body: fd });
      if (!r.ok) {
        const body = await r.json().catch(() => ({ error: r.statusText }));
        throw new Error(body.error || ('upload failed (' + r.status + ')'));
      }
      overlay.remove();
      toast('Attachment uploaded');
      renderAttachments(document.getElementById('app'));
    } catch (e) {
      errEl.textContent = e.message;
      errEl.style.display = '';
    }
  };
}

// ─── Projects ────────────────────────────────────────────────────────────────
async function renderProjects(el) {
  try {
    const data = await api.get('/projects' + (state.projectsShowArchived ? '?include_archived=true' : ''));
    state.projects = data.projects || [];
    updateProjectSelect();
  } catch (e) {
    el.innerHTML = pageHeader({ title: 'Projects' }) +
      `<div class="empty"><div class="empty-text" style="color:var(--red-fg)">Failed to load projects: ${escapeHtml(e.message)}</div></div>`;
    return;
  }

  const search = (state.projectsSearch || '').toLowerCase();
  const filtered = search
    ? state.projects.filter(p =>
        (p.alias || '').toLowerCase().includes(search) ||
        (p.name || '').toLowerCase().includes(search) ||
        (p.description || '').toLowerCase().includes(search))
    : state.projects;
  await Promise.all(filtered.map(p => getLabelsForProject(p.alias)));
  const subtitle = search
    ? `${filtered.length} of ${state.projects.length} project${state.projects.length !== 1 ? 's' : ''}`
    : `${state.projects.length} project${state.projects.length !== 1 ? 's' : ''}`;

  const toolbarHTML =
    `<input class="search-input" id="projects-search" placeholder="Search projects…" value="${escapeHtml(state.projectsSearch || '')}">` +
    `<button class="btn btn-ghost btn-sm ${state.projectsShowArchived ? 'archived-toggle-active' : ''}" id="btn-projects-archived">${state.projectsShowArchived ? '✓ Archived' : 'Archived'}</button>`;

  el.innerHTML =
    pageHeader({
      title: 'Projects',
      subtitle,
      primary: { id: 'btn-new-project', label: '+ New Project', shortcut: 'N' },
      toolbar: toolbarHTML,
    }) + html`
    <div class="card">
      <div class="table-wrap">
        <table>
          <thead><tr><th>Alias</th><th>Name</th><th>Description</th><th>Labels</th><th></th></tr></thead>
          <tbody>
            ${filtered.length === 0
              ? `<tr><td colspan="5"><div class="empty"><div class="empty-text">${search ? 'No projects match the search.' : 'No projects yet — click <strong>+ New Project</strong> to create one.'}</div></div></td></tr>`
              : filtered.map(p => {
                const archived = !!p.archived_at;
                const labels = state.projectLabels[p.alias] || [];
                return `
                <tr style="cursor:pointer" class="proj-row ${archived ? 'project-row-archived' : ''}" data-alias="${p.alias}">
                  <td class="text-muted text-sm mono">${escapeHtml(p.alias)}</td>
                  <td><strong>${escapeHtml(p.name)}</strong> ${archived ? '<span class="badge badge-archived">Archived</span>' : ''}</td>
                  <td class="text-muted text-sm">${escapeHtml(p.description) || '—'}</td>
                  <td>
                    <div class="project-labels-cell">
                      ${labels.length === 0 ? '<span class="text-muted text-sm">—</span>' : labels.map(l => `<span class="badge label-badge">${escapeHtml(l.name)}</span>`).join('')}
                    </div>
                  </td>
                  <td class="project-row-actions">
                    <button class="btn btn-ghost btn-sm project-add-label" data-alias="${escapeHtml(p.alias)}">+ Label</button>
                    ${archived ? `
                      <button class="btn btn-ghost btn-sm project-restore" data-alias="${escapeHtml(p.alias)}">Restore</button>
                      <button class="btn btn-ghost btn-sm btn-danger-ghost project-delete" data-alias="${escapeHtml(p.alias)}" data-name="${escapeHtml(p.name || p.alias)}">Delete</button>
                    ` : `
                      <button class="btn btn-ghost btn-sm btn-danger-ghost project-archive" data-alias="${escapeHtml(p.alias)}" data-name="${escapeHtml(p.name || p.alias)}">Archive</button>
                    `}
                  </td>
                </tr>`;
              }).join('')}
          </tbody>
        </table>
      </div>
    </div>`;

  let projectsSearchTimer;
  $('#projects-search').oninput = e => {
    clearTimeout(projectsSearchTimer);
    projectsSearchTimer = setTimeout(() => { state.projectsSearch = e.target.value; renderProjects(el); }, 250);
  };

  $('#btn-new-project').onclick = () => showNewProjectModal();
  $('#btn-projects-archived').onclick = () => {
    state.projectsShowArchived = !state.projectsShowArchived;
    renderProjects(el);
  };

  $$('.proj-row').forEach(row => row.onclick = () => {
    state.project = row.dataset.alias;
    updateProjectSelect();
    navigate('tasks');
  });
  $$('.project-add-label').forEach(btn => btn.onclick = e => {
    e.stopPropagation();
    showNewLabelModal(btn.dataset.alias);
  });
  $$('.project-archive').forEach(btn => btn.onclick = async e => {
    e.stopPropagation();
    const alias = btn.dataset.alias;
    const name = btn.dataset.name || alias;
    const ok = await confirm(`Archive project "${name}"? Archived projects are hidden by default.`, {
      confirmLabel: 'Archive project',
      confirmClass: 'btn-primary',
    });
    if (!ok) return;
    try {
      await api.patch('/projects/' + encodeURIComponent(alias) + '/archive', {});
      if (state.project === alias) {
        state.project = '';
        syncUrlProject();
      }
      toast('Project archived');
      renderProjects(el);
    } catch (err) { toast(err.message, 'error'); }
  });
  $$('.project-restore').forEach(btn => btn.onclick = async e => {
    e.stopPropagation();
    try {
      await api.patch('/projects/' + encodeURIComponent(btn.dataset.alias) + '/unarchive', {});
      toast('Project restored');
      renderProjects(el);
    } catch (err) { toast(err.message, 'error'); }
  });
  $$('.project-delete').forEach(btn => btn.onclick = async e => {
    e.stopPropagation();
    const alias = btn.dataset.alias;
    const name = btn.dataset.name || alias;
    const ok = await confirm(`Delete archived project "${name}"? This cannot be undone.`, {
      confirmLabel: 'Delete project',
      confirmClass: 'btn-danger',
    });
    if (!ok) return;
    try {
      await api.del('/projects/' + encodeURIComponent(alias));
      delete state.projectLabels[alias];
      if (state.project === alias) {
        state.project = '';
        updateProjectSelect();
      }
      toast('Project deleted');
      renderProjects(el);
    } catch (err) { toast(err.message, 'error'); }
  });
}

function showNewLabelModal(projectAlias) {
  const project = state.projects.find(p => p.alias === projectAlias);
  const overlay = showModal(html`
    <div class="modal modal-sm">
      <div class="modal-header"><span class="modal-title">New Label</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Project</label>
          <input class="form-control" value="${escapeHtml(project?.name || projectAlias)}" disabled>
        </div>
        <div class="form-group">
          <label class="form-label">Name *</label>
          <input class="form-control" id="new-label-name" placeholder="e.g. security">
        </div>
        <div class="form-group">
          <label class="form-label">Color <span class="text-muted">(optional)</span></label>
          <input class="form-control" id="new-label-color" type="color" value="#6940a5">
        </div>
      </div>
      <div class="modal-footer">
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-create-label">Create Label</button>
      </div>
    </div>`);

  const doCreate = async () => {
    const name = overlay.querySelector('#new-label-name').value.trim();
    if (!name) { toast('Label name is required', 'error'); return; }
    try {
      await api.post('/labels', {
        project: projectAlias,
        name,
        color: overlay.querySelector('#new-label-color').value,
      });
      delete state.projectLabels[projectAlias];
      overlay.remove();
      toast('Label created');
      renderProjects(document.getElementById('app'));
    } catch (e) { toast(e.message, 'error'); }
  };

  overlay.querySelector('#btn-create-label').onclick = doCreate;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doCreate(); });
}

function showNewProjectModal() {
  const overlay = showModal(html`
    <div class="modal modal-lg">
      <div class="modal-header"><span class="modal-title">New Project</span><button class="modal-close">×</button></div>
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Alias *</label>
          <input class="form-control" id="new-project-alias" placeholder="e.g. my-app">
          <div class="text-muted text-sm" style="margin-top:4px">Lowercase letters, digits, and hyphens. Must start with a letter or digit. Max 64 chars.</div>
        </div>
        <div class="form-group">
          <label class="form-label">Name *</label>
          <input class="form-control" id="new-project-name" placeholder="Display name">
        </div>
        <div class="form-group">
          <label class="form-label">Description</label>
          <textarea class="form-control" id="new-project-desc" placeholder="Optional — supports markdown"></textarea>
        </div>
        <div class="form-group">
          <label class="form-label">Repo path <span class="text-muted">(optional)</span></label>
          <input class="form-control" id="new-project-repo" placeholder="e.g. /Users/you/Projects/my-app">
        </div>
        <div id="new-project-error" class="text-sm" style="color:var(--red-fg);display:none;margin-top:8px"></div>
      </div>
      <div class="modal-footer">
        <span class="text-muted text-sm" style="margin-right:auto">Ctrl+Enter to save</span>
        <button class="btn btn-ghost modal-close">Cancel</button>
        <button class="btn btn-primary" id="btn-create-project">Create Project</button>
      </div>
    </div>`);

  const mde = createMDE(overlay.querySelector('#new-project-desc'), { minHeight: '160px' });

  const doCreate = async () => {
    const errEl = overlay.querySelector('#new-project-error');
    errEl.style.display = 'none';
    const alias = overlay.querySelector('#new-project-alias').value.trim();
    const name = overlay.querySelector('#new-project-name').value.trim();
    if (!alias) { errEl.textContent = 'Alias is required'; errEl.style.display = ''; return; }
    if (!/^[a-z0-9][a-z0-9-]{0,63}$/.test(alias)) {
      errEl.textContent = 'Alias must be lowercase alphanumeric with optional hyphens';
      errEl.style.display = '';
      return;
    }
    if (!name) { errEl.textContent = 'Name is required'; errEl.style.display = ''; return; }
    try {
      const created = await api.post('/projects', {
        alias, name,
        description: mde.value(),
        repo_path: overlay.querySelector('#new-project-repo').value.trim(),
      });
      overlay.remove();
      toast('Project created');
      state.project = created.alias;
      navigate('projects');
    } catch (e) {
      errEl.textContent = e.message;
      errEl.style.display = '';
    }
  };

  overlay.querySelector('#btn-create-project').onclick = doCreate;
  overlay.addEventListener('keydown', e => { if (e.key === 'Enter' && e.ctrlKey) doCreate(); });
}

// ─── Activity ────────────────────────────────────────────────────────────────

const ACTIVITY_ENTITY_KINDS = ['task', 'comment', 'plan', 'doc', 'memory', 'attachment', 'project', 'label'];
const ACTIVITY_EXPORT_LIMIT = 10000;
const ACTIVITY_EXPORT_LIMIT_LABEL = formatNumber(ACTIVITY_EXPORT_LIMIT);

function formatNumber(n) {
  return Number(n || 0).toLocaleString('en-US');
}

function buildActivityQuery(limit = ACTIVITY_EXPORT_LIMIT, offset = 0) {
  const q = new URLSearchParams();
  if (state.project) q.set('project', state.project);
  if (state.activityKindFilter) q.set('kind', state.activityKindFilter);
  if (state.activityActorFilter) q.set('actor', state.activityActorFilter);
  if (limit !== null) q.set('limit', limit);
  if (offset !== null) q.set('offset', offset);
  return q;
}

async function activityForExport() {
  const data = await api.get('/activity?' + buildActivityQuery(ACTIVITY_EXPORT_LIMIT, 0));
  return data.events || [];
}

function showActivityExportPopover(anchorEl) {
  showDownloadMenu(anchorEl, 'Export activity', [
    {
      label: 'All activity (.md)',
      run: async () => {
        const events = await activityForExport();
        downloadTextFile(activityExportFilename('all', 'md'), buildActivityMarkdown(events, { groupByDay: false }), 'text/markdown;charset=utf-8');
        toast('Activity exported');
      },
    },
    {
      label: 'Activity by day (.md)',
      run: async () => {
        const events = await activityForExport();
        downloadTextFile(activityExportFilename('by-day', 'md'), buildActivityMarkdown(events, { groupByDay: true }), 'text/markdown;charset=utf-8');
        toast('Activity exported');
      },
    },
    {
      label: 'JSON (.json)',
      run: async () => {
        const events = await activityForExport();
        const payload = {
          exported_at: new Date().toISOString(),
          scope: activityExportScope(),
          limit: ACTIVITY_EXPORT_LIMIT,
          events,
        };
        downloadTextFile(activityExportFilename('all', 'json'), JSON.stringify(payload, null, 2) + '\n', 'application/json;charset=utf-8');
        toast('Activity exported');
      },
    },
  ], `Uses current filters and exports the latest ${ACTIVITY_EXPORT_LIMIT_LABEL} events.`);
}

function activityExportScope() {
  return {
    project: state.project || null,
    project_name: selectedProjectName() || null,
    kind: state.activityKindFilter || null,
    actor: state.activityActorFilter || null,
  };
}

function activityExportFilename(mode, ext) {
  const date = new Date().toISOString().slice(0, 10);
  const parts = [exportScopeName({ aggregated: !state.project })];
  if (state.activityKindFilter) parts.push(state.activityKindFilter);
  if (state.activityActorFilter) parts.push(state.activityActorFilter);
  return `backlog-activity-${mode}-${safeFilenamePart(parts.join('-'))}-${date}.${ext}`;
}

function activityDayExportFilename(day, ext) {
  const parts = [exportScopeName({ aggregated: !state.project }), activityDayKey(day)];
  if (state.activityKindFilter) parts.push(state.activityKindFilter);
  if (state.activityActorFilter) parts.push(state.activityActorFilter);
  return `backlog-activity-day-${safeFilenamePart(parts.join('-'))}.${ext}`;
}

function buildActivityMarkdown(events, opts = {}) {
  const scopeName = state.project ? (selectedProjectName() || state.project) : 'All Projects';
  const title = opts.groupByDay ? 'Backlog Activity By Day' : 'Backlog Activity';
  const lines = [
    `# ${markdownLine(title + ' - ' + scopeName)}`,
    '',
    `Exported: ${new Date().toLocaleString()}`,
    `Limit: latest ${ACTIVITY_EXPORT_LIMIT_LABEL} events`,
  ];
  if (state.activityKindFilter) lines.push(`Kind: ${state.activityKindFilter}`);
  if (state.activityActorFilter) lines.push(`Actor: ${state.activityActorFilter}`);
  lines.push('');

  if (!events.length) {
    lines.push('No activity found.');
    return lines.join('\n') + '\n';
  }

  if (opts.groupByDay) {
    activityDayGroups(events).forEach(group => {
      lines.push(`## ${markdownLine(timelineDayLabel(group.day))}`, '');
      group.items.forEach(ev => appendActivityMarkdownLine(lines, ev));
      lines.push('');
    });
  } else {
    events.forEach(ev => appendActivityMarkdownLine(lines, ev));
  }

  return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

function buildActivityDayMarkdown(group) {
  const scopeName = state.project ? (selectedProjectName() || state.project) : 'All Projects';
  const dayLabel = timelineDayLabel(group.day);
  const lines = [
    `# ${markdownLine('Backlog Activity - ' + dayLabel)}`,
    '',
    `Project: ${scopeName}`,
    `Exported: ${new Date().toLocaleString()}`,
    `Source: latest ${ACTIVITY_EXPORT_LIMIT_LABEL} matching events, narrowed to this day`,
  ];
  if (state.activityKindFilter) lines.push(`Kind: ${state.activityKindFilter}`);
  if (state.activityActorFilter) lines.push(`Actor: ${state.activityActorFilter}`);
  lines.push('');

  if (!group.items.length) {
    lines.push('No activity found for this day.');
    return lines.join('\n') + '\n';
  }

  group.items.forEach(ev => appendActivityMarkdownLine(lines, ev));
  return lines.join('\n').replace(/\n{3,}/g, '\n\n') + '\n';
}

function appendActivityMarkdownLine(lines, ev) {
  const actor = `${ev.actor?.kind || ''}:${ev.actor?.name || ''}`.replace(/^:|:$/g, '');
  const parts = [
    formatDate(ev.created_at),
    ev.entity || 'event',
    ev.action || '',
    actor,
  ].filter(Boolean).join(' | ');
  lines.push(`- ${parts}${ev.summary ? ' - ' + ev.summary : ''}`);
}

function activityDayKey(day) {
  const y = day.getFullYear();
  const m = String(day.getMonth() + 1).padStart(2, '0');
  const d = String(day.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

function activityDayGroups(events) {
  const groups = [];
  let currentKey = '';
  let currentGroup = null;
  events.forEach(ev => {
    const d = new Date((ev.created_at || 0) / 1e6);
    const key = d.toDateString();
    if (key !== currentKey) {
      currentKey = key;
      currentGroup = { day: d, items: [] };
      groups.push(currentGroup);
    }
    currentGroup.items.push(ev);
  });
  return groups;
}

async function renderActivity(el) {
  const kindFilter = state.activityKindFilter || '';
  const actorFilter = state.activityActorFilter || '';
  const q = buildActivityQuery(ACTIVITY_EXPORT_LIMIT, 0);

  let events = [];
  let errorMsg = '';
  try {
    const data = await api.get('/activity?' + q);
    events = data.events || [];
  } catch (e) {
    errorMsg = e.message;
  }

  const subtitle = errorMsg
    ? 'Failed to load'
    : (events.length + ' event' + (events.length !== 1 ? 's' : '') + ` · Newest first · Showing latest ${ACTIVITY_EXPORT_LIMIT_LABEL}`);

  const toolbarHTML =
    `<select class="filter-select" id="activity-kind-filter">` +
      `<option value="">All kinds</option>` +
      ACTIVITY_ENTITY_KINDS.map(k =>
        `<option value="${k}" ${kindFilter === k ? 'selected' : ''}>${k}</option>`
      ).join('') +
    `</select>` +
    `<input class="search-input activity-actor-input" id="activity-actor-filter" placeholder="Actor name or kind:name" value="${escapeHtml(actorFilter)}">` +
    (kindFilter || actorFilter ? `<button class="btn btn-ghost btn-sm" id="btn-clear-activity-filters">Clear</button>` : '');

  let body;
  if (errorMsg) {
    body = `<div class="empty"><div class="empty-icon"></div><div class="empty-text" style="color:var(--red-fg)">Failed to load activity: ${escapeHtml(errorMsg)}</div></div>`;
  } else if (events.length === 0) {
    body = `<div class="empty"><div class="empty-icon"></div><div class="empty-text">${
      kindFilter || actorFilter ? 'No activity matches the current filters.'
      : (state.project ? 'No activity yet for this project.' : 'No activity yet.')
    }</div></div>`;
  } else {
    const groups = activityDayGroups(events);
    body = '<div class="activity-timeline">' + groups.map(g => `
      <section class="activity-day">
        <div class="activity-day-head">
          <div class="activity-day-label">${escapeHtml(timelineDayLabel(g.day))}</div>
          <button class="btn btn-ghost btn-sm activity-day-export" data-day-key="${activityDayKey(g.day)}">Export</button>
        </div>
        ${g.items.map(ev => `
          <div class="activity-event">
            <div class="activity-event-dot"></div>
            <div class="activity-event-main">
              <div class="activity-event-head">
                ${activityEntityLink(ev)}
                <span class="badge activity-kind">${escapeHtml(ev.entity || '')}</span>
                <span class="text-muted text-sm">${escapeHtml(ev.action || '')}</span>
                <span class="activity-time" title="${formatDate(ev.created_at)}">${timeAgo(ev.created_at)}</span>
              </div>
              <div class="activity-summary">${escapeHtml(ev.summary || '')}</div>
              <div class="activity-actor mono">${escapeHtml(ev.actor?.kind || '')}:${escapeHtml(ev.actor?.name || '')}</div>
            </div>
          </div>`).join('')}
      </section>`).join('') + '</div>';
  }

  el.innerHTML =
    pageHeader({
      title: 'Activity',
      subtitle,
      viewToggle: '<button class="btn btn-ghost" id="btn-activity-export">Export</button>',
      toolbar: toolbarHTML,
    }) + body;

  $('#btn-activity-export').onclick = e => showActivityExportPopover(e.currentTarget);

  $$('.activity-day-export').forEach(btn => btn.onclick = e => {
    e.preventDefault();
    const group = activityDayGroups(events).find(g => activityDayKey(g.day) === btn.dataset.dayKey);
    if (!group) return;
    downloadTextFile(activityDayExportFilename(group.day, 'md'), buildActivityDayMarkdown(group), 'text/markdown;charset=utf-8');
    toast('Activity exported');
  });

  $$('.activity-task-link').forEach(link => link.onclick = e => {
    e.preventDefault();
    navigate('task-detail', { currentTaskId: link.dataset.id });
  });
  $$('.activity-doc-link').forEach(link => link.onclick = e => {
    e.preventDefault();
    navigate('docs', { docId: link.dataset.id });
  });

  $('#activity-kind-filter').onchange = e => {
    state.activityKindFilter = e.target.value;
    renderActivity(el);
  };
  let actorTimer;
  $('#activity-actor-filter').oninput = e => {
    clearTimeout(actorTimer);
    actorTimer = setTimeout(() => {
      state.activityActorFilter = e.target.value.trim();
      renderActivity(el);
    }, 300);
  };
  const clearBtn = $('#btn-clear-activity-filters');
  if (clearBtn) clearBtn.onclick = () => {
    state.activityKindFilter = '';
    state.activityActorFilter = '';
    renderActivity(el);
  };
  clearTimeout(state.activityRefreshTimer);
  state.activityRefreshTimer = setTimeout(() => {
    if (state.page === 'activity') renderActivity(el);
  }, 30000);
}

function activityEntityLink(ev) {
  const entity = ev.entity || '';
  const id = ev.entity_id || '';
  if (entity === 'task' && id) {
    return `<a href="#" class="activity-entity-link activity-task-link" data-id="${escapeHtml(id)}">Task</a>`;
  }
  if (entity === 'doc' && id) {
    return `<a href="#" class="activity-entity-link activity-doc-link" data-id="${escapeHtml(id)}">Doc</a>`;
  }
  return `<span class="activity-entity-link">${escapeHtml(entity || 'event')}</span>`;
}

// ─── Keyboard shortcuts ───────────────────────────────────────────────────────
// `N` opens the page-appropriate "create" flow. Pages without a create action
// (plans, activity) intentionally fall through.
document.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    const modals = $$('.modal-overlay');
    if (modals.length) { modals[modals.length - 1].remove(); return; }
  }
  if (e.key !== 'n' || e.ctrlKey || e.metaKey || e.altKey) return;
  if ($('.modal-overlay')) return;
  if (e.target.closest('input,textarea,select,[contenteditable],[contenteditable=""],.CodeMirror,.EasyMDEContainer')) return;
  let action;
  switch (state.page) {
    case 'tasks':       action = showNewTaskModal; break;
    case 'docs':        action = showNewDocModal; break;
    case 'memory':      action = showNewMemoryModal; break;
    case 'attachments': action = showUploadAttachmentModal; break;
    case 'projects':    action = showNewProjectModal; break;
  }
  if (!action) return;
  e.preventDefault();
  action();
});

// ─── Project select helper ────────────────────────────────────────────────────
function updateProjectSelect() {
  const psel = $('#project-select');
  if (!psel) return;
  const visibleProjects = state.projects.filter(p => !p.archived_at || p.alias === state.project);
  psel.innerHTML =
    `<option value="">All projects</option>` +
    visibleProjects.map(p =>
      `<option value="${p.alias}" ${p.alias===state.project?'selected':''}>${escapeHtml(p.name || p.alias)}</option>`
    ).join('');
  psel.value = state.project;
}

// ─── Init ────────────────────────────────────────────────────────────────────
async function init() {
  // Parse the URL first so the initial fetch can honour ?project=<alias>.
  const initialRoute = parseRoute(location.pathname, location.search);

  // Restore the last-used Tasks view from cookie. URL ?view= will overwrite
  // this below via applyRoute (URL > cookie > default precedence).
  const savedView = getCookie('backlog.taskView');
  if (savedView && VALID_TASK_VIEWS.includes(savedView)) {
    state.taskView = savedView;
  }
  const savedGroupBy = getCookie('backlog.kanbanGroupBy');
  if (savedGroupBy && ['status','priority','type','assignee'].includes(savedGroupBy)) {
    state.kanbanGroupBy = savedGroupBy;
  }
  const savedArchivedView = getCookie('backlog.archivedView');
  if (savedArchivedView === 'archived' || savedArchivedView === 'active') {
    state.archivedView = savedArchivedView;
  }

  try {
    const data = await api.get('/projects');
    state.projects = data.projects || [];
  } catch (e) { /* server may not be ready */ }

  // URL wins over the "first project" default so deep links restore filters.
  if (initialRoute.params.project !== undefined) {
    state.project = initialRoute.params.project;
  } else if (state.projects.length > 0) {
    state.project = state.projects[0].alias;
  }
  applyRoute(initialRoute);

  updateProjectSelect();

  $('#project-select').onchange = e => {
    state.project = e.target.value;
    // Reset memory tag filter when changing project
    state.memoryTagFilter = '';
    syncUrlProject();
    render();
  };

  $$('.sidebar-item[data-page]').forEach(el =>
    el.onclick = e => { e.preventDefault(); navigate(el.dataset.page); });

  // Replace the initial entry so back-button leaves the SPA cleanly instead
  // of cycling between "/" and the parsed URL.
  try {
    history.replaceState(
      { page: state.page, stateOverrides: {} },
      '',
      buildRoute(state.page, {
        taskRef: state.currentTaskSeq || state.currentTaskId,
      }),
    );
  } catch (_) { /* ignore */ }

  window.addEventListener('popstate', () => {
    const route = parseRoute(location.pathname, location.search);
    applyRoute(route);
    updateProjectSelect();
    updateCmdBar();
    render();
  });
  window.addEventListener('focus', () => {
    if (state.page === 'activity') render();
  });

  render();
}

document.addEventListener('DOMContentLoaded', init);
