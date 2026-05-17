import { describe, expect, test } from "bun:test";
import { readFile } from "node:fs/promises";
import { parse } from "svelte/compiler";

type SvelteAstNode = {
  type?: string;
  name?: string;
  attributes?: SvelteAstNode[];
  value?: SvelteAstNode | SvelteAstNode[] | string | boolean;
  data?: string;
  [key: string]: unknown;
};

function walkSvelteAst(node: unknown, visit: (node: SvelteAstNode) => void): void {
  if (!node || typeof node !== "object") return;

  const astNode = node as SvelteAstNode;
  visit(astNode);

  for (const value of Object.values(astNode)) {
    if (Array.isArray(value)) {
      for (const child of value) {
        walkSvelteAst(child, visit);
      }
      continue;
    }
    walkSvelteAst(value, visit);
  }
}

function readStaticAttribute(attribute: SvelteAstNode | undefined): string | null {
  if (!attribute) return null;
  if (typeof attribute.value === "string") return attribute.value;
  if (Array.isArray(attribute.value) && attribute.value.length === 1) {
    const [value] = attribute.value;
    return typeof value?.data === "string" ? value.data : null;
  }
  return null;
}

describe("ArtifactsPanel", () => {
  test("sandboxes the visible HTML artifact preview iframe with only scripts allowed", async () => {
    const source = await readFile(new URL("./ArtifactsPanel.svelte", import.meta.url), "utf8");
    const ast = parse(source, { modern: true }) as unknown as SvelteAstNode;
    const iframes: SvelteAstNode[] = [];

    walkSvelteAst(ast, (node) => {
      if (node.type === "RegularElement" && node.name === "iframe") {
        iframes.push(node);
      }
    });

    const visiblePreview = iframes.find((iframe) =>
      iframe.attributes?.some(
        (attribute) =>
          attribute.name === "class" &&
          readStaticAttribute(attribute)?.split(/\s+/).includes("html-preview"),
      ),
    );
    const sandbox = readStaticAttribute(
      visiblePreview?.attributes?.find((attribute) => attribute.name === "sandbox"),
    );

    expect(sandbox).toBe("allow-scripts");
  });
});
