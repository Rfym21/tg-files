# TgNAS

`tgnas` 是一个以 Telegram 作为存储后端、以本地 SQLite 保存元数据的网关，兼容 S3 协议并支持 WebDAV。

## 服务模式

默认情况下，`tgnas` 会读取 `data/config.yaml` 并启动一个同时启用两种协议的 HTTP 服务：

```bash
tgnas
```

S3 API 通过常规的 bucket 路径提供服务。WebDAV 在配置的 WebDAV 前缀下提供服务，默认为 `/dav/`。

如果只需要其中一种协议接口，可以使用单协议模式：

```bash
tgnas -c config.yaml s3
tgnas -c config.yaml dav
```

`-c` 是 `-config` 的简写别名。在同一次调用中同时传入 `-config` 和 `-c` 属于使用错误。`-debug` 是全局参数，必须放在所有子命令之前。

## 配置

默认配置文件路径为 `data/config.yaml`。

常用环境变量：

- `TGNAS_LISTEN`：当 `server.listen_env` 被配置时，会覆盖 `server.listen`。默认值是 `:9000`。
- `TGNAS_SECRET_KEY`：示例中使用的 S3 / WebDAV 凭据密钥。
- `TGNAS_TELEGRAM_BOT_TOKEN`：默认 Docker 配置中的 Telegram bot token。
- `TGNAS_TELEGRAM_CHAT_ID`：默认 bucket 引用的 chat ID。
- `TGNAS_SQLITE_PATH`：可覆盖元数据 SQLite 文件路径。

可以为 bucket 启用匿名 S3 对象读取（public read）：

```yaml
buckets:
  public-files:
    chat_id: "${TGNAS_TELEGRAM_CHAT_ID}"
    public_read: true
```

`public_read` 默认是 `false`。启用后，匿名 S3 客户端在已经知道对象 key 的前提下，仅可对该 bucket 中的对象执行 `GET` 和 `HEAD`。Bucket 列表、根 bucket 列表、写入、删除以及 WebDAV 仍需认证。

反向代理（例如 cloudflared、nginx）的可信代理配置：

```yaml
server:
  trusted_proxies:
    - "127.0.0.1/32"
    - "172.16.0.0/12"
  trusted_proxy_hosts:
    - "s3.example.com"
```

当请求的远端 IP 命中 `trusted_proxies` 中的某个 CIDR 范围时，`X-Forwarded-Host`（或 `Forwarded: host=`）会被信任并应用到请求上，不论 host 值是什么。当转发过来的 host 命中 `trusted_proxy_hosts` 中的某一项（不区分大小写）时，请求也会被信任，不论远端 IP。两者满足其一即可。请求被信任时，`X-Forwarded-Proto`（或 `Forwarded: proto=`）也会被应用。未被信任时，转发头会被忽略。

警告：`trusted_proxy_hosts` 信任的是转发的 host 值本身，而该值在没有真实代理剥离和重写的情况下是受客户端控制的。使用此选项时请屏蔽对 TgNAS 的直接非信任访问。在可能的情况下优先使用 `trusted_proxies` 的 CIDR 范围。

WebDAV 配置：

```yaml
webdav:
  # prefix: "/dav/"
```

前缀必须以 `/` 开头，会被规范化为以 `/` 结尾，不能为 `/`，并且不能与任何已配置 bucket 的第一段路径冲突。

## Docker

使用 `data/config.yaml` 中默认的 Docker 配置运行：

```bash
mkdir -p data
wget -P data https://github.com/aahl/tgnas/raw/refs/heads/dev/data/config.yaml

docker run --rm -u root -v "$PWD/data:/app/data" ghcr.io/aahl/tgnas chown -R app:app /app/data

docker run -d \
  --name tgnas \
  -p 9000:9000 \
  -v "$PWD/data:/app/data" \
  -e TGNAS_SECRET_KEY="your-s3-and-webdav-password" \
  -e TGNAS_TELEGRAM_CHAT_ID="-1001234567890" \
  -e TGNAS_TELEGRAM_BOT_TOKEN="123456:telegram-bot-token" \
  ghcr.io/aahl/tgnas
```

容器以非 root 用户 `app` 运行，工作目录是 `/app`。挂载的 `data` 目录必须对容器内的该用户可写，因为 SQLite 元数据默认存储在 `/app/data` 下。如果 SQLite 无法打开或创建 `metadata.sqlite`，请在重启容器前先修正宿主目录的属主或权限。

## Docker Compose

仓库内 `docker-compose.yml` 使用 GHCR 上发布的镜像，并将 `./data` 挂载到 `/app/data`：

