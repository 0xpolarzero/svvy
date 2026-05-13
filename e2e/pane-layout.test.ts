import { beforeAll, expect, setDefaultTimeout, test } from "bun:test";
import { connect, type Page } from "electrobun-browser-tools";
import { ensureBuilt, type SvvyApp, withSvvyApp } from "./harness";
import { assistantTextMessage, seedSessions, userMessage } from "./support";

setDefaultTimeout(120_000);

const PANE_LAYOUT_BRIDGE_TIMEOUT_MS = 10_000;

beforeAll(async () => {
  await ensureBuilt();
});

async function createPaneLayoutPage(app: SvvyApp): Promise<Page> {
  const driver = await connect({
    ...(app.bridgeUrl ? { url: app.bridgeUrl } : { app: app.appId }),
    timeout: PANE_LAYOUT_BRIDGE_TIMEOUT_MS,
  });
  return driver.page("active");
}

async function waitForDockviewShell(page: Page): Promise<void> {
  await page.locator('[data-testid="dockview-workbench"]').waitFor({ state: "visible" });
  await page.locator('[data-testid="workspace-pane"]').first().waitFor({ state: "visible" });
}

async function runPaneCommand(page: Page, command: string): Promise<void> {
  await page.getByRole("button", { name: "Open command palette" }).click({ force: true });
  await page.getByTestId("command-palette").waitFor({ state: "visible" });
  await page.locator("[data-cmdk-input]").fill(command);
  await page.locator("[data-cmdk-input]").press("Enter");
  await page.getByTestId("command-palette").waitFor({ state: "hidden" });
}

test("opens, duplicates, resizes, and closes Dockview panels without custom pane chrome", async () => {
  await withSvvyApp(
    {
      beforeLaunch: async ({ homeDir: seededHome, workspaceDir }) => {
        await seedSessions(
          seededHome,
          [
            {
              title: "Dockview Layout Seed",
              messages: [
                userMessage("Seed Dockview layout session.", 1_730_000_000_000),
                assistantTextMessage("Dockview layout session is ready.", {
                  timestamp: 1_730_000_000_001,
                }),
              ],
            },
          ],
          workspaceDir,
        );
      },
    },
    async (app) => {
      const page = await createPaneLayoutPage(app);
      await waitForDockviewShell(page);

      expect(await page.locator('[data-testid="workspace-pane"]').count()).toBe(1);

      await runPaneCommand(page, "Duplicate Pane Right");
      await page.locator('[data-testid="workspace-pane"]').nth(1).waitFor({ state: "visible" });
      expect(await page.locator('[data-testid="workspace-pane"]').count()).toBe(2);

      const firstBox = await page.locator('[data-testid="workspace-pane"]').nth(0).boundingBox();
      const secondBox = await page.locator('[data-testid="workspace-pane"]').nth(1).boundingBox();
      expect(firstBox).not.toBeNull();
      expect(secondBox).not.toBeNull();
      expect(Math.abs(firstBox!.x - secondBox!.x) + Math.abs(firstBox!.y - secondBox!.y)).toBeGreaterThan(20);

      await runPaneCommand(page, "Duplicate Pane Below");
      await page.locator('[data-testid="workspace-pane"]').nth(2).waitFor({ state: "visible" });
      expect(await page.locator('[data-testid="workspace-pane"]').count()).toBe(3);

      await runPaneCommand(page, "Close Pane");
      expect(await page.locator('[data-testid="workspace-pane"]').count()).toBe(2);
    },
  );
});
