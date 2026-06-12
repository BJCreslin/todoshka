import { route, go, escapeHtml } from '../router.js';
import { store } from '../store.js';
import { api } from '../api.js';

route('/login',    () => render());
route('/register', () => render());

async function render() {
  const app = document.getElementById('app');
  const isRegister = location.hash === '#/register';
  app.innerHTML = `
    <div class="auth">
      <h1>${isRegister ? 'Создать аккаунт' : 'Войти'}</h1>
      <form id="authForm">
        <input name="username" placeholder="Имя пользователя" required minlength="3" maxlength="32" autocomplete="username">
        <input name="password" type="password" placeholder="Пароль (мин. 8)" required minlength="8" autocomplete="${isRegister ? 'new-password' : 'current-password'}">
        <button type="submit">${isRegister ? 'Создать' : 'Войти'}</button>
      </form>
      <p>${isRegister ? 'Уже есть аккаунт?' : 'Нет аккаунта?'}
        <a href="#/${isRegister ? 'login' : 'register'}">${isRegister ? 'Войти' : 'Создать'}</a>
      </p>
    </div>`;
  document.getElementById('authForm').onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    const fn = isRegister ? api.register : api.login;
    try {
      const r = await fn(f.username.value, f.password.value);
      store.setSession(r.user, r.token);
      go('/tasks');
    } catch (err) { alert(err.message); }
  };
}
