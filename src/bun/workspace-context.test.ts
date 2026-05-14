import { afterEach, describe, expect, it } from "bun:test";
import { mkdtempSync, realpathSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { canonicalizeWorkspaceCwd, resolveWorkspaceCwd } from "./workspace-context";

const ORIGINAL_SVVY_WORKSPACE_CWD = process.env.SVVY_WORKSPACE_CWD;
const ORIGINAL_INIT_CWD = process.env.INIT_CWD;
const ORIGINAL_PWD = process.env.PWD;
const TEMP_DIRS: string[] = [];

afterEach(() => {
  restoreEnv("SVVY_WORKSPACE_CWD", ORIGINAL_SVVY_WORKSPACE_CWD);
  restoreEnv("INIT_CWD", ORIGINAL_INIT_CWD);
  restoreEnv("PWD", ORIGINAL_PWD);
  for (const dir of TEMP_DIRS.splice(0)) {
    rmSync(dir, { recursive: true, force: true });
  }
});

function restoreEnv(name: "SVVY_WORKSPACE_CWD" | "INIT_CWD" | "PWD", value: string | undefined) {
  if (value === undefined) {
    delete process.env[name];
    return;
  }

  process.env[name] = value;
}

describe("resolveWorkspaceCwd", () => {
  it("prefers the explicit svvy workspace override", () => {
    const svvyWorkspace = tempWorkspace("svvy-workspace");
    const initWorkspace = tempWorkspace("init-cwd");
    const pwdWorkspace = tempWorkspace("pwd-cwd");
    process.env.SVVY_WORKSPACE_CWD = svvyWorkspace;
    process.env.INIT_CWD = initWorkspace;
    process.env.PWD = pwdWorkspace;

    expect(resolveWorkspaceCwd()).toBe(realpathSync.native(svvyWorkspace));
  });

  it("falls back to INIT_CWD and PWD before process.cwd()", () => {
    const initWorkspace = tempWorkspace("init-cwd");
    const pwdWorkspace = tempWorkspace("pwd-cwd");
    delete process.env.SVVY_WORKSPACE_CWD;
    process.env.INIT_CWD = initWorkspace;
    process.env.PWD = pwdWorkspace;

    expect(resolveWorkspaceCwd()).toBe(realpathSync.native(initWorkspace));

    delete process.env.INIT_CWD;
    expect(resolveWorkspaceCwd()).toBe(realpathSync.native(pwdWorkspace));
  });

  it("canonicalizes workspace cwd through realpath", () => {
    const workspace = tempWorkspace("canonical");

    expect(canonicalizeWorkspaceCwd(join(workspace, "."))).toBe(realpathSync.native(workspace));
  });
});

function tempWorkspace(name: string): string {
  const dir = mkdtempSync(join(tmpdir(), `svvy-${name}-`));
  TEMP_DIRS.push(dir);
  return dir;
}
