import { useEffect, useMemo, useRef, useState } from "react"

export interface WoodpeckerStageProps {
  /** Hook for the eventual production animation or analytics integration. */
  onAppEaten?: (appId: string) => void
  disabled?: boolean
}

type Point = { x: number; y: number }
type Phase = "walking" | "pecking" | "eating"

interface DesktopApp {
  id: string
  name: string
  icon: string
  point: Point
  bugVariant: number
  bugTone: string
}

const BUGS = [
  { variant: 0, tone: "peck-bug--green" },
  { variant: 1, tone: "peck-bug--blue" },
  { variant: 2, tone: "peck-bug--red" },
  { variant: 3, tone: "peck-bug--gold" },
]

const APP_SEEDS = [
  { id: "finder", name: "Finder", icon: "/images/macos-icons/finder.png", point: { x: 9, y: 17 } },
  { id: "notes", name: "Notes", icon: "/images/macos-icons/notes.png", point: { x: 24, y: 33 } },
  { id: "deeplinks", name: "Deeplinks", icon: "/images/peck-mark-color.svg", point: { x: 73, y: 19 } },
  { id: "terminal", name: "Terminal", icon: "/images/macos-icons/terminal.png", point: { x: 84, y: 39 } },
  { id: "peck", name: "Peck", icon: "/images/peck-mark-color.svg", point: { x: 58, y: 68 } },
]

function distance(from: Point, to: Point) {
  return Math.hypot(to.x - from.x, to.y - from.y)
}

function approachPoint(app: Point, from: Point): Point {
  const horizontalSide = app.x >= from.x ? -1 : 1
  return { x: app.x + horizontalSide * 4.2, y: app.y + 4.5 }
}

function nextTarget(apps: DesktopApp[], current: string | null) {
  const available = apps.filter((app) => app.id !== current)
  return available[Math.floor(Math.random() * available.length)] ?? apps[0]
}

function createApps(randomizeBugs = false): DesktopApp[] {
  return APP_SEEDS.map((app, index) => {
    const bug = BUGS[randomizeBugs ? Math.floor(Math.random() * BUGS.length) : index % BUGS.length]
    return { ...app, bugVariant: bug.variant, bugTone: bug.tone }
  })
}

function BugArtwork({ variant }: { variant: number }) {
  if (variant === 1) {
    return <svg viewBox="0 0 32 32" aria-hidden><path d="m10 12-5-3m5 9-6 1m8 4-5 4m15-15 5-3m-5 9 6 1m-8 4 5 4m-13-20-2-3m8 3 2-3" stroke="#5b4436" strokeWidth="1.5" strokeLinecap="round" /><ellipse cx="16" cy="18" rx="8.5" ry="10" fill="#bf7657" /><path d="M16 9v18" stroke="#79523c" strokeWidth="1.4" /><ellipse cx="16" cy="8" rx="5" ry="4" fill="#4e4537" /><circle cx="13" cy="7" r=".8" fill="#f4e9d4" /><circle cx="19" cy="7" r=".8" fill="#f4e9d4" /><circle cx="11.5" cy="17" r="1.7" fill="#875038" /><circle cx="20.5" cy="21" r="1.8" fill="#875038" /></svg>
  }
  if (variant === 2) {
    return <svg viewBox="0 0 32 32" aria-hidden><path d="M5 24c7 3 5-10 11-7 5 2 1-10 8-10" fill="none" stroke="#aa7d48" strokeWidth="9" strokeLinecap="round" /><path d="M5 23c7 3 5-10 11-7 5 2 1-10 8-10" fill="none" stroke="#d3b477" strokeWidth="6" strokeLinecap="round" /><path d="m8 22 3 2m2-7 2 4m4-7 3 1" stroke="#aa7d48" strokeWidth="1.2" strokeLinecap="round" /><circle cx="25" cy="6" r="4" fill="#d3b477" /><circle cx="26" cy="5" r=".9" fill="#4b4932" /><path d="m24 2-1-2m5 3 2-2" stroke="#8b754e" strokeWidth="1.2" strokeLinecap="round" /></svg>
  }
  if (variant === 3) {
    return <svg viewBox="0 0 32 32" aria-hidden><path d="M15 15C3-1-2 15 13 20 1 26 11 32 16 21c6 11 15 5 3-1C34 15 29-1 17 15" fill="#88b7a0" /><path d="M15 16C6 7 4 15 13 18m5-2c8-9 11-1 2 2" fill="none" stroke="#bfd5aa" strokeWidth="2.5" strokeLinecap="round" /><ellipse cx="16" cy="19" rx="2.3" ry="8" fill="#4b6755" /><circle cx="16" cy="10" r="3" fill="#4b6755" /><path d="m15 8-4-4m6 4 4-4" stroke="#4b6755" strokeWidth="1.3" strokeLinecap="round" /></svg>
  }
  return <svg viewBox="0 0 32 32" aria-hidden><path d="m6 22-1 3m6-2v3m6-3 1 3m6-6 2 2M22 7l-1-3m6 4 2-3" stroke="#4e6844" strokeWidth="1.5" strokeLinecap="round" /><circle cx="7" cy="19" r="4" fill="#8aa66c" /><circle cx="13" cy="18" r="4.7" fill="#a4bb7b" /><circle cx="19" cy="16" r="4.9" fill="#b4c88c" /><circle cx="24" cy="12" r="5.5" fill="#a4bb7b" /><circle cx="26" cy="11" r="1" fill="#273c2e" /><path d="m26 15 2-1" stroke="#4e6844" strokeWidth="1.2" strokeLinecap="round" /></svg>
}

