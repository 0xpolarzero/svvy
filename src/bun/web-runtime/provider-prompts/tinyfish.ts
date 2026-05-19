export const TINYFISH_WEB_PROMPT = [
  "TinyFish provider notes:",
  "- TinyFish Search is exposed through the official TypeScript SDK `search.query` contract.",
  "- TinyFish Fetch is exposed through the official TypeScript SDK `fetch.getContents` contract.",
  "- Use `web_search` for ranked public web discovery; use `web_fetch` with a `urls` array for selected source URLs.",
  "- TinyFish Fetch can return markdown, HTML, or JSON and can include extracted links or image links when requested.",
  "- svvy passes TinyFish credentials only through scoped SDK runtime configuration and does not write them into global TinyFish config files.",
  "- After `web_fetch`, inspect the returned artifact paths with file tools when you need page details.",
].join("\n");
