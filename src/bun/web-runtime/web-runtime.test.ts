import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "bun:test";
import { createWebProvider } from "./provider-registry";
import { buildWebPromptContext } from "./prompt-context";
import { createWebTools } from "./tools";

describe("web runtime", () => {
  it("resolves provider readiness without leaking API keys into prompt context", () => {
    expect(createWebProvider({ provider: null })).toBeUndefined();

    const noProviderPrompt = buildWebPromptContext("orchestrator");
    expect(noProviderPrompt).toContain("Selected Web Provider: none");
    expect(noProviderPrompt).toContain("Web tools available: no");
    expect(noProviderPrompt).toContain(
      "No `web_search` or `web_fetch` direct tools or `api.web_*` helpers",
    );

    const tinyfish = createWebProvider({ provider: "tinyfish" });
    expect(tinyfish!.checkReady()).toMatchObject({
      ready: false,
      providerId: "tinyfish",
      missingRequirement: "TinyFish API key",
    });

    const configured = createWebProvider(
      { provider: "firecrawl" },
      { firecrawlApiKey: "fc-secret-value" },
    );
    const prompt = buildWebPromptContext("orchestrator", configured);
    expect(prompt).toContain("Selected Web Provider: Firecrawl");
    expect(prompt).toContain("web_fetch");
    expect(prompt).toContain("artifact-backed");
    expect(prompt).not.toContain("fc-secret-value");
  });

  it("does not register hosted provider tools until required keys exist", () => {
    const unavailable = createWebTools({
      cwd: mkdtempSync(join(tmpdir(), "svvy-web-unavailable-")),
      runtime: { current: null },
      provider: createWebProvider({ provider: "tinyfish" })!,
      store: { createArtifact: () => ({ id: "artifact" }) },
    });
    expect(unavailable.map((tool) => tool.name)).toEqual([]);

    const available = createWebTools({
      cwd: mkdtempSync(join(tmpdir(), "svvy-web-available-")),
      runtime: { current: null },
      provider: createWebProvider({ provider: "firecrawl" }, { firecrawlApiKey: "fc-key" })!,
      store: { createArtifact: () => ({ id: "artifact" }) },
    });
    expect(available.map((tool) => tool.name)).toEqual(["web_search", "web_fetch"]);
    expect(JSON.stringify(available[0]?.parameters)).toContain("scrapeOptions");
  });

  it("firecrawl fetch writes artifact-backed output and rejects private URLs", async () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-web-fetch-"));
    const artifacts: Array<{ id: string; path: string; content: string }> = [];
    const previousFetch = globalThis.fetch;
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          data: {
            markdown: "Fetched evidence",
            url: "https://example.com/docs",
            metadata: { title: "Docs" },
          },
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      )) as unknown as typeof fetch;
    try {
      const tools = createWebTools({
        cwd: root,
        runtime: { current: null },
        provider: createWebProvider({ provider: "firecrawl" }, { firecrawlApiKey: "fc-key" })!,
        store: {
          createArtifact(input) {
            const id = `artifact-${artifacts.length + 1}`;
            const path = join(root, `${id}-${input.name}`);
            writeFileSync(path, input.content ?? "");
            artifacts.push({ id, path, content: input.content ?? "" });
            return { id, path };
          },
        },
      });
      const fetchTool = tools.find((tool) => tool.name === "web_fetch");
      expect(fetchTool).toBeTruthy();
      const result = await fetchTool!.execute("structured-command:web-command", {
        url: "https://example.com/docs",
      });
      expect(result.details.status).toBe("succeeded");
      expect(result.details.artifacts?.[0]?.path).toContain("web-fetch");
      expect(result.content[0]?.type).toBe("text");
      expect((result.content[0] as { text: string }).text).not.toContain("Fetched evidence");
      expect(readFileSync(result.details.artifacts![0]!.path, "utf8")).toContain(
        "Fetched evidence",
      );
      expect(result.details.commandFacts).toMatchObject({
        providerId: "firecrawl",
        toolName: "web_fetch",
        metadataArtifactId: "artifact-2",
      });

      const rejected = await fetchTool!.execute("structured-command:web-command-2", {
        url: "http://127.0.0.1:3000",
      });
      expect(rejected.details.status).toBe("failed");
      expect(rejected.details.error?.category).toBe("invalid_url");
    } finally {
      globalThis.fetch = previousFetch;
    }
  });

  it("tinyfish uses the SDK fetch shape and writes each fetched URL to artifacts", async () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-web-tinyfish-"));
    const artifacts: Array<{ id: string; path: string; content: string }> = [];
    const requests: Array<{ url: string; init?: RequestInit }> = [];
    const previousFetch = globalThis.fetch;
    globalThis.fetch = (async (url: RequestInfo | URL, init?: RequestInit) => {
      requests.push({ url: String(url), init });
      return new Response(
        JSON.stringify({
          results: [
            {
              url: "https://example.com/a",
              final_url: "https://example.com/a",
              title: "A",
              description: null,
              language: "en",
              author: null,
              published_date: null,
              latency_ms: 12,
              format: "markdown",
              text: "TinyFish A",
            },
            {
              url: "https://example.com/b",
              final_url: "https://example.com/b",
              title: "B",
              description: null,
              language: "en",
              author: null,
              published_date: null,
              links: ["https://example.com/b/ref"],
              latency_ms: 18,
              format: "markdown",
              text: "TinyFish B",
            },
          ],
          errors: [{ url: "https://example.com/missing", error: "not found" }],
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      );
    }) as unknown as typeof fetch;
    try {
      const tools = createWebTools({
        cwd: root,
        runtime: { current: null },
        provider: createWebProvider({ provider: "tinyfish" }, { tinyfishApiKey: "tf-key" })!,
        store: {
          createArtifact(input) {
            const id = `artifact-${artifacts.length + 1}`;
            const path = join(root, `${id}-${input.name}`);
            writeFileSync(path, input.content ?? "");
            artifacts.push({ id, path, content: input.content ?? "" });
            return { id, path };
          },
        },
      });
      const fetchTool = tools.find((tool) => tool.name === "web_fetch");
      expect(fetchTool?.parameters).toMatchObject({
        properties: {
          urls: { type: "array", minItems: 1, maxItems: 10 },
          image_links: { type: "boolean" },
        },
      });
      const result = await fetchTool!.execute("structured-command:tinyfish-fetch", {
        urls: ["https://example.com/a", "https://example.com/b"],
        links: true,
      });
      expect(requests[0]?.url).toBe("https://agent.tinyfish.ai/v1/fetch");
      expect(JSON.parse(String(requests[0]?.init?.body))).toMatchObject({
        urls: ["https://example.com/a", "https://example.com/b"],
        format: "markdown",
        links: true,
      });
      expect(result.details.status).toBe("succeeded");
      expect(
        result.details.artifacts?.map((artifact: { title?: string }) => artifact.title),
      ).toEqual(["A", "B"]);
      expect(result.details.commandFacts).toMatchObject({
        providerId: "tinyfish",
        toolName: "web_fetch",
        metadataArtifactId: "artifact-3",
        fetchErrorCount: 1,
      });
      expect((result.content[0] as { text: string }).text).not.toContain("TinyFish A");
      expect(artifacts.map((artifact) => artifact.content)).toEqual([
        "TinyFish A",
        "TinyFish B",
        expect.stringContaining("not found"),
      ]);
    } finally {
      globalThis.fetch = previousFetch;
    }
  });
});
