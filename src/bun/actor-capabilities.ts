export type SvvyActorKind = "orchestrator" | "handler" | "workflow-task";

export type ExecuteTypescriptApiNamespace =
  | "read"
  | "grep"
  | "find"
  | "ls"
  | "bash"
  | "cx"
  | "artifact"
  | "workflow"
  | "web";

export interface ExecuteTypescriptActorCapabilityProfile {
  actor: SvvyActorKind;
  executeTypescript: {
    apiNamespaces: Readonly<Record<ExecuteTypescriptApiNamespace, boolean>>;
  };
}

const BASE_EXECUTE_TYPESCRIPT_API_NAMESPACES: Readonly<
  Record<ExecuteTypescriptApiNamespace, boolean>
> = {
  read: true,
  grep: true,
  find: true,
  ls: true,
  bash: true,
  cx: true,
  artifact: true,
  workflow: false,
  web: true,
};

export const EXECUTE_TYPESCRIPT_ACTOR_CAPABILITY_PROFILES: Readonly<
  Record<SvvyActorKind, ExecuteTypescriptActorCapabilityProfile>
> = {
  orchestrator: {
    actor: "orchestrator",
    executeTypescript: {
      apiNamespaces: BASE_EXECUTE_TYPESCRIPT_API_NAMESPACES,
    },
  },
  handler: {
    actor: "handler",
    executeTypescript: {
      apiNamespaces: {
        ...BASE_EXECUTE_TYPESCRIPT_API_NAMESPACES,
        workflow: true,
      },
    },
  },
  "workflow-task": {
    actor: "workflow-task",
    executeTypescript: {
      apiNamespaces: BASE_EXECUTE_TYPESCRIPT_API_NAMESPACES,
    },
  },
};

export function getExecuteTypescriptActorCapabilityProfile(
  actor: SvvyActorKind,
): ExecuteTypescriptActorCapabilityProfile {
  return EXECUTE_TYPESCRIPT_ACTOR_CAPABILITY_PROFILES[actor];
}

export function canUseExecuteTypescriptApiNamespace(
  actor: SvvyActorKind,
  namespace: ExecuteTypescriptApiNamespace,
): boolean {
  return getExecuteTypescriptActorCapabilityProfile(actor).executeTypescript.apiNamespaces[
    namespace
  ];
}
