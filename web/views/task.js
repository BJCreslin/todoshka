import { route } from '../router.js';
route('/tasks/{id}', (p) => {
  document.getElementById('app').innerHTML = `<h2>Задача #${p.id}</h2>`;
});
