import { readFile } from 'node:fs/promises'

const registry = JSON.parse(await readFile(new URL('../../data/project-links.json', import.meta.url), 'utf8'))
if (!Array.isArray(registry.projects) || registry.projects.length === 0) throw new Error('projects must be a non-empty array')
const ids = new Set()
const required = ['name', 'description', 'description_en', 'homepage', 'bundle_id', 'icon', 'status', 'status_url']
for (const project of registry.projects) {
  for (const field of required) {
    if (typeof project[field] !== 'string' || !project[field].trim()) throw new Error(`Missing ${field} for ${project.name || 'project'}`)
    if (/[\p{Cc}\p{Cf}]/u.test(project[field])) throw new Error(`Invalid character in ${field} for ${project.name}`)
  }
  if ([...project.name].length > 128 || !/^[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*$/.test(project.bundle_id) || project.bundle_id.length > 255) throw new Error(`Invalid project name or bundle ID: ${project.name}`)
  if (ids.has(project.bundle_id.toLowerCase())) throw new Error(`Duplicate bundle ID: ${project.bundle_id}`)
  ids.add(project.bundle_id.toLowerCase())
  for (const field of ['homepage', 'icon', 'status_url']) {
    const url = new URL(project[field])
    if (url.protocol !== 'https:' || url.username || url.password) throw new Error(`Invalid ${field} for ${project.name}`)
  }
  const home = new URL(project.homepage)
  if (project.homepage.length > 2048 || (home.port && home.port !== '443') || !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$/i.test(home.hostname) || /(?:^|\.)(?:localhost|local|lan|internal|home|home\.arpa)$/i.test(home.hostname) || /^\d+(?:\.\d+){3}$/.test(home.hostname)) throw new Error(`Unsupported homepage for ${project.name}`)
  if ('release' in project && (typeof project.release !== 'string' || !project.release.trim() || [...project.release].length > 64 || /[\p{Cc}\p{Cf}]/u.test(project.release))) throw new Error(`Invalid release for ${project.name}`)
  const params = new URLSearchParams({ v: '1', project: project.name, bundle_id: project.bundle_id, homepage: project.homepage })
  if (project.release) params.set('release', project.release)
  if (`peck://prepare?${params}`.length > 4096) throw new Error(`Deeplink too long for ${project.name}`)
}
console.log(`Validated ${registry.projects.length} project links`)
