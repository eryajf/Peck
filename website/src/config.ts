import { deeplinks } from "./data/deeplinks"

// Product copy and navigation live here. Deeplink entries live in
// src/data/deeplinks.ts so community additions have one small, reviewable file.
export const config = {
  name: "Peck",
  username: "peck",
  title: "macOS 应用入口",
  email: "",
  bio: "让值得信任的 Mac 应用，一键抵达。",
  currentWork: { label: "Deeplink 目录", url: "/deeplinks" },
  siteUrl: "",
  siteTitle: "Peck — 为 Mac 应用啄掉打开障碍",
  siteDescription:
    "Peck 是一款开源 macOS 工具，帮助你检查并处理信任的应用遇到的隔离和签名问题。",
  profileImage: "/images/peck-mark-color.svg",
  social: {
    github: "https://github.com/eryajf/Peck",
    linkedin: "",
    twitter: "",
    instagram: "",
  },
  categories: [
    { id: "peck", label: "Peck Deeplinks", icon: "code" as const },
  ],
  sidebarLocations: [
    { label: "Deeplinks", href: "/deeplinks", icon: "globe" as const },
    { label: "代码仓库", href: "https://github.com/eryajf/Peck", icon: "github" as const },
  ],
  projects: deeplinks.map((link) => ({
    name: link.name,
    description: link.description,
    category: "peck",
    icon: { type: "image" as const, src: link.icon },
    action: "navigate" as const,
    target: "/deeplinks",
  })),
  articles: [] as {
    slug: string
    title: string
    meta: string
    excerpt: string
    tags: string[]
  }[],
  terminal: {
    hostname: "peck",
    greeting: "Last login: Peck desktop · macOS",
    name: "Peck",
    tagline: "macOS 应用 · Deeplink · 开源目录",
    aboutLines: [
      "把值得信任的 Mac 应用收进一个轻巧的入口。",
      "点击 Dock 里的 Deeplinks，查看当前可用的应用链接。",
    ],
  },
}

export type CategoryId = "all" | (typeof config.categories)[number]["id"]
export type SidebarIconType = (typeof config.categories)[number]["icon"]
export type LocationIconType = (typeof config.sidebarLocations)[number]["icon"]
