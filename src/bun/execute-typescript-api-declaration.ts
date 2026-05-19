import { EXECUTE_TYPESCRIPT_API_DECLARATION } from "../../generated/execute-typescript-api.generated";
import { canUseExecuteTypescriptApiNamespace, type SvvyActorKind } from "./actor-capabilities";
import type { WebProvider } from "./web-runtime/contracts";

export function buildExecuteTypescriptApiDeclaration(
  actor: SvvyActorKind,
  webProvider?: WebProvider,
): string {
  const sections = [EXECUTE_TYPESCRIPT_API_DECLARATION.trim()];
  if (canUseExecuteTypescriptApiNamespace(actor, "workflow")) {
    sections.push(buildHandlerWorkflowDeclaration());
  }
  if (canUseExecuteTypescriptApiNamespace(actor, "web") && webProvider?.checkReady().ready) {
    sections.push(buildActiveWebDeclaration(webProvider));
  }
  return sections.join("\n\n");
}

export function buildHandlerWorkflowDeclaration(): string {
  return [
    "/** Handler-only workflow discovery helpers. */",
    "interface SvvyApi {",
    "  workflow_list_assets(input?: WorkflowListAssetsInput): Promise<ToolResult<WorkflowListAssetsDetails>>;",
    "  workflow_list_models(): Promise<ToolResult<WorkflowListModelsDetails>>;",
    "}",
  ].join("\n");
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
    "  web_search(input: ActiveWebSearchInput): Promise<ToolResult<ActiveWebSearchOutput>>;",
    "  web_fetch(input: ActiveWebFetchInput): Promise<ToolResult<ActiveWebFetchOutput>>;",
    "}",
  ].join("\n");
}
