import { mkdirSync, realpathSync, statSync } from "node:fs";
import { join, resolve } from "node:path";

export function resolveWorkspaceCwd(): string {
  return canonicalizeWorkspaceCwd(
    process.env.SVVY_WORKSPACE_CWD ?? process.env.INIT_CWD ?? process.env.PWD ?? process.cwd(),
  );
}

export function canonicalizeWorkspaceCwd(cwd: string): string {
  const resolved = resolve(cwd);
  const stats = statSync(resolved);
  if (!stats.isDirectory()) {
    throw new Error(`Workspace path is not a directory: ${cwd}`);
  }
  return realpathSync.native(resolved);
}

export function getDefaultWorkspaceCwd(appDataDir: string): string {
  const defaultWorkspaceDir = join(appDataDir, "default-workspace");
  mkdirSync(defaultWorkspaceDir, { recursive: true });
  return canonicalizeWorkspaceCwd(defaultWorkspaceDir);
}