```bash
cat << EOF > .env
TGNAS_PORT_EXPOSED=9000
TGNAS_SECRET_KEY="your-s3-and-webdav-password"
TGNAS_TELEGRAM_CHAT_ID="-1001234567890"
TGNAS_TELEGRAM_BOT_TOKEN="123456:telegram-bot-token"
EOF

docker compose run --rm -u root tgnas chown -R app:app /app/data
wget -P data https://github.com/aahl/tgnas/raw/refs/heads/dev/data/config.yaml

docker compose up -d
```

如果宿主机上的 `data` 目录属主是 root 或其他用户，请把写权限授予容器内 `app` 用户对应的 UID，或者使用其他权限方案（例如对 `./data` 使用可写的用户组）。不要把配置或 SQLite 目录设置为只读。

## 认证

S3 默认使用 SigV4 认证；例外是对配置了 `public_read: true` 的 bucket 进行匿名 `GET` 和 `HEAD` 对象请求。

S3 对象 `GET` 和 `HEAD` 也可以使用 SigV4 查询字符串认证（即常说的预签名 URL）。预签名 URL 使用现有配置的凭据，`X-Amz-Expires` 最大允许 `604800` 秒（7 天）。预签名 URL 不授权 bucket 列表、根列表、写入、删除、复制操作以及 WebDAV 请求。

预签名对象 URL 与普通 S3 对象访问的 path-style 形式一致：

```text
https://s3.example.com/tgnas/test.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=...&X-Amz-Date=...&X-Amz-Expires=900&X-Amz-SignedHeaders=host&X-Amz-Signature=...
```

如果 TgNAS 部署在会修改源 host 的反向代理之后，请配置 `trusted_proxies` 或 `trusted_proxy_hosts`，确保验签时看到的是被签名的外部 host。

WebDAV 使用 HTTP Basic Auth，并复用 `auth.credentials`：

- 用户名：`access_key`
- 密码：`secret_key_env` 解析后的值

## 本地元数据 CLI

`tgnas` 还提供一些只读的本地列表命令，用于检查所配置的 SQLite 元数据数据库。这些命令不会启动 HTTP 服务，也不会与 Telegram 通信。

```text
tgnas [-debug] [-c|-config config.yaml] ls [-n|-limit N] bucket[/prefix]
tgnas [-debug] [-c|-config config.yaml] lsd [bucket[/prefix]]
tgnas [-debug] [-c|-config config.yaml] bucket rename [--dry-run] old-bucket new-bucket
```

`ls` 按行输出对象 key，默认 1000 条；`-limit N` 和 `-n N` 用于设置返回数量上限，`0` 表示不限制总数（但内部仍然分页读取）。

`lsd` 不带路径时打印已启用的 bucket 名称。`lsd bucket/prefix` 以 `/` 为分隔符，打印该前缀下的一级伪目录。

`bucket rename` 在 SQLite 元数据数据库中重命名 bucket。目标 bucket 名称必须在当前配置文件中存在，并且其 `chat_id` 与源 bucket 的元数据一致。`--dry-run` 只打印会发生的改动而不实际修改数据。如果源 bucket 仍然出现在配置文件中，会在 stderr 输出一条警告。

## WebDAV 行为

WebDAV 将对象前缀暴露为目录。`MKCOL /dav/photos/2026/` 会创建一个 key 为 `2026/`、大小为 0 字节的目录标记对象，从而保留空目录。

支持的常用操作包括 `OPTIONS`、`PROPFIND`、`GET`、`HEAD`、`PUT`、`DELETE`、`MKCOL`、`COPY` 和 `MOVE`。`LOCK` 和 `UNLOCK` 会返回 not implemented，且 `OPTIONS` 不会声明锁支持。

同一 bucket 内的 `COPY` 和 `MOVE` 是仅元数据操作（包括递归目录拷贝/移动），它们会复用已有的 Telegram 文件/分块元数据，而不会下载并重新上传内容。

Bucket 仍然只能通过配置创建。如果某个 bucket 从配置中移除后仍残留在元数据里，会被视为孤儿：常规对象访问会被禁止，但可以通过 `DELETE /dav/{bucket}` 或 S3 `DELETE /{bucket}` 来清理本地元数据记录及关联的对象/分块元数据。

Bucket 的 `chat_id` 可以是字面量 Telegram chat ID，也可以是完整的环境变量引用，例如 `chat_id: "${TGNAS_PRIVATE_CHAT_ID}"`。不支持部分插值。如果引用的环境变量未设置或为空，解析得到的 `chat_id` 即为空，配置校验会失败。

## 相关链接
- https://deepwiki.com/aahl/tgnas
- https://zread.ai/aahl/tgnas
