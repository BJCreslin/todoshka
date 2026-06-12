export const store = {
  user: JSON.parse(localStorage.getItem('user') || 'null'),
  token: localStorage.getItem('token') || '',
  setSession(user, token) {
    this.user = user; this.token = token;
    localStorage.setItem('user', JSON.stringify(user));
    localStorage.setItem('token', token);
  },
  clear() {
    this.user = null; this.token = '';
    localStorage.removeItem('user'); localStorage.removeItem('token');
  },
  isAuthed() { return !!this.token; },
};
