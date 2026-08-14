import { describe, expect, it } from 'vitest'
import { renderMarkdown } from './markdown'

describe('safe Markdown', () => {
  it('renders formatting while escaping raw HTML and disabling unsafe links', () => {
    const html = renderMarkdown('Hello **team** <img src=x onerror=alert(1)> [bad](javascript:alert(1)) [good](https://example.com)')
    expect(html).toContain('<strong>team</strong>')
    expect(html).toContain('&lt;img')
    expect(html).not.toContain('href="javascript:')
    expect(html).toContain('href="https://example.com"')
    expect(html).toContain('rel="noopener noreferrer"')
  })
})
