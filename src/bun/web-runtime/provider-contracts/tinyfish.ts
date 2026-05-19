import {
  fetchGetContentsParamsSchema,
  searchQueryParamsSchema,
  type FetchError,
  type FetchResult,
  type SearchResult,
} from "@tiny-fish/sdk";
import type { TSchema } from "typebox";
import { z } from "zod";
import type { WebProviderToolContracts } from "../contracts";

export type TinyFishWebSearchInput = z.infer<typeof searchQueryParamsSchema>;
export type TinyFishWebFetchInput = z.infer<typeof fetchGetContentsParamsSchema>;
export type TinyFishWebSearchOutput = {
  providerId: "tinyfish";
  query: string;
  page: number;
  totalResults: number;
  results: SearchResult[];
};
export type TinyFishWebFetchOutput = {
  providerId: "tinyfish";
  artifacts: Array<{
    artifactId: string;
    path: string;
    url: string;
    finalUrl?: string;
    title?: string;
    format: "markdown" | "html" | "json";
  }>;
  metadataArtifact: { artifactId: string; path: string; format: "json" };
  errors: FetchError[];
  warnings?: string[];
};

export const tinyfishSearchInputSchema = z.toJSONSchema(
  searchQueryParamsSchema,
) as unknown as TSchema;
export const tinyfishFetchInputSchema = z.toJSONSchema(
  fetchGetContentsParamsSchema,
) as unknown as TSchema;

export const TINYFISH_TOOL_CONTRACTS: WebProviderToolContracts = {
  search: {
    name: "web.search",
    description: "Query TinyFish Search using the official TinyFish TypeScript SDK schema.",
    inputSchema: tinyfishSearchInputSchema,
    outputTypeName: "TinyFishWebSearchOutput",
    inputTypeDeclaration:
      "interface ActiveWebSearchInput { query: string; location?: string; language?: string; page?: number };",
    outputTypeDeclaration:
      'interface ActiveWebSearchOutput { providerId: "tinyfish"; query: string; page: number; totalResults: number; results: Array<{ position: number; site_name: string; snippet: string; title: string; url: string }> };',
  },
  fetch: {
    name: "web.fetch",
    description:
      "Fetch rendered page content through the official TinyFish TypeScript SDK and write fetched bodies to svvy artifacts.",
    inputSchema: tinyfishFetchInputSchema,
    outputTypeName: "TinyFishWebFetchOutput",
    inputTypeDeclaration:
      'interface ActiveWebFetchInput { urls: string[]; format?: "markdown" | "html" | "json"; links?: boolean; image_links?: boolean };',
    outputTypeDeclaration:
      'interface ActiveWebFetchOutput { providerId: "tinyfish"; artifacts: Array<{ artifactId: string; path: string; url: string; finalUrl?: string; title?: string; format: "markdown" | "html" | "json" }>; metadataArtifact: { artifactId: string; path: string; format: "json" }; errors: Array<{ url: string; error: string }>; warnings?: string[] };',
  },
};

export type TinyFishFetchResult = FetchResult;
