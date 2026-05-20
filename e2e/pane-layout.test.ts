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
  await expectNoUnavailablePane(page);
}

async function expectNoUnavailablePane(page: Page): Promise<void> {
  expect(await page.locator(".dockview-empty-panel").count()).toBe(0);
  expect((await page.locator("body").textContent()).includes("Surface unavailable")).toBe(false);
}

async function waitForWorkspacePaneCount(
  page: Page,
  expectedCount: number,
  timeoutMs = 15_000,
): Promise<void> {
  const panes = page.locator('[data-testid="workspace-pane"]');
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if ((await panes.count()) === expectedCount) {
      return;
    }
    await Bun.sleep(100);
  }
  throw new Error(`Timed out waiting for ${expectedCount} visible workspace panes.`);
}

async function waitForDockviewTabCount(
  page: Page,
  expectedCount: number,
  timeoutMs = 15_000,
): Promise<void> {
  const tabs = page.locator(".dockview-surface-tab");
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if ((await tabs.count()) === expectedCount) {
      return;
    }
    await Bun.sleep(100);
  }
  throw new Error(`Timed out waiting for ${expectedCount} Dockview tabs.`);
}

async function clickPaneAction(page: Page, name: string): Promise<void> {
  const actionClass =
    name === "Duplicate pane right"
      ? "action-split-right"
      : name === "Duplicate pane below"
        ? "action-split-below"
        : "action-close";
  await page.locator(`.dockview-surface-action.${actionClass}`).first().click({ force: true });
}

async function clickSessionByTitle(page: Page, title: string): Promise<void> {
  const sessionButton = page
    .locator(".session-main")
    .filter({
      has: page.locator("strong").filter({ hasText: title }),
    })
    .first();
  await sessionButton.waitFor({ state: "visible" });
  await sessionButton.click({ force: true });
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

      await waitForWorkspacePaneCount(page, 0);
      await waitForDockviewTabCount(page, 0);
      await clickSessionByTitle(page, "Dockview Layout Seed");
      await waitForWorkspacePaneCount(page, 1);
      await waitForDockviewTabCount(page, 1);
      await expectNoUnavailablePane(page);

      await clickPaneAction(page, "Duplicate pane right");
      await waitForWorkspacePaneCount(page, 2);
      await waitForDockviewTabCount(page, 2);
      await expectNoUnavailablePane(page);

      const firstBox = await page.locator('[data-testid="workspace-pane"]').nth(0).boundingBox();
      const secondBox = await page.locator('[data-testid="workspace-pane"]').nth(1).boundingBox();
      expect(firstBox).not.toBeNull();
      expect(secondBox).not.toBeNull();
      expect(
        Math.abs(firstBox!.x - secondBox!.x) + Math.abs(firstBox!.y - secondBox!.y),
      ).toBeGreaterThan(20);

      await clickPaneAction(page, "Duplicate pane below");
      await waitForWorkspacePaneCount(page, 3);
      await waitForDockviewTabCount(page, 3);
      await expectNoUnavailablePane(page);

      await clickPaneAction(page, "Close pane");
      await waitForWorkspacePaneCount(page, 2);
      await waitForDockviewTabCount(page, 2);
      await expectNoUnavailablePane(page);
    },
  );
});

test("opens session and workspace-scoped surface panes without unavailable Dockview panels", async () => {
  await withSvvyApp(
    {
      beforeLaunch: async ({ homeDir: seededHome, workspaceDir }) => {
        await seedSessions(
          seededHome,
          [
            {
              title: "First Pane Target",
              messages: [
                userMessage("Seed first pane target.", 1_730_000_000_100),
                assistantTextMessage("First pane target is ready.", {
                  timestamp: 1_730_000_000_101,
                }),
              ],
            },
            {
              title: "Second Pane Target",
              messages: [
                userMessage("Seed second pane target.", 1_730_000_000_200),
                assistantTextMessage("Second pane target is ready.", {
                  timestamp: 1_730_000_000_201,
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

      await waitForWorkspacePaneCount(page, 0);
      await waitForDockviewTabCount(page, 0);
      const openedSessionTitle = "First Pane Target";
      await clickSessionByTitle(page, openedSessionTitle);
      await waitForWorkspacePaneCount(page, 1);
      await waitForDockviewTabCount(page, 1);
      expect(openedSessionTitle).toMatch(/Pane Target/);
      await expectNoUnavailablePane(page);

      await page
        .getByRole("button", { name: "Open workflows" })
        .filter({ visible: true })
        .first()
        .click({ force: true });
      await page.locator(".saved-workflow-library").waitFor({
        state: "visible",
      });
      await waitForDockviewTabCount(page, 2);
      await expectNoUnavailablePane(page);

      await page
        .getByRole("button", { name: "Open app logs" })
        .filter({ visible: true })
        .first()
        .click({ force: true });
      await page.locator(".app-logs-pane").waitFor({ state: "visible" });
      await waitForDockviewTabCount(page, 3);
      await expectNoUnavailablePane(page);
    },
  );
});
