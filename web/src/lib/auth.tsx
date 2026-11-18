import { useCallback, useEffect, useEffectEvent, useState, type ReactNode } from 'react';
import { useApi } from './api';
import { AuthContext } from './auth-context';
import type { User } from './types';

export function AuthProvider({ children }: { children: ReactNode }) {
  const api = useApi();
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [expired, setExpired] = useState(false);

  useEffect(() => {
    let alive = true;
    api
      .me()
      .then((u) => alive && setUser(u))
      .catch(() => alive && setUser(null))
      .finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [api]);

  const sessionRejected = useEffectEvent(() => {
    if (user) setExpired(true);
    setUser(null);
  });
  useEffect(() => api.onUnauthorized(() => sessionRejected()), [api]);

  const signOut = useCallback(async () => {
    await api.logout();
    setUser(null);
  }, [api]);

  return <AuthContext.Provider value={{ user, loading, expired, signOut }}>{children}</AuthContext.Provider>;
}
