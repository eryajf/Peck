import { useState } from "react"
import React from "react"
import { useNavigate } from "react-router-dom"
import { useDesktop } from "./DesktopContext"
import { config, type CategoryId } from "../../config"

// --- Types ---

type ViewMode = "grid" | "list"

interface PortfolioItem {
  name: string
  description: string
  category: CategoryId
  icon: string | { type: "image"; src: string }
  action: "navigate" | "external"
  target: string
}

// --- Build data from config ---

const FAVOURITES: { id: CategoryId; label: string }[] = config.categories.map(
  (cat) => ({ id: cat.id as CategoryId, label: cat.label })
)

const LOCATIONS = config.sidebarLocations

const ITEMS: PortfolioItem[] = config.projects.map((p) => ({
  name: p.name,
  description: p.description,
  category: p.category as CategoryId,
  icon: p.icon,
  action: p.action as "navigate" | "external",
  target: p.target,
}))

// --- Category -> kind label ---

const KIND_LABELS: Record<string, string> = { all: "Project" }
for (const cat of config.categories) {
  KIND_LABELS[cat.id] = cat.label
}

// --- Sidebar SVG icons ---

function IcoRecents({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <circle cx="8" cy="8" r="6" />
      <polyline points="8,4.5 8,8 10.5,10" />
    </svg>
  )
}

function IcoFolder({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 13" fill="currentColor" aria-hidden>
      <path d="M0 3.5A2.5 2.5 0 012.5 1h2.25c.464 0 .91.184 1.238.513L7.12 2.633A.5.5 0 007.474 2.8H13.5A2.5 2.5 0 0116 5.3v5.2A2.5 2.5 0 0113.5 13h-11A2.5 2.5 0 010 10.5V3.5z" />
    </svg>
  )
}

function IcoPhone({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <rect x="4" y="1" width="8" height="14" rx="2" />
      <circle cx="8" cy="12.5" r="0.7" fill="currentColor" />
    </svg>
  )
}

function IcoPuzzle({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M6 2h4v2.5c.6-.4 1.5-.4 1.5.8S10.6 7 10 6.5V9H7.5c.4-.6.4-1.5-.8-1.5S5 8.4 5.5 9H3V6c-.6.4-1.5.4-1.5-.8S2.4 3.5 3 4V2h3z" />
    </svg>
  )
}

function IcoCode({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <polyline points="5,4 1.5,8 5,12" />
      <polyline points="11,4 14.5,8 11,12" />
      <line x1="9.5" y1="3" x2="6.5" y2="13" />
    </svg>
  )
}

function IcoDoc({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 14 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M2 1h7l3 3v11H2V1z" />
      <polyline points="9,1 9,4 12,4" />
      <line x1="4" y1="7" x2="10" y2="7" />
      <line x1="4" y1="10" x2="8" y2="10" />
    </svg>
  )
}

function IcoGlobe({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.3} strokeLinecap="round" aria-hidden>
      <circle cx="8" cy="8" r="6" />
      <ellipse cx="8" cy="8" rx="2.5" ry="6" />
      <line x1="2" y1="8" x2="14" y2="8" />
    </svg>
  )
}

function IcoPerson({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" aria-hidden>
      <circle cx="8" cy="5.5" r="2.5" />
      <path d="M2.5 14.5c0-2.761 2.462-5 5.5-5s5.5 2.239 5.5 5" />
    </svg>
  )
}

function IcoGitHub({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="currentColor" aria-hidden>
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
    </svg>
  )
}

function IcoTrash({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <line x1="3" y1="4" x2="13" y2="4" />
      <path d="M5 4V2.5A.5.5 0 015.5 2h5a.5.5 0 01.5.5V4" />
      <path d="M4 4l.667 9.333A1 1 0 005.662 14h4.676a1 1 0 00.995-.667L12 4" />
    </svg>
  )
}

// --- Map config icon strings to icon components ---

const ICON_MAP: Record<string, (p: { className?: string }) => React.ReactElement> = {
  folder: IcoFolder,
  phone: IcoPhone,
  puzzle: IcoPuzzle,
  code: IcoCode,
  doc: IcoDoc,
}

// --- Location icon mapping ---

