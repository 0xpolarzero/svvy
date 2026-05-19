import {
  APIConnectionError,
  APIStatusError,
  APITimeoutError,
  AuthenticationError,
  FetchFormat,
  RateLimitError,
  TinyFish,
  type FetchError,
  type FetchGetContentsParams,
  type FetchResult,
  type SearchQueryParams,
} from "@tiny-fish/sdk";
import { TINYFISH_TOOL_CONTRACTS } from "../provider-contracts/tinyfish";
import { TINYFISH_WEB_PROMPT } from "../provider-prompts/tinyfish";
import type {
  WebArtifactRef,
  WebInvocationContext,
  WebProvider,
  WebProviderCapabilities,
  WebProviderPromptNotes,
  WebProviderReadyState,
  WebProviderToolContracts,
  WebProviderToolResult,
  WebToolName,
} from "../contracts";
import { assertPublicWebUrl, providerError, textResult } from "./shared";

export class TinyFishWebProvider implements WebProvider {
  readonly id = "tinyfish" as const;
  readonly label = "TinyFish";
  readonly capabilities: WebProviderCapabilities = {
    search: true,
    fetch: true,
    extraTools: [],
    supportsSiteSearch: false,
    supportsRecency: false,
    supportsRenderedFetch: true,
  };

  constructor(private readonly apiKey?: string) {}

  checkReady(): WebProviderReadyState {
    return this.apiKey
      ? { ready: true, providerId: this.id, label: this.label }
      : {
          ready: false,
          providerId: this.id,
          label: this.label,
          category: "provider_not_configured",
          message:
            "TinyFish web tools are unavailable until a TinyFish API key is configured in Settings.",
          missingRequirement: "TinyFish API key",
        };
  }

  getToolContracts(): WebProviderToolContracts {
    return TINYFISH_TOOL_CONTRACTS;
  }

  buildPromptNotes(): WebProviderPromptNotes {
    return {
      source: "src/bun/web-runtime/provider-prompts/tinyfish.ts",
      text: TINYFISH_WEB_PROMPT,
    };
  }

  async invoke(
    toolName: WebToolName,
    input: unknown,
    context: WebInvocationContext,
  ): Promise<WebProviderToolResult> {
    if (!this.apiKey) throw new Error("TinyFish API key is not configured.");
    return toolName === "web_search"
      ? this.search(input as SearchQueryParams)
      : this.fetch(input as FetchGetContentsParams, context);
  }

  private async search(input: SearchQueryParams): Promise<WebProviderToolResult> {
    try {
      const response = await this.client().search.query(input);
      return textResult(JSON.stringify({ providerId: this.id, ...response }, null, 2), {
        providerId: this.id,
        toolName: "web_search",
        status: "succeeded",
        query: input.query,
        resultCount: response.results.length,
        commandFacts: {
          providerId: this.id,
          toolName: "web_search",
          status: "succeeded",
          query: input.query,
          page: response.page,
          totalResults: response.total_results,
          resultCount: response.results.length,
        },
      });
    } catch (error) {
      throw mapTinyFishError(error);
    }
  }

  private async fetch(
    input: FetchGetContentsParams,
    context: WebInvocationContext,
  ): Promise<WebProviderToolResult> {
    const urls = input.urls.map(assertPublicWebUrl);
    try {
      const response = await this.client().fetch.getContents({
        ...input,
        urls,
        format: input.format ?? FetchFormat.Markdown,
      });
      const artifactSet = createTinyFishFetchArtifactSet(
        context,
        response.results,
        response.errors,
      );
      return textResult(
        JSON.stringify(
          {
            providerId: this.id,
            artifacts: artifactSet.artifacts,
            metadataArtifact: artifactSet.metadataArtifact,
            errors: response.errors,
            warnings: artifactSet.warnings,
          },
          null,
          2,
        ),
        {
          providerId: this.id,
          toolName: "web_fetch",
          status: "succeeded",
          url: urls[0],
          finalUrl: artifactSet.artifacts[0]?.finalUrl,
          fetchedAt: artifactSet.fetchedAt,
          format: input.format ?? FetchFormat.Markdown,
          artifacts: artifactSet.artifacts,
          metadataArtifact: artifactSet.metadataArtifact,
          warnings: artifactSet.warnings,
          commandFacts: artifactSet.commandFacts,
        },
      );
    } catch (error) {
      throw mapTinyFishError(error);
    }
  }