function WoodpeckerArtwork() {
  return (
    <svg className="peck-woodpecker__svg" viewBox="0 0 160 180" role="img" aria-label="啄木鸟">
      <g className="peck-bird-svg__body">
        <path d="M48 115 29 166 67 146" fill="#252d28" />
        <path d="M59 64c-21 14-26 51-14 70 13 22 39 22 52 4 12-18 10-47-5-64Z" fill="#29332c" />
        <path d="M72 79c-14 4-22 26-16 43 4 13 14 17 27 17 13-18 14-39-11-60Z" fill="#f7f4e8" />
        <path d="M47 90c-10 19-6 42 13 48l11-46Z" fill="#424f41" />
        <path d="m47 97 15 6m-16 7 13 6" stroke="#b6c2a8" strokeWidth="3" strokeLinecap="round" />
      </g>
      <g className="peck-bird-svg__head">
        <path d="M48 65C30 46 43 18 66 19c26 0 42 21 34 42l-9 23-29-5Z" fill="#29332c" />
        <path d="M49 24 43 8l16 7L64 1l9 15 20-5-7 18Z" fill="#c84b3a" />
        <path d="M69 43c9-8 25-7 31 3l-9 27-20-5-18-12Z" fill="#f5f2e6" />
        <path d="m98 43 42 10-43 9Z" fill="#b78c4d" />
        <path d="m99 53 41 0-43 9Z" fill="#927044" />
        <circle cx="88" cy="43" r="3.3" fill="#151c17" />
        <circle cx="89" cy="42" r="0.8" fill="#fff" />
      </g>
    </svg>
  )
}