const LOCATION_ICON_MAP: Record<string, (p: { className?: string }) => React.ReactElement> = {
  github: IcoGitHub,
  person: IcoPerson,
  globe: IcoGlobe,
}

// --- Sidebar component ---

export function FinderSidebar({
  selectedCategory,
  onSelectCategory,
}: {
  selectedCategory: CategoryId
  onSelectCategory: (id: CategoryId) => void
}) {
  function rowClass(active: boolean) {
    return [
      "flex items-center gap-2 w-[calc(100%-8px)] mx-1 px-2 py-[3px] rounded-md",
      "text-left cursor-default transition-colors duration-100 text-[13px]",
      active
        ? "bg-blue-500 text-white"
        : "text-gray-700 dark:text-gray-300 hover:bg-black/5 dark:hover:bg-white/5",
    ].join(" ")
  }

  function iconClass(active: boolean) {
    return ["w-4 h-4 flex-shrink-0", active ? "text-white" : ""].join(" ")
  }

  function sectionHeader(label: string) {
    return (
      <div className="px-3 pt-3 pb-0.5 select-none">
        <span className="text-[11px] font-semibold text-gray-500 dark:text-gray-500">
          {label}
        </span>
      </div>
    )
  }

  return (
    <nav className="flex flex-col py-1 h-full overflow-y-auto">
      {/* Top item -- Recents (all) */}
      <div className="pt-1 pb-1">
        <button
          type="button"
          onClick={() => onSelectCategory("all")}
          className={rowClass(selectedCategory === "all")}
        >
          <IcoRecents className={[iconClass(selectedCategory === "all"), selectedCategory === "all" ? "" : "text-blue-500 dark:text-blue-400"].join(" ")} />
          <span className="truncate">Recents</span>
        </button>
      </div>

      {/* Favourites */}
      {sectionHeader("Favourites")}
      <div className="pb-1">
        {FAVOURITES.map((item) => {
          const active = selectedCategory === item.id
          // Look up the icon string from config for this category
          const catConfig = config.categories.find((c) => c.id === item.id)
          const Icon = (catConfig?.icon ? ICON_MAP[catConfig.icon] : null) ?? IcoFolder
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => onSelectCategory(item.id)}
              className={rowClass(active)}
            >
              <Icon className={[iconClass(active), active ? "" : "text-blue-500 dark:text-blue-400"].join(" ")} />
              <span className="truncate">{item.label}</span>
            </button>
          )
        })}
      </div>

      {/* Locations */}
      {sectionHeader("Locations")}
      <div className="pb-1">
        {LOCATIONS.map((loc) => {
          const Icon = LOCATION_ICON_MAP[loc.icon] ?? IcoGlobe
          return (
            <button
              key={loc.label}
              type="button"
              onClick={() => window.open(loc.href, "_blank")}
              className={rowClass(false)}
            >
              <Icon className="w-4 h-4 flex-shrink-0 text-gray-500 dark:text-gray-400" />
              <span className="truncate">{loc.label}</span>
            </button>
          )
        })}
        <button type="button" className={rowClass(false)} disabled>
          <IcoTrash className="w-4 h-4 flex-shrink-0 text-gray-500 dark:text-gray-400" />
          <span className="truncate text-gray-600 dark:text-gray-400">Bin</span>
        </button>
      </div>

      {/* Tags */}
      {sectionHeader("Tags")}
    </nav>
  )
}

// --- Item card (grid view) ---

function FinderItem({
  item,
  onActivate,
}: {
  item: PortfolioItem
  onActivate: () => void
}) {
  const isImage = typeof item.icon === "object" && item.icon.type === "image"

  return (
    <button
      type="button"
      onClick={onActivate}
      className={[
        "flex flex-col items-center text-center gap-2 p-3 rounded-lg",
        "transition-all duration-150 cursor-default group",
        "hover:bg-black/5 dark:hover:bg-white/8 hover:scale-105",
        "focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400",
      ].join(" ")}
    >
      {/* Icon */}
      {isImage ? (
        <img
          src={(item.icon as { type: "image"; src: string }).src}
          alt={item.name}
          className="w-16 h-16 rounded-xl object-cover shadow-sm"
          draggable={false}
        />
      ) : (
        <div
          className={[
            "w-16 h-16 rounded-xl flex items-center justify-center text-2xl shadow-sm",
            "bg-gray-500/20",
          ].join(" ")}
        >
          {item.icon as string}
        </div>
      )}

      {/* Name */}
      <span className="text-xs font-medium leading-tight text-gray-800 dark:text-gray-200 line-clamp-2 max-w-[90px]">
        {item.name}
      </span>
    </button>
  )
}