  private client(): TinyFish {
    return new TinyFish({ apiKey: this.apiKey, maxRetries: 2 });
  }
}

function createTinyFishFetchArtifactSet(
  context: WebInvocationContext,
  results: FetchResult[],
  errors: FetchError[],
): {
  fetchedAt: string;
  artifacts: WebArtifactRef[];
  metadataArtifact: WebArtifactRef;
  warnings: string[];
  commandFacts: Record<string, unknown>;
} {
  const fetchedAt = new Date().toISOString();
  const warnings = errors.length > 0 ? ["TinyFish returned per-URL fetch errors."] : [];
  const artifacts = results.map((result) => {
    const extension =
      result.format === FetchFormat.Json ? "json" : result.format === "html" ? "html" : "md";
    const contentArtifact = context.createArtifact({
      kind: result.format === FetchFormat.Json ? "json" : "text",
      name: `web-fetch.${extension}`,
      content: tinyFishFetchContent(result),
    });
    return {
      artifactId: contentArtifact.id,
      path: contentArtifact.path ?? "",
      url: result.url,
      finalUrl: result.final_url ?? undefined,
      title: result.title ?? undefined,
      format: result.format,
    };
  });
  const metadata = {
    providerId: "tinyfish",
    fetchedAt,
    artifacts,
    errors,
    warnings,
    results: results.map((result) => ({
      url: result.url,
      finalUrl: result.final_url,
      title: result.title,
      description: result.description,
      language: result.language,
      author: result.author,
      publishedDate: result.published_date,
      links: result.links,
      imageLinks: result.image_links,
      latencyMs: result.latency_ms,
      format: result.format,
    })),
  };
  const metadataArtifactRaw = context.createArtifact({
    kind: "json",
    name: "web-fetch.metadata.json",
    content: JSON.stringify(metadata, null, 2),
  });
  const metadataArtifact: WebArtifactRef = {
    artifactId: metadataArtifactRaw.id,
    path: metadataArtifactRaw.path ?? "",
    format: "json",
  };
  const commandFacts = {
    providerId: "tinyfish",
    toolName: "web_fetch",
    status: "succeeded",
    url: artifacts[0]?.url,
    finalUrl: artifacts[0]?.finalUrl,
    format: artifacts[0]?.format,
    fetchedAt,
    artifactPaths: artifacts.map((artifact) => artifact.path),
    artifactIds: artifacts.map((artifact) => artifact.artifactId),
    metadataArtifactPath: metadataArtifact.path,
    metadataArtifactId: metadataArtifact.artifactId,
    fetchErrorCount: errors.length,
    warnings,
  };
  return { fetchedAt, artifacts, metadataArtifact, warnings, commandFacts };
}

function tinyFishFetchContent(result: FetchResult): string {
  if (result.format === FetchFormat.Json) {
    return JSON.stringify(result.text ?? {}, null, 2);
  }
  return result.text ?? "";
}

function mapTinyFishError(error: unknown): Error {
  if (error instanceof AuthenticationError) {
    return providerError("provider_authentication_failed", error.message);
  }
  if (error instanceof RateLimitError) {
    return providerError("rate_limited", error.message);
  }
  if (error instanceof APITimeoutError) {
    return providerError("timeout", error.message);
  }
  if (error instanceof APIConnectionError) {
    return providerError("provider_unavailable", error.message);
  }
  if (error instanceof APIStatusError) {
    return providerError(
      error.statusCode >= 500 ? "provider_unavailable" : "fetch_failed",
      error.message,
    );
  }
  if (error instanceof Error) {
    return error;
  }
  return new Error(String(error));
}