export default function WoodpeckerStage({ onAppEaten, disabled = false }: WoodpeckerStageProps) {
  const initialApps = useMemo(() => createApps(), [])
  const [apps, setApps] = useState<DesktopApp[]>(initialApps)
  const [bird, setBird] = useState<Point>({ x: 50, y: 78 })
  const birdRef = useRef<Point>({ x: 50, y: 78 })
  const [targetId, setTargetId] = useState<string | null>(initialApps[0]?.id ?? null)
  const [phase, setPhase] = useState<Phase>("walking")
  const [flyingId, setFlyingId] = useState<string | null>(null)
  const [heading, setHeading] = useState<1 | -1>(1)
  const aliveApps = apps.filter((app) => !flyingId || app.id !== flyingId)
  const target = apps.find((app) => app.id === targetId) ?? aliveApps[0]
  const onAppEatenRef = useRef(onAppEaten)

  useEffect(() => {
    onAppEatenRef.current = onAppEaten
  }, [onAppEaten])

  useEffect(() => {
    if (disabled || !target) return

    if (phase === "walking") {
      const from = birdRef.current
      const to = approachPoint(target.point, from)
      const distanceToTarget = distance(from, to)
      const duration = Math.max(1300, Math.min(4200, distanceToTarget * 38))
      const nextHeading: 1 | -1 = to.x >= from.x ? 1 : -1
      // The walking heading is part of the animation state and changes once per route.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setHeading(nextHeading)
      const startedAt = performance.now()
      let frame = 0

      const walk = (now: number) => {
        const progress = Math.min(1, (now - startedAt) / duration)
        const eased = progress < 0.5
          ? 2 * progress * progress
          : 1 - Math.pow(-2 * progress + 2, 2) / 2
        const next = {
          x: from.x + (to.x - from.x) * eased,
          y: from.y + (to.y - from.y) * eased,
        }
        birdRef.current = next
        setBird(next)

        if (progress < 1) {
          frame = window.requestAnimationFrame(walk)
        } else {
          setPhase("pecking")
        }
      }

      frame = window.requestAnimationFrame(walk)
      return () => window.cancelAnimationFrame(frame)
    }

    if (phase === "pecking") {
      const timer = window.setTimeout(() => setPhase("eating"), 880)
      return () => window.clearTimeout(timer)
    }

    const timer = window.setTimeout(() => {
      setFlyingId(target.id)
      onAppEatenRef.current?.(target.id)
      window.setTimeout(() => {
        setApps((current) => current.filter((app) => app.id !== target.id))
        setFlyingId(null)
        setTargetId(null)
        setPhase("walking")
      }, 850)
    }, 420)
    return () => window.clearTimeout(timer)
  }, [disabled, phase, target, targetId])

  useEffect(() => {
    if (targetId || aliveApps.length === 0) return
    const next = nextTarget(aliveApps, null)
    // Select the next target after the previous app finishes its flight.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTargetId(next?.id ?? null)
  }, [aliveApps, targetId])

  useEffect(() => {
    if (apps.length > 0) return
    const timer = window.setTimeout(() => {
      const nextApps = createApps(true)
      setApps(nextApps)
      const resetPoint = { x: 50, y: 78 }
      birdRef.current = resetPoint
      setBird(resetPoint)
      setHeading(1)
      setTargetId(nextApps[0]?.id ?? null)
      setPhase("walking")
    }, 1500)
    return () => window.clearTimeout(timer)
  }, [apps.length, initialApps])

  return (
    <div className="pointer-events-none absolute inset-x-0 bottom-0 top-7 z-[1]" aria-label="啄木鸟桌面走动动画">
      <div className="peck-desktop-stage">
        <div className="peck-desktop-stage__title">
          <span className="peck-desktop-stage__eyebrow">Peck desktop habitat</span>
          <span className="peck-desktop-stage__status">{phase === "pecking" ? "啄击中…" : phase === "eating" ? "吃掉了" : "寻找下一只虫子"}</span>
        </div>

        {apps.map((app) => {
          const isFlying = flyingId === app.id
          const isTarget = targetId === app.id
          const bugDrift = heading === 1 ? "-2.4rem" : "2.4rem"
          return (
            <div key={app.id} className={["peck-desktop-app", isFlying ? "peck-desktop-app--flying" : "", isTarget && phase === "pecking" ? "peck-desktop-app--target" : ""].join(" ")} style={{ left: `${app.point.x}%`, top: `${app.point.y}%`, "--bug-drift": bugDrift } as React.CSSProperties}>
              <span className={["peck-bug", app.bugTone, phase === "eating" && isTarget ? "peck-bug--eaten" : ""].join(" ")}><BugArtwork variant={app.bugVariant} /></span>
              <span className="peck-desktop-app__icon"><img src={app.icon} alt="" draggable={false} /></span>
              <span className="peck-desktop-app__wings" aria-hidden><i /><i /></span>
              <span className="peck-desktop-app__spark peck-desktop-app__spark--one" aria-hidden />
              <span className="peck-desktop-app__spark peck-desktop-app__spark--two" aria-hidden />
              <span className="peck-desktop-app__label">{app.name}</span>
            </div>
          )
        })}

        <div className={["peck-woodpecker", `peck-woodpecker--${phase}`, phase === "pecking" ? "peck-woodpecker--pecking" : ""].join(" ")} style={{ left: `${bird.x}%`, top: `${bird.y}%`, transform: `translate(-50%, -50%) scaleX(${heading})` }}>
          <span className="peck-woodpecker__motion" />
          <span className="peck-woodpecker__feet" aria-hidden><i /><i /></span>
          <span className="peck-woodpecker__body"><WoodpeckerArtwork /></span>
          <span className="peck-woodpecker__beak" />
          <span className="peck-woodpecker__dust" aria-hidden><i /><i /><i /></span>
        </div>

        <div className="peck-desktop-stage__hint">随机走动 · 啄击 · 吃虫 · 应用起飞</div>
      </div>
    </div>
  )
}