// --- List view row ---

function FinderListRow({
  item,
  onActivate,
}: {
  item: PortfolioItem
  onActivate: () => void
}) {
  const isImage = typeof item.icon === "object" && item.icon.type === "image"

  return (
    <button
      type="button"
      onClick={onActivate}
      className={[
        "w-full flex items-center px-3 py-1 text-left cursor-default group",
        "border-b border-gray-100 dark:border-white/5",
        "hover:bg-blue-500 focus:outline-none focus-visible:bg-blue-500",
      ].join(" ")}
    >
      {/* Icon */}
      <div className="w-5 h-5 flex-shrink-0 mr-2.5">
        {isImage ? (
          <img
            src={(item.icon as { type: "image"; src: string }).src}
            alt={item.name}
            className="w-full h-full rounded object-cover"
            draggable={false}
          />
        ) : (
          <span className="text-sm leading-none">{item.icon as string}</span>
        )}
      </div>

      {/* Name */}
      <span className="flex-1 text-[13px] text-gray-800 dark:text-gray-200 group-hover:text-white truncate">
        {item.name}
      </span>

      {/* Description */}
      <span className="w-52 text-[12px] text-gray-500 dark:text-gray-400 group-hover:text-white/80 truncate mr-4 hidden sm:block">
        {item.description}
      </span>

      {/* Kind */}
      <span className="w-36 text-[12px] text-gray-500 dark:text-gray-400 group-hover:text-white/80 truncate">
        {KIND_LABELS[item.category] || "Project"}
      </span>
    </button>
  )
}

// --- Main content component ---

