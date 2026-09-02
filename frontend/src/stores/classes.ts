import { defineStore } from "pinia";
import { api, type ClassItem } from "@/api/client";

function cloneClassItem(item: ClassItem): ClassItem {
  return {
    id: item.id,
    name: item.name,
    joinCode: item.joinCode,
    joinCodeStatus: item.joinCodeStatus,
    joinCodeHint: item.joinCodeHint,
    registrationEnabled: item.registrationEnabled,
    registrationExpiresAt: item.registrationExpiresAt,
    examLockEnabled: item.examLockEnabled,
  };
}

export const useClassesStore = defineStore("classes", {
  state: () => ({
    classes: [] as ClassItem[],
    loading: false,
  }),
  actions: {
    clear() {
      this.classes = [];
      this.loading = false;
    },
    apply(items: ClassItem[]) {
      this.classes = items.map(cloneClassItem);
    },
    async load(force = false) {
      if (this.classes.length > 0 && !force) {
        return this.classes;
      }
      this.loading = true;
      try {
        const response = await api.classes();
        this.apply(response.classes ?? []);
        return this.classes;
      } finally {
        this.loading = false;
      }
    },
    async create(name: string) {
      const created = await api.createClass(name);
      this.classes = [...this.classes, cloneClassItem(created)];
      return created;
    },
    async update(classId: number, name: string) {
      const updated = await api.updateClass(classId, name);
      this.classes = this.classes.map((item) => (item.id === classId ? cloneClassItem(updated) : item));
      return updated;
    },
    async remove(classId: number) {
      await api.deleteClass(classId);
      this.classes = this.classes.filter((item) => item.id !== classId);
    },
    async refreshJoinCode(classId: number) {
      const result = await api.refreshClassJoinCode(classId);
      this.classes = this.classes.map((item) =>
        item.id === classId
          ? {
              ...item,
              joinCode: result.joinCode,
              joinCodeHint: result.joinCodeHint,
              joinCodeStatus: result.joinCodeStatus,
              registrationEnabled: result.registrationEnabled,
              registrationExpiresAt: result.registrationExpiresAt,
            }
          : item,
      );
      return result;
    },
    async updateRegistration(classId: number, enabled: boolean) {
      const result = await api.updateClassRegistration(classId, enabled);
      this.classes = this.classes.map((item) =>
        item.id === classId
          ? {
              ...item,
              joinCode: result.joinCode,
              joinCodeHint: result.joinCodeHint,
              joinCodeStatus: result.joinCodeStatus,
              registrationEnabled: result.registrationEnabled,
              registrationExpiresAt: result.registrationExpiresAt,
            }
          : item,
      );
      return result;
    },
    async updateExamLock(classId: number, enabled: boolean) {
      const updated = await api.updateClassExamLock(classId, enabled);
      this.classes = this.classes.map((item) => (item.id === classId ? cloneClassItem(updated) : item));
      return updated;
    },
  },
});
