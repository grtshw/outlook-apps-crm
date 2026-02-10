import DOMPurify from 'dompurify'

export function renderRichText(html: string | undefined | null): string {
  if (!html) return ''
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'p', 'br', 'strong', 'b', 'em', 'i', 'u', 'a',
      'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'blockquote', 'pre', 'code', 'hr', 'table', 'thead', 'tbody',
      'tr', 'th', 'td', 'span', 'sub', 'sup', 'del', 's',
    ],
    ALLOWED_ATTR: ['href', 'target', 'rel', 'class'],
  })
}
