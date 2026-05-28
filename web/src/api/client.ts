import type {
  ApiError,
  BucketInfo,
  CreateLinkRequest,
  DirectLink,
  ListFilesResponse,
  ListLinksResponse,
  LoginRequest,
  LoginResponse,
  MeResponse,
  UploadResponse,
} from "./types";

class ApiClientError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(
  input: RequestInfo,
  init: RequestInit = {},
): Promise<T> {
  const response = await fetch(input, {
    credentials: "same-origin",
    ...init,
  });
  if (response.status === 204) {
    return undefined as T;
  }
  const text = await response.text();
  const data = text ? safeJSON(text) : undefined;
  if (!response.ok) {
    const apiErr = (data as ApiError | undefined)?.error;
    throw new ApiClientError(
      response.status,
      apiErr?.code ?? "http_error",
      apiErr?.message ?? response.statusText,
    );
  }
  return data as T;
}

function safeJSON(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}

function jsonInit(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

export const api = {
  login: (req: LoginRequest) =>
    request<LoginResponse>("/api/v1/auth/login", jsonInit("POST", req)),

  logout: () => request<void>("/api/v1/auth/logout", { method: "POST" }),

  me: () => request<MeResponse>("/api/v1/auth/me", { method: "GET" }),

  listBuckets: () =>
    request<BucketInfo[]>("/api/v1/buckets", { method: "GET" }),

  listFiles: (bucket: string, prefix = "", after = "") => {
    const params = new URLSearchParams({ bucket });
    if (prefix) params.set("prefix", prefix);
    if (after) params.set("after", after);
    return request<ListFilesResponse>(`/api/v1/files?${params.toString()}`, {
      method: "GET",
    });
  },

  deleteFile: (bucket: string, key: string) => {
    const params = new URLSearchParams({ bucket, key });
    return request<void>(`/api/v1/files?${params.toString()}`, {
      method: "DELETE",
    });
  },

  uploadFile: (
    bucket: string,
    key: string,
    file: File,
    onProgress?: (loaded: number, total: number) => void,
    createLink = true,
  ): Promise<UploadResponse> => {
    const params = new URLSearchParams({ bucket, key });
    if (createLink) params.set("create_link", "1");
    const url = `/api/v1/files?${params.toString()}`;
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", url, true);
      xhr.withCredentials = true;
      xhr.setRequestHeader(
        "Content-Type",
        file.type || "application/octet-stream",
      );
      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable && onProgress) {
          onProgress(event.loaded, event.total);
        }
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText));
          } catch (err) {
            reject(err);
          }
        } else {
          let code = "http_error";
          let message = xhr.statusText;
          try {
            const body = JSON.parse(xhr.responseText) as ApiError;
            code = body.error?.code ?? code;
            message = body.error?.message ?? message;
          } catch {
            // ignore
          }
          reject(new ApiClientError(xhr.status, code, message));
        }
      };
      xhr.onerror = () =>
        reject(new ApiClientError(0, "network_error", "network error"));
      xhr.send(file);
    });
  },

  createLink: (req: CreateLinkRequest) =>
    request<DirectLink>("/api/v1/links", jsonInit("POST", req)),

  listLinks: (bucket = "", after = "") => {
    const params = new URLSearchParams();
    if (bucket) params.set("bucket", bucket);
    if (after) params.set("after", after);
    const q = params.toString();
    return request<ListLinksResponse>(
      `/api/v1/links${q ? "?" + q : ""}`,
      { method: "GET" },
    );
  },

  revokeLink: (token: string) =>
    request<void>(`/api/v1/links/${encodeURIComponent(token)}`, {
      method: "DELETE",
    }),
};

export { ApiClientError };
