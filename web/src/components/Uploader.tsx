import { useRef, useState } from "react";
import { api, ApiClientError } from "../api/client";
import type { UploadResponse } from "../api/types";

interface UploaderProps {
  bucket: string;
  prefix?: string;
  onUploaded: (res: UploadResponse) => void;
}

export function Uploader({ bucket, prefix = "", onUploaded }: UploaderProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const [progress, setProgress] = useState<{
    name: string;
    loaded: number;
    total: number;
  } | null>(null);
  const [error, setError] = useState<string | null>(null);

  const submit = async (file: File) => {
    if (!bucket) {
      setError("请先选择 bucket");
      return;
    }
    setError(null);
    const key = (prefix ? prefix.replace(/^\/|\/$/g, "") + "/" : "") + file.name;
    setProgress({ name: file.name, loaded: 0, total: file.size });
    try {
      const res = await api.uploadFile(bucket, key, file, (loaded, total) => {
        setProgress({ name: file.name, loaded, total });
      });
      onUploaded(res);
    } catch (err) {
      if (err instanceof ApiClientError) {
        setError(`${err.code}: ${err.message}`);
      } else {
        setError("上传失败");
      }
    } finally {
      setProgress(null);
    }
  };

  const onSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) await submit(file);
    if (inputRef.current) inputRef.current.value = "";
  };

  const onDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    setDragging(false);
    const file = e.dataTransfer.files?.[0];
    if (file) await submit(file);
  };

  return (
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
        style={{ display: "none" }}
        onChange={onSelect}
      />
      <div>拖拽文件到此处，或 <span style={{ color: "var(--color-accent)" }}>点击选择</span></div>
      <div className="uploader__hint">
        上传完成后会自动生成可分享的直链
      </div>
      {progress && (
        <div className="uploader__progress">
          {progress.name} —{" "}
          {Math.round((progress.loaded / Math.max(progress.total, 1)) * 100)}%
        </div>
      )}
      {error && (
        <div className="banner-error" style={{ marginTop: 8 }}>
          {error}
        </div>
      )}
    </div>
  );
}
