import { store } from '../store.js';
import { go } from '../router.js';

export function renderSidebar(currentPath) {
  return `
    <aside class="sidebar">
      <div class="me"><strong>${esc(store.user?.username || '')}</strong>
        <button id="logout">Выйти</button>
      </div>
      <nav>
        <a href="#/tasks"  class="${currentPath === '/tasks'  ? 'active' : ''}">📋 Задачи</a>
        <a href="#/notes"  class="${currentPath === '/notes'  ? 'active' : ''}">📝 Заметки</a>
        <a href="#/shared" class="${currentPath === '/shared' ? 'active' : ''}">👥 Общие</a>
      </nav>
      <form id="searchForm" class="search">
        <input name="q" placeholder="Поиск…">
        <button type="submit">Найти</button>
      </form>
    </aside>`;
}

export function bindSidebar() {
  document.getElementById('logout')?.addEventListener('click', () => { store.clear(); go('/login'); });
  document.getElementById('searchForm')?.addEventListener('submit', (e) => {
    e.preventDefault();
    const q = e.target.q.value.trim();
    if (q) go('/search?q=' + encodeURIComponent(q));
  });
}

function esc(s) { return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c])); }
