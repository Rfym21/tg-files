import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../hooks/useAuth";
import { api } from "../api/client";

export function AppShell() {
  const { user, loading } = useAuth();
  const navigate = useNavigate();

  if (loading) {
    return <div style={{ padding: 24 }} className="muted">加载中…</div>;
  }

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch {
      // ignore
    }
    navigate("/login", { replace: true });
  };

  return (
    <div className="app-shell">
      <aside className="app-shell__nav">
        <div className="app-shell__brand">TgNAS</div>
        <NavLink
          to="/files"
          className={({ isActive }) =>
            "app-shell__nav-link" + (isActive ? " is-active" : "")
          }
        >
          文件
        </NavLink>
        <NavLink
          to="/links"
          className={({ isActive }) =>
            "app-shell__nav-link" + (isActive ? " is-active" : "")
          }
        >
          直链
        </NavLink>
        <div className="app-shell__user">
          <span>{user ?? "未登录"}</span>
          <button className="button is-link" onClick={handleLogout}>
            退出
          </button>
        </div>
      </aside>
      <main className="app-shell__content">
        <Outlet />
      </main>
    </div>
  );
}
