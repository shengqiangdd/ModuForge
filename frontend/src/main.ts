import { mount } from 'svelte';
import App from './App.svelte';
import 'uno.css';
import './app.css';

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    // ?v= busts the SW registration URL so a stale in-browser SW is always
    // replaced by the newest sw.js (which clears old caches on activate).
    navigator.serviceWorker.register('/sw.js?v=15');
  });
}

const app = mount(App, { target: document.getElementById('app')! });
export default app;
