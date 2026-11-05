import registry from '../../../data/project-links.json'

export interface DeeplinkEntry {
  id: string
  name: string
  description: string
  descriptionEn: string
  category: string
  categoryEn: string
  deeplink: string
  icon: string
  repository: string
  bundleId: string
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
    category: project.category,
    categoryEn: project.category_en,
    deeplink: `peck://prepare?${params}`,
    icon: project.icon,
    repository: project.homepage,
    bundleId: project.bundle_id,
    status: project.status,
    statusUrl: project.status_url,
  }
})
