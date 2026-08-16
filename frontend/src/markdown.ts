import MarkdownIt from 'markdown-it'

const markdown = new MarkdownIt({ html: false, linkify: true, breaks: true })
const defaultLinkOpen = markdown.renderer.rules.link_open ?? ((tokens, index, options, _env, self) => self.renderToken(tokens, index, options))
markdown.renderer.rules.link_open = (tokens, index, options, env, self) => {
  tokens[index].attrSet('rel', 'noopener noreferrer')
  tokens[index].attrSet('target', '_blank')
  return defaultLinkOpen(tokens, index, options, env, self)
}

export function renderMarkdown(source: string): string {
  return markdown.render(source)
}
