import '@mantine/core/styles.css';
import '@mantine/dates/styles.css';
import '@mantine/notifications/styles.css';
import '@fontsource-variable/inter';
import './styles.css';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { createApi } from './lib/api';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App api={createApi()} />
  </StrictMode>,
);
