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
    <ul class="links" id="linkedTasks">${(n.linked_tasks || []).map((tid) => `<li><a href="#/tasks/${tid}">Задача #${tid}</a> <button class="unlink-task" data-tid="${tid}">×</button></li>`).join('') || '<li class="empty">—</li>'}</ul>
    <form id="addTaskLink" class="link-form">
      <select name="task_id" id="taskSelect"><option value="">— загрузка… —</option></select>
      <button type="submit">+</button>
    </form>

    <h3>Доступ</h3>
    <ul id="shareList"><li class="empty">загрузка…</li></ul>
  `;
}

async function initEditor(n) {
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

  // Shares list — independent failure mode
  try {
    const shares = await fetch('/api/notes/' + editor.id + '/shares', {
      headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('token') || '') },
    }).then((r) => r.ok ? r.json() : []);
    const list = document.getElementById('shareList');
    if (!list) return;
    if (!shares.length) {
      list.innerHTML = '<li class="empty">никого</li>';
    } else {
      list.innerHTML = shares.map((s) => `<li>${escapeHtml(s.username)} <button class="revoke-share" data-uid="${s.user_id}">×</button></li>`).join('');
      list.querySelectorAll('.revoke-share').forEach((btn) => {
        btn.onclick = async () => {
          if (!confirm('Отозвать доступ?')) return;
          try { await api.unshare('note', editor.id, parseInt(btn.dataset.uid, 10)); location.reload(); }
          catch (e) { alert(e.message); }
        };
      });
    }
  } catch (e) { /* ignore shares failure */ }

  // Link picker for tasks — INDEPENDENT of shares fetch
  try {
    const allTasks = await api.listTasks();
    const linkedIds = new Set((n.linked_tasks || []).map(Number));
    const available = allTasks.filter((tk) => !linkedIds.has(tk.id));
    const select = document.getElementById('taskSelect');
    if (!available.length) {
      select.innerHTML = '<option value="">— нет доступных —</option>';
      select.disabled = true;
    } else {
      select.innerHTML = '<option value="">— выберите задачу —</option>' +
        available.map((tk) => `<option value="${tk.id}">${escapeHtml(tk.title)} (#${tk.id})</option>`).join('');
    }
    document.getElementById('addTaskLink').onsubmit = async (e) => {
      e.preventDefault();
      const taskId = parseInt(select.value, 10);
      if (!taskId) return;
      try { await api.linkTaskToNote(editor.id, taskId); location.reload(); }
      catch (e) { alert(e.message); }
    };
  } catch (e) { /* ignore link picker failure */ }

  // Unlink buttons
  document.querySelectorAll('.unlink-task').forEach((btn) => {
    btn.onclick = async () => {
      const tid = btn.dataset.tid;
      const token = localStorage.getItem('token') || '';
      const r = await fetch('/api/notes/' + editor.id + '/tasks/' + tid, {
        method: 'DELETE', headers: { 'Authorization': 'Bearer ' + token },
      });
      if (!r.ok) { alert('Не удалось открепить'); return; }
      location.reload();
    };
  });
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
