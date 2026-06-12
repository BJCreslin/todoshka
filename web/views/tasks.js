import { route, go, escapeHtml } from '../router.js';
import { api } from '../api.js';
import { layout, bindLayout } from '../components/layout.js';

route('/tasks', () => render());

async function render() {
  const app = document.getElementById('app');
  app.innerHTML = layout('/tasks', '<div class="loading">Загрузка…</div>');
  bindLayout();
  const tasks = await api.listTasks();
  app.innerHTML = layout('/tasks', boardHtml(tasks));
  bindLayout();
  bindBoard();
}

function boardHtml(tasks) {
  const cols = ['todo', 'in_progress', 'done'];
  const labels = { todo: 'К выполнению', in_progress: 'В работе', done: 'Готово' };
  const card = (t) => `
    <article class="card" draggable="true" data-id="${t.id}">
      <h4>${escapeHtml(t.title)}</h4>
      ${t.priority ? `<span class="prio prio-${escapeHtml(t.priority)}">${escapeHtml(t.priority)}</span>` : ''}
      ${t.due_date ? `<small>${escapeHtml(t.due_date)}</small>` : ''}
    </article>`;
  const cards = (status) => {
    const filtered = tasks.filter((t) => t.status === status);
    return filtered.length ? filtered.map(card).join('') : '<p class="empty">—</p>';
  };
  return `
    <h2>Задачи</h2>
    <div class="board">
      ${cols.map((c) => `
        <section class="col">
          <header><h3>${labels[c]}</h3><button class="add" data-status="${c}">+</button></header>
          <div class="cards" data-status="${c}">${cards(c)}</div>
        </section>`).join('')}
    </div>`;
}

function bindBoard() {
  document.querySelectorAll('.add').forEach((btn) => {
    btn.onclick = () => openCreate(btn.dataset.status);
  });
  document.querySelectorAll('.card').forEach((card) => {
    card.onclick = (e) => {
      if (e.target.closest('button')) return;
      go('/tasks/' + card.dataset.id);
    };
    card.ondragstart = (e) => e.dataTransfer.setData('text/plain', card.dataset.id);
  });
  document.querySelectorAll('.cards').forEach((zone) => {
    zone.ondragover = (e) => { e.preventDefault(); zone.classList.add('over'); };
    zone.ondragleave = () => zone.classList.remove('over');
    zone.ondrop = async (e) => {
      e.preventDefault();
      zone.classList.remove('over');
      const id = e.dataTransfer.getData('text/plain');
      const status = zone.dataset.status;
      try { await api.updateTask(id, { status }); location.reload(); }
      catch (err) { alert(err.message); }
    };
  });
}

function openCreate(defaultStatus) {
  const title = prompt('Название задачи?');
  if (!title) return;
  api.createTask({ title, status: defaultStatus })
    .then(() => location.reload())
    .catch((err) => alert(err.message));
}
