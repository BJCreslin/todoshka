import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/tasks/{id}', (p) => render(p.id));

async function render(id) {
  const app = document.getElementById('app');
  app.innerHTML = layout('/tasks', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const t = await api.getTask(id);
  app.innerHTML = layout('/tasks', detailHtml(t));
  bindLayout();
  bindDetail(t);
}

function detailHtml(t) {
  return `
    <a href="#/tasks">← К доске</a>
    <h2>${escapeHtml(t.title)}</h2>
    <p>${t.description ? escapeHtml(t.description).replaceAll('\n', '<br>') : '<em>нет описания</em>'}</p>
    <div class="row">
      <label>Статус:
        <select id="status">
          <option value="todo"        ${t.status === 'todo'        ? 'selected' : ''}>К выполнению</option>
          <option value="in_progress" ${t.status === 'in_progress' ? 'selected' : ''}>В работе</option>
          <option value="done"        ${t.status === 'done'        ? 'selected' : ''}>Готово</option>
        </select>
      </label>
      <label>Приоритет:
        <select id="priority">
          <option value="low"    ${t.priority === 'low'    ? 'selected' : ''}>Низкий</option>
          <option value="medium" ${t.priority === 'medium' ? 'selected' : ''}>Средний</option>
          <option value="high"   ${t.priority === 'high'   ? 'selected' : ''}>Высокий</option>
        </select>
      </label>
      <label>Срок: <input id="due_date" type="date" value="${t.due_date || ''}"></label>
    </div>

    <h3>Подзадачи</h3>
    <ul class="subtasks">
      ${(t.subtasks || []).map((s) => `
        <li>
          <input type="checkbox" data-sid="${s.id}" ${s.done ? 'checked' : ''}>
          ${escapeHtml(s.title)}
          <button class="del-sub" data-sid="${s.id}">×</button>
        </li>`).join('') || '<li class="empty">—</li>'}
    </ul>
    <form id="addSubtask"><input name="title" placeholder="Новая подзадача"><button>+</button></form>

    <h3>Теги</h3>
    <ul class="tags">${(t.tags || []).map((tag) => `<li>${escapeHtml(tag)} <button class="del-tag" data-tag="${escapeHtml(tag)}">×</button></li>`).join('') || '<li class="empty">—</li>'}</ul>
    <form id="addTag"><input name="tag" placeholder="новый-тег"><button>+</button></form>

    <h3>Прикреплённые заметки</h3>
    <ul class="links" id="linkedNotes">${(t.linked_notes || []).map((nid) => `<li><a href="#/notes/${nid}">Заметка #${nid}</a> <button class="unlink-note" data-nid="${nid}">×</button></li>`).join('') || '<li class="empty">—</li>'}</ul>
    <form id="addNoteLink" class="link-form">
      <select name="note_id" id="noteSelect"><option value="">— загрузка… —</option></select>
      <button type="submit">+</button>
    </form>

    <h3>Доступ</h3>
    <ul id="shareList"><li class="empty">загрузка…</li></ul>
    <button id="shareBtn">Поделиться…</button>

    <p><button id="deleteBtn" class="danger">Удалить задачу</button></p>
  `;
}

async function bindDetail(t) {
  document.getElementById('status').onchange  = (e) => api.updateTask(t.id, { status: e.target.value }).then(refresh);
  document.getElementById('priority').onchange = (e) => api.updateTask(t.id, { priority: e.target.value }).then(refresh);
  document.getElementById('due_date').onchange = (e) => api.updateTask(t.id, { due_date: e.target.value || null }).then(refresh);

  document.getElementById('addSubtask').onsubmit = async (e) => {
    e.preventDefault();
    const title = e.target.title.value.trim();
    if (!title) return;
    await api.addSubtask(t.id, title);
    refresh();
  };
  document.querySelectorAll('.del-sub').forEach((btn) => {
    btn.onclick = async () => {
      const sid = btn.dataset.sid;
      const token = localStorage.getItem('token') || '';
      await fetch('/api/tasks/' + t.id + '/subtasks/' + sid, {
        method: 'DELETE',
        headers: { 'Authorization': 'Bearer ' + token },
      });
      refresh();
    };
  });
  document.querySelectorAll('input[type=checkbox][data-sid]').forEach((cb) => {
    cb.onchange = async () => {
      const sid = cb.dataset.sid;
      const token = localStorage.getItem('token') || '';
      await fetch('/api/tasks/' + t.id + '/subtasks/' + sid, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
        body: JSON.stringify({ done: cb.checked }),
      });
    };
  });

  document.getElementById('addTag').onsubmit = async (e) => {
    e.preventDefault();
    const tag = e.target.tag.value.trim();
    if (!tag) return;
    await api.addTaskTag(t.id, tag);
    refresh();
  };
  document.querySelectorAll('.del-tag').forEach((btn) => {
    btn.onclick = async () => { await api.removeTaskTag(t.id, btn.dataset.tag); refresh(); };
  });

  document.getElementById('shareBtn').onclick = () => openShare('task', t.id);
  document.getElementById('deleteBtn').onclick = async () => {
    if (!confirm('Удалить задачу?')) return;
    await api.deleteTask(t.id);
    go('/tasks');
  };

  // Load and render the current share list
  try {
    const shares = await fetch('/api/tasks/' + t.id + '/shares', {
      headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('token') || '') },
    }).then((r) => r.ok ? r.json() : []);
    const list = document.getElementById('shareList');
    if (!shares.length) { list.innerHTML = '<li class="empty">никого</li>'; }
    else {
      list.innerHTML = shares.map((s) => `<li>${escapeHtml(s.username)} <button class="revoke-share" data-uid="${s.user_id}">×</button></li>`).join('');
      list.querySelectorAll('.revoke-share').forEach((btn) => {
        btn.onclick = async () => {
          if (!confirm('Отозвать доступ у ' + btn.parentElement.textContent.trim().replace('×','').trim() + '?')) return;
          try { await api.unshare('task', t.id, parseInt(btn.dataset.uid, 10)); location.reload(); }
          catch (e) { alert(e.message); }
        };
      });
    }

    document.querySelectorAll('.unlink-note').forEach((btn) => {
      btn.onclick = async () => {
        const nid = btn.dataset.nid;
        const token = localStorage.getItem('token') || '';
        await fetch('/api/tasks/' + t.id + '/notes/' + nid, {
          method: 'DELETE', headers: { 'Authorization': 'Bearer ' + token },
        });
        location.reload();
      };
    });

    try {
      const allNotes = await api.listNotes();
      const linkedIds = new Set((t.linked_notes || []).map(Number));
      const available = allNotes.filter((n) => !linkedIds.has(n.id));
      const select = document.getElementById('noteSelect');
      if (!available.length) {
        select.innerHTML = '<option value="">— нет доступных —</option>';
        select.disabled = true;
      } else {
        select.innerHTML = '<option value="">— выберите заметку —</option>' +
          available.map((n) => `<option value="${n.id}">${escapeHtml(n.title)} (#${n.id})</option>`).join('');
      }
      document.getElementById('addNoteLink').onsubmit = async (e) => {
        e.preventDefault();
        const noteId = parseInt(select.value, 10);
        if (!noteId) return;
        try { await api.linkNoteToTask(t.id, noteId); location.reload(); }
        catch (e) { alert(e.message); }
      };
    } catch (e) { /* ignore */ }
  } catch (e) { /* ignore */ }
}

function refresh() { location.reload(); }

async function openShare(type, id) {
  const username = prompt('Имя пользователя для шаринга?');
  if (!username) return;
  try { await api.share(type, id, username); alert('Доступ открыт'); location.reload(); }
  catch (e) { alert(e.message); }
}
