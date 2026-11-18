import { createContext, useContext } from 'react';
import type { User } from './types';

export interface AuthState {
  user: User | null;
  loading: boolean;
  /** True when the user was signed in but the server rejected the session. */
  expired: boolean;
  signOut: () => Promise<void>;
}

export const AuthContext = createContext<AuthState>({
  user: null,
  loading: true,
  expired: false,
  signOut: () => Promise.resolve(),
});

/** Builds the URL starting the Google sign-in flow. */
export function signInUrl(next = '/'): string {
  return `/auth/login?next=${encodeURIComponent(next)}`;
}

export function useAuth() {
  return useContext(AuthContext);
}
