import { writable } from 'svelte/store';

// Token session disimpan di localStorage
const initialToken = localStorage.getItem('garuda_token') || '';
export const token = writable(initialToken);
export const currentUser = writable(null);
export const currentRoute = writable(window.location.hash.slice(1) || '/');

token.subscribe((val) => {
  if (val) {
    localStorage.setItem('garuda_token', val);
  } else {
    localStorage.removeItem('garuda_token');
  }
});

// Update currentRoute saat hashchange
window.addEventListener('hashchange', () => {
  currentRoute.set(window.location.hash.slice(1) || '/');
});

export function navigate(path) {
  window.location.hash = path;
}

// Global API helper
export async function apiFetch(endpoint, options = {}) {
  let authToken = '';
  token.subscribe((t) => (authToken = t))();

  const headers = {
    ...(options.headers || {}),
  };

  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`;
  }

  // default Content-Type json if body is string / object and not FormData
  if (options.body && !(options.body instanceof FormData) && !(options.body instanceof Blob)) {
    if (typeof options.body === 'object') {
      options.body = JSON.stringify(options.body);
    }
    if (!headers['Content-Type']) {
      headers['Content-Type'] = 'application/json';
    }
  }

  const res = await fetch(endpoint, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    token.set('');
    currentUser.set(null);
    navigate('/login');
    throw new Error('Unauthorized');
  }

  return res;
}
