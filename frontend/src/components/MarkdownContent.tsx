import type { ReactNode } from 'react';

type MarkdownBlock =
  | { type: 'heading'; level: number; text: string }
  | { type: 'paragraph'; text: string }
  | { type: 'unordered-list'; items: string[] }
  | { type: 'ordered-list'; items: string[] }
  | { type: 'blockquote'; text: string }
  | { type: 'code'; text: string }
  | { type: 'divider' };

interface Props {
  markdown: string;
}

export function MarkdownContent({ markdown }: Props) {
  const blocks = parseBlocks(markdown);

  return (
    <div className="markdown-content">
      {blocks.map((block, index) => renderBlock(block, index))}
    </div>
  );
}

function parseBlocks(markdown: string): MarkdownBlock[] {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const blocks: MarkdownBlock[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];
    const trimmed = line.trim();

    if (!trimmed) {
      i += 1;
      continue;
    }

    if (trimmed.startsWith('```')) {
      const code: string[] = [];
      i += 1;
      while (i < lines.length && !lines[i].trim().startsWith('```')) {
        code.push(lines[i]);
        i += 1;
      }
      blocks.push({ type: 'code', text: code.join('\n') });
      i += 1;
      continue;
    }

    const heading = trimmed.match(/^(#{1,6})\s+(.+)$/);
    if (heading) {
      blocks.push({
        type: 'heading',
        level: heading[1].length,
        text: heading[2].trim(),
      });
      i += 1;
      continue;
    }

    if (/^([-*_])(?:\s*\1){2,}\s*$/.test(trimmed)) {
      blocks.push({ type: 'divider' });
      i += 1;
      continue;
    }

    if (/^[-*+]\s+/.test(trimmed)) {
      const items: string[] = [];
      while (i < lines.length && /^[-*+]\s+/.test(lines[i].trim())) {
        items.push(lines[i].trim().replace(/^[-*+]\s+/, '').trim());
        i += 1;
      }
      blocks.push({ type: 'unordered-list', items });
      continue;
    }

    if (/^\d+[.)]\s+/.test(trimmed)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+[.)]\s+/.test(lines[i].trim())) {
        items.push(lines[i].trim().replace(/^\d+[.)]\s+/, '').trim());
        i += 1;
      }
      blocks.push({ type: 'ordered-list', items });
      continue;
    }

    if (/^>\s?/.test(trimmed)) {
      const quoteLines: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i].trim())) {
        quoteLines.push(lines[i].trim().replace(/^>\s?/, '').trim());
        i += 1;
      }
      blocks.push({ type: 'blockquote', text: quoteLines.join(' ') });
      continue;
    }

    const paragraph: string[] = [];
    while (i < lines.length && lines[i].trim() && !isBlockStart(lines[i].trim())) {
      paragraph.push(lines[i].trim());
      i += 1;
    }
    blocks.push({ type: 'paragraph', text: paragraph.join(' ') });
  }

  return blocks;
}

function isBlockStart(line: string): boolean {
  return (
    line.startsWith('```') ||
    /^(#{1,6})\s+/.test(line) ||
    /^([-*_])(?:\s*\1){2,}\s*$/.test(line) ||
    /^[-*+]\s+/.test(line) ||
    /^\d+[.)]\s+/.test(line) ||
    /^>\s?/.test(line)
  );
}

function renderBlock(block: MarkdownBlock, index: number): ReactNode {
  switch (block.type) {
    case 'heading': {
      if (block.level === 1) return <h1 key={index}>{renderInline(block.text)}</h1>;
      if (block.level === 2) return <h2 key={index}>{renderInline(block.text)}</h2>;
      if (block.level === 3) return <h3 key={index}>{renderInline(block.text)}</h3>;
      if (block.level === 4) return <h4 key={index}>{renderInline(block.text)}</h4>;
      if (block.level === 5) return <h5 key={index}>{renderInline(block.text)}</h5>;
      return <h6 key={index}>{renderInline(block.text)}</h6>;
    }
    case 'paragraph':
      return <p key={index}>{renderInline(block.text)}</p>;
    case 'unordered-list':
      return (
        <ul key={index}>
          {block.items.map((item, itemIndex) => (
            <li key={itemIndex}>{renderInline(item)}</li>
          ))}
        </ul>
      );
    case 'ordered-list':
      return (
        <ol key={index}>
          {block.items.map((item, itemIndex) => (
            <li key={itemIndex}>{renderInline(item)}</li>
          ))}
        </ol>
      );
    case 'blockquote':
      return <blockquote key={index}>{renderInline(block.text)}</blockquote>;
    case 'code':
      return (
        <pre key={index}>
          <code>{block.text}</code>
        </pre>
      );
    case 'divider':
      return <hr key={index} />;
  }
}

function renderInline(text: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  const pattern = /(!?\[[^\]]*]\([^)]+\)|`[^`]+`|\*\*[^*]+\*\*|__[^_]+__|\*[^*]+\*|_[^_]+_)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(text)) !== null) {
    if (match.index > lastIndex) {
      nodes.push(text.slice(lastIndex, match.index));
    }

    nodes.push(renderInlineToken(match[0], nodes.length));
    lastIndex = pattern.lastIndex;
  }

  if (lastIndex < text.length) {
    nodes.push(text.slice(lastIndex));
  }

  return nodes;
}

function renderInlineToken(token: string, key: number): ReactNode {
  const image = token.match(/^!\[([^\]]*)]\(([^)]+)\)$/);
  if (image) {
    const src = safeMediaURL(image[2]);
    if (!src) return image[1];
    return <img key={key} src={src} alt={image[1]} loading="lazy" />;
  }

  const link = token.match(/^\[([^\]]+)]\(([^)]+)\)$/);
  if (link) {
    const href = safeLinkURL(link[2]);
    if (!href) return link[1];
    return (
      <a key={key} href={href} target="_blank" rel="noopener noreferrer">
        {link[1]}
      </a>
    );
  }

  if (token.startsWith('`') && token.endsWith('`')) {
    return <code key={key}>{token.slice(1, -1)}</code>;
  }

  if ((token.startsWith('**') && token.endsWith('**')) || (token.startsWith('__') && token.endsWith('__'))) {
    return <strong key={key}>{renderInline(token.slice(2, -2))}</strong>;
  }

  if ((token.startsWith('*') && token.endsWith('*')) || (token.startsWith('_') && token.endsWith('_'))) {
    return <em key={key}>{renderInline(token.slice(1, -1))}</em>;
  }

  return token;
}

function safeLinkURL(raw: string): string | null {
  const value = raw.trim();
  try {
    const parsed = new URL(value);
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:' || parsed.protocol === 'mailto:') {
      return parsed.toString();
    }
  } catch {
    return null;
  }
  return null;
}

function safeMediaURL(raw: string): string | null {
  const value = raw.trim();
  try {
    const parsed = new URL(value);
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
      return parsed.toString();
    }
  } catch {
    return null;
  }
  return null;
}
