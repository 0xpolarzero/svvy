import type { WebProvider, WebProviderId, WebProviderSecrets, WebSettings } from "./contracts";
import { FirecrawlWebProvider } from "./providers/firecrawl";
import { TinyFishWebProvider } from "./providers/tinyfish";

export const WEB_PROVIDER_LABELS: Record<WebProviderId, string> = {
  tinyfish: "TinyFish",
  firecrawl: "Firecrawl",
};

export function createWebProvider(
  settings: WebSettings,
  secrets: WebProviderSecrets = {},
): WebProvider | undefined {
  if (settings.provider === "tinyfish") return new TinyFishWebProvider(secrets.tinyfishApiKey);
  if (settings.provider === "firecrawl") return new FirecrawlWebProvider(secrets.firecrawlApiKey);
  return undefined;
}

export function normalizeWebProviderId(value: string | null | undefined): WebProviderId | null {
  return value === "tinyfish" || value === "firecrawl" ? value : null;
}
