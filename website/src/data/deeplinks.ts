import registry from '../../../data/project-links.json'

export interface DeeplinkEntry {
  id: string
  name: string
  description: string
  descriptionEn: string
  category: string
  deeplink: string
  icon: string
  repository: string
  status: string
  statusUrl: string
}

export const deeplinks: DeeplinkEntry[] = registry.projects.map((project) => {
  const params = new URLSearchParams({ v: '1', project: project.name, bundle_id: project.bundle_id, homepage: project.homepage })
  if ('release' in project && typeof project.release === 'string' && project.release) params.set('release', project.release)
  return {
    id: project.bundle_id,
    name: project.name,
    description: project.description,
    descriptionEn: project.description_en,
    category: 'macOS',
    deeplink: `peck://prepare?${params}`,
    icon: project.icon,
    repository: project.homepage,
    status: project.status,
    statusUrl: project.status_url,
  }
})
