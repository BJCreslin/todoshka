import { route } from '../router.js';
import { layout, bindLayout } from '../components/layout.js';
route('/tasks', async () => {
  document.getElementById('app').innerHTML = layout('/tasks', '<h2>Задачи</h2><p>В разработке…</p>');
  bindLayout();
});
