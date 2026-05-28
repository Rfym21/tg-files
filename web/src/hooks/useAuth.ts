import { useEffect, useState } from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { api, ApiClientError } from "../api/client";

interface AuthState {
  user: string | null;
  loading: boolean;
}

export function useAuth(redirectIfMissing = true): AuthState {
  const [state, setState] = useState<AuthState>({ user: null, loading: true });
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    let mounted = true;
    api
      .me()
      .then((res) => {
        if (!mounted) return;
        setState({ user: res.user, loading: false });
      })
      .catch((err) => {
        if (!mounted) return;
        setState({ user: null, loading: false });
        if (redirectIfMissing && err instanceof ApiClientError && err.status === 401) {
          navigate("/login", { replace: true, state: { from: location.pathname } });
        }
      });
    return () => {
      mounted = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return state;
}
