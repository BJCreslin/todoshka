import { route } from '../router.js';
route('/shared', () => { document.getElementById('app').innerHTML = '<h2>Общие</h2>'; });