export default function FinderContent() {
  const [selectedCategory, setSelectedCategory] = useState<CategoryId>("all")
  const [viewMode, setViewMode] = useState<ViewMode>("list")
  const navigate = useNavigate()
  const { openWindow, navigateSafari } = useDesktop()

  const filteredItems =
    selectedCategory === "all"
      ? ITEMS
      : ITEMS.filter((item) => item.category === selectedCategory)

  function handleItemActivate(item: PortfolioItem) {
    if (item.action === "navigate") {
      if (item.target === "/deeplinks") openWindow("deeplinks")
      else navigate(item.target)
    } else {
      navigateSafari({ url: item.target, title: item.name, description: item.description })
      openWindow("safari")
    }
  }

  const count = filteredItems.length

  return (
    <div className="flex h-full">
      {/* Sidebar */}
      <div className="w-48 shrink-0 border-r border-gray-200 dark:border-white/10 bg-gray-50/80 dark:bg-white/3 overflow-y-auto">
        <FinderSidebar
          selectedCategory={selectedCategory}
          onSelectCategory={setSelectedCategory}
        />
      </div>

      {/* Main area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Toolbar */}
        <div className="flex items-center gap-3 px-4 py-2 border-b border-gray-200 dark:border-white/10 bg-gray-50/50 dark:bg-white/3">
          {/* Back/forward (decorative) */}
          <div className="flex gap-1">
            <button
              type="button"
              className="w-7 h-7 rounded flex items-center justify-center text-gray-400 dark:text-gray-500"
              disabled
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                strokeWidth={2}
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M15 19l-7-7 7-7"
                />
              </svg>
            </button>
            <button
              type="button"
              className="w-7 h-7 rounded flex items-center justify-center text-gray-400 dark:text-gray-500"
              disabled
            >
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                strokeWidth={2}
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </button>
          </div>

          {/* Title */}
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
            {selectedCategory === "all"
              ? "Recents"
              : FAVOURITES.find((s) => s.id === selectedCategory)?.label ||
                "All Projects"}
          </span>

          <div className="flex-1" />

          {/* View toggle icons */}
          <div className="flex gap-0.5">
            <button
              type="button"
              onClick={() => setViewMode("grid")}
              className={[
                "w-7 h-7 rounded flex items-center justify-center",
                viewMode === "grid"
                  ? "bg-gray-200/70 dark:bg-white/10 text-gray-700 dark:text-gray-200"
                  : "text-gray-400 dark:text-gray-500 hover:bg-gray-200/50 dark:hover:bg-white/5",
              ].join(" ")}
              aria-label="Grid view"
            >
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 16 16">
                <rect x="1" y="1" width="6" height="6" rx="1" />
                <rect x="9" y="1" width="6" height="6" rx="1" />
                <rect x="1" y="9" width="6" height="6" rx="1" />
                <rect x="9" y="9" width="6" height="6" rx="1" />
              </svg>
            </button>
            <button
              type="button"
              onClick={() => setViewMode("list")}
              className={[
                "w-7 h-7 rounded flex items-center justify-center",
                viewMode === "list"
                  ? "bg-gray-200/70 dark:bg-white/10 text-gray-700 dark:text-gray-200"
                  : "text-gray-400 dark:text-gray-500 hover:bg-gray-200/50 dark:hover:bg-white/5",
              ].join(" ")}
              aria-label="List view"
            >
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 16 16">
                <rect x="1" y="2" width="14" height="2" rx="0.5" />
                <rect x="1" y="7" width="14" height="2" rx="0.5" />
                <rect x="1" y="12" width="14" height="2" rx="0.5" />
              </svg>
            </button>
          </div>
        </div>

        {/* Peck landing intro: keep the desktop homepage self-explanatory. */}
        <div className="border-b border-gray-200 bg-gradient-to-r from-blue-50 via-white to-amber-50 px-5 py-4 dark:border-white/10 dark:from-blue-950/30 dark:via-[#1e1e1e] dark:to-amber-950/20">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0">
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-blue-600 dark:text-blue-300">Peck Directory</p>
              <h1 className="mt-1 text-base font-semibold text-gray-900 dark:text-gray-100">让值得信任的 Mac 应用，一键抵达。</h1>
              <p className="mt-1 max-w-xl text-xs leading-5 text-gray-500 dark:text-gray-400">Peck 收集开源项目的 Deeplink，帮助你从网页、README 或工作流直接打开 Mac 应用。</p>
            </div>
            <button type="button" onClick={() => openWindow("deeplinks")} className="shrink-0 rounded-lg bg-blue-600 px-3 py-2 text-xs font-semibold text-white shadow-sm transition hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
              浏览 Deeplinks
            </button>
          </div>
        </div>

        {/* Content area */}
        {viewMode === "list" ? (
          <div className="flex-1 overflow-y-auto bg-white dark:bg-gray-900/50">
            {/* Column headers */}
            <div className="sticky top-0 flex items-center px-3 py-1 border-b border-gray-200 dark:border-white/10 bg-gray-50/90 dark:bg-gray-800/90 text-[11px] font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide select-none">
              <div className="w-5 mr-2.5" />
              <span className="flex-1">Name</span>
              <span className="w-52 mr-4 hidden sm:block">Description</span>
              <span className="w-36">Kind</span>
            </div>

            {filteredItems.map((item) => (
              <FinderListRow
                key={item.name}
                item={item}
                onActivate={() => handleItemActivate(item)}
              />
            ))}

            {filteredItems.length === 0 && (
              <div className="flex items-center justify-center h-40 text-sm text-gray-400 dark:text-gray-500">
                No items in this category
              </div>
            )}
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto bg-white dark:bg-gray-900/50">
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4 p-6">
              {filteredItems.map((item) => (
                <FinderItem
                  key={item.name}
                  item={item}
                  onActivate={() => handleItemActivate(item)}
                />
              ))}
            </div>

            {filteredItems.length === 0 && (
              <div className="flex items-center justify-center h-40 text-sm text-gray-400 dark:text-gray-500">
                No items in this category
              </div>
            )}
          </div>
        )}

        {/* Status bar */}
        <div className="flex items-center px-4 py-1.5 border-t border-gray-200 dark:border-white/10 bg-gray-50/50 dark:bg-white/3 text-[11px] text-gray-500 dark:text-gray-500">
          {count} item{count !== 1 ? "s" : ""}
        </div>
      </div>
    </div>
  )
}

export type { CategoryId as FinderCategoryId }
