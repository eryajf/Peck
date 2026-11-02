import { useState, useEffect, useRef } from "react"
import { config } from "../../config"

// --- Terminal line definitions ---

interface TerminalLine {
  type: "prompt" | "output" | "blank" | "social" | "name" | "tagline" | "currentWork"
  text?: string
  links?: { label: string; url: string }[]
  delay: number // ms before this line appears
}

// Build LINES dynamically from config
function buildLines(): TerminalLine[] {
  const lines: TerminalLine[] = []

  // 1. Greeting line
  lines.push({
    type: "output",
    text: config.terminal.greeting,
    delay: 0,
  })

  // 2. whoami prompt -> name + tagline
  lines.push({ type: "prompt", text: "whoami", delay: 600 })
  lines.push({ type: "blank", delay: 300 })
  lines.push({ type: "name", text: config.terminal.name, delay: 200 })
  lines.push({ type: "tagline", text: config.terminal.tagline, delay: 100 })
  lines.push({ type: "blank", delay: 100 })

  // 4. If currentWork exists, show it
  if (config.currentWork) {
    lines.push({
      type: "currentWork",
      text: `Currently building ${config.currentWork.label} (${config.currentWork.url})`,
      delay: 200,
    })
    lines.push({ type: "blank", delay: 400 })
  }

  // 3. cat about.txt prompt -> aboutLines
  lines.push({ type: "prompt", text: "cat about.txt", delay: 800 })
  lines.push({ type: "blank", delay: 300 })
  config.terminal.aboutLines.forEach((line, i) => {
    lines.push({ type: "output", text: line, delay: i === 0 ? 200 : 150 })
  })
  lines.push({ type: "blank", delay: 400 })

  // 5. ls ~/social/ prompt -> social links (only non-empty)
  const socialLinks: { label: string; url: string }[] = []
  const socialEntries = Object.entries(config.social) as [string, string][]
  for (const [key, url] of socialEntries) {
    if (url) {
      socialLinks.push({
        label: key.charAt(0).toUpperCase() + key.slice(1),
        url,
      })
    }
  }

  if (socialLinks.length > 0) {
    lines.push({ type: "prompt", text: "ls ~/social/", delay: 800 })
    lines.push({ type: "social", links: socialLinks, delay: 300 })
    lines.push({ type: "blank", delay: 400 })
  }

  // 6. Final empty prompt with blinking cursor
  lines.push({ type: "prompt", text: "", delay: 600 })

  return lines
}

const LINES = buildLines()

// --- Component ---

export default function TerminalContent() {
  const [visibleCount, setVisibleCount] = useState(0)
  const containerRef = useRef<HTMLDivElement>(null)

  // Animate lines appearing one by one
  useEffect(() => {
    if (visibleCount >= LINES.length) return

    const nextLine = LINES[visibleCount]
    const timer = setTimeout(() => {
      setVisibleCount((c) => c + 1)
    }, nextLine.delay)

    return () => clearTimeout(timer)
  }, [visibleCount])

  // Auto-scroll to bottom as lines appear
  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight
    }
  }, [visibleCount])

  const visibleLines = LINES.slice(0, visibleCount)
  const isLastLineVisible = visibleCount >= LINES.length

  // Render the prompt prefix
  function renderPrompt() {
    return (
      <>
        <span style={{ color: "#4ec9b0" }}>guest</span>
        <span style={{ color: "#808080" }}>@</span>
        <span style={{ color: "#569cd6" }}>{config.terminal.hostname}</span>
        <span style={{ color: "#808080" }}>{" ~ % "}</span>
      </>
    )
  }

  // Render a linkified "Currently building ..." line
  function renderCurrentWork(text: string) {
    if (!config.currentWork) return <span>{text}</span>

    const url = config.currentWork.url
    const label = config.currentWork.label
    const prefix = `Currently building ${label} (`
    const suffix = ")"

    return (
      <span style={{ color: "#d4d4d4" }}>
        {prefix}
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          style={{ color: "#569cd6", textDecoration: "none" }}
          onMouseEnter={(e) => {
            ;(e.target as HTMLAnchorElement).style.color = "#9cdcfe"
          }}
          onMouseLeave={(e) => {
            ;(e.target as HTMLAnchorElement).style.color = "#569cd6"
          }}
        >
          {url}
        </a>
        {suffix}
      </span>
    )
  }

  return (
    <div
      ref={containerRef}
      className="h-full overflow-y-auto p-4 select-text"
      style={{
        backgroundColor: "#1e1e1e",
        fontFamily: "'Fira Code', 'Courier New', Courier, monospace",
        fontSize: "13px",
        lineHeight: "1.7",
      }}
    >
      {visibleLines.map((line, i) => {
        if (line.type === "blank") {
          return <div key={i} className="h-[1.7em]" />
        }

        if (line.type === "prompt") {
          const isLastPrompt = i === visibleLines.length - 1 && isLastLineVisible
          return (
            <div key={i} className="flex flex-wrap items-center">
              {renderPrompt()}
              {line.text && (
                <span style={{ color: "#d4d4d4" }}>{line.text}</span>
              )}
              {isLastPrompt && !line.text && (
                <span className="cursor" />
              )}
            </div>
          )
        }

        if (line.type === "name") {
          return (
            <div
              key={i}
              className="text-lg font-bold"
              style={{ color: "#00ff41" }}
            >
              {line.text}
            </div>
          )
        }

        if (line.type === "tagline") {
          return (
            <div key={i} style={{ color: "#808080" }}>
              {line.text}
            </div>
          )
        }

        if (line.type === "currentWork") {
          return (
            <div key={i}>
              {renderCurrentWork(line.text ?? "")}
            </div>
          )
        }

        if (line.type === "social") {
          return (
            <div key={i} className="flex flex-wrap gap-x-6 gap-y-1">
              {line.links?.map((link) => (
                <a
                  key={link.label}
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:underline"
                  style={{ color: "#569cd6", textDecoration: "none" }}
                  onMouseEnter={(e) => {
                    ;(e.target as HTMLAnchorElement).style.color = "#9cdcfe"
                  }}
                  onMouseLeave={(e) => {
                    ;(e.target as HTMLAnchorElement).style.color = "#569cd6"
                  }}
                >
                  {link.label}
                </a>
              ))}
            </div>
          )
        }

        // output
        return (
          <div key={i} style={{ color: "#d4d4d4" }}>
            {line.text}
          </div>
        )
      })}
    </div>
  )
}
