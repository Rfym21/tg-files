import { useRef, useState } from "react";
import { api, ApiClientError } from "../api/client";
import type { UploadResponse } from "../api/types";
import { UploadIcon } from "./Icons";

interface UploaderProps {
  bucket: string;
  prefix?: string;
  onUploaded: (res: UploadResponse) => void;
}

interface FileProgress {
  name: string;
  loaded: number;
  total: number;
  status: "uploading" | "done" | "error";
  error?: string;
}

export function Uploader({ bucket, prefix = "", onUploaded }: UploaderProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const [items, setItems] = useState<FileProgress[]>([]);
  const [error, setError] = useState<string | null>(null);

  const submitOne = async (file: File, index: number) => {
    const key = (prefix ? prefix.replace(/^\/|\/$/g, "") + "/" : "") + file.name;
    try {
      const res = await api.uploadFile(bucket, key, file, (loaded, total) => {
        setItems((prev) => {
          const next = prev.slice();
          next[index] = { ...next[index], loaded, total };
          return next;
        });
      });
      setItems((prev) => {
        const next = prev.slice();
        next[index] = { ...next[index], status: "done", loaded: file.size };
        return next;
      });
      onUploaded(res);
    } catch (err) {
      const message =
        err instanceof ApiClientError ? `${err.code}: ${err.message}` : "上传失败";
      setItems((prev) => {
        const next = prev.slice();
        next[index] = { ...next[index], status: "error", error: message };
        return next;
      });
    }
  };

  const submitMany = async (files: File[]) => {
    if (!bucket) {
      setError("请先选择 bucket");
      return;
    }
    setError(null);
    const start = items.length;
    const initial: FileProgress[] = files.map((f) => ({
      name: f.name,
      loaded: 0,
      total: f.size,
      status: "uploading",
    }));
    setItems((prev) => [...prev, ...initial]);
    for (let i = 0; i < files.length; i++) {
      await submitOne(files[i], start + i);
    }
  };

  const onSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const list = e.target.files;
    if (list && list.length > 0) {
      await submitMany(Array.from(list));
    }
    if (inputRef.current) inputRef.current.value = "";
  };

  const onDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    setDragging(false);
    const list = e.dataTransfer.files;
    if (list && list.length > 0) {
      await submitMany(Array.from(list));
    }
  };

  const clearCompleted = () => {
    setItems((prev) => prev.filter((it) => it.status === "uploading"));
  };

  return (
    <div>
      <div
        className={"uploader" + (dragging ? " is-dragging" : "")}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        onClick={() => inputRef.current?.click()}
      >
        <input
          ref={inputRef}
          type="file"
          multiple
          style={{ display: "none" }}
          onChange={onSelect}
        />
        <div className="uploader__icon">
          <UploadIcon />
        </div>
        <div className="uploader__title">拖拽文件到此处，或点击选择</div>
        <div className="uploader__hint">
          支持多文件批量上传，上传完成后会生成可分享的直链
        </div>
      </div>

      {error && <div className="banner-error" style={{ marginTop: 12 }}>{error}</div>}

      {items.length > 0 && (
        <div className="upload-list">
          <div className="upload-list__header">
            <span>上传队列 ({items.length})</span>
            <button className="button is-ghost" onClick={clearCompleted}>
              清除已完成
            </button>
          </div>
          {items.map((it, i) => {
            const pct = Math.round((it.loaded / Math.max(it.total, 1)) * 100);
            return (
              <div key={i} className="upload-row">
                <div className="upload-row__name truncate" title={it.name}>
                  {it.name}
                </div>
                <div className="upload-row__bar">
                  <div
                    className={
                      "upload-row__bar-fill" +
                      (it.status === "error" ? " is-error" : "") +
                      (it.status === "done" ? " is-done" : "")
                    }
                    style={{ width: `${it.status === "error" ? 100 : pct}%` }}
                  />
                </div>
                <div className="upload-row__status">
                  {it.status === "uploading" && <span className="chip">{pct}%</span>}
                  {it.status === "done" && <span className="chip is-success">已完成</span>}
                  {it.status === "error" && (
                    <span className="chip is-danger" title={it.error}>失败</span>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
