import { startRouter } from './router.js';
import { store } from './store.js';
import './views/login.js';
import './views/tasks.js';
import './views/task.js';
import './views/notes.js';
import './views/note.js';
import './views/shared.js';
import './views/search.js';

const root = document.getElementById('app');
if (!store.isAuthed() && !location.hash.startsWith('#/login') && !location.hash.startsWith('#/register')) {
  location.hash = '#/login';
} else if (store.isAuthed() && (location.hash === '' || location.hash === '#/')) {
  location.hash = '#/tasks';
}
startRouter(root);
