import { route } from '../router.js';
route('/notes/{id}', (p) => {
  document.getElementById('app').innerHTML = `<h2>Заметка #${p.id}</h2>`;
});
