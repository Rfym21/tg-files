import { useRef, useState } from "react";
import { api, ApiClientError } from "../api/client";
import type { UploadResponse } from "../api/types";
import { UploadIcon } from "./Icons";

interface UploaderProps {
  bucket: string;
  prefix?: string;
  onUploaded: (res: UploadResponse) => void;
  onBatchComplete?: () => void | Promise<void>;
}

interface FileProgress {
  id: string;
  name: string;
  loaded: number;
  total: number;
  status: "uploading" | "done" | "error";
  error?: string;
}

const uploadConcurrency = 3;

function createUploadId() {
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

export function Uploader({
  bucket,
  prefix = "",
  onUploaded,
  onBatchComplete,
}: UploaderProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const [items, setItems] = useState<FileProgress[]>([]);
  const [error, setError] = useState<string | null>(null);

  const updateItem = (id: string, updater: (item: FileProgress) => FileProgress) => {
    setItems((prev) => prev.map((item) => (item.id === id ? updater(item) : item)));
  };

  const submitOne = async (file: File, id: string) => {
    const key = (prefix ? prefix.replace(/^\/|\/$/g, "") + "/" : "") + file.name;
    try {
      // 初始化进度为 0
      updateItem(id, (item) => ({ ...item, loaded: 0, total: file.size }));

      const res = await api.uploadFile(bucket, key, file, (loaded, total) => {
        // 确保进度更新
        updateItem(id, (item) => ({ ...item, loaded, total }));
      });

      // 上传完成，确保进度显示为 100%
      updateItem(id, (item) => ({ ...item, status: "done", loaded: file.size, total: file.size }));
      onUploaded(res);
    } catch (err) {
      const message =
        err instanceof ApiClientError ? `${err.code}: ${err.message}` : "上传失败";
      updateItem(id, (item) => ({ ...item, status: "error", error: message }));
    }
  };

  const submitMany = async (files: File[]) => {
    if (!bucket) {
      setError("请先选择 bucket");
      return;
    }
    setError(null);
    const queued = files.map((file) => ({
      id: createUploadId(),
      file,
    }));
    const initial: FileProgress[] = queued.map(({ id, file }) => ({
      id,
      name: file.name,
      loaded: 0,
      total: file.size,
      status: "uploading" as const,
    }));
    setItems((prev) => [...prev, ...initial]);

    let nextIndex = 0;
    const workerCount = Math.min(uploadConcurrency, queued.length);
    const worker = async () => {
      while (true) {
        const current = nextIndex;
        nextIndex += 1;
        if (current >= queued.length) {
          return;
        }
        await submitOne(queued[current].file, queued[current].id);
      }
    };

    try {
      await Promise.all(Array.from({ length: workerCount }, () => worker()));
    } finally {
      await onBatchComplete?.();
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
          {items.map((it) => {
            const pct = Math.round((it.loaded / Math.max(it.total, 1)) * 100);
            return (
              <div key={it.id} className="upload-row">
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
