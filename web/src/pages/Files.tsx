import { useCallback, useEffect, useMemo, useState } from "react";
import { Uploader } from "../components/Uploader";
import { LinkDialog } from "../components/LinkDialog";
import { api, ApiClientError } from "../api/client";
import type { DirectLink, ObjectInfo, UploadResponse } from "../api/types";
import { formatBytes } from "../lib/formatBytes";
import { formatDate } from "../lib/formatDate";
import {
  ArchiveIcon,
  CopyIcon,
  FileIcon,
  FolderIcon,
  ImageIcon,
  LinkIcon,
  RefreshIcon,
  TrashIcon,
  VideoIcon,
} from "../components/Icons";

function fileIconFor(name: string, contentType: string) {
  const lower = name.toLowerCase();
  if (contentType.startsWith("image/")) return <ImageIcon />;
  if (contentType.startsWith("video/")) return <VideoIcon />;
  if (/\.(zip|tar|gz|7z|rar)$/i.test(lower)) return <ArchiveIcon />;
  return <FileIcon />;
}

function shortType(contentType: string) {
  if (!contentType) return "-";
  const slash = contentType.indexOf("/");
  return slash >= 0 ? contentType.slice(slash + 1).toUpperCase() : contentType.toUpperCase();
}

export function Files() {
  const [bucket, setBucket] = useState<string>("");
  const [objects, setObjects] = useState<ObjectInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeLink, setActiveLink] = useState<DirectLink | null>(null);

  const refresh = useCallback(async (target?: string) => {
    const name = target ?? bucket;
    if (!name) return;
    setLoading(true);
    setError(null);
    try {
      const res = await api.listFiles(name);
      setObjects(res.objects);
    } catch (err) {
      if (err instanceof ApiClientError) setError(err.message);
      else setError("加载失败");
    } finally {
      setLoading(false);
    }
  }, [bucket]);

  useEffect(() => {
    api
      .listBuckets()
      .then((list) => {
        if (list.length > 0) {
          setBucket(list[0].name);
        }
      })
      .catch((err) => {
        if (err instanceof ApiClientError) setError(err.message);
      });
  }, []);

  useEffect(() => {
    if (bucket) refresh(bucket);
  }, [bucket, refresh]);

  const onUploaded = (res: UploadResponse) => {
    refresh();
    if (res.link) setActiveLink(res.link);
  };

  const onDelete = async (key: string) => {
    if (!confirm(`确认删除 ${key}？`)) return;
    try {
      await api.deleteFile(bucket, key);
      refresh();
    } catch (err) {
      if (err instanceof ApiClientError) setError(err.message);
    }
  };

  const onCopyLink = async (key: string) => {
    try {
      const link = await api.createLink({ bucket, key });
      setActiveLink(link);
    } catch (err) {
      if (err instanceof ApiClientError) setError(err.message);
    }
  };

  const totalSize = useMemo(
    () => objects.reduce((acc, o) => acc + o.size, 0),
    [objects],
  );

  return (
    <>
      <header className="page-header">
        <div>
          <h1 className="page-header__title">文件</h1>
          <p className="page-header__subtitle">{objects.length} 个对象</p>
        </div>
        <div className="page-header__actions">
          <button className="button is-secondary" onClick={() => refresh()}>
            <RefreshIcon size={16} />
            刷新
          </button>
        </div>
      </header>

      <div className="stat-grid">
        <div className="stat-card">
          <span className="stat-card__icon is-amber">
            <FolderIcon size={20} />
          </span>
          <div>
            <p className="stat-card__label">文件数量</p>
            <p className="stat-card__value">{objects.length}</p>
          </div>
        </div>
        <div className="stat-card">
          <span className="stat-card__icon is-green">
            <FileIcon size={20} />
          </span>
          <div>
            <p className="stat-card__label">总占用</p>
            <p className="stat-card__value">{formatBytes(totalSize)}</p>
          </div>
        </div>
      </div>

      {error && <div className="banner-error">{error}</div>}

      <div className="surface" style={{ marginBottom: "var(--space-5)" }}>
        <Uploader bucket={bucket} onUploaded={onUploaded} />
      </div>

      <div className="surface surface--flush">
        <div className="surface__header">
          <h2 className="surface__title">
            文件列表
            <span className="surface__count">{objects.length}</span>
          </h2>
        </div>
        <table className="table">
          <thead>
            <tr>
              <th>名称</th>
              <th style={{ width: 120 }}>类型</th>
              <th style={{ width: 120 }}>大小</th>
              <th style={{ width: 180 }}>修改时间</th>
              <th style={{ width: 140 }}></th>
            </tr>
          </thead>
          <tbody>
            {objects.map((obj) => (
              <tr key={obj.key}>
                <td>
                  <div className="cell-with-icon">
                    <span className="cell-with-icon__icon">
                      {fileIconFor(obj.key, obj.content_type)}
                    </span>
                    <div style={{ minWidth: 0 }}>
                      <div className="cell-with-icon__primary" title={obj.key}>
                        {obj.key}
                      </div>
                      {obj.etag && (
                        <div className="cell-with-icon__secondary mono truncate">
                          {obj.etag.replace(/^"|"$/g, "").slice(0, 16)}
                        </div>
                      )}
                    </div>
                  </div>
                </td>
                <td><span className="chip">{shortType(obj.content_type)}</span></td>
                <td className="muted">{formatBytes(obj.size)}</td>
                <td className="muted">{formatDate(obj.last_modified)}</td>
                <td>
                  <div className="row-actions">
                    <button
                      className="button is-ghost is-icon"
                      title="生成直链"
                      onClick={() => onCopyLink(obj.key)}
                    >
                      <LinkIcon size={16} />
                    </button>
                    <button
                      className="button is-ghost is-icon"
                      title="复制 key"
                      onClick={() => navigator.clipboard.writeText(obj.key).catch(() => {})}
                    >
                      <CopyIcon size={16} />
                    </button>
                    <button
                      className="button is-danger-ghost is-icon"
                      title="删除"
                      onClick={() => onDelete(obj.key)}
                    >
                      <TrashIcon size={16} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {!loading && objects.length === 0 && (
              <tr>
                <td colSpan={5} className="table__empty">暂无文件</td>
              </tr>
            )}
            {loading && (
              <tr>
                <td colSpan={5} className="table__empty">加载中…</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <LinkDialog link={activeLink} onClose={() => setActiveLink(null)} />
    </>
  );
}
