import { getModels, getProviders, modelsAreEqual, type Model } from "@mariozechner/pi-ai";
import { discoverModels } from "./model-discovery";
import type { ChatStorage } from "./chat-storage";

export type ModelComboboxOption = {
  value: string;
  label: string;
  triggerLabel?: string;
  searchText?: string;
  disabled?: boolean;
  model: Model<any>;
};

export function getModelComboboxValue(model: Model<any>): string {
  return `${model.provider}:${model.id}`;
}

export async function listModelComboboxOptions(
  currentModel: Model<any>,
  storage: ChatStorage,
  configuredProviders: string[],
): Promise<ModelComboboxOption[]> {
  const providerAllowlist = new Set([currentModel.provider, ...configuredProviders]);
  const entries: Array<{ id: string; provider: string; model: Model<any> }> = [];

  for (const provider of getProviders()) {
    for (const model of getModels(provider)) {
      entries.push({ id: model.id, provider, model });
    }
  }

  try {
    const customProviders = await storage.customProviders.getAll();
    for (const provider of customProviders) {
      if (
        (provider.type === "ollama" ||
          provider.type === "llama.cpp" ||
          provider.type === "vllm" ||
          provider.type === "lmstudio") &&
        provider.baseUrl
      ) {
        try {
          const discovered = await discoverModels(provider.type, provider.baseUrl, provider.apiKey);
          entries.push(
            ...discovered.map((model) => ({
              id: model.id,
              provider: provider.name,
              model: { ...model, provider: provider.name },
            })),
          );
        } catch (error) {
          console.debug(`Failed to discover models for ${provider.name}:`, error);
        }
        continue;
      }

      if (!provider.models) continue;
      entries.push(
        ...provider.models.map((model) => ({
          id: model.id,
          provider: provider.name,
          model: { ...model, provider: provider.name },
        })),
      );
    }
  } catch (error) {
    console.error("Failed to load custom providers:", error);
  }

  return entries
    .filter((entry) => providerAllowlist.has(entry.provider))
    .toSorted((left, right) => {
      const leftIsCurrent = modelsAreEqual(currentModel, left.model);
      const rightIsCurrent = modelsAreEqual(currentModel, right.model);
      if (leftIsCurrent && !rightIsCurrent) return -1;
      if (!leftIsCurrent && rightIsCurrent) return 1;
      const providerComparison = left.provider.localeCompare(right.provider);
      return providerComparison === 0 ? left.model.name.localeCompare(right.model.name) : providerComparison;
    })
    .map((entry) => ({
      value: getModelComboboxValue(entry.model),
      label: entry.model.name,
      triggerLabel: entry.model.name,
      searchText: `${entry.model.name} ${entry.id} ${entry.provider}`,
      model: entry.model,
    }));
}
