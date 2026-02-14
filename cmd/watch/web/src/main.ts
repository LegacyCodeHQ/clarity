import './app.css';
import App from './App.svelte';
import { mount } from 'svelte';

// Get page title from meta tag or default
const pageTitleMeta = document.querySelector('meta[name="page-title"]');
const pageTitle = pageTitleMeta?.getAttribute('content') || 'Clarity Watch';

const app = mount(App, {
  target: document.getElementById('app')!,
  props: {
    pageTitle,
  },
});

export default app;
