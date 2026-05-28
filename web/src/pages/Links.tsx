import { useEffect, useMemo, useState } from "react";
import { api, ApiClientError } from "../api/client";
import type { DirectLink } from "../api/types";
import { formatDate } from "../lib/formatDate";
import { CopyIcon, LinkIcon, RefreshIcon, TrashIcon } from "../components/Icons";

export function Links() {
  const [links, setLinks] = useState<DirectLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.listLinks();
      setLinks(res.links);
    } catch (err) {
      if (err instanceof ApiClientError) setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  const onCopy = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url);
    } catch {
      // ignore
    }
  };

  const onRevoke = async (token: string) => {
    if (!confirm("撤销后该直链将立即失效，确认？")) return;
    try {
      await api.revokeLink(token);
      refresh();
    } catch (err) {
      if (err instanceof ApiClientError) setError(err.message);
    }
  };

  const totalClicks = useMemo(
    () => links.reduce((acc, l) => acc + l.click_count, 0),
    [links],
  );
  const activeCount = useMemo(
    () => links.filter((l) => !l.revoked).length,
    [links],
  );

  return (
    <>
      <header className="page-header">
        <div>
          <h1 className="page-header__title">直链</h1>
          <p className="page-header__subtitle">管理已生成的对外分享链接</p>
        </div>
        <div className="page-header__actions">
          <button className="button is-secondary" onClick={refresh}>
            <RefreshIcon size={16} />
            刷新
          </button>
        </div>
      </header>

      <div className="stat-grid">
        <div className="stat-card">
          <span className="stat-card__icon">
            <LinkIcon size={20} />
          </span>
          <div>
            <p className="stat-card__label">直链总数</p>
            <p className="stat-card__value">{links.length}</p>
          </div>
        </div>
        <div className="stat-card">
          <span className="stat-card__icon is-green">
            <LinkIcon size={20} />
          </span>
          <div>
            <p className="stat-card__label">生效中</p>
            <p className="stat-card__value">{activeCount}</p>
          </div>
        </div>
        <div className="stat-card">
          <span className="stat-card__icon is-amber">
            <CopyIcon size={20} />
          </span>
          <div>
            <p className="stat-card__label">累计访问</p>
            <p className="stat-card__value">{totalClicks}</p>
          </div>
        </div>
      </div>

      {error && <div className="banner-error">{error}</div>}

      <div className="surface surface--flush">
        <div className="surface__header">
          <h2 className="surface__title">
            全部直链
            <span className="surface__count">{links.length}</span>
          </h2>
        </div>
        <table className="table">
          <thead>
            <tr>
              <th>Bucket / Key</th>
              <th>URL</th>
              <th style={{ width: 160 }}>创建时间</th>
              <th style={{ width: 160 }}>过期时间</th>
              <th style={{ width: 80 }}>点击</th>
              <th style={{ width: 120 }}></th>
            </tr>
          </thead>
          <tbody>
            {links.map((link) => (
              <tr key={link.token}>
                <td>
                  <div className="col-stack">
                    <span className="col-stack__primary truncate" style={{ maxWidth: 280 }}>
                      {link.bucket} / {link.key}
                    </span>
                    {link.revoked && (
                      <span className="chip is-danger" style={{ marginTop: 4, width: "fit-content" }}>
                        已撤销
                      </span>
                    )}
                  </div>
                </td>
                <td>
                  <span className="mono truncate" style={{ display: "inline-block", maxWidth: 360 }}>
                    {link.url}
                  </span>
                </td>
                <td className="muted">{formatDate(link.created_at)}</td>
                <td className="muted">
                  {link.expires_at ? formatDate(link.expires_at) : (
                    <span className="chip">永久</span>
                  )}
                </td>
                <td>{link.click_count}</td>
                <td>
                  <div className="row-actions">
                    <button
                      className="button is-ghost is-icon"
                      title="复制 URL"
                      onClick={() => onCopy(link.url)}
                    >
                      <CopyIcon size={16} />
                    </button>
                    {!link.revoked && (
                      <button
                        className="button is-danger-ghost is-icon"
                        title="撤销"
                        onClick={() => onRevoke(link.token)}
                      >
                        <TrashIcon size={16} />
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
            {!loading && links.length === 0 && (
              <tr>
                <td colSpan={6} className="table__empty">暂无直链</td>
              </tr>
            )}
            {loading && (
              <tr>
                <td colSpan={6} className="table__empty">加载中…</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}
