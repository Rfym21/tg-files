import { useEffect, useState } from "react";
import { api, ApiClientError } from "../api/client";
import type { DirectLink } from "../api/types";
import { formatDate } from "../lib/formatDate";

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

  return (
    <>
      <h2 className="section-title">直链</h2>
      <div className="toolbar">
        <button className="button is-secondary" onClick={refresh}>刷新</button>
      </div>
      {error && <div className="banner-error" style={{ marginBottom: 12 }}>{error}</div>}
      <div className="surface" style={{ padding: 0 }}>
        <table className="table">
          <thead>
            <tr>
              <th>Bucket / Key</th>
              <th>URL</th>
              <th style={{ width: 140 }}>创建时间</th>
              <th style={{ width: 140 }}>过期时间</th>
              <th style={{ width: 70 }}>点击</th>
              <th style={{ width: 140 }}></th>
            </tr>
          </thead>
          <tbody>
            {links.map((link) => (
              <tr key={link.token}>
                <td>
                  <div>{link.bucket} / {link.key}</div>
                  {link.revoked && <div className="muted">（已撤销）</div>}
                </td>
                <td style={{ fontFamily: "ui-monospace,monospace", fontSize: 12 }}>{link.url}</td>
                <td className="muted">{formatDate(link.created_at)}</td>
                <td className="muted">{link.expires_at ? formatDate(link.expires_at) : "永久"}</td>
                <td>{link.click_count}</td>
                <td className="row-actions">
                  <button className="button is-link" onClick={() => onCopy(link.url)}>复制</button>
                  {!link.revoked && (
                    <button className="button is-link" onClick={() => onRevoke(link.token)}>撤销</button>
                  )}
                </td>
              </tr>
            ))}
            {!loading && links.length === 0 && (
              <tr>
                <td colSpan={6} style={{ textAlign: "center", padding: 24 }} className="muted">
                  暂无直链
                </td>
              </tr>
            )}
            {loading && (
              <tr>
                <td colSpan={6} style={{ textAlign: "center", padding: 24 }} className="muted">
                  加载中…
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}
