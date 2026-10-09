import {createElement, useMemo, type ReactNode} from 'react';
import {Lexer, type Token, type Tokens} from 'marked';

// Parse Markdown into data tokens. React escapes every text node; raw HTML and
// external images never execute or fetch. Only safe, user-clicked links render.
function safeLink(href: string) {
  if (href.startsWith('#')) return href;
  try {const url = new URL(href);return ['http:', 'https:', 'mailto:'].includes(url.protocol) ? url.href : undefined;}
  catch {return undefined;}
}
function inline(tokens: Token[]): ReactNode[] {
  return tokens.map((token, key) => {
    switch (token.type) {
      case 'strong': return <strong key={key}>{inline((token as Tokens.Strong).tokens)}</strong>;
      case 'em': return <em key={key}>{inline((token as Tokens.Em).tokens)}</em>;
      case 'del': return <del key={key}>{inline((token as Tokens.Del).tokens)}</del>;
      case 'codespan': return <code key={key}>{(token as Tokens.Codespan).text}</code>;
      case 'br': return <br key={key}/>;
      case 'link': {const value = token as Tokens.Link, href = safeLink(value.href);return href ? <a key={key} href={href} title={value.title ?? undefined} target="_blank" rel="noopener noreferrer">{inline(value.tokens)}</a> : <span key={key}>{inline(value.tokens)}</span>;}
      case 'text': {const value = token as Tokens.Text;return <span key={key}>{value.tokens ? inline(value.tokens) : value.text}</span>;}
      case 'image': return <span key={key}>{(token as Tokens.Image).text}</span>;
      default: return <span key={key}>{token.raw}</span>;
    }
  });
}
function blocks(tokens: Token[]): ReactNode[] {
  return tokens.map((token, key) => {
    switch (token.type) {
      case 'space': return null;
      case 'heading': {const value = token as Tokens.Heading;return createElement('h' + value.depth, {key}, inline(value.tokens));}
      case 'paragraph': return <p key={key}>{inline((token as Tokens.Paragraph).tokens)}</p>;
      case 'text': {const value = token as Tokens.Text;return <p key={key}>{value.tokens ? inline(value.tokens) : value.text}</p>;}
      case 'code': return <pre key={key}><code>{(token as Tokens.Code).text}</code></pre>;
      case 'blockquote': return <blockquote key={key}>{blocks((token as Tokens.Blockquote).tokens)}</blockquote>;
      case 'hr': return <hr key={key}/>;
      case 'list': {const value = token as Tokens.List, items = value.items.map((item, n) => <li key={n}>{item.task && <span aria-hidden="true">{item.checked ? '☑ ' : '☐ '}</span>}{blocks(item.tokens)}</li>);return value.ordered ? <ol key={key} start={typeof value.start === 'number' ? value.start : undefined}>{items}</ol> : <ul key={key}>{items}</ul>;}
      case 'table': {const value = token as Tokens.Table;return <div className="judex-material-markdown-table" key={key}><table><thead><tr>{value.header.map((cell, n) => <th key={n}>{inline(cell.tokens)}</th>)}</tr></thead><tbody>{value.rows.map((row, n) => <tr key={n}>{row.map((cell, c) => <td key={c}>{inline(cell.tokens)}</td>)}</tr>)}</tbody></table></div>;}
      case 'html': return <pre className="judex-material-markdown-html" key={key}>{token.raw}</pre>;
      default: return <p key={key}>{token.raw}</p>;
    }
  });
}
export function MarkdownReader({text}: {text: string}) {
  const content = useMemo(() => blocks(Lexer.lex(text, {gfm: true})), [text]);
  return <article className="judex-material-markdown">{content}</article>;
}
