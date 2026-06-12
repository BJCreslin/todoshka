import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/notes/{id}', (p) => render(p.id));

let editor = null;

async function render(id) {
  const app = document.getElementById('app');
  app.innerHTML = layout('/notes', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const n = await api.getNote(id);
  app.innerHTML = layout('/notes', editorHtml(n));
  bindLayout();
  initEditor(n);
}

function editorHtml(n) {
  return `
    <a href="#/notes">← К заметкам</a>
    <div class="note-toolbar">
      <input id="title" value="${escapeHtml(n.title)}" placeholder="Заголовок">
      <label><input type="checkbox" id="pinned" ${n.pinned ? 'checked' : ''}> Закрепить</label>
      <button id="saveBtn">Сохранить</button>
      <button id="versionsBtn">История</button>
      <button id="shareBtn">Поделиться</button>
      <button id="deleteBtn" class="danger">Удалить</button>
    </div>
    <div class="editor-split">
      <textarea id="body" placeholder="Markdown...">${escapeHtml(n.body_md)}</textarea>
      <div id="preview" class="preview"></div>
    </div>
    <h3>Теги</h3>
    <ul class="tags">${(n.tags || []).map((tag) => `<li>${escapeHtml(tag)} <button class="del-tag" data-tag="${escapeHtml(tag)}">×</button></li>`).join('') || '<li class="empty">—</li>'}</ul>
    <form id="addTag"><input name="tag" placeholder="новый-тег"><button>+</button></form>

    <h3>Прикреплённые задачи</h3>
    <ul class="links">${(n.linked_tasks || []).map((tid) => `<li><a href="#/tasks/${tid}">Задача #${tid}</a></li>`).join('') || '<li class="empty">—</li>'}</ul>

    <h3>Доступ</h3>
    <ul id="shareList"><li class="empty">загрузка…</li></ul>
  `;
}

function initEditor(n) {
  editor = { id: n.id };
  const marked = window.marked;
  const bodyEl = document.getElementById('body');
  const previewEl = document.getElementById('preview');
  const render = () => { previewEl.innerHTML = marked.parse(bodyEl.value); };
  bodyEl.addEventListener('input', render);
  render();

  document.getElementById('title').addEventListener('change', save);
  bodyEl.addEventListener('change', save);
  document.getElementById('pinned').addEventListener('change', save);
  document.getElementById('saveBtn').onclick = () => save().then(() => alert('Сохранено'));

  document.getElementById('addTag').onsubmit = async (e) => {
    e.preventDefault();
    const tag = e.target.tag.value.trim();
    if (!tag) return;
    await api.addNoteTag(editor.id, tag);
    location.reload();
  };
  document.querySelectorAll('.del-tag').forEach((btn) => {
    btn.onclick = async () => {
      const token = localStorage.getItem('token') || '';
      await fetch('/api/notes/' + editor.id + '/tags/' + encodeURIComponent(btn.dataset.tag), {
        method: 'DELETE', headers: { 'Authorization': 'Bearer ' + token },
      });
      location.reload();
    };
  });

  document.getElementById('versionsBtn').onclick = () => openVersions(editor.id);
  document.getElementById('shareBtn').onclick = () => {
    const username = prompt('Имя пользователя для шаринга?');
    if (!username) return;
    api.share('note', editor.id, username).then(() => { alert('Доступ открыт'); location.reload(); }).catch((e) => alert(e.message));
  };
  document.getElementById('deleteBtn').onclick = async () => {
    if (!confirm('Удалить заметку?')) return;
    await api.deleteNote(editor.id);
    go('/notes');
  };

  // Render share list
  fetch('/api/notes/' + editor.id + '/shares', {
    headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('token') || '') },
  }).then((r) => r.ok ? r.json() : []).then((shares) => {
    const list = document.getElementById('shareList');
    if (!list) return;
    if (!shares.length) { list.innerHTML = '<li class="empty">никого</li>'; return; }
    list.innerHTML = shares.map((s) => `<li>${escapeHtml(s.username)} (id=${s.user_id})</li>`).join('');
  }).catch(() => {});
}

async function save() {
  await api.updateNote(editor.id, {
    title:  document.getElementById('title').value,
    body_md: document.getElementById('body').value,
    pinned: document.getElementById('pinned').checked,
  });
}

async function openVersions(id) {
  const vs = await api.versions(id);
  if (!vs.length) { alert('История пуста'); return; }
  const lines = vs.map((v, i) => `${i + 1}. [${new Date(v.saved_at).toLocaleString('ru')}] ${v.editor_name || '—'} — ${escapeHtml(v.title)}`);
  const choice = prompt('Введите номер версии для восстановления:\n\n' + lines.join('\n'));
  if (!choice) return;
  const idx = parseInt(choice, 10) - 1;
  if (isNaN(idx) || idx < 0 || idx >= vs.length) { alert('Неверный номер'); return; }
  await api.restore(id, vs[idx].id);
  location.reload();
}
