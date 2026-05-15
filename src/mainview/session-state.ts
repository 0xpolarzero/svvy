import type {
  WorkspaceSessionNavigationReadModel,
  WorkspaceSessionNavigationSectionState,
  WorkspaceSessionSummary,
} from "../shared/workspace-contract";

function descendingTimestamp(
  left: string | null | undefined,
  right: string | null | undefined,
): number {
  return new Date(right ?? 0).getTime() - new Date(left ?? 0).getTime();
}

export function sortVisibleSessionsByRecency(
  sessions: WorkspaceSessionSummary[],
): WorkspaceSessionSummary[] {
  return sessions.toSorted(
    (left, right) => new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime(),
  );
}

export function buildWorkspaceSessionNavigation(
  sessions: WorkspaceSessionSummary[],
  collapsed = true,
): WorkspaceSessionNavigationReadModel {
  const sections: WorkspaceSessionNavigationReadModel["sections"] = {
    pinned: { collapsed: false, sizePx: DEFAULT_SESSION_SECTION_SIZES.pinned },
    active: { collapsed: false, sizePx: DEFAULT_SESSION_SECTION_SIZES.active },
    archived: { collapsed, sizePx: DEFAULT_SESSION_SECTION_SIZES.archived },
  };

  return {
    pinnedSessions: sessions
      .filter((session) => session.isPinned && !session.isArchived)
      .toSorted((left, right) => descendingTimestamp(left.pinnedAt, right.pinnedAt)),
    activeSessions: sessions
      .filter((session) => !session.isPinned && !session.isArchived)
      .toSorted((left, right) => descendingTimestamp(left.updatedAt, right.updatedAt)),
    sections,
    archived: {
      collapsed,
      sessions: sessions
        .filter((session) => session.isArchived)
        .toSorted((left, right) => descendingTimestamp(left.archivedAt, right.archivedAt)),
    },
  };
}

export function flattenWorkspaceSessionNavigation(
  navigation: WorkspaceSessionNavigationReadModel,
): WorkspaceSessionSummary[] {
  return [
    ...navigation.pinnedSessions,
    ...navigation.activeSessions,
    ...navigation.archived.sessions,
  ];
}

export const DEFAULT_SESSION_SECTION_SIZES = {
  pinned: 150,
  active: 260,
  archived: 190,
} satisfies Record<string, number>;

export function getDefaultSessionNavigationSectionState(
  section: keyof typeof DEFAULT_SESSION_SECTION_SIZES,
): WorkspaceSessionNavigationSectionState {
  return {
    collapsed: section === "archived",
    sizePx: DEFAULT_SESSION_SECTION_SIZES[section],
  };
}
