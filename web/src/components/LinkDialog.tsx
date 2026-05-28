import { useEffect, useRef, useState } from "react";
import type { DirectLink } from "../api/types";

interface LinkDialogProps {
  link: DirectLink | null;
  onClose: () => void;
}

export function LinkDialog({ link, onClose }: LinkDialogProps) {
  const [copied, setCopied] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setCopied(false);
    if (link && inputRef.current) {
      inputRef.current.select();
    }
  }, [link]);

  if (!link) return null;

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(link.url);
      setCopied(true);
    } catch {
      inputRef.current?.select();
      document.execCommand("copy");
      setCopied(true);
    }
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-title">直链已生成</div>
        <div className="muted" style={{ fontSize: 12 }}>
          {link.bucket} / {link.key}
        </div>
        <div className="link-url-row">
          <input ref={inputRef} readOnly value={link.url} />
          <button className="button" onClick={copy}>
            {copied ? "已复制" : "复制"}
          </button>
        </div>
        {link.expires_at && (
          <div className="muted" style={{ marginTop: 8, fontSize: 12 }}>
            过期时间：{new Date(link.expires_at).toLocaleString()}
          </div>
        )}
        <div style={{ marginTop: 16, textAlign: "right" }}>
          <button className="button is-secondary" onClick={onClose}>
            关闭
          </button>
        </div>
      </div>
    </div>
  );
}
