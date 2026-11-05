# Peck 官网

官网基于 Vite、React 和 TypeScript，是独立于 Go 应用的静态网站。`/deeplinks` 在构建时读取仓库根目录的 `data/project-links.json`，按项目生成 Peck 准备链接，并展示项目简介和仓库入口。更新数据无需手动修改官网代码；数据格式见 `data/project-links.schema.json`。数据中的状态信息应随项目发行情况核验更新，使用者仍需检查实际下载的应用。每个项目的 `category` 和 `category_en` 分别填写中英文分类名称；同一分类应使用一致的名称，页面会自动生成分类筛选。

## 本地运行

需要 Node.js 22：

```sh
cd website
npm ci
npm run dev
```

执行 `npm run lint` 检查代码，`npm run build` 生成 `website/dist/`。前端路由需要服务器将 `/deeplinks` 等路径回退至 `index.html`。

## 自动发布到服务器

推送到 `main` 时，只要 `website/`、`data/`、`assets/peck-icon-animated.svg` 或 `.github/workflows/website.yml` 发生变化，就自动安装依赖、检查、构建并部署。也可在 Actions 中手动运行 **Build and deploy website**（仅 `main`）。现有 GitHub Pages 链接页工作流独立运行。

在仓库 **Settings → Secrets and variables → Actions** 中配置以下 repository secrets：

| Secret | 内容 |
| --- | --- |
| `WEBSITE_SSH_HOST` | 服务器主机名或 IPv4 地址，不带协议或端口 |
| `WEBSITE_SSH_PORT` | SSH 端口，可不填，默认 `22` |
| `WEBSITE_SSH_USER` | SSH 登录用户名 |
| `WEBSITE_SSH_KEY` | 用于部署的私钥全文（含 BEGIN/END 行） |
| `WEBSITE_DEPLOY_PATH` | 服务器上的绝对部署目录，如 `/var/www/peck`；只允许字母、数字、`.`、`_`、`-` 和目录分隔符，不能包含 `.` 或 `..` 路径段 |

先在服务器上创建部署目录并授予该 SSH 用户写权限，在 `authorized_keys` 中配置对应的公钥。工作流在每次运行时自动接受首次连接的服务器主机公钥，因此首次连接不会验证服务器身份。部署使用 `rsync` 和 Linux 的 `mv -T`，服务器需要安装 rsync。每次构建上传到 `releases/<运行编号>-<尝试次数>`，完成后切换 `current` 符号链接；不会删除旧版本。`current` 不可预先是普通目录。服务器信息配置完成前，自动部署会失败并显示缺少配置。

Nginx 配置示例（替换域名和路径，并配置 HTTPS）：

```nginx
server {
    listen 80;
    server_name peck.example.com;
    root /var/www/peck/current;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

服务器需允许 Web 服务用户读取部署文件和遍历各级目录。旧版本可在确认不再需要后由管理员自行清理。

## 维护

- `src/pages/Home.tsx`、`src/pages/home.css`：官网首页。
- `src/pages/Deeplinks.tsx`、`src/data/deeplinks.ts`：项目目录和链接映射。
- `src/i18n.tsx`：中英文切换与语言偏好。
- `src/config.ts`：站点元信息；正式域名确定后设置 `siteUrl`。
- `src/components/macos/`、`src/components/peck/`：早期桌面交互原型。

早期桌面原型基于 [`nerdynikhil/macos-tahoe-portfolio`](https://github.com/nerdynikhil/macos-tahoe-portfolio)，其 MIT 许可保留在本目录的 `LICENSE`。
