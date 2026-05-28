import { useEffect, useRef, useState } from "react";
import type { DirectLink } from "../api/types";
import { CloseIcon, CopyIcon } from "./Icons";

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
        <div className="modal-card__header">
          <div>
            <h3 className="modal-title">直链已生成</h3>
            <div className="muted" style={{ fontSize: "var(--font-size-sm)", marginTop: 4 }}>
              {link.bucket} / {link.key}
            </div>
          </div>
          <button className="modal-card__close" onClick={onClose} aria-label="关闭">
            <CloseIcon />
          </button>
        </div>
        <div className="link-url-row">
          <input ref={inputRef} readOnly value={link.url} />
          <button className="button is-primary" onClick={copy}>
            <CopyIcon size={14} />
            {copied ? "已复制" : "复制"}
          </button>
        </div>
        {link.expires_at && (
          <div className="muted" style={{ marginTop: 12, fontSize: "var(--font-size-sm)" }}>
            过期时间：{new Date(link.expires_at).toLocaleString()}
          </div>
        )}
      </div>
    </div>
  );
}
