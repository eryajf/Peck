import { Helmet } from 'react-helmet-async'
import { config } from '../config'

interface SEOProps {
  title: string
  description: string
  url?: string
  image?: string
  type?: string
  twitterCard?: string
  jsonLd?: Record<string, unknown>
}

export default function SEO({
  title,
  description,
  url = config.siteUrl,
  image = config.profileImage,
  type = 'website',
  twitterCard = 'summary_large_image',
  jsonLd,
}: SEOProps) {
  return (
    <Helmet>
      <title>{title}</title>
      <meta name="description" content={description} />
      <meta property="og:type" content={type} />
      {url && <meta property="og:url" content={url} />}
      <meta property="og:title" content={title} />
      <meta property="og:description" content={description} />
      {url && <meta property="og:image" content={new URL(image, url).href} />}
      <meta name="twitter:card" content={twitterCard} />
      <meta name="twitter:title" content={title} />
      <meta name="twitter:description" content={description} />
      {url && <meta name="twitter:image" content={new URL(image, url).href} />}
      {jsonLd && (
        <script type="application/ld+json">
          {JSON.stringify(jsonLd)}
        </script>
      )}
    </Helmet>
  )
}
