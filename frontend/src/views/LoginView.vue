<template>
  <div class="login-page">
    <section class="login-card">
      <div class="login-card__brand">
        <img class="login-card__brand-logo" src="/logo.svg" alt="ClassDrive" data-testid="app-brand-logo" />
        <div class="login-card__eyebrow">ClassDrive</div>
      </div>
      <h1>{{ activeRole === "teacher" ? "老师登录" : "学生登录" }}</h1>

      <div class="login-tabs" role="tablist" aria-label="登录角色">
        <button
          :class="['login-tab', { 'is-active': activeRole === 'teacher' }]"
          type="button"
          role="tab"
          :aria-selected="activeRole === 'teacher'"
          data-testid="login-tab-teacher"
          @click="selectRole('teacher')"
        >
          教师登录
        </button>
        <button
          :class="['login-tab', { 'is-active': activeRole === 'student' }]"
          type="button"
          role="tab"
          :aria-selected="activeRole === 'student'"
          data-testid="login-tab-student"
          @click="selectRole('student')"
        >
          学生登录
        </button>
      </div>

      <form v-if="activeRole === 'teacher'" @submit.prevent="submitTeacher">
        <label class="field">
          <span>账号</span>
          <input v-model="username" data-testid="teacher-login-username" autocomplete="username" />
        </label>

        <label class="field">
          <span>密码</span>
          <div class="login-password-field">
            <input
              v-model="password"
              data-testid="teacher-login-password"
              :type="showTeacherPassword ? 'text' : 'password'"
              autocomplete="current-password"
            />
            <button
              class="login-password-toggle"
              type="button"
              data-testid="teacher-login-password-toggle"
              :aria-label="showTeacherPassword ? '隐藏密码' : '显示密码'"
              @click="showTeacherPassword = !showTeacherPassword"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M12 5C6.8 5 2.4 8.1 1 12c1.4 3.9 5.8 7 11 7s9.6-3.1 11-7c-1.4-3.9-5.8-7-11-7Zm0 11.2A4.2 4.2 0 1 1 12 7.8a4.2 4.2 0 0 1 0 8.4Zm0-6.6a2.4 2.4 0 1 0 0 4.8 2.4 2.4 0 0 0 0-4.8Z"
                  fill="currentColor"
                />
              </svg>
            </button>
          </div>
        </label>

        <p v-if="teacherErrorText" class="form-error">{{ teacherErrorText }}</p>

        <button class="button button--primary button--wide" type="submit" data-testid="teacher-login-submit">登录</button>
      </form>

      <form v-else @submit.prevent="submitStudent">
        <label class="field">
          <span>学号</span>
          <input v-model="studentNo" data-testid="student-login-no" autocomplete="username" />
        </label>

        <label class="field">
          <span>密码</span>
          <div class="login-password-field">
            <input
              v-model="studentPassword"
              data-testid="student-login-password"
              :type="showStudentPassword ? 'text' : 'password'"
              autocomplete="current-password"
            />
            <button
              class="login-password-toggle"
              type="button"
              data-testid="student-login-password-toggle"
              :aria-label="showStudentPassword ? '隐藏密码' : '显示密码'"
              @click="showStudentPassword = !showStudentPassword"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M12 5C6.8 5 2.4 8.1 1 12c1.4 3.9 5.8 7 11 7s9.6-3.1 11-7c-1.4-3.9-5.8-7-11-7Zm0 11.2A4.2 4.2 0 1 1 12 7.8a4.2 4.2 0 0 1 0 8.4Zm0-6.6a2.4 2.4 0 1 0 0 4.8 2.4 2.4 0 0 0 0-4.8Z"
                  fill="currentColor"
                />
              </svg>
            </button>
          </div>
        </label>

        <p v-if="studentErrorText" class="form-error">{{ studentErrorText }}</p>

        <button class="button button--primary button--wide" type="submit" data-testid="student-login-submit">登录</button>
        <RouterLink class="text-button" to="/student/activate">首次使用先激活</RouterLink>
      </form>

      <footer class="login-card__footer" data-testid="login-footer">
        <span>Author: <a
            href="https://github.com/wuerzh/ClassDrive"
            target="_blank"
            rel="noopener noreferrer"
            data-testid="app-author-link"
          >wuerzh</a> | Ver: 1.7 | WX/QQ: 709868663</span>
      </footer>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ApiError } from "@/api/client";
import { useAuthStore } from "@/stores/auth";
import { useStudentAuthStore } from "@/stores/student-auth";

type LoginRole = "teacher" | "student";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const studentAuthStore = useStudentAuthStore();

const activeRole = ref<LoginRole>(route.path === "/student/login" ? "student" : "teacher");
const username = ref("");
const password = ref("");
const teacherErrorText = ref("");
const showTeacherPassword = ref(false);
const studentNo = ref("");
const studentPassword = ref("");
const studentErrorText = ref("");
const showStudentPassword = ref(false);

function selectRole(role: LoginRole) {
  activeRole.value = role;
  teacherErrorText.value = "";
  studentErrorText.value = "";

  if (role === "teacher" && route.path === "/student/login") {
    void router.replace("/");
  }
}

async function submitTeacher() {
  teacherErrorText.value = "";
  try {
    await authStore.login(username.value, password.value);
    await router.push("/files/library");
  } catch (error) {
    teacherErrorText.value = error instanceof ApiError ? error.message : "登录失败";
  }
}

async function submitStudent() {
  studentErrorText.value = "";
  try {
    await studentAuthStore.login(studentNo.value, studentPassword.value);
    await router.push(studentAuthStore.user?.mustChangePassword === true ? "/student/change-password" : "/student/assignments");
  } catch (error) {
    studentErrorText.value = error instanceof ApiError ? error.message : "登录失败";
  }
}
</script>

<style scoped>
.login-password-field {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
}

.login-password-field input {
  min-width: 0;
}

.login-password-toggle {
  width: 44px;
  min-width: 44px;
  min-height: 44px;
  border: 1px solid var(--border-soft);
  border-radius: 16px;
  background: var(--bg-surface);
  color: var(--text-secondary);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.login-password-toggle svg {
  width: 18px;
  height: 18px;
}

.login-card__footer {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--border-soft);
  display: grid;
  gap: 4px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}

.login-card__footer a {
  color: var(--accent-strong);
  font-weight: 700;
  text-decoration-line: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.login-card__footer a:hover {
  color: var(--accent);
}
</style>
