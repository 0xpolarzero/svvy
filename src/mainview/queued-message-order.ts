export type QueuedMessageOrderItem = {
  id: string;
};

export function reorderQueuedMessageItems<T extends QueuedMessageOrderItem>(
  items: readonly T[],
  movingId: string | null,
  beforeId: string | null,
): T[] {
  if (!movingId) {
    return [...items];
  }

  const moving = items.find((item) => item.id === movingId);
  if (!moving || movingId === beforeId) {
    return [...items];
  }

  const remaining = items.filter((item) => item.id !== movingId);
  const beforeIndex = beforeId
    ? remaining.findIndex((item) => item.id === beforeId)
    : remaining.length;
  if (beforeIndex < 0) {
    return [...items];
  }

  return [...remaining.slice(0, beforeIndex), moving, ...remaining.slice(beforeIndex)];
}

export function queuedMessageOrderChanged<T extends QueuedMessageOrderItem>(
  items: readonly T[],
  movingId: string | null,
  beforeId: string | null,
): boolean {
  const reordered = reorderQueuedMessageItems(items, movingId, beforeId);
  return reordered.some((item, index) => item.id !== items[index]?.id);
}
