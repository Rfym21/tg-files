import { FormEvent, useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { api, ApiClientError } from "../api/client";
import { CloudIcon } from "../components/Icons";

export function Login() {
  const navigate = useNavigate();
  const location = useLocation();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api
      .me()
      .then(() => {
        if (!cancelled) navigate("/files", { replace: true });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [navigate]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api.login({ username, password });
      const from = (location.state as { from?: string } | null)?.from ?? "/files";
      navigate(from, { replace: true });
    } catch (err) {
      if (err instanceof ApiClientError) {
        setError(err.message);
      } else {
        setError("登录失败");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="login-shell">
      <form className="login-card" onSubmit={submit}>
        <div className="login-brand">
          <span className="login-brand__mark">
            <CloudIcon size={24} />
          </span>
          <div style={{ textAlign: "center" }}>
            <h1 className="login-title">TgNAS</h1>
            <p className="login-subtitle">使用管理员账号登录</p>
          </div>
        </div>
        <div className="login-form">
          <div className="form-field">
            <label htmlFor="username">用户名</label>
            <input
              id="username"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </div>
          <div className="form-field">
            <label htmlFor="password">密码</label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          {error && <div className="banner-error" style={{ margin: 0 }}>{error}</div>}
          <button className="button is-primary is-block" disabled={submitting}>
            {submitting ? "登录中…" : "登录"}
          </button>
        </div>
      </form>
    </div>
  );
}
