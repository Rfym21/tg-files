export interface ApiError {
  error: { code: string; message: string };
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  user: string;
  expires_at: number;
}

export interface MeResponse {
  user: string;
  expires_at: number;
}

export interface BucketInfo {
  name: string;
  public_read: boolean;
}

export interface ObjectInfo {
  key: string;
  size: number;
  content_type: string;
  etag: string;
  last_modified: string;
}

export interface ListFilesResponse {
  objects: ObjectInfo[];
  next_after?: string;
  is_truncated: boolean;
}

export interface DirectLink {
  token: string;
  url: string;
  bucket: string;
  key: string;
  filename?: string;
  created_at: string;
  expires_at?: string;
  click_count: number;
  revoked: boolean;
}

export interface UploadResponse {
  bucket: string;
  object: ObjectInfo;
  link?: DirectLink;
}

export interface CreateLinkRequest {
  bucket: string;
  key: string;
  ttl_seconds?: number;
  filename?: string;
}

export interface ListLinksResponse {
  links: DirectLink[];
  next_after?: string;
}
