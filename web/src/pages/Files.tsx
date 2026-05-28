import { useCallback, useEffect, useState } from "react";
import { Uploader } from "../components/Uploader";
import { LinkDialog } from "../components/LinkDialog";
import { api, ApiClientError } from "../api/client";
import type { BucketInfo, DirectLink, ObjectInfo, UploadResponse } from "../api/types";
import { formatBytes } from "../lib/formatBytes";
import { formatDate } from "../lib/formatDate";

export function Files() {
  const [buckets, setBuckets] = useState<BucketInfo[]>([]);
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
        setBuckets(list);
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

  return (
    <>
      <h2 className="section-title">文件</h2>
      <div className="toolbar">
        <label className="muted">Bucket</label>
        <select
          className="bucket-select"
          value={bucket}
          onChange={(e) => setBucket(e.target.value)}
        >
          {buckets.map((b) => (
            <option key={b.name} value={b.name}>
              {b.name}
            </option>
          ))}
        </select>
        <button className="button is-secondary" onClick={() => refresh()}>
          刷新
        </button>
      </div>

      {error && <div className="banner-error" style={{ marginBottom: 12 }}>{error}</div>}

      <div className="surface" style={{ padding: 12 }}>
        <Uploader bucket={bucket} onUploaded={onUploaded} />
      </div>

      <div className="surface" style={{ padding: 0 }}>
        <table className="table">
          <thead>
            <tr>
              <th>名称</th>
              <th style={{ width: 100 }}>大小</th>
              <th style={{ width: 160 }}>类型</th>
              <th style={{ width: 160 }}>修改时间</th>
              <th style={{ width: 200 }}></th>
            </tr>
          </thead>
          <tbody>
            {objects.map((obj) => (
              <tr key={obj.key}>
                <td>{obj.key}</td>
                <td>{formatBytes(obj.size)}</td>
                <td className="muted">{obj.content_type || "-"}</td>
                <td className="muted">{formatDate(obj.last_modified)}</td>
                <td className="row-actions">
                  <button className="button is-link" onClick={() => onCopyLink(obj.key)}>
                    直链
                  </button>
                  <button className="button is-link" onClick={() => onDelete(obj.key)}>
                    删除
                  </button>
                </td>
              </tr>
            ))}
            {!loading && objects.length === 0 && (
              <tr>
                <td colSpan={5} style={{ textAlign: "center", padding: 24 }} className="muted">
                  暂无文件
                </td>
              </tr>
            )}
            {loading && (
              <tr>
                <td colSpan={5} style={{ textAlign: "center", padding: 24 }} className="muted">
                  加载中…
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <LinkDialog link={activeLink} onClose={() => setActiveLink(null)} />
    </>
  );
}
