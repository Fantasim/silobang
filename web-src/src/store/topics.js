import { signal, computed } from '@preact/signals';
import { api } from '@services/api';

// State
export const topics = signal([]);
export const serviceInfo = signal(null);
export const topicsLoading = signal(false);
export const topicsError = signal(null);

// Computed
export const healthyTopics = computed(() =>
  topics.value.filter(t => t.healthy)
);

export const unhealthyTopics = computed(() =>
  topics.value.filter(t => !t.healthy)
);

// Actions
export async function fetchTopics() {
  topicsLoading.value = true;
  topicsError.value = null;

  try {
    const data = await api.getTopics();
    topics.value = data.topics || [];
    serviceInfo.value = data.service || null;
  } catch (err) {
    topicsError.value = err.message;
  } finally {
    topicsLoading.value = false;
  }
}

export async function createTopic(name) {
  try {
    await api.createTopic(name);
    await fetchTopics(); // Refresh list
    return { success: true };
  } catch (err) {
    return { success: false, error: err.message };
  }
}

export function getTopicByName(name) {
  return topics.value.find(t => t.name === name);
}
