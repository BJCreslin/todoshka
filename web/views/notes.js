import { route } from '../router.js';
route('/notes', () => { document.getElementById('app').innerHTML = '<h2>Заметки</h2>'; });
