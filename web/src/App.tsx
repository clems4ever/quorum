import { Center, Loader, MantineProvider } from '@mantine/core';
import { DatesProvider } from '@mantine/dates';
import { Notifications } from '@mantine/notifications';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { Layout } from './components/Layout';
import './i18n';
import { ApiContext, type Api } from './lib/api';
import { AuthProvider } from './lib/auth';
import { useAuth } from './lib/auth-context';
import { Dashboard } from './pages/Dashboard';
import { EventEditor } from './pages/EventEditor';
import { EventPage } from './pages/EventPage';
import { Landing } from './pages/Landing';
import { theme } from './theme';

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading, expired } = useAuth();
  const location = useLocation();
  if (loading) {
    return (
      <Center py={120}>
        <Loader />
      </Center>
    );
  }
  if (!user) {
    const next = location.pathname === '/' ? undefined : location.pathname + location.search;
    return <Landing next={next} expired={expired} />;
  }
  return <>{children}</>;
}

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route
          path="/"
          element={
            <RequireAuth>
              <Dashboard />
            </RequireAuth>
          }
        />
        <Route
          path="/new"
          element={
            <RequireAuth>
              <EventEditor />
            </RequireAuth>
          }
        />
        <Route
          path="/e/:id"
          element={
            <RequireAuth>
              <EventPage />
            </RequireAuth>
          }
        />
        <Route
          path="/e/:id/edit"
          element={
            <RequireAuth>
              <EventEditor />
            </RequireAuth>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  );
}

/** Gives Mantine date components the active language. */
function LocalizedDates({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation();
  return <DatesProvider settings={{ locale: i18n.resolvedLanguage, firstDayOfWeek: 1 }}>{children}</DatesProvider>;
}

export function Providers({ api, children }: { api: Api; children: ReactNode }) {
  return (
    <MantineProvider theme={theme} defaultColorScheme="auto">
      <Notifications position="top-right" />
      <LocalizedDates>
        <ApiContext.Provider value={api}>
          <AuthProvider>{children}</AuthProvider>
        </ApiContext.Provider>
      </LocalizedDates>
    </MantineProvider>
  );
}

export function App({ api }: { api: Api }) {
  return (
    <Providers api={api}>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </Providers>
  );
}
