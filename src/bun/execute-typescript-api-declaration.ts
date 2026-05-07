import { EXECUTE_TYPESCRIPT_API_DECLARATION } from "../../generated/execute-typescript-api.generated";
import type { WebProvider } from "./web-runtime/contracts";

export function buildExecuteTypescriptApiDeclaration(webProvider?: WebProvider): string {
  if (!webProvider?.checkReady().ready) {
    return EXECUTE_TYPESCRIPT_API_DECLARATION.trim();
  }
  return [EXECUTE_TYPESCRIPT_API_DECLARATION.trim(), buildActiveWebDeclaration(webProvider)].join(
    "\n\n",
  );
}

export function buildActiveWebDeclaration(webProvider: WebProvider): string {
  const contracts = webProvider.getToolContracts();
  return [
    "/** Active ready web provider contract selected from checked-in provider contracts. */",
    contracts.search.inputTypeDeclaration,
    contracts.search.outputTypeDeclaration,
    contracts.fetch.inputTypeDeclaration,
    contracts.fetch.outputTypeDeclaration,
    "interface SvvyApi {",
    "  web: {",
    "    search(input: ActiveWebSearchInput): Promise<ToolResult<ActiveWebSearchOutput>>;",
    "    fetch(input: ActiveWebFetchInput): Promise<ToolResult<ActiveWebFetchOutput>>;",
    "  };",
    "}",
  ].join("\n");
}
