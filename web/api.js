import { store } from './store.js';

async function request(method, path, body) {
  const headers = { 'Content-Type': 'application/json' };
  if (store.token) headers['Authorization'] = 'Bearer ' + store.token;
  const res = await fetch(path, {
    method, headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 401) {
    store.clear(); location.hash = '#/login';
    throw new Error('unauthorized');
  }
  if (res.status === 204) return null;
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) throw new Error((data && data.error) || res.statusText);
  return data;
}

const q = (params) => '?' + new URLSearchParams(params).toString();
const enc = encodeURIComponent;

export const api = {
  register: (username, password) => request('POST', 'api/register', { username, password }),
  login:    (username, password) => request('POST', 'api/login',    { username, password }),
  me:       ()                   => request('GET',  'api/me'),
  searchUsers: (s)               => request('GET',  'api/users?q=' + enc(s)),

  listTasks:   (f)               => request('GET',  'api/tasks' + (f ? q(f) : '')),
  createTask:  (b)               => request('POST', 'api/tasks', b),
  getTask:     (id)              => request('GET',  'api/tasks/' + id),
  updateTask:  (id, b)           => request('PATCH','api/tasks/' + id, b),
  deleteTask:  (id)              => request('DELETE','api/tasks/' + id),
  addSubtask:  (id, title)       => request('POST', 'api/tasks/' + id + '/subtasks', { title }),
  addTaskTag:  (id, tag)         => request('POST', 'api/tasks/' + id + '/tags', { tag }),
  removeTaskTag: (id, tag)       => request('DELETE','api/tasks/' + id + '/tags/' + enc(tag)),
  linkNoteToTask: (tid, nid)     => request('POST', 'api/tasks/' + tid + '/notes/' + nid),

  listNotes:   (f)               => request('GET',  'api/notes' + (f ? q(f) : '')),
  createNote:  (b)               => request('POST', 'api/notes', b),
  getNote:     (id)              => request('GET',  'api/notes/' + id),
  updateNote:  (id, b)           => request('PATCH','api/notes/' + id, b),
  deleteNote:  (id)              => request('DELETE','api/notes/' + id),
  versions:    (id)              => request('GET',  'api/notes/' + id + '/versions'),
  restore:     (id, vid)         => request('POST', 'api/notes/' + id + '/restore/' + vid),
  addNoteTag:  (id, tag)         => request('POST', 'api/notes/' + id + '/tags', { tag }),
  linkTaskToNote: (nid, tid)     => request('POST', 'api/notes/' + nid + '/tasks/' + tid),

  share:   (resource_type, resource_id, username) => request('POST',   'api/share', { resource_type, resource_id, username }),
  unshare: (resource_type, resource_id, user_id)  => request('DELETE', 'api/share', { resource_type, resource_id, user_id }),
  shared:  ()                                      => request('GET',    'api/shared'),
};
