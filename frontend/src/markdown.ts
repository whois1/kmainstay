import MarkdownIt from 'markdown-it'
import type { MentionedUser } from './types'

const markdown = new MarkdownIt({ html: false, linkify: true, breaks: true })
const defaultLinkOpen = markdown.renderer.rules.link_open ?? ((tokens, index, options, _env, self) => self.renderToken(tokens, index, options))
markdown.renderer.rules.link_open = (tokens, index, options, env, self) => {
  tokens[index].attrSet('rel', 'noopener noreferrer')
  tokens[index].attrSet('target', '_blank')
  return defaultLinkOpen(tokens, index, options, env, self)
}

export function renderMarkdown(source: string, mentions: MentionedUser[] = []): string {
  const environment = { mentions }
  return markdown.render(source, environment)
}

markdown.core.ruler.after('inline', 'recognised_mentions', state => {
  const mentions = (state.env?.mentions ?? []) as MentionedUser[]
  if (!mentions.length) return
  for (const block of state.tokens) {
    if (!block.children) continue
    const children = []
    for (const token of block.children) {
      if (token.type !== 'text') { children.push(token); continue }
      const pattern = mentionPattern(mentions)
      let offset = 0
      for (const match of token.content.matchAll(pattern)) {
        if (match.index! > offset) {
          const plain = new state.Token('text', '', 0); plain.content = token.content.slice(offset, match.index); children.push(plain)
        }
        const mention = mentions.find(item => item.name.toLocaleLowerCase() === match[1].toLocaleLowerCase())!
        const styled = new state.Token('html_inline', '', 0)
        styled.content = `<span class="mention" data-user-id="${markdown.utils.escapeHtml(mention.id)}">${markdown.utils.escapeHtml(match[0])}</span>`
        children.push(styled)
        offset = match.index! + match[0].length
      }
      if (offset < token.content.length) { const plain = new state.Token('text', '', 0); plain.content = token.content.slice(offset); children.push(plain) }
    }
    block.children = children
  }
})

function mentionPattern(mentions: MentionedUser[]) {
  const names = mentions.map(({ name }) => name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).sort((a, b) => b.length - a.length)
  return new RegExp(`(?<![\\p{L}\\p{N}\\p{M}_])@(${names.join('|')})(?![\\p{L}\\p{N}\\p{M}_])`, 'giu')
}
