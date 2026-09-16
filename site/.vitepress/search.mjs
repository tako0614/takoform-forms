// Build-only word expansion. The browser keeps VitePress's default tokenizer;
// a Japanese query word can then match a term inside an unspaced sentence.
// Do not put a function in serializable themeConfig: VitePress 1.6 restores it
// with new Function, which is incompatible with a strict script-src CSP.
export function tokenize(text) {
  return Array.from(
    new Intl.Segmenter("ja", { granularity: "word" }).segment(text),
  )
    .filter((part) => part.isWordLike)
    .map((part) => part.segment);
}

function searchWords(text) {
  if (!/[\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Han}]/u.test(text))
    return "";
  return tokenize(text)
    .join(" ")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

export function renderForSearch(source, env, markdown) {
  const html = markdown.render(source, env);
  if (env.frontmatter?.search === false) return "";
  // Only search's private HTML is expanded. Published prose, code, IDs and
  // links are untouched. Keep heading labels intact in search results while
  // adding their words to the associated section's searchable body.
  return html.replace(
    /<h([1-6])\b[^>]*>[\s\S]*?<\/h\1>|<[^>]*>|[^<]+/gu,
    (part) => {
      if (/^<h[1-6]\b/u.test(part)) {
        const words = searchWords(part.replace(/<[^>]*>/gu, ""));
        return words ? `${part}<p>${words}</p>` : part;
      }
      if (part.startsWith("<")) return part;
      const words = searchWords(part);
      return words ? `${part} ${words} ` : part;
    },
  );
}
