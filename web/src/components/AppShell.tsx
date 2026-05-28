import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../hooks/useAuth";
import { api } from "../api/client";
import { CloudIcon, FolderIcon, LinkIcon, LogoutIcon } from "./Icons";

export function AppShell() {
  const { loading } = useAuth();
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
        <div className="app-shell__brand">
          <span className="app-shell__brand-mark">
            <CloudIcon size={18} />
          </span>
          <span>TgNAS</span>
        </div>

        <div className="app-shell__nav-label">导航</div>
        <div className="app-shell__nav-section">
          <NavLink
            to="/files"
            className={({ isActive }) =>
              "app-shell__nav-link" + (isActive ? " is-active" : "")
            }
          >
            <FolderIcon />
            <span>文件</span>
          </NavLink>
          <NavLink
            to="/links"
            className={({ isActive }) =>
              "app-shell__nav-link" + (isActive ? " is-active" : "")
            }
          >
            <LinkIcon />
            <span>直链</span>
          </NavLink>
        </div>

        <div className="app-shell__nav-footer">
          <button
            className="app-shell__nav-link"
            onClick={handleLogout}
            style={{ width: "100%", background: "transparent", border: "none", cursor: "pointer", textAlign: "left" }}
          >
            <LogoutIcon />
            <span>退出登录</span>
          </button>
        </div>
      </aside>
      <main className="app-shell__content">
        <Outlet />
      </main>
    </div>
  );
}
