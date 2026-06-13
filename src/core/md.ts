// Markdown helpers shared by parsers/scanners.

/**
 * Blank out HTML comment regions (<!-- ... -->) while preserving line count, so that example
 * snippets in template stubs are never parsed as real content and reported line numbers stay
 * accurate against the original file.
 */
export function stripHtmlComments(text: string): string {
  return text.replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, ""));
}
