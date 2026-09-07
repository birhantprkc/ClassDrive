package server

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type apiErrorResponse struct {
	Error apiError `json:"error"`
}

type loginResponse struct {
	User sessionUser `json:"user"`
}

type testTeacherProfilePreferencesPayload struct {
	CompactListEnabled bool `json:"compactListEnabled"`
}

type testTeacherProfilePayload struct {
	ID          int64                                `json:"id"`
	Username    string                               `json:"username"`
	DisplayName string                               `json:"displayName"`
	Role        string                               `json:"role"`
	Preferences testTeacherProfilePreferencesPayload `json:"preferences"`
}

type testTeacherProfileResponse struct {
	Profile testTeacherProfilePayload `json:"profile"`
}

type testSystemSettingsPayload struct {
	UploadPanelEnabled        bool   `json:"uploadPanelEnabled"`
	SingleAccountLoginEnabled bool   `json:"singleAccountLoginEnabled"`
	ServerPort                string `json:"serverPort"`
	ServerHost                string `json:"serverHost"`
}

type testSystemSettingsResponse struct {
	Settings testSystemSettingsPayload `json:"settings"`
}

type testTeacherUserPayload struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Disabled    bool   `json:"disabled"`
}

type testTeacherUsersResponse struct {
	Teachers []testTeacherUserPayload `json:"teachers"`
}

type testTeacherUserResponse struct {
	Teacher testTeacherUserPayload `json:"teacher"`
}

type studentSessionResponse struct {
	User studentSessionPayload `json:"user"`
}

type studentSessionPayload struct {
	ID                 int64  `json:"id"`
	ClassID            int64  `json:"classId"`
	StudentNo          string `json:"studentNo"`
	DisplayName        string `json:"displayName"`
	ClassName          string `json:"className"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

type studentSubmissionPayload struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submittedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type studentAssignmentPayload struct {
	ID                    int64                     `json:"id"`
	ClassID               int64                     `json:"classId"`
	Title                 string                    `json:"title"`
	Description           string                    `json:"description"`
	DueAt                 string                    `json:"dueAt"`
	Status                string                    `json:"status"`
	SubmissionMode        string                    `json:"submissionMode"`
	MinFileCount          int                       `json:"minFileCount"`
	CreatedAt             string                    `json:"createdAt"`
	UpdatedAt             string                    `json:"updatedAt"`
	Overdue               bool                      `json:"overdue"`
	Submission            *studentSubmissionPayload `json:"submission"`
	SubmissionConstraints struct {
		AllowedTypesLabel string `json:"allowedTypesLabel"`
		MaxFileSizeBytes  int64  `json:"maxFileSizeBytes"`
		MaxFileSizeLabel  string `json:"maxFileSizeLabel"`
	} `json:"submissionConstraints"`
	AssignmentAttachments []fileSummary `json:"assignmentAttachments"`
	Items                 []fileSummary `json:"items"`
}

type studentAssignmentsResponse struct {
	Assignments           []studentAssignmentPayload `json:"assignments"`
	Pagination            paginationPayload          `json:"pagination"`
	SubmissionConstraints struct {
		AllowedTypesLabel string `json:"allowedTypesLabel"`
		MaxFileSizeBytes  int64  `json:"maxFileSizeBytes"`
		MaxFileSizeLabel  string `json:"maxFileSizeLabel"`
	} `json:"submissionConstraints"`
}

type studentSubmissionResponse struct {
	Submission *studentSubmissionPayload `json:"submission"`
	Items      []fileSummary             `json:"items"`
}

type teacherAssignmentSubmissionPayload struct {
	ID                    int64         `json:"id"`
	StudentID             int64         `json:"studentId"`
	StudentNo             string        `json:"studentNo"`
	DisplayName           string        `json:"displayName"`
	Status                string        `json:"status"`
	SubmittedAt           string        `json:"submittedAt"`
	UpdatedAt             string        `json:"updatedAt"`
	ReviewStatus          string        `json:"reviewStatus"`
	TeacherCommentSummary string        `json:"teacherCommentSummary"`
	ReviewedAt            string        `json:"reviewedAt"`
	ReviewerName          string        `json:"reviewerName"`
	Items                 []fileSummary `json:"items"`
}

type teacherAssignmentSubmissionsResponse struct {
	Submissions []teacherAssignmentSubmissionPayload `json:"submissions"`
	Pagination  paginationPayload                    `json:"pagination"`
}

type assignmentStatisticsRowPayload struct {
	StudentID      int64  `json:"studentId"`
	StudentNo      string `json:"studentNo"`
	DisplayName    string `json:"displayName"`
	SubmittedCount int    `json:"submittedCount"`
	MissingCount   int    `json:"missingCount"`
}

type assignmentStatisticsResponse struct {
	ClassID         int64                            `json:"classId"`
	AssignmentIDs   []int64                          `json:"assignmentIds"`
	RosterTotal     int                              `json:"rosterTotal"`
	AssignmentTotal int                              `json:"assignmentTotal"`
	ExpectedTotal   int                              `json:"expectedTotal"`
	SubmittedTotal  int                              `json:"submittedTotal"`
	MissingTotal    int                              `json:"missingTotal"`
	Rows            []assignmentStatisticsRowPayload `json:"rows"`
}

type shellResponse struct {
	User  sessionUser `json:"user"`
	Items []navItem   `json:"items"`
}

type navItem struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Href        string `json:"href"`
	Placeholder bool   `json:"placeholder"`
}

type classesResponse struct {
	Classes []classSummary `json:"classes"`
}

type classJoinCodeResponse struct {
	ClassID               int64  `json:"classId"`
	JoinCode              string `json:"joinCode"`
	JoinCodeHint          string `json:"joinCodeHint"`
	RegistrationExpiresAt string `json:"registrationExpiresAt"`
}

type recentCopyTargetsResponse struct {
	Items []recentCopyTarget `json:"items"`
}

type recentCopyTarget struct {
	Space   string `json:"space"`
	ClassID *int64 `json:"classId,omitempty"`
	Path    string `json:"path"`
	Label   string `json:"label"`
	Pinned  bool   `json:"pinned"`
}

type studentsResponse struct {
	Students []studentSummary `json:"students"`
}

type studentSummary struct {
	ID          int64  `json:"id"`
	ClassID     int64  `json:"classId"`
	StudentNo   string `json:"studentNo"`
	DisplayName string `json:"displayName"`
	ActivatedAt string `json:"activatedAt"`
}

type assignmentsResponse struct {
	Assignments []assignmentPayload `json:"assignments"`
}

type assignmentPayload struct {
	ID             int64  `json:"id"`
	ClassID        int64  `json:"classId"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	DueAt          string `json:"dueAt"`
	Status         string `json:"status"`
	SubmissionMode string `json:"submissionMode"`
	MinFileCount   int    `json:"minFileCount"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type filesResponse struct {
	Space       string            `json:"space"`
	ClassID     int64             `json:"classId,omitempty"`
	CurrentPath string            `json:"currentPath"`
	Items       []fileSummary     `json:"items"`
	Pagination  paginationPayload `json:"pagination"`
}

type loginLogsResponse struct {
	Logs       []testLoginLogPayload `json:"logs"`
	Pagination paginationPayload     `json:"pagination"`
}

type testLoginLogPayload struct {
	ID         int64  `json:"id"`
	OccurredAt string `json:"occurredAt"`
	ActorType  string `json:"actorType"`
	ActorName  string `json:"actorName"`
	Username   string `json:"username"`
	Status     string `json:"status"`
	IPAddress  string `json:"ipAddress"`
	Message    string `json:"message"`
}

type operationLogsResponse struct {
	Logs       []testOperationLogPayload `json:"logs"`
	Pagination paginationPayload         `json:"pagination"`
}

type auditLogsResponse struct {
	Logs       []testAuditLogPayload `json:"logs"`
	Pagination paginationPayload     `json:"pagination"`
}

type testAuditLogPayload struct {
	ID         int64  `json:"id"`
	LogType    string `json:"logType"`
	OccurredAt string `json:"occurredAt"`
	ActorType  string `json:"actorType"`
	Account    string `json:"account"`
	ActorName  string `json:"actorName"`
	Action     string `json:"action"`
	Result     string `json:"result"`
	IPAddress  string `json:"ipAddress"`
}

type clearAuditLogsResponse struct {
	DeletedLoginLogs     int64 `json:"deletedLoginLogs"`
	DeletedOperationLogs int64 `json:"deletedOperationLogs"`
}

type testOperationLogPayload struct {
	ID         int64  `json:"id"`
	OccurredAt string `json:"occurredAt"`
	ActorType  string `json:"actorType"`
	ActorName  string `json:"actorName"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"statusCode"`
	IPAddress  string `json:"ipAddress"`
	Summary    string `json:"summary"`
}

type assignmentAttachmentsResponse struct {
	Items []fileSummary `json:"items"`
}

func TestTeacherLoginSessionAndLogout(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": "admin",
		"password": "demo123",
	}))
	loginRequest.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	cookies := loginRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie to be set")
	}

	var loginPayload loginResponse
	decodeJSON(t, loginRecorder.Body, &loginPayload)
	if loginPayload.User.Username != "admin" {
		t.Fatalf("expected teacher user, got %#v", loginPayload.User)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	sessionRequest.AddCookie(cookies[0])
	handler.ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected session status 200, got %d: %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}

	var sessionPayload sessionResponse
	decodeJSON(t, sessionRecorder.Body, &sessionPayload)
	if sessionPayload.User.Username != "admin" {
		t.Fatalf("expected teacher session user, got %#v", sessionPayload.User)
	}

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.AddCookie(cookies[0])
	handler.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("expected logout status 200, got %d: %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	postLogoutRecorder := httptest.NewRecorder()
	postLogoutRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	postLogoutRequest.AddCookie(cookies[0])
	handler.ServeHTTP(postLogoutRecorder, postLogoutRequest)
	if postLogoutRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized after logout, got %d", postLogoutRecorder.Code)
	}
}

func TestLoginSetsStrictSessionAndCSRFCookies(t *testing.T) {
	t.Parallel()

	_, handler := newStrictCSRFTestServer(t)

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "https://example.com/api/auth/login", jsonBody(t, map[string]string{
		"username": "admin",
		"password": "demo123",
	}))
	loginRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	sessionCookie := findCookie(t, loginRecorder.Result().Cookies(), sessionCookieName)
	if !sessionCookie.HttpOnly {
		t.Fatalf("expected session cookie to be http-only")
	}
	if !sessionCookie.Secure {
		t.Fatalf("expected session cookie to be secure for HTTPS requests")
	}
	if sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("expected session cookie SameSite=Strict, got %v", sessionCookie.SameSite)
	}

	csrfCookie := findCookie(t, loginRecorder.Result().Cookies(), csrfCookieName)
	if csrfCookie.HttpOnly {
		t.Fatalf("expected csrf cookie to be readable by the frontend")
	}
	if !csrfCookie.Secure {
		t.Fatalf("expected csrf cookie to be secure for HTTPS requests")
	}
	if csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("expected csrf cookie SameSite=Strict, got %v", csrfCookie.SameSite)
	}
	if strings.TrimSpace(csrfCookie.Value) == "" {
		t.Fatalf("expected csrf cookie value")
	}
}

func TestCSRFRejectsBrowserWriteWithoutTokenAndAllowsMatchingToken(t *testing.T) {
	t.Parallel()

	_, handler := newStrictCSRFTestServer(t)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": "admin",
		"password": "demo123",
	}))
	loginRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	cookies := loginRecorder.Result().Cookies()
	sessionCookie := findCookie(t, cookies, sessionCookieName)
	csrfCookie := findCookie(t, cookies, csrfCookieName)

	missingTokenRecorder := httptest.NewRecorder()
	missingTokenRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	missingTokenRequest.Header.Set("Origin", "http://example.com")
	missingTokenRequest.AddCookie(sessionCookie)
	missingTokenRequest.AddCookie(csrfCookie)
	handler.ServeHTTP(missingTokenRecorder, missingTokenRequest)
	if missingTokenRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected missing csrf token 403, got %d: %s", missingTokenRecorder.Code, missingTokenRecorder.Body.String())
	}

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Origin", "http://example.com")
	logoutRequest.Header.Set(csrfHeaderName, csrfCookie.Value)
	logoutRequest.AddCookie(sessionCookie)
	logoutRequest.AddCookie(csrfCookie)
	handler.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("expected matching csrf token logout 200, got %d: %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}
}

func TestCSRFRejectsCrossSiteBrowserWrite(t *testing.T) {
	t.Parallel()

	_, handler := newStrictCSRFTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.Header.Set("Origin", "http://attacker.example")
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected cross-site request 403, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCSRFRejectsWriteWithoutTokenEvenWithoutBrowserOriginHeaders(t *testing.T) {
	t.Parallel()

	_, handler := newStrictCSRFTestServer(t)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": "admin",
		"password": "demo123",
	}))
	loginRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("expected login status 200, got %d: %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	sessionCookie := findCookie(t, loginRecorder.Result().Cookies(), sessionCookieName)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.AddCookie(sessionCookie)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected missing csrf token 403 without browser origin headers, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestInternalServerErrorResponseIsSanitized(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	recorder := httptest.NewRecorder()
	app.writeError(recorder, http.StatusInternalServerError, "server_error", "sqlite secret path C:/private/classdrive.db")

	var payload apiErrorResponse
	decodeJSON(t, recorder.Body, &payload)
	if payload.Error.Message != serverErrorMessage {
		t.Fatalf("expected sanitized server error message, got %#v", payload.Error)
	}
	if strings.Contains(payload.Error.Message, "sqlite") || strings.Contains(payload.Error.Message, "C:/private") {
		t.Fatalf("expected internal details to be hidden, got %q", payload.Error.Message)
	}
}

func TestDefaultTeacherSeedUsesAdminAccount(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)

	adminRecorder := httptest.NewRecorder()
	adminRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": "admin",
		"password": "demo123",
	}))
	adminRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(adminRecorder, adminRequest)
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("expected default admin login 200, got %d: %s", adminRecorder.Code, adminRecorder.Body.String())
	}

	teacherRecorder := httptest.NewRecorder()
	teacherRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": "teacher",
		"password": "demo123",
	}))
	teacherRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(teacherRecorder, teacherRequest)
	if teacherRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected retired teacher login 401, got %d: %s", teacherRecorder.Code, teacherRecorder.Body.String())
	}
}

func TestTeacherLoginInvalidatesPreviousSessionAndRecordsLoginLogs(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)

	firstCookie := loginAndGetCookie(t, app)
	secondCookie := loginAndGetCookie(t, app)

	oldSessionRecorder := httptest.NewRecorder()
	oldSessionRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	oldSessionRequest.AddCookie(firstCookie)
	app.ServeHTTP(oldSessionRecorder, oldSessionRequest)
	if oldSessionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected old session to be invalidated, got %d: %s", oldSessionRecorder.Code, oldSessionRecorder.Body.String())
	}

	currentSessionRecorder := httptest.NewRecorder()
	currentSessionRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	currentSessionRequest.AddCookie(secondCookie)
	app.ServeHTTP(currentSessionRecorder, currentSessionRequest)
	if currentSessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected latest session to remain valid, got %d: %s", currentSessionRecorder.Code, currentSessionRecorder.Body.String())
	}

	failedLoginRecorder := httptest.NewRecorder()
	failedLoginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": "admin",
		"password": "wrong-password",
	}))
	failedLoginRequest.RemoteAddr = "203.0.113.20:54001"
	failedLoginRequest.Header.Set("Content-Type", "application/json")
	failedLoginRequest.Header.Set("X-Forwarded-For", "192.0.2.10")
	app.ServeHTTP(failedLoginRecorder, failedLoginRequest)
	if failedLoginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected failed login 401, got %d: %s", failedLoginRecorder.Code, failedLoginRecorder.Body.String())
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/login-logs", nil)
	logsRequest.AddCookie(secondCookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected login logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}

	var logs loginLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) < 3 {
		t.Fatalf("expected at least three login log entries, got %#v", logs.Logs)
	}
	if logs.Logs[0].Status != "failure" || logs.Logs[0].Username != "admin" {
		t.Fatalf("expected latest failed login log first, got %#v", logs.Logs[0])
	}
	if logs.Logs[1].Status != "success" || logs.Logs[1].ActorName != "示例老师" {
		t.Fatalf("expected previous successful login log, got %#v", logs.Logs[1])
	}
}

func TestTeacherLogoutRecordsLoginLogs(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	app.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d: %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	logs, err := app.listLoginLogs(loginLogFilters{ActorType: "teacher", Query: "admin", Limit: 100})
	if err != nil {
		t.Fatalf("list login logs: %v", err)
	}
	if len(logs) == 0 || logs[0].Message != "老师退出登录" || logs[0].Username != "admin" {
		t.Fatalf("expected teacher logout login log, got %#v", logs)
	}
}

func TestStudentLogoutRecordsLoginLogs(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	cookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	app.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d: %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	logs, err := app.listLoginLogs(loginLogFilters{ActorType: "student", Query: "20260001", Limit: 100})
	if err != nil {
		t.Fatalf("list login logs: %v", err)
	}
	if len(logs) == 0 || logs[0].Message != "学生退出登录" || logs[0].Username != "20260001" {
		t.Fatalf("expected student logout login log, got %#v", logs)
	}
}

func TestSingleAccountLoginCanBeDisabledForTeacherSessions(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	patchRecorder := httptest.NewRecorder()
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/settings/system", jsonBody(t, map[string]any{
		"uploadPanelEnabled":        true,
		"singleAccountLoginEnabled": false,
		"serverPort":                "80",
	}))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.AddCookie(cookie)
	app.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("expected system settings patch 200, got %d: %s", patchRecorder.Code, patchRecorder.Body.String())
	}

	firstCookie := loginAndGetCookie(t, app)
	secondCookie := loginAndGetCookie(t, app)

	firstSessionRecorder := httptest.NewRecorder()
	firstSessionRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	firstSessionRequest.AddCookie(firstCookie)
	app.ServeHTTP(firstSessionRecorder, firstSessionRequest)
	if firstSessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected first session to remain valid when single-account login is disabled, got %d: %s", firstSessionRecorder.Code, firstSessionRecorder.Body.String())
	}

	secondSessionRecorder := httptest.NewRecorder()
	secondSessionRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	secondSessionRequest.AddCookie(secondCookie)
	app.ServeHTTP(secondSessionRecorder, secondSessionRequest)
	if secondSessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected second session to remain valid, got %d: %s", secondSessionRecorder.Code, secondSessionRecorder.Body.String())
	}
}

func TestSystemSettingsPortChangePreparesAndCommitsRuntimePort(t *testing.T) {
	t.Parallel()

	preparedPorts := make([]string, 0)
	committedPorts := make([]string, 0)
	canceledPorts := make([]string, 0)
	handler, err := New(Config{
		BaseDir: t.TempDir(),
		Seed:    true,
		PrepareServerPortChange: func(port string) (PreparedServerPortChange, error) {
			preparedPorts = append(preparedPorts, port)
			return PreparedServerPortChange{
				Commit: func() {
					committedPorts = append(committedPorts, port)
				},
				Cancel: func() {
					canceledPorts = append(canceledPorts, port)
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	app := handler.(*App)
	app.disableCSRFForTests = true
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("close app: %v", err)
		}
	})
	cookie := loginAndGetCookie(t, app)

	patchRecorder := httptest.NewRecorder()
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/settings/system", jsonBody(t, map[string]any{
		"uploadPanelEnabled":        true,
		"singleAccountLoginEnabled": true,
		"serverPort":                "777",
	}))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.AddCookie(cookie)
	app.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("expected system settings patch 200, got %d: %s", patchRecorder.Code, patchRecorder.Body.String())
	}
	if strings.Join(preparedPorts, ",") != "777" {
		t.Fatalf("prepared ports = %#v, want [777]", preparedPorts)
	}
	if strings.Join(committedPorts, ",") != "777" {
		t.Fatalf("committed ports = %#v, want [777]", committedPorts)
	}
	if len(canceledPorts) != 0 {
		t.Fatalf("expected no canceled ports, got %#v", canceledPorts)
	}
}

func TestSystemSettingsPortChangeFailureKeepsPreviousPort(t *testing.T) {
	t.Parallel()

	handler, err := New(Config{
		BaseDir: t.TempDir(),
		Seed:    true,
		PrepareServerPortChange: func(port string) (PreparedServerPortChange, error) {
			return PreparedServerPortChange{}, domainError{
				Status:  http.StatusConflict,
				Code:    "port_unavailable",
				Message: "端口不可用",
			}
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	app := handler.(*App)
	app.disableCSRFForTests = true
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("close app: %v", err)
		}
	})
	cookie := loginAndGetCookie(t, app)

	patchRecorder := httptest.NewRecorder()
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/settings/system", jsonBody(t, map[string]any{
		"uploadPanelEnabled":        true,
		"singleAccountLoginEnabled": true,
		"serverPort":                "777",
	}))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.AddCookie(cookie)
	app.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusConflict {
		t.Fatalf("expected system settings patch 409, got %d: %s", patchRecorder.Code, patchRecorder.Body.String())
	}

	settings, err := app.loadSystemSettings()
	if err != nil {
		t.Fatalf("load system settings: %v", err)
	}
	if settings.ServerPort != "80" {
		t.Fatalf("server port = %q, want 80", settings.ServerPort)
	}
}

func TestTeacherOperationLogsRecordMutatingRequests(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/classes", jsonBody(t, map[string]string{
		"name": "日志测试班",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	app.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create class 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	downloadEntry, err := app.createFile("library", nil, "/", "日志下载.txt", strings.NewReader("download log body"))
	if err != nil {
		t.Fatalf("create download file: %v", err)
	}
	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(downloadEntry.ID)+"/download", nil)
	downloadRequest.AddCookie(cookie)
	app.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/operation-logs", nil)
	logsRequest.AddCookie(cookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected operation logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}

	var logs operationLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) == 0 {
		t.Fatalf("expected operation log entries")
	}
	latest := logs.Logs[0]
	if latest.Method != http.MethodGet || latest.Path != "/api/files/"+itoa(downloadEntry.ID)+"/download" || latest.StatusCode != http.StatusOK {
		t.Fatalf("unexpected operation log %#v", latest)
	}
	if latest.ActorName != "示例老师" || latest.Summary != "老师下载资料：日志下载.txt" || latest.IPAddress == "" {
		t.Fatalf("unexpected operation actor/summary %#v", latest)
	}
}

func TestOperationLogsDescribeAssignmentSubmissionActions(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "可疑链路作业",
		Description: "用于审计下载和提交行为",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	body, contentType := multipartFileBody(t, map[string]string{}, "files", "答案.txt", []byte("student answer"))
	submitRecorder := httptest.NewRecorder()
	submitRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", body)
	submitRequest.Header.Set("Content-Type", contentType)
	submitRequest.AddCookie(studentCookie)
	app.ServeHTTP(submitRecorder, submitRequest)
	if submitRecorder.Code != http.StatusOK {
		t.Fatalf("expected student submission 200, got %d: %s", submitRecorder.Code, submitRecorder.Body.String())
	}
	var submitPayload studentSubmissionResponse
	decodeJSON(t, submitRecorder.Body, &submitPayload)
	if len(submitPayload.Items) != 1 {
		t.Fatalf("expected one submitted file, got %#v", submitPayload)
	}

	studentDownloadRecorder := httptest.NewRecorder()
	studentDownloadRequest := httptest.NewRequest(http.MethodGet, submitPayload.Items[0].DownloadURL, nil)
	studentDownloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(studentDownloadRecorder, studentDownloadRequest)
	if studentDownloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected student submission file download 200, got %d: %s", studentDownloadRecorder.Code, studentDownloadRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/submissions?classId=1", nil)
	listRequest.AddCookie(teacherCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submissions list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var submissions teacherAssignmentSubmissionsResponse
	decodeJSON(t, listRecorder.Body, &submissions)
	if len(submissions.Submissions) != 1 || len(submissions.Submissions[0].Items) != 1 {
		t.Fatalf("expected one teacher-visible submission file, got %#v", submissions)
	}

	teacherDownloadRecorder := httptest.NewRecorder()
	teacherDownloadRequest := httptest.NewRequest(http.MethodGet, submissions.Submissions[0].Items[0].DownloadURL, nil)
	teacherDownloadRequest.AddCookie(teacherCookie)
	app.ServeHTTP(teacherDownloadRecorder, teacherDownloadRequest)
	if teacherDownloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submission file download 200, got %d: %s", teacherDownloadRecorder.Code, teacherDownloadRecorder.Body.String())
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/operation-logs", nil)
	logsRequest.AddCookie(teacherCookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected operation logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}
	var logs operationLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	summaryByPath := make(map[string]string, len(logs.Logs))
	for _, item := range logs.Logs {
		summaryByPath[item.Method+" "+item.Path] = item.Summary
	}

	expectedSubmitPath := "POST /api/student/assignments/" + itoa(assignment.ID) + "/submission"
	expectedStudentDownloadPath := "GET /api/student/assignments/" + itoa(assignment.ID) + "/submission/files/" + itoa(submitPayload.Items[0].ID) + "/download"
	expectedTeacherDownloadPath := "GET /api/assignments/" + itoa(assignment.ID) + "/submissions/files/" + itoa(submitPayload.Items[0].ID) + "/download"
	expected := map[string]string{
		expectedSubmitPath:          "学生提交作业：作业《可疑链路作业》",
		expectedStudentDownloadPath: "学生下载本人提交文件：作业《可疑链路作业》/答案.txt",
		expectedTeacherDownloadPath: "老师下载学生提交文件：作业《可疑链路作业》/张小明/答案.txt",
	}
	for path, want := range expected {
		if got := summaryByPath[path]; got != want {
			t.Fatalf("summary for %s = %q, want %q; logs=%#v", path, got, want, logs.Logs)
		}
	}
}

func TestOperationLogsDescribeStudentAssignmentAttachmentDownloads(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "附件辨识作业",
		Description: "用于审计学生下载老师布置附件",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	attachment, err := app.createFile("assignment", optionalInt64(1), assignmentAttachmentParentPath(assignment.ID), "作业要求.pdf", strings.NewReader("assignment attachment"))
	if err != nil {
		t.Fatalf("create assignment attachment: %v", err)
	}
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments/"+itoa(assignment.ID)+"/attachments/"+itoa(attachment.ID)+"/download", nil)
	downloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected student assignment attachment download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/operation-logs", nil)
	logsRequest.AddCookie(teacherCookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected operation logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}
	var logs operationLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) == 0 {
		t.Fatalf("expected operation log entries")
	}
	latest := logs.Logs[0]
	expectedPath := "/api/student/assignments/" + itoa(assignment.ID) + "/attachments/" + itoa(attachment.ID) + "/download"
	if latest.Method != http.MethodGet || latest.Path != expectedPath {
		t.Fatalf("unexpected latest operation log %#v", latest)
	}
	if latest.Summary != "学生下载作业附件：作业《附件辨识作业》/作业要求.pdf" {
		t.Fatalf("unexpected attachment download summary %#v", latest)
	}
}

func TestAuditLogsSupportFilters(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	failedLoginRecorder := httptest.NewRecorder()
	failedLoginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": "admin",
		"password": "wrong-password",
	}))
	failedLoginRequest.RemoteAddr = "203.0.113.20:54001"
	failedLoginRequest.Header.Set("Content-Type", "application/json")
	failedLoginRequest.Header.Set("X-Forwarded-For", "192.0.2.10")
	app.ServeHTTP(failedLoginRecorder, failedLoginRequest)
	if failedLoginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected failed login 401, got %d: %s", failedLoginRecorder.Code, failedLoginRecorder.Body.String())
	}

	loginLogsRecorder := httptest.NewRecorder()
	loginLogsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/login-logs?actorType=teacher&status=failure&q=admin&ip=203.0.113.20", nil)
	loginLogsRequest.AddCookie(cookie)
	app.ServeHTTP(loginLogsRecorder, loginLogsRequest)
	if loginLogsRecorder.Code != http.StatusOK {
		t.Fatalf("expected filtered login logs 200, got %d: %s", loginLogsRecorder.Code, loginLogsRecorder.Body.String())
	}
	var loginLogs loginLogsResponse
	decodeJSON(t, loginLogsRecorder.Body, &loginLogs)
	if len(loginLogs.Logs) != 1 || loginLogs.Logs[0].Status != "failure" || loginLogs.Logs[0].Username != "admin" {
		t.Fatalf("unexpected filtered login logs %#v", loginLogs.Logs)
	}

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/classes", jsonBody(t, map[string]string{
		"name": "筛选测试班",
	}))
	createRequest.RemoteAddr = "203.0.113.20:54002"
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("X-Forwarded-For", "192.0.2.10")
	createRequest.AddCookie(cookie)
	app.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create class 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	operationLogsRecorder := httptest.NewRecorder()
	operationLogsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/operation-logs?actorType=teacher&method=POST&q=classes&ip=203.0.113.20", nil)
	operationLogsRequest.AddCookie(cookie)
	app.ServeHTTP(operationLogsRecorder, operationLogsRequest)
	if operationLogsRecorder.Code != http.StatusOK {
		t.Fatalf("expected filtered operation logs 200, got %d: %s", operationLogsRecorder.Code, operationLogsRecorder.Body.String())
	}
	var operationLogs operationLogsResponse
	decodeJSON(t, operationLogsRecorder.Body, &operationLogs)
	if len(operationLogs.Logs) != 1 || operationLogs.Logs[0].Method != http.MethodPost || operationLogs.Logs[0].Path != "/api/classes" {
		t.Fatalf("unexpected filtered operation logs %#v", operationLogs.Logs)
	}
	if operationLogs.Logs[0].IPAddress != "203.0.113.20" {
		t.Fatalf("expected filtered operation log IP address, got %#v", operationLogs.Logs[0])
	}
}

func TestOperationLogSummaryUsesReadableFallbacks(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"/api/auth/logout":                       "老师退出登录",
		"/api/student/auth/logout":               "学生退出登录",
		"/api/student/assignments/12/submission": "提交作业",
	}
	for path, expected := range cases {
		if actual := operationLogSummary(http.MethodPost, path); actual != expected {
			t.Fatalf("operationLogSummary(%q) = %q, want %q", path, actual, expected)
		}
	}
	if actual := operationLogSummary(http.MethodPost, "/api/unknown/path"); strings.Contains(actual, "/api/") {
		t.Fatalf("expected readable fallback without API path, got %q", actual)
	}
}

func TestClientIPAddressOnlyTrustsConfiguredProxyHeaders(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	request.RemoteAddr = "203.0.113.20:54321"
	request.Header.Set("X-Forwarded-For", "192.0.2.10")

	t.Setenv("CLASSDRIVE_TRUSTED_PROXIES", "")
	if got := clientIPAddress(request); got != "203.0.113.20" {
		t.Fatalf("expected remote address when proxy is not trusted, got %q", got)
	}

	t.Setenv("CLASSDRIVE_TRUSTED_PROXIES", "203.0.113.20")
	if got := clientIPAddress(request); got != "192.0.2.10" {
		t.Fatalf("expected forwarded address from trusted proxy, got %q", got)
	}
}

func TestAuditLogWriteFailuresAreLogged(t *testing.T) {
	_, app := newTestServer(t)
	if err := app.db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	var logBuffer bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&logBuffer)
	t.Cleanup(func() {
		log.SetOutput(originalOutput)
	})

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	app.recordLoginLog(request, "teacher", 1, "admin", "管理员", "failure", "测试写入失败")
	app.recordOperationLog(request, "teacher", 1, "管理员", http.StatusInternalServerError)

	logText := logBuffer.String()
	if !strings.Contains(logText, "record login log") {
		t.Fatalf("expected login log write failure to be logged, got %q", logText)
	}
	if !strings.Contains(logText, "record operation log") {
		t.Fatalf("expected operation log write failure to be logged, got %q", logText)
	}
}

func TestOperationLogsNormalizeStoredPathSummaries(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	if _, err := app.db.Exec(`
insert into operation_logs (occurred_at, actor_type, actor_name, method, path, status_code, ip_address, user_agent, summary)
values ('2026-05-14T16:10:58Z', 'student', '陈嘉瑜', 'POST', '/api/student/auth/logout', 200, '10.172.45.173', '', 'POST /api/student/auth/logout')`); err != nil {
		t.Fatalf("seed operation log: %v", err)
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/operation-logs?actorType=student&q=退出", nil)
	logsRequest.AddCookie(cookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected operation logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}

	var logs operationLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) != 1 {
		t.Fatalf("expected one normalized operation log, got %#v", logs.Logs)
	}
	if logs.Logs[0].Summary != "学生退出登录" {
		t.Fatalf("expected readable operation summary, got %#v", logs.Logs[0])
	}
}

func TestAuditLogsDefaultLimitCoversMoreThanOneHundredRows(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	if _, err := app.db.Exec(`delete from operation_logs`); err != nil {
		t.Fatalf("clear seeded operation logs: %v", err)
	}
	for index := 0; index < 120; index++ {
		occurredAt := time.Date(2026, 5, 14, 16, index, 0, 0, time.UTC).Format(time.RFC3339)
		if _, err := app.db.Exec(`
insert into operation_logs (occurred_at, actor_type, actor_name, method, path, status_code, ip_address, user_agent, summary)
values (?, 'teacher', '示例老师', 'POST', '/api/classes', 201, '127.0.0.1', '', ?)`,
			occurredAt,
			fmt.Sprintf("批量日志 %03d", index),
		); err != nil {
			t.Fatalf("seed operation log %d: %v", index, err)
		}
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/operation-logs", nil)
	logsRequest.AddCookie(cookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected operation logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}

	var logs operationLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) != 120 {
		t.Fatalf("expected default operation log limit to include 120 rows, got %d", len(logs.Logs))
	}
}

func TestAuditLogsUseServerPagination(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	if _, err := app.db.Exec(`delete from login_logs`); err != nil {
		t.Fatalf("clear seeded login logs: %v", err)
	}
	for index := 0; index < 9; index++ {
		occurredAt := time.Date(2026, 5, 14, 16, index, 0, 0, time.UTC).Format(time.RFC3339)
		if _, err := app.db.Exec(`
insert into login_logs (occurred_at, actor_type, actor_name, username, status, ip_address, user_agent, message)
values (?, 'teacher', '示例老师', ?, 'success', '127.0.0.1', '', ?)`,
			occurredAt,
			fmt.Sprintf("admin-%03d", index),
			fmt.Sprintf("分页登录 %03d", index),
		); err != nil {
			t.Fatalf("seed login log %d: %v", index, err)
		}
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/login-logs?page=2&pageSize=8", nil)
	logsRequest.AddCookie(cookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected login logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}

	var logs loginLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) != 1 {
		t.Fatalf("expected one login log on page 2, got %#v", logs.Logs)
	}
	if logs.Pagination.Page != 2 || logs.Pagination.PageSize != 8 || logs.Pagination.Total != 9 || logs.Pagination.TotalPages != 2 {
		t.Fatalf("unexpected audit log pagination %#v", logs.Pagination)
	}
	if logs.Logs[0].Username != "admin-000" {
		t.Fatalf("expected oldest row on page 2, got %#v", logs.Logs[0])
	}
}

func TestAuditLogsMergedTimelineUsesServerPagination(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	if _, err := app.db.Exec(`delete from login_logs`); err != nil {
		t.Fatalf("clear seeded login logs: %v", err)
	}
	if _, err := app.db.Exec(`delete from operation_logs`); err != nil {
		t.Fatalf("clear seeded operation logs: %v", err)
	}
	if _, err := app.db.Exec(`
insert into login_logs (occurred_at, actor_type, actor_name, username, status, ip_address, user_agent, message)
values ('2026-05-14T16:10:58Z', 'student', '陈嘉瑜', '20260001', 'success', '10.172.45.173', '', '学生退出登录')`); err != nil {
		t.Fatalf("seed login log: %v", err)
	}
	if _, err := app.db.Exec(`
insert into operation_logs (occurred_at, actor_type, actor_name, method, path, status_code, ip_address, user_agent, summary)
values ('2026-05-14T16:11:58Z', 'student', '陈嘉瑜', 'POST', '/api/student/auth/logout', 200, '10.172.45.173', '', 'POST /api/student/auth/logout')`); err != nil {
		t.Fatalf("seed operation log: %v", err)
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/logs?q=退出&ip=10.172&page=1&pageSize=8", nil)
	logsRequest.AddCookie(cookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected merged logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}

	var logs auditLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) != 2 {
		t.Fatalf("expected two merged logs, got %#v", logs.Logs)
	}
	if logs.Logs[0].LogType != "operation" || logs.Logs[0].Action != "学生退出登录" || logs.Logs[0].Result != "成功" {
		t.Fatalf("expected latest operation logout row first, got %#v", logs.Logs[0])
	}
	if logs.Logs[1].LogType != "login" || logs.Logs[1].Account != "陈嘉瑜" || logs.Logs[1].Action != "学生退出登录" {
		t.Fatalf("expected login logout row second, got %#v", logs.Logs[1])
	}
	if logs.Pagination.Page != 1 || logs.Pagination.PageSize != 8 || logs.Pagination.Total != 2 || logs.Pagination.TotalPages != 1 {
		t.Fatalf("unexpected merged pagination %#v", logs.Pagination)
	}
}

func TestAuditLogsMergedTimelineFiltersByLogType(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	if _, err := app.db.Exec(`delete from login_logs`); err != nil {
		t.Fatalf("clear seeded login logs: %v", err)
	}
	if _, err := app.db.Exec(`delete from operation_logs`); err != nil {
		t.Fatalf("clear seeded operation logs: %v", err)
	}
	if _, err := app.db.Exec(`
insert into login_logs (occurred_at, actor_type, actor_name, username, status, ip_address, user_agent, message)
values ('2026-05-14T16:10:58Z', 'teacher', '示例老师', 'admin', 'success', '127.0.0.1', '', '登录成功')`); err != nil {
		t.Fatalf("seed login log: %v", err)
	}
	if _, err := app.db.Exec(`
insert into operation_logs (occurred_at, actor_type, actor_name, method, path, status_code, ip_address, user_agent, summary)
values ('2026-05-14T16:11:58Z', 'teacher', '示例老师', 'POST', '/api/classes', 201, '127.0.0.1', '', '创建班级')`); err != nil {
		t.Fatalf("seed operation log: %v", err)
	}

	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/logs?logType=login&page=1&pageSize=8", nil)
	logsRequest.AddCookie(cookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected merged logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}

	var logs auditLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) != 1 || logs.Logs[0].LogType != "login" || logs.Logs[0].Action != "登录成功" {
		t.Fatalf("expected only login logs, got %#v", logs.Logs)
	}
}

func TestAuditLogsCanBeClearedBeforeDateByOwner(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	if _, err := app.db.Exec(`delete from login_logs`); err != nil {
		t.Fatalf("clear seeded login logs: %v", err)
	}
	if _, err := app.db.Exec(`delete from operation_logs`); err != nil {
		t.Fatalf("clear seeded operation logs: %v", err)
	}
	if _, err := app.db.Exec(`
insert into login_logs (occurred_at, actor_type, actor_name, username, status, ip_address, user_agent, message)
values
  ('2026-04-20T08:00:00Z', 'teacher', '示例老师', 'admin', 'success', '127.0.0.1', '', '旧登录'),
  ('2026-05-02T08:00:00Z', 'teacher', '示例老师', 'admin', 'success', '127.0.0.1', '', '新登录')`); err != nil {
		t.Fatalf("seed login logs: %v", err)
	}
	if _, err := app.db.Exec(`
insert into operation_logs (occurred_at, actor_type, actor_name, method, path, status_code, ip_address, user_agent, summary)
values
  ('2026-04-20T09:00:00Z', 'teacher', '示例老师', 'POST', '/api/classes', 201, '127.0.0.1', '', '旧操作'),
  ('2026-05-02T09:00:00Z', 'teacher', '示例老师', 'POST', '/api/classes', 201, '127.0.0.1', '', '新操作')`); err != nil {
		t.Fatalf("seed operation logs: %v", err)
	}

	clearRecorder := httptest.NewRecorder()
	clearRequest := httptest.NewRequest(http.MethodDelete, "/api/audit/logs?before=2026-05-01", nil)
	clearRequest.AddCookie(cookie)
	app.ServeHTTP(clearRecorder, clearRequest)
	if clearRecorder.Code != http.StatusOK {
		t.Fatalf("expected clear logs 200, got %d: %s", clearRecorder.Code, clearRecorder.Body.String())
	}
	var payload clearAuditLogsResponse
	decodeJSON(t, clearRecorder.Body, &payload)
	if payload.DeletedLoginLogs != 1 || payload.DeletedOperationLogs != 1 {
		t.Fatalf("unexpected clear log counts %#v", payload)
	}
	if countRows(t, app.db, `select count(*) from login_logs where message = '旧登录'`) != 0 {
		t.Fatalf("expected old login log to be removed")
	}
	if countRows(t, app.db, `select count(*) from operation_logs where summary = '旧操作'`) != 0 {
		t.Fatalf("expected old operation log to be removed")
	}
	if countRows(t, app.db, `select count(*) from login_logs where message = '新登录'`) != 1 {
		t.Fatalf("expected new login log to remain")
	}
	if countRows(t, app.db, `select count(*) from operation_logs where summary = '新操作'`) != 1 {
		t.Fatalf("expected new operation log to remain")
	}
}

func TestAuditLogsClearRequiresOwnerAndValidDate(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	ownerCookie := loginAndGetCookie(t, app)
	missingDateRecorder := httptest.NewRecorder()
	missingDateRequest := httptest.NewRequest(http.MethodDelete, "/api/audit/logs", nil)
	missingDateRequest.AddCookie(ownerCookie)
	app.ServeHTTP(missingDateRecorder, missingDateRequest)
	if missingDateRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected clear logs 422 without date, got %d: %s", missingDateRecorder.Code, missingDateRecorder.Body.String())
	}

	createTeacherAccount(t, app, "assistant", "助教老师", "assistant123", "staff", false)
	staffCookie := loginAndGetCookieAs(t, app, "assistant", "assistant123")
	staffRecorder := httptest.NewRecorder()
	staffRequest := httptest.NewRequest(http.MethodDelete, "/api/audit/logs?before=2026-05-01", nil)
	staffRequest.AddCookie(staffCookie)
	app.ServeHTTP(staffRecorder, staffRequest)
	if staffRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected clear logs 403 for staff, got %d: %s", staffRecorder.Code, staffRecorder.Body.String())
	}
}

func TestStudentActivateLoginSessionAndLogoutFlow(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")

	activateRecorder := httptest.NewRecorder()
	activateRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/activate", jsonBody(t, map[string]any{
		"joinCode":  "ABCD1234",
		"studentNo": "20260001",
		"password":  "student123",
	}))
	activateRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(activateRecorder, activateRequest)
	if activateRecorder.Code != http.StatusOK {
		t.Fatalf("expected activate 200, got %d: %s", activateRecorder.Code, activateRecorder.Body.String())
	}

	passwordHash, activatedAt := studentAuthState(t, app.db, 1)
	if strings.TrimSpace(passwordHash) == "" {
		t.Fatalf("expected password hash to be written after activation")
	}
	if strings.TrimSpace(activatedAt) == "" {
		t.Fatalf("expected activated_at to be written after activation")
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]interface{}{
		"studentNo": "20260001",
		"password":  "student123",
	}))
	loginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	cookies := loginRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected student session cookie to be set")
	}

	var loginPayload studentSessionResponse
	decodeJSON(t, loginRecorder.Body, &loginPayload)
	if loginPayload.User.StudentNo != "20260001" || loginPayload.User.ClassName != "一年级一班" {
		t.Fatalf("unexpected student login payload %#v", loginPayload.User)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/student/session", nil)
	sessionRequest.AddCookie(cookies[0])
	app.ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected session 200, got %d: %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}

	var sessionPayload studentSessionResponse
	decodeJSON(t, sessionRecorder.Body, &sessionPayload)
	if sessionPayload.User.DisplayName != "张小明" || sessionPayload.User.ClassID != 1 {
		t.Fatalf("unexpected student session payload %#v", sessionPayload.User)
	}

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/logout", nil)
	logoutRequest.AddCookie(cookies[0])
	app.ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("expected logout 200, got %d: %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	postLogoutRecorder := httptest.NewRecorder()
	postLogoutRequest := httptest.NewRequest(http.MethodGet, "/api/student/session", nil)
	postLogoutRequest.AddCookie(cookies[0])
	app.ServeHTTP(postLogoutRecorder, postLogoutRequest)
	if postLogoutRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected session 401 after logout, got %d: %s", postLogoutRecorder.Code, postLogoutRecorder.Body.String())
	}
}

func TestStudentLoginInvalidatesPreviousSession(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	firstCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	secondCookie := studentLoginAndGetCookie(t, app, "20260001", "student123")

	oldSessionRecorder := httptest.NewRecorder()
	oldSessionRequest := httptest.NewRequest(http.MethodGet, "/api/student/session", nil)
	oldSessionRequest.AddCookie(firstCookie)
	app.ServeHTTP(oldSessionRecorder, oldSessionRequest)
	if oldSessionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected old student session to be invalidated, got %d: %s", oldSessionRecorder.Code, oldSessionRecorder.Body.String())
	}

	currentSessionRecorder := httptest.NewRecorder()
	currentSessionRequest := httptest.NewRequest(http.MethodGet, "/api/student/session", nil)
	currentSessionRequest.AddCookie(secondCookie)
	app.ServeHTTP(currentSessionRecorder, currentSessionRequest)
	if currentSessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected latest student session to remain valid, got %d: %s", currentSessionRecorder.Code, currentSessionRecorder.Body.String())
	}

	teacherCookie := loginAndGetCookie(t, app)
	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/login-logs?actorType=student&status=success", nil)
	logsRequest.AddCookie(teacherCookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected login logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}
	var logs loginLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) == 0 {
		t.Fatalf("expected student login logs")
	}
	if !strings.Contains(logs.Logs[0].Message, "旧登录已失效") {
		t.Fatalf("expected old-session invalidation in login log, got %#v", logs.Logs[0])
	}
	if strings.TrimSpace(logs.Logs[0].IPAddress) == "" {
		t.Fatalf("expected login log ip address, got %#v", logs.Logs[0])
	}
}

func TestStudentFailedLoginLogUsesMatchedDisplayName(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	_ = activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	failedLoginRecorder := httptest.NewRecorder()
	failedLoginRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]string{
		"studentNo": "20260001",
		"password":  "wrong-password",
	}))
	failedLoginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(failedLoginRecorder, failedLoginRequest)
	if failedLoginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected failed student login 401, got %d: %s", failedLoginRecorder.Code, failedLoginRecorder.Body.String())
	}

	teacherCookie := loginAndGetCookie(t, app)
	logsRecorder := httptest.NewRecorder()
	logsRequest := httptest.NewRequest(http.MethodGet, "/api/audit/logs?logType=login&actorType=student&result=failure", nil)
	logsRequest.AddCookie(teacherCookie)
	app.ServeHTTP(logsRecorder, logsRequest)
	if logsRecorder.Code != http.StatusOK {
		t.Fatalf("expected audit logs 200, got %d: %s", logsRecorder.Code, logsRecorder.Body.String())
	}
	var logs auditLogsResponse
	decodeJSON(t, logsRecorder.Body, &logs)
	if len(logs.Logs) == 0 {
		t.Fatalf("expected failed student login log")
	}
	if logs.Logs[0].Account != "张小明" || logs.Logs[0].ActorName != "张小明" {
		t.Fatalf("expected failed student login log to use display name, got %#v", logs.Logs[0])
	}
}

func TestTeacherResetStudentPasswordRequiresPasswordChange(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	oldStudentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	teacherCookie := loginAndGetCookie(t, app)

	resetRecorder := httptest.NewRecorder()
	resetRequest := httptest.NewRequest(http.MethodPost, "/api/students/1/reset-password", nil)
	resetRequest.AddCookie(teacherCookie)
	app.ServeHTTP(resetRecorder, resetRequest)
	if resetRecorder.Code != http.StatusOK {
		t.Fatalf("expected reset password 200, got %d: %s", resetRecorder.Code, resetRecorder.Body.String())
	}

	oldSessionRecorder := httptest.NewRecorder()
	oldSessionRequest := httptest.NewRequest(http.MethodGet, "/api/student/session", nil)
	oldSessionRequest.AddCookie(oldStudentCookie)
	app.ServeHTTP(oldSessionRecorder, oldSessionRequest)
	if oldSessionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected reset to invalidate existing session, got %d: %s", oldSessionRecorder.Code, oldSessionRecorder.Body.String())
	}

	oldPasswordRecorder := httptest.NewRecorder()
	oldPasswordRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]any{
		"studentNo": "20260001",
		"password":  "student123",
	}))
	oldPasswordRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(oldPasswordRecorder, oldPasswordRequest)
	if oldPasswordRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password to fail after reset, got %d: %s", oldPasswordRecorder.Code, oldPasswordRecorder.Body.String())
	}

	resetLoginRecorder := httptest.NewRecorder()
	resetLoginRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]any{
		"studentNo": "20260001",
		"password":  "123456",
	}))
	resetLoginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(resetLoginRecorder, resetLoginRequest)
	if resetLoginRecorder.Code != http.StatusOK {
		t.Fatalf("expected reset password login 200, got %d: %s", resetLoginRecorder.Code, resetLoginRecorder.Body.String())
	}
	var resetLoginPayload studentSessionResponse
	decodeJSON(t, resetLoginRecorder.Body, &resetLoginPayload)
	if !resetLoginPayload.User.MustChangePassword {
		t.Fatalf("expected reset login payload to require password change, got %#v", resetLoginPayload.User)
	}
	resetStudentCookie := resetLoginRecorder.Result().Cookies()[0]

	blockedRecorder := httptest.NewRecorder()
	blockedRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments", nil)
	blockedRequest.AddCookie(resetStudentCookie)
	app.ServeHTTP(blockedRecorder, blockedRequest)
	if blockedRecorder.Code != http.StatusConflict {
		t.Fatalf("expected student resources blocked before password change, got %d: %s", blockedRecorder.Code, blockedRecorder.Body.String())
	}

	changeRecorder := httptest.NewRecorder()
	changeRequest := httptest.NewRequest(http.MethodPost, "/api/student/password", jsonBody(t, map[string]any{
		"currentPassword": "123456",
		"newPassword":     "newpass123",
	}))
	changeRequest.Header.Set("Content-Type", "application/json")
	changeRequest.AddCookie(resetStudentCookie)
	app.ServeHTTP(changeRecorder, changeRequest)
	if changeRecorder.Code != http.StatusOK {
		t.Fatalf("expected student password change 200, got %d: %s", changeRecorder.Code, changeRecorder.Body.String())
	}
	var changedPayload studentSessionResponse
	decodeJSON(t, changeRecorder.Body, &changedPayload)
	if changedPayload.User.MustChangePassword {
		t.Fatalf("expected password change to clear required flag, got %#v", changedPayload.User)
	}

	allowedRecorder := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments", nil)
	allowedRequest.AddCookie(resetStudentCookie)
	app.ServeHTTP(allowedRecorder, allowedRequest)
	if allowedRecorder.Code != http.StatusOK {
		t.Fatalf("expected student resources after password change, got %d: %s", allowedRecorder.Code, allowedRecorder.Body.String())
	}

	newPasswordCookie := studentLoginAndGetCookie(t, app, "20260001", "newpass123")
	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/student/session", nil)
	sessionRequest.AddCookie(newPasswordCookie)
	app.ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected new password login session 200, got %d: %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}
	var sessionPayload studentSessionResponse
	decodeJSON(t, sessionRecorder.Body, &sessionPayload)
	if sessionPayload.User.MustChangePassword {
		t.Fatalf("expected new password login not to require change, got %#v", sessionPayload.User)
	}
}

func TestStudentResetPasswordCanBeOverriddenByEnvironment(t *testing.T) {
	t.Setenv("CLASSDRIVE_DEFAULT_STUDENT_RESET_PASSWORD", "reset987")

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	teacherCookie := loginAndGetCookie(t, app)

	resetRecorder := httptest.NewRecorder()
	resetRequest := httptest.NewRequest(http.MethodPost, "/api/students/1/reset-password", nil)
	resetRequest.AddCookie(teacherCookie)
	app.ServeHTTP(resetRecorder, resetRequest)
	if resetRecorder.Code != http.StatusOK {
		t.Fatalf("expected reset password 200, got %d: %s", resetRecorder.Code, resetRecorder.Body.String())
	}
	var resetPayload studentPasswordResetResult
	decodeJSON(t, resetRecorder.Body, &resetPayload)
	if resetPayload.DefaultPassword != "reset987" {
		t.Fatalf("expected override reset password in response, got %#v", resetPayload)
	}

	resetLoginRecorder := httptest.NewRecorder()
	resetLoginRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]any{
		"studentNo": "20260001",
		"password":  "reset987",
	}))
	resetLoginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(resetLoginRecorder, resetLoginRequest)
	if resetLoginRecorder.Code != http.StatusOK {
		t.Fatalf("expected override reset password login 200, got %d: %s", resetLoginRecorder.Code, resetLoginRecorder.Body.String())
	}

	builtInLoginRecorder := httptest.NewRecorder()
	builtInLoginRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]any{
		"studentNo": "20260001",
		"password":  "123456",
	}))
	builtInLoginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(builtInLoginRecorder, builtInLoginRequest)
	if builtInLoginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected built-in reset password to fail after override, got %d: %s", builtInLoginRecorder.Code, builtInLoginRecorder.Body.String())
	}
}

func TestPasswordComplexityValidationRejectsTrivialPatterns(t *testing.T) {
	t.Parallel()

	trivialPasswords := []string{
		"123456",
		"654321",
		"111111",
		"abcdef",
	}
	for _, password := range trivialPasswords {
		if _, err := validateStudentPassword(password); err == nil {
			t.Fatalf("expected student password %q to be rejected", password)
		}
		if _, err := validateTeacherPassword(password); err == nil {
			t.Fatalf("expected teacher password %q to be rejected", password)
		}
	}

	numericPasswords := []string{
		"938271",
		"520839",
	}
	for _, password := range numericPasswords {
		if _, err := validateStudentPassword(password); err != nil {
			t.Fatalf("expected student password %q to be accepted: %v", password, err)
		}
		if _, err := validateTeacherPassword(password); err != nil {
			t.Fatalf("expected teacher password %q to be accepted: %v", password, err)
		}
	}
}

func TestStudentActivateValidation(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")

	wrongCodeRecorder := httptest.NewRecorder()
	wrongCodeRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/activate", jsonBody(t, map[string]any{
		"joinCode":  "ZZZZ9999",
		"studentNo": "20260001",
		"password":  "student123",
	}))
	wrongCodeRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(wrongCodeRecorder, wrongCodeRequest)
	if wrongCodeRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected wrong join code 401, got %d: %s", wrongCodeRecorder.Code, wrongCodeRecorder.Body.String())
	}

	firstActivateRecorder := httptest.NewRecorder()
	firstActivateRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/activate", jsonBody(t, map[string]any{
		"joinCode":  "ABCD1234",
		"studentNo": "20260001",
		"password":  "student123",
	}))
	firstActivateRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(firstActivateRecorder, firstActivateRequest)
	if firstActivateRecorder.Code != http.StatusOK {
		t.Fatalf("expected first activate 200, got %d: %s", firstActivateRecorder.Code, firstActivateRecorder.Body.String())
	}

	repeatedRecorder := httptest.NewRecorder()
	repeatedRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/activate", jsonBody(t, map[string]any{
		"joinCode":  "ABCD1234",
		"studentNo": "20260001",
		"password":  "student123",
	}))
	repeatedRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(repeatedRecorder, repeatedRequest)
	if repeatedRecorder.Code != http.StatusConflict {
		t.Fatalf("expected repeated activate 409, got %d: %s", repeatedRecorder.Code, repeatedRecorder.Body.String())
	}

	if _, err := app.db.Exec(`update classes set registration_enabled = 0 where id = 1`); err != nil {
		t.Fatalf("close registration: %v", err)
	}

	closedRecorder := httptest.NewRecorder()
	closedRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/activate", jsonBody(t, map[string]any{
		"joinCode":  "ABCD1234",
		"studentNo": "20260001",
		"password":  "student123",
	}))
	closedRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(closedRecorder, closedRequest)
	if closedRecorder.Code != http.StatusConflict {
		t.Fatalf("expected closed registration 409, got %d: %s", closedRecorder.Code, closedRecorder.Body.String())
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]any{
		"studentNo": "20260001",
		"password":  "wrong-password",
	}))
	loginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected wrong password 401, got %d: %s", loginRecorder.Code, loginRecorder.Body.String())
	}
}

func TestStudentAssignmentsListShowsPublishedOnly(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")

	if _, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "草稿作业",
		Description: "不应出现在学生端",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "draft",
	}); err != nil {
		t.Fatalf("create draft assignment: %v", err)
	}

	published, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "第一单元练习",
		Description: "完成练习册第 8 页",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create published assignment: %v", err)
	}

	if _, err := app.createAssignment(createAssignmentRequest{
		ClassID:     2,
		Title:       "其他班级作业",
		Description: "不属于当前学生",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	}); err != nil {
		t.Fatalf("create other class assignment: %v", err)
	}

	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/student/assignments", nil)
	request.AddCookie(studentCookie)
	app.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload studentAssignmentsResponse
	decodeJSON(t, recorder.Body, &payload)
	if len(payload.Assignments) != 1 {
		t.Fatalf("expected 1 published assignment, got %#v", payload.Assignments)
	}
	if payload.Assignments[0].ID != published.ID || payload.Assignments[0].Title != "第一单元练习" {
		t.Fatalf("unexpected student assignments payload %#v", payload.Assignments)
	}
	if payload.Assignments[0].Submission != nil {
		t.Fatalf("expected empty submission summary before submit, got %#v", payload.Assignments[0].Submission)
	}
	if payload.SubmissionConstraints.AllowedTypesLabel != "PDF、Word、Excel、PPT、TXT、HTML、Markdown、CSV、JSON、图片、音视频、压缩包" || payload.SubmissionConstraints.MaxFileSizeLabel != "100 MB" {
		t.Fatalf("expected submission constraints in list payload, got %#v", payload.SubmissionConstraints)
	}
}

func TestStudentAssignmentsUseServerPagination(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	for index := 0; index < 9; index++ {
		if _, err := app.createAssignment(createAssignmentRequest{
			ClassID:     1,
			Title:       fmt.Sprintf("分页作业 %03d", index),
			Description: "学生端分页测试",
			DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
			Status:      "published",
		}); err != nil {
			t.Fatalf("create published assignment %d: %v", index, err)
		}
	}

	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/student/assignments?page=2&pageSize=8", nil)
	request.AddCookie(studentCookie)
	app.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload studentAssignmentsResponse
	decodeJSON(t, recorder.Body, &payload)
	if len(payload.Assignments) != 1 {
		t.Fatalf("expected 1 assignment on page 2, got %#v", payload.Assignments)
	}
	if payload.Pagination.Page != 2 || payload.Pagination.PageSize != 8 || payload.Pagination.Total != 9 || payload.Pagination.TotalPages != 2 {
		t.Fatalf("unexpected student assignment pagination %#v", payload.Pagination)
	}
	if payload.Assignments[0].Title != "分页作业 000" {
		t.Fatalf("expected oldest assignment on page 2, got %#v", payload.Assignments[0])
	}
}

func TestStudentAssignmentDetailIncludesTeacherAttachments(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	if _, err := app.createStudent(createStudentRequest{
		ClassID:     1,
		StudentNo:   "20260002",
		DisplayName: "李待交",
	}); err != nil {
		t.Fatalf("create missing student: %v", err)
	}

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "带附件作业",
		Description: "学生应能下载老师附件",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	uploadBody, uploadContentType := multipartFileBody(t, map[string]string{}, "files", "teacher-guide.txt", []byte("teacher attachment body"))
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/assignments/"+itoa(assignment.ID)+"/attachments?classId=1", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(teacherCookie)
	app.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected teacher attachment upload 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	var uploaded assignmentAttachmentsResponse
	decodeJSON(t, uploadRecorder.Body, &uploaded)
	if len(uploaded.Items) != 1 {
		t.Fatalf("expected uploaded teacher attachment, got %#v", uploaded.Items)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments/"+itoa(assignment.ID), nil)
	detailRequest.AddCookie(studentCookie)
	app.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected student detail 200, got %d: %s", detailRecorder.Code, detailRecorder.Body.String())
	}

	var detailPayload studentAssignmentPayload
	decodeJSON(t, detailRecorder.Body, &detailPayload)
	if len(detailPayload.AssignmentAttachments) != 1 {
		t.Fatalf("expected teacher attachment in student detail, got %#v", detailPayload.AssignmentAttachments)
	}
	if detailPayload.AssignmentAttachments[0].Name != "teacher-guide.txt" {
		t.Fatalf("unexpected teacher attachment %#v", detailPayload.AssignmentAttachments[0])
	}
	expectedDownloadURL := "/api/student/assignments/" + itoa(assignment.ID) + "/attachments/" + itoa(uploaded.Items[0].ID) + "/download"
	if detailPayload.AssignmentAttachments[0].DownloadURL != expectedDownloadURL {
		t.Fatalf("expected student attachment download url %q, got %#v", expectedDownloadURL, detailPayload.AssignmentAttachments[0])
	}

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, detailPayload.AssignmentAttachments[0].DownloadURL, nil)
	downloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected student attachment download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	if !strings.Contains(downloadRecorder.Body.String(), "teacher attachment body") {
		t.Fatalf("expected teacher attachment body in student download response")
	}
}

func TestSearchTypeFiltersSupportCommonFilebrowserSyntax(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	const searchRoot = "/搜索类型样例"
	if _, err := app.createFolder("library", nil, "/", "搜索类型样例"); err != nil {
		t.Fatalf("create search root: %v", err)
	}
	if _, err := app.createFolder("library", nil, searchRoot, "课件目录"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := app.createFolder("library", nil, searchRoot, "媒体目录"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := app.createFile("library", nil, searchRoot, "LessonPlan.PDF", strings.NewReader("%PDF-1.7 lesson plan")); err != nil {
		t.Fatalf("create pdf file: %v", err)
	}
	if _, err := app.createFile("library", nil, searchRoot, "作业无扩展", strings.NewReader("%PDF-1.7 fallback pdf")); err != nil {
		t.Fatalf("create extensionless pdf file: %v", err)
	}
	if _, err := app.createFile("library", nil, searchRoot, "photo.png", strings.NewReader("png image body")); err != nil {
		t.Fatalf("create image file: %v", err)
	}
	imageByExtensionOnly, err := app.createFile("library", nil, searchRoot, "poster.webp", strings.NewReader("webp image by extension"))
	if err != nil {
		t.Fatalf("create extension-only image file: %v", err)
	}
	if _, err := app.db.Exec(`update file_entries set mime_type = ? where id = ?`, "application/octet-stream", imageByExtensionOnly.ID); err != nil {
		t.Fatalf("downgrade image mime type: %v", err)
	}
	imageByMimeOnly, err := app.createFile("library", nil, searchRoot, "图片无扩展", strings.NewReader("image by mime fallback"))
	if err != nil {
		t.Fatalf("create mime-only image file: %v", err)
	}
	if _, err := app.db.Exec(`update file_entries set mime_type = ? where id = ?`, "image/png", imageByMimeOnly.ID); err != nil {
		t.Fatalf("set image mime fallback: %v", err)
	}
	if _, err := app.createFile("library", nil, searchRoot, "voice.mp3", strings.NewReader("mp3 audio body")); err != nil {
		t.Fatalf("create audio file: %v", err)
	}
	audioByExtensionOnly, err := app.createFile("library", nil, searchRoot, "podcast.ogg", strings.NewReader("ogg audio by extension"))
	if err != nil {
		t.Fatalf("create extension-only audio file: %v", err)
	}
	if _, err := app.db.Exec(`update file_entries set mime_type = ? where id = ?`, "application/octet-stream", audioByExtensionOnly.ID); err != nil {
		t.Fatalf("downgrade audio mime type: %v", err)
	}
	audioByMimeOnly, err := app.createFile("library", nil, searchRoot, "音频无扩展", strings.NewReader("audio by mime fallback"))
	if err != nil {
		t.Fatalf("create mime-only audio file: %v", err)
	}
	if _, err := app.db.Exec(`update file_entries set mime_type = ? where id = ?`, "audio/mpeg", audioByMimeOnly.ID); err != nil {
		t.Fatalf("set audio mime fallback: %v", err)
	}
	if _, err := app.createFile("library", nil, searchRoot, "movie.mp4", strings.NewReader("mp4 video body")); err != nil {
		t.Fatalf("create video file: %v", err)
	}
	videoByExtensionOnly, err := app.createFile("library", nil, searchRoot, "clip.mkv", strings.NewReader("mkv video by extension"))
	if err != nil {
		t.Fatalf("create extension-only video file: %v", err)
	}
	if _, err := app.db.Exec(`update file_entries set mime_type = ? where id = ?`, "application/octet-stream", videoByExtensionOnly.ID); err != nil {
		t.Fatalf("downgrade video mime type: %v", err)
	}
	videoByMimeOnly, err := app.createFile("library", nil, searchRoot, "视频无扩展", strings.NewReader("video by mime fallback"))
	if err != nil {
		t.Fatalf("create mime-only video file: %v", err)
	}
	if _, err := app.db.Exec(`update file_entries set mime_type = ? where id = ?`, "video/mp4", videoByMimeOnly.ID); err != nil {
		t.Fatalf("set video mime fallback: %v", err)
	}
	if _, err := app.createFile("library", nil, searchRoot, "notes.txt", strings.NewReader("plain text body")); err != nil {
		t.Fatalf("create text file: %v", err)
	}

	testCases := []struct {
		name          string
		query         string
		expectedNames []string
	}{
		{
			name:          "dir",
			query:         "type:dir",
			expectedNames: []string{"媒体目录", "课件目录"},
		},
		{
			name:          "pdf",
			query:         "type:pdf",
			expectedNames: []string{"LessonPlan.PDF", "作业无扩展"},
		},
		{
			name:          "image",
			query:         "type:image",
			expectedNames: []string{"photo.png", "poster.webp", "图片无扩展"},
		},
		{
			name:          "audio",
			query:         "type:audio",
			expectedNames: []string{"podcast.ogg", "voice.mp3", "音频无扩展"},
		},
		{
			name:          "video",
			query:         "type:video",
			expectedNames: []string{"clip.mkv", "movie.mp4", "视频无扩展"},
		},
		{
			name:          "file",
			query:         "type:file",
			expectedNames: []string{"LessonPlan.PDF", "clip.mkv", "movie.mp4", "notes.txt", "photo.png", "podcast.ogg", "poster.webp", "voice.mp3", "作业无扩展", "图片无扩展", "视频无扩展", "音频无扩展"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			response := searchFiles(t, app, cookie, url.Values{
				"space": {"library"},
				"path":  {searchRoot},
				"q":     {tc.query},
			})
			if got := len(response.Items); got != len(tc.expectedNames) {
				t.Fatalf("expected %d search results, got %#v", len(tc.expectedNames), response.Items)
			}
			for index, name := range tc.expectedNames {
				if response.Items[index].Name != name {
					t.Fatalf("expected result %d to be %q, got %#v", index, name, response.Items[index])
				}
			}
		})
	}

	invalidTypeRecorder := httptest.NewRecorder()
	invalidTypeRequest := httptest.NewRequest(http.MethodGet, "/api/files/search?space=library&q=type:zip", nil)
	invalidTypeRequest.AddCookie(cookie)
	app.ServeHTTP(invalidTypeRecorder, invalidTypeRequest)
	if invalidTypeRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid type 422, got %d: %s", invalidTypeRecorder.Code, invalidTypeRecorder.Body.String())
	}
}

func TestSearchCaseDirectiveControlsKeywordMatching(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	if _, err := app.createFile("library", nil, "/", "LessonPlan.txt", strings.NewReader("uppercase prefix")); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := app.createFile("library", nil, "/", "lesson-notes.txt", strings.NewReader("lowercase prefix")); err != nil {
		t.Fatalf("create file: %v", err)
	}

	insensitive := searchFiles(t, app, cookie, url.Values{
		"space": {"library"},
		"q":     {"lesson case:insensitive"},
	})
	if got := len(insensitive.Items); got != 2 {
		t.Fatalf("expected 2 case-insensitive results, got %#v", insensitive.Items)
	}
	if insensitive.Items[0].Name != "LessonPlan.txt" || insensitive.Items[1].Name != "lesson-notes.txt" {
		t.Fatalf("unexpected case-insensitive results %#v", insensitive.Items)
	}

	sensitiveLowercase := searchFiles(t, app, cookie, url.Values{
		"space": {"library"},
		"q":     {"lesson case:sensitive"},
	})
	if got := len(sensitiveLowercase.Items); got != 1 || sensitiveLowercase.Items[0].Name != "lesson-notes.txt" {
		t.Fatalf("unexpected lowercase-sensitive results %#v", sensitiveLowercase.Items)
	}

	sensitiveUppercase := searchFiles(t, app, cookie, url.Values{
		"space": {"library"},
		"q":     {"Lesson case:sensitive"},
	})
	if got := len(sensitiveUppercase.Items); got != 1 || sensitiveUppercase.Items[0].Name != "LessonPlan.txt" {
		t.Fatalf("unexpected uppercase-sensitive results %#v", sensitiveUppercase.Items)
	}

	invalidCaseRecorder := httptest.NewRecorder()
	invalidCaseRequest := httptest.NewRequest(http.MethodGet, "/api/files/search?space=library&q=lesson+case:mixed", nil)
	invalidCaseRequest.AddCookie(cookie)
	app.ServeHTTP(invalidCaseRecorder, invalidCaseRequest)
	if invalidCaseRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid case 422, got %d: %s", invalidCaseRecorder.Code, invalidCaseRecorder.Body.String())
	}
}

func TestFilesListAndSearchSupportSortAndPagination(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	if _, err := app.createFolder("library", nil, "/", "分页测试"); err != nil {
		t.Fatalf("create root folder: %v", err)
	}
	if _, err := app.createFolder("library", nil, "/分页测试", "资料夹"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := app.createFile("library", nil, "/分页测试/资料夹", "nested-large.bin", strings.NewReader(strings.Repeat("x", 2048))); err != nil {
		t.Fatalf("create nested folder file: %v", err)
	}
	for _, name := range []string{"lesson-a.txt", "lesson-b.txt", "other.txt"} {
		if _, err := app.createFile("library", nil, "/分页测试", name, strings.NewReader(name)); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	listed := listFiles(t, app, cookie, url.Values{
		"space":    {"library"},
		"path":     {"/分页测试"},
		"sort":     {"name-asc"},
		"page":     {"2"},
		"pageSize": {"2"},
	})
	if listed.Pagination.Page != 2 || listed.Pagination.PageSize != 2 {
		t.Fatalf("unexpected files pagination page/pageSize %#v", listed.Pagination)
	}
	if listed.Pagination.Total != 4 || listed.Pagination.TotalPages != 2 {
		t.Fatalf("expected files total 4/2, got %#v", listed.Pagination)
	}
	if len(listed.Items) != 2 || listed.Items[0].Name != "other.txt" || listed.Items[1].Name != "资料夹" {
		t.Fatalf("unexpected paged file items %#v", listed.Items)
	}

	sizeSorted := listFiles(t, app, cookie, url.Values{
		"space": {"library"},
		"path":  {"/分页测试"},
		"sort":  {"size-desc"},
	})
	if len(sizeSorted.Items) != 4 || sizeSorted.Items[0].Name != "资料夹" || sizeSorted.Items[0].Size != 2048 {
		t.Fatalf("expected size-desc to use recursive folder size, got %#v", sizeSorted.Items)
	}

	searched := searchFiles(t, app, cookie, url.Values{
		"space":    {"library"},
		"path":     {"/分页测试"},
		"q":        {"lesson"},
		"sort":     {"name-asc"},
		"page":     {"2"},
		"pageSize": {"1"},
	})
	if searched.Pagination.Page != 2 || searched.Pagination.PageSize != 1 {
		t.Fatalf("unexpected search pagination page/pageSize %#v", searched.Pagination)
	}
	if searched.Pagination.Total != 2 || searched.Pagination.TotalPages != 2 {
		t.Fatalf("expected search total 2/2, got %#v", searched.Pagination)
	}
	if len(searched.Items) != 1 || searched.Items[0].Name != "lesson-b.txt" {
		t.Fatalf("unexpected paged search items %#v", searched.Items)
	}
}

func TestListFileEntriesPagePaginatesBeforeReturningRows(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)

	if _, err := app.createFolder("library", nil, "/", "SQL分页"); err != nil {
		t.Fatalf("create root folder: %v", err)
	}
	if _, err := app.createFolder("library", nil, "/SQL分页", "资料夹"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := app.createFile("library", nil, "/SQL分页/资料夹", "nested-large.bin", strings.NewReader(strings.Repeat("x", 4096))); err != nil {
		t.Fatalf("create nested folder file: %v", err)
	}
	for _, name := range []string{"alpha.txt", "beta.txt", "gamma.txt"} {
		if _, err := app.createFile("library", nil, "/SQL分页", name, strings.NewReader(name)); err != nil {
			t.Fatalf("create file %s: %v", name, err)
		}
	}

	query := teacherListQuery{
		Sort:     "name-asc",
		Page:     2,
		PageSize: 2,
		Offset:   2,
		Paged:    true,
	}
	items, pagination, err := app.listFileEntriesPage("library", nil, "/SQL分页", query)
	if err != nil {
		t.Fatalf("list file entries page: %v", err)
	}
	if pagination.Page != 2 || pagination.PageSize != 2 || pagination.Total != 4 || pagination.TotalPages != 2 {
		t.Fatalf("unexpected pagination %#v", pagination)
	}
	if len(items) != 2 {
		t.Fatalf("expected only the requested page rows, got %#v", items)
	}
	if items[0].Name != "gamma.txt" || items[1].Name != "资料夹" {
		t.Fatalf("unexpected page order %#v", items)
	}

	sizeQuery := teacherListQuery{
		Sort:     "size-desc",
		Page:     1,
		PageSize: 2,
		Offset:   0,
		Paged:    true,
	}
	sizeSorted, _, err := app.listFileEntriesPage("library", nil, "/SQL分页", sizeQuery)
	if err != nil {
		t.Fatalf("list size sorted page: %v", err)
	}
	if len(sizeSorted) != 2 || sizeSorted[0].Name != "资料夹" || sizeSorted[0].Size != 4096 {
		t.Fatalf("expected recursive folder size to participate in SQL paging, got %#v", sizeSorted)
	}
}

func TestApplyFileEntryFolderSizesUsesBatchQuery(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	if bytes.Contains(source, []byte("fileEntryFolderSize(entries[index])")) {
		t.Fatalf("applyFileEntryFolderSizes should not query once per directory")
	}
}

func TestFileArchivesStreamZipPayloads(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	if bytes.Contains(source, []byte("func (app *App) buildDirectoryArchive(entry *fileEntry) ([]byte, error)")) {
		t.Fatalf("directory archives should stream to an io.Writer instead of returning []byte")
	}
	if bytes.Contains(source, []byte("func (app *App) buildFileEntriesArchive(entries []fileEntry) ([]byte, error)")) {
		t.Fatalf("batch file archives should stream to an io.Writer instead of returning []byte")
	}
}

func TestSearchSupportsPlainKeywordsAndDirectiveCombinations(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	if _, err := app.createFolder("library", nil, "/", "课件目录"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := app.createFile("library", nil, "/", "Alpha课件.PDF", strings.NewReader("%PDF-1.7 alpha courseware")); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := app.createFile("library", nil, "/", "alpha课件.txt", strings.NewReader("alpha text courseware")); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := app.createFile("library", nil, "/", "Alpha讲义.PDF", strings.NewReader("%PDF-1.7 alpha handout")); err != nil {
		t.Fatalf("create file: %v", err)
	}

	plainKeyword := searchFiles(t, app, cookie, url.Values{
		"space": {"library"},
		"q":     {"课件"},
	})
	if got := len(plainKeyword.Items); got != 3 {
		t.Fatalf("expected 3 plain keyword results, got %#v", plainKeyword.Items)
	}

	combined := searchFiles(t, app, cookie, url.Values{
		"space": {"library"},
		"q":     {"alpha type:pdf case:insensitive"},
	})
	if got := len(combined.Items); got != 2 {
		t.Fatalf("expected 2 combined results, got %#v", combined.Items)
	}
	if combined.Items[0].Name != "Alpha讲义.PDF" || combined.Items[1].Name != "Alpha课件.PDF" {
		t.Fatalf("unexpected combined search results %#v", combined.Items)
	}
}

func TestStudentCannotDownloadDraftOrOtherClassAssignmentAttachments(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	draftAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "草稿附件作业",
		Description: "学生不可下载草稿附件",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "draft",
	})
	if err != nil {
		t.Fatalf("create draft assignment: %v", err)
	}
	otherClassAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     2,
		Title:       "其他班附件作业",
		Description: "学生不可下载其他班附件",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create other class assignment: %v", err)
	}

	draftBody, draftContentType := multipartFileBody(t, map[string]string{}, "files", "draft.txt", []byte("draft body"))
	draftRecorder := httptest.NewRecorder()
	draftRequest := httptest.NewRequest(http.MethodPost, "/api/assignments/"+itoa(draftAssignment.ID)+"/attachments?classId=1", draftBody)
	draftRequest.Header.Set("Content-Type", draftContentType)
	draftRequest.AddCookie(teacherCookie)
	app.ServeHTTP(draftRecorder, draftRequest)
	if draftRecorder.Code != http.StatusCreated {
		t.Fatalf("expected draft attachment upload 201, got %d: %s", draftRecorder.Code, draftRecorder.Body.String())
	}
	var draftUploaded assignmentAttachmentsResponse
	decodeJSON(t, draftRecorder.Body, &draftUploaded)

	otherBody, otherContentType := multipartFileBody(t, map[string]string{}, "files", "other-class.txt", []byte("other class body"))
	otherRecorder := httptest.NewRecorder()
	otherRequest := httptest.NewRequest(http.MethodPost, "/api/assignments/"+itoa(otherClassAssignment.ID)+"/attachments?classId=2", otherBody)
	otherRequest.Header.Set("Content-Type", otherContentType)
	otherRequest.AddCookie(teacherCookie)
	app.ServeHTTP(otherRecorder, otherRequest)
	if otherRecorder.Code != http.StatusCreated {
		t.Fatalf("expected other class attachment upload 201, got %d: %s", otherRecorder.Code, otherRecorder.Body.String())
	}
	var otherUploaded assignmentAttachmentsResponse
	decodeJSON(t, otherRecorder.Body, &otherUploaded)

	testCases := []struct {
		name string
		path string
	}{
		{
			name: "draft attachment",
			path: "/api/student/assignments/" + itoa(draftAssignment.ID) + "/attachments/" + itoa(draftUploaded.Items[0].ID) + "/download",
		},
		{
			name: "other class attachment",
			path: "/api/student/assignments/" + itoa(otherClassAssignment.ID) + "/attachments/" + itoa(otherUploaded.Items[0].ID) + "/download",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			request.AddCookie(studentCookie)
			app.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("expected attachment download 404, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestStudentCanReadPublicAndOwnClassFilesOnly(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	otherClass, err := app.createClass("隔壁班")
	if err != nil {
		t.Fatalf("create other class: %v", err)
	}
	otherClassFile, err := app.createSeedFile("class", optionalInt64(otherClass.ID), "/", "隔壁班资料.txt", []byte("not visible"))
	if err != nil {
		t.Fatalf("create other class file: %v", err)
	}
	libraryFile, err := app.createSeedFile("library", nil, "/", "老师个人资料.txt", []byte("teacher private"))
	if err != nil {
		t.Fatalf("create teacher library file: %v", err)
	}
	if _, err := app.createFolder("public", nil, "/", "公开文件夹"); err != nil {
		t.Fatalf("create public folder: %v", err)
	}
	if _, err := app.createFile("public", nil, "/公开文件夹", "nested-public.bin", strings.NewReader(strings.Repeat("x", 1536))); err != nil {
		t.Fatalf("create nested public file: %v", err)
	}

	publicRecorder := httptest.NewRecorder()
	publicRequest := httptest.NewRequest(http.MethodGet, "/api/student/files?space=public", nil)
	publicRequest.AddCookie(studentCookie)
	app.ServeHTTP(publicRecorder, publicRequest)
	if publicRecorder.Code != http.StatusOK {
		t.Fatalf("expected public student files 200, got %d: %s", publicRecorder.Code, publicRecorder.Body.String())
	}
	var publicPayload filesResponse
	decodeJSON(t, publicRecorder.Body, &publicPayload)
	publicFile := findFileByName(t, publicPayload.Items, "公共通知.txt")
	publicFolder := findFileByName(t, publicPayload.Items, "公开文件夹")
	if publicFolder.Size != 1536 {
		t.Fatalf("expected student folder recursive size 1536, got %#v", publicFolder)
	}
	expectedPublicDownload := "/api/student/files/" + itoa(publicFile.ID) + "/download"
	if publicFile.DownloadURL != expectedPublicDownload {
		t.Fatalf("expected student public download url %q, got %#v", expectedPublicDownload, publicFile)
	}

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, publicFile.DownloadURL, nil)
	downloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK || !strings.Contains(downloadRecorder.Body.String(), "公共资料") {
		t.Fatalf("expected public file download, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}

	publicPreviewRecorder := httptest.NewRecorder()
	publicPreviewRequest := httptest.NewRequest(http.MethodGet, publicFile.PreviewURL, nil)
	publicPreviewRequest.AddCookie(studentCookie)
	app.ServeHTTP(publicPreviewRecorder, publicPreviewRequest)
	if publicPreviewRecorder.Code != http.StatusOK || !strings.Contains(publicPreviewRecorder.Body.String(), "公共资料") {
		t.Fatalf("expected public file preview, got %d: %s", publicPreviewRecorder.Code, publicPreviewRecorder.Body.String())
	}

	classRecorder := httptest.NewRecorder()
	classRequest := httptest.NewRequest(http.MethodGet, "/api/student/files?space=class", nil)
	classRequest.AddCookie(studentCookie)
	app.ServeHTTP(classRecorder, classRequest)
	if classRecorder.Code != http.StatusOK {
		t.Fatalf("expected class student files 200, got %d: %s", classRecorder.Code, classRecorder.Body.String())
	}
	var classPayload filesResponse
	decodeJSON(t, classRecorder.Body, &classPayload)
	classFile := findFileByName(t, classPayload.Items, "班级守则.txt")
	for _, item := range classPayload.Items {
		if item.Name == otherClassFile.Name {
			t.Fatalf("student class files leaked other class file %#v", item)
		}
	}

	classPreviewRecorder := httptest.NewRecorder()
	classPreviewRequest := httptest.NewRequest(http.MethodGet, classFile.PreviewURL, nil)
	classPreviewRequest.AddCookie(studentCookie)
	app.ServeHTTP(classPreviewRecorder, classPreviewRequest)
	if classPreviewRecorder.Code != http.StatusOK || !strings.Contains(classPreviewRecorder.Body.String(), "课堂整洁") {
		t.Fatalf("expected class file preview, got %d: %s", classPreviewRecorder.Code, classPreviewRecorder.Body.String())
	}

	libraryRecorder := httptest.NewRecorder()
	libraryRequest := httptest.NewRequest(http.MethodGet, "/api/student/files?space=library", nil)
	libraryRequest.AddCookie(studentCookie)
	app.ServeHTTP(libraryRecorder, libraryRequest)
	if libraryRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected library student files 403, got %d: %s", libraryRecorder.Code, libraryRecorder.Body.String())
	}

	for _, endpoint := range []string{"download", "preview", "archive"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/student/files/"+itoa(libraryFile.ID)+"/"+endpoint, nil)
		request.AddCookie(studentCookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("expected teacher library %s 404 for student, got %d: %s", endpoint, recorder.Code, recorder.Body.String())
		}
	}

	otherDownloadRecorder := httptest.NewRecorder()
	otherDownloadRequest := httptest.NewRequest(http.MethodGet, "/api/student/files/"+itoa(otherClassFile.ID)+"/download", nil)
	otherDownloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(otherDownloadRecorder, otherDownloadRequest)
	if otherDownloadRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected other class download 404, got %d: %s", otherDownloadRecorder.Code, otherDownloadRecorder.Body.String())
	}

	otherPreviewRecorder := httptest.NewRecorder()
	otherPreviewRequest := httptest.NewRequest(http.MethodGet, "/api/student/files/"+itoa(otherClassFile.ID)+"/preview", nil)
	otherPreviewRequest.AddCookie(studentCookie)
	app.ServeHTTP(otherPreviewRecorder, otherPreviewRequest)
	if otherPreviewRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected other class preview 404, got %d: %s", otherPreviewRecorder.Code, otherPreviewRecorder.Body.String())
	}
}

func TestStudentFilesSearchFiltersReadableFiles(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	if _, err := app.createSeedFile("public", nil, "/", "学生可见图片.png", []byte("visible image")); err != nil {
		t.Fatalf("create public search match: %v", err)
	}
	if _, err := app.createSeedFile("public", nil, "/", "普通资料.txt", []byte("ordinary public")); err != nil {
		t.Fatalf("create public search miss: %v", err)
	}
	if _, err := app.createSeedFile("class", optionalInt64(1), "/", "班级搜索资料.txt", []byte("class search")); err != nil {
		t.Fatalf("create class search match: %v", err)
	}
	if _, err := app.createSeedFile("class", optionalInt64(1), "/", "班级常规资料.txt", []byte("class ordinary")); err != nil {
		t.Fatalf("create class search miss: %v", err)
	}

	publicRecorder := httptest.NewRecorder()
	publicRequest := httptest.NewRequest(http.MethodGet, "/api/student/files?space=public&q="+url.QueryEscape("学生可见"), nil)
	publicRequest.AddCookie(studentCookie)
	app.ServeHTTP(publicRecorder, publicRequest)
	if publicRecorder.Code != http.StatusOK {
		t.Fatalf("expected public student search 200, got %d: %s", publicRecorder.Code, publicRecorder.Body.String())
	}
	var publicPayload filesResponse
	decodeJSON(t, publicRecorder.Body, &publicPayload)
	if len(publicPayload.Items) != 1 || publicPayload.Items[0].Name != "学生可见图片.png" {
		t.Fatalf("expected only matching public file, got %#v", publicPayload.Items)
	}

	classRecorder := httptest.NewRecorder()
	classRequest := httptest.NewRequest(http.MethodGet, "/api/student/files?space=class&q="+url.QueryEscape("班级搜索"), nil)
	classRequest.AddCookie(studentCookie)
	app.ServeHTTP(classRecorder, classRequest)
	if classRecorder.Code != http.StatusOK {
		t.Fatalf("expected class student search 200, got %d: %s", classRecorder.Code, classRecorder.Body.String())
	}
	var classPayload filesResponse
	decodeJSON(t, classRecorder.Body, &classPayload)
	if len(classPayload.Items) != 1 || classPayload.Items[0].Name != "班级搜索资料.txt" {
		t.Fatalf("expected only matching class file, got %#v", classPayload.Items)
	}
}

func TestStudentFileAccessIsReadOnly(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	publicRecorder := httptest.NewRecorder()
	publicRequest := httptest.NewRequest(http.MethodGet, "/api/student/files?space=public", nil)
	publicRequest.AddCookie(studentCookie)
	app.ServeHTTP(publicRecorder, publicRequest)
	if publicRecorder.Code != http.StatusOK {
		t.Fatalf("expected public student files 200, got %d: %s", publicRecorder.Code, publicRecorder.Body.String())
	}
	var publicPayload filesResponse
	decodeJSON(t, publicRecorder.Body, &publicPayload)
	publicFile := findFileByName(t, publicPayload.Items, "公共通知.txt")

	studentWriteEndpoints := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/student/files?space=public"},
		{method: http.MethodPatch, path: "/api/student/files/" + itoa(publicFile.ID) + "/download"},
		{method: http.MethodDelete, path: "/api/student/files/" + itoa(publicFile.ID) + "/preview"},
	}
	for _, endpoint := range studentWriteEndpoints {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(endpoint.method, endpoint.path, nil)
		request.AddCookie(studentCookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected student endpoint %s %s to be read-only, got %d: %s", endpoint.method, endpoint.path, recorder.Code, recorder.Body.String())
		}
	}

	teacherWriteEndpoints := []struct {
		method string
		path   string
		body   io.Reader
	}{
		{method: http.MethodDelete, path: "/api/files/" + itoa(publicFile.ID), body: nil},
		{method: http.MethodPost, path: "/api/files/copy", body: strings.NewReader(`{}`)},
		{method: http.MethodPost, path: "/api/files/move", body: strings.NewReader(`{}`)},
	}
	for _, endpoint := range teacherWriteEndpoints {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(endpoint.method, endpoint.path, endpoint.body)
		request.AddCookie(studentCookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected teacher write endpoint %s %s to reject student session, got %d: %s", endpoint.method, endpoint.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestStudentSubmissionCreateIncrementalAndDeadlineGuard(t *testing.T) {
	t.Parallel()

	baseDir, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	currentStudent, err := app.findStudentByClassAndNo(1, "20260001")
	if err != nil || currentStudent == nil {
		t.Fatalf("find current student: %v, student=%#v", err, currentStudent)
	}

	published, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "未来作业",
		Description: "允许提交",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create future assignment: %v", err)
	}

	firstBody, firstContentType := multipartFileBody(t, map[string]string{}, "files", "first.txt", []byte("first submission"))
	firstRecorder := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(published.ID)+"/submission", firstBody)
	firstRequest.Header.Set("Content-Type", firstContentType)
	firstRequest.AddCookie(studentCookie)
	app.ServeHTTP(firstRecorder, firstRequest)
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("expected first submission 200, got %d: %s", firstRecorder.Code, firstRecorder.Body.String())
	}

	var firstPayload studentSubmissionResponse
	decodeJSON(t, firstRecorder.Body, &firstPayload)
	if firstPayload.Submission == nil || firstPayload.Submission.Status != "submitted" || len(firstPayload.Items) != 1 || firstPayload.Items[0].Name != "first.txt" {
		t.Fatalf("unexpected first submission payload %#v", firstPayload)
	}
	if countSubmissionFileEntries(t, app.db, firstPayload.Submission.ID, 1) != 1 {
		t.Fatalf("expected 1 submission file entry after first submit")
	}
	firstSubmissionDir := filepath.Join(baseDir, "var", "storage", "submissions", "1", itoa(firstPayload.Submission.ID))
	if _, err := os.Stat(firstSubmissionDir); err != nil {
		t.Fatalf("expected first submission directory to exist: %v", err)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments/"+itoa(published.ID), nil)
	detailRequest.AddCookie(studentCookie)
	app.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected detail 200, got %d: %s", detailRecorder.Code, detailRecorder.Body.String())
	}

	var detailPayload studentAssignmentPayload
	decodeJSON(t, detailRecorder.Body, &detailPayload)
	if detailPayload.Submission == nil || detailPayload.Submission.ID != firstPayload.Submission.ID || len(detailPayload.Items) != 1 {
		t.Fatalf("unexpected detail payload %#v", detailPayload)
	}
	if detailPayload.Overdue {
		t.Fatalf("expected future assignment detail not overdue")
	}

	secondBody, secondContentType := multipartFileBody(t, map[string]string{}, "files", "second.txt", []byte("second submission"))
	secondRecorder := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(published.ID)+"/submission", secondBody)
	secondRequest.Header.Set("Content-Type", secondContentType)
	secondRequest.AddCookie(studentCookie)
	app.ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("expected overwrite submission 200, got %d: %s", secondRecorder.Code, secondRecorder.Body.String())
	}

	var secondPayload studentSubmissionResponse
	decodeJSON(t, secondRecorder.Body, &secondPayload)
	if secondPayload.Submission == nil || secondPayload.Submission.ID != firstPayload.Submission.ID {
		t.Fatalf("expected incremental submit to reuse submission id, got first=%d second=%#v", firstPayload.Submission.ID, secondPayload.Submission)
	}
	if len(secondPayload.Items) != 2 {
		t.Fatalf("expected incremental payload to keep previous file, got %#v", secondPayload)
	}
	if secondPayload.Items[0].Name != "first.txt" || secondPayload.Items[1].Name != "second.txt" {
		t.Fatalf("expected first and second files after incremental submit, got %#v", secondPayload.Items)
	}
	if countSubmissionFileEntries(t, app.db, secondPayload.Submission.ID, 1) != 2 {
		t.Fatalf("expected 2 submission file entries after incremental submit")
	}
	if countSubmissionsByAssignmentAndStudent(t, app.db, published.ID, currentStudent.ID) != 1 {
		t.Fatalf("expected only one current submission row")
	}

	expired, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "已截止作业",
		Description: "不可再提交",
		DueAt:       time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create expired assignment: %v", err)
	}

	expiredBody, expiredContentType := multipartFileBody(t, map[string]string{}, "files", "late.txt", []byte("late submission"))
	expiredRecorder := httptest.NewRecorder()
	expiredRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(expired.ID)+"/submission", expiredBody)
	expiredRequest.Header.Set("Content-Type", expiredContentType)
	expiredRequest.AddCookie(studentCookie)
	app.ServeHTTP(expiredRecorder, expiredRequest)
	if expiredRecorder.Code != http.StatusConflict {
		t.Fatalf("expected deadline guard 409, got %d: %s", expiredRecorder.Code, expiredRecorder.Body.String())
	}
}

func TestStudentSubmissionIncrementalPartialAndDeleteFlow(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	currentStudent, err := app.findStudentByClassAndNo(1, "20260001")
	if err != nil || currentStudent == nil {
		t.Fatalf("find current student: %v, student=%#v", err, currentStudent)
	}

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:        1,
		Title:          "分次提交作业",
		Description:    "允许学生做完一个交一个",
		DueAt:          time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:         "published",
		SubmissionMode: "files",
		MinFileCount:   3,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	submitFiles := func(files []multipartFileSpec) studentSubmissionResponse {
		t.Helper()
		body, contentType := multipartFilesBody(t, map[string]string{}, files)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", body)
		request.Header.Set("Content-Type", contentType)
		request.AddCookie(studentCookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected submission 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var payload studentSubmissionResponse
		decodeJSON(t, recorder.Body, &payload)
		return payload
	}
	itemByName := func(items []fileSummary, name string) fileSummary {
		t.Helper()
		for _, item := range items {
			if item.Name == name {
				return item
			}
		}
		t.Fatalf("expected item %q in %#v", name, items)
		return fileSummary{}
	}
	deleteSubmissionFile := func(fileID int64) studentSubmissionResponse {
		t.Helper()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/api/student/assignments/"+itoa(assignment.ID)+"/submission/files/"+itoa(fileID), nil)
		request.AddCookie(studentCookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected delete submission file 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var payload studentSubmissionResponse
		decodeJSON(t, recorder.Body, &payload)
		return payload
	}

	firstPayload := submitFiles([]multipartFileSpec{{
		FieldName: "files",
		FileName:  "first.txt",
		Contents:  []byte("first version"),
	}})
	if firstPayload.Submission == nil || firstPayload.Submission.Status != "partial" || len(firstPayload.Items) != 1 || firstPayload.Items[0].Name != "first.txt" {
		t.Fatalf("expected partial first submission with one file, got %#v", firstPayload)
	}
	if countSubmissionsByAssignmentAndStudent(t, app.db, assignment.ID, currentStudent.ID) != 1 {
		t.Fatalf("expected one submission row after partial save")
	}

	statsAfterPartial, err := app.buildAssignmentStatistics(1, []int64{assignment.ID})
	if err != nil {
		t.Fatalf("build stats after partial: %v", err)
	}
	if statsAfterPartial.SubmittedTotal != 0 || statsAfterPartial.MissingTotal != 1 {
		t.Fatalf("partial submission should not count as submitted, got %#v", statsAfterPartial)
	}

	secondPayload := submitFiles([]multipartFileSpec{{
		FieldName: "files",
		FileName:  "second.txt",
		Contents:  []byte("second version"),
	}})
	if secondPayload.Submission == nil || secondPayload.Submission.ID != firstPayload.Submission.ID || secondPayload.Submission.Status != "partial" {
		t.Fatalf("expected same partial submission row after second file, got %#v", secondPayload)
	}
	if len(secondPayload.Items) != 2 {
		t.Fatalf("expected previous and new files after second submit, got %#v", secondPayload.Items)
	}
	itemByName(secondPayload.Items, "first.txt")
	itemByName(secondPayload.Items, "second.txt")

	thirdPayload := submitFiles([]multipartFileSpec{
		{
			FieldName: "files",
			FileName:  "first.txt",
			Contents:  []byte("first replacement"),
		},
		{
			FieldName: "files",
			FileName:  "third.txt",
			Contents:  []byte("third version"),
		},
	})
	if thirdPayload.Submission == nil || thirdPayload.Submission.Status != "submitted" {
		t.Fatalf("expected submission to become submitted after third file, got %#v", thirdPayload.Submission)
	}
	if len(thirdPayload.Items) != 3 {
		t.Fatalf("expected three current files after merge, got %#v", thirdPayload.Items)
	}
	firstItem := itemByName(thirdPayload.Items, "first.txt")
	itemByName(thirdPayload.Items, "second.txt")
	itemByName(thirdPayload.Items, "third.txt")

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, firstItem.DownloadURL, nil)
	downloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK || downloadRecorder.Body.String() != "first replacement" {
		t.Fatalf("expected replaced first file body, got status=%d body=%q", downloadRecorder.Code, downloadRecorder.Body.String())
	}

	secondItem := itemByName(thirdPayload.Items, "second.txt")
	afterDelete := deleteSubmissionFile(secondItem.ID)
	if afterDelete.Submission == nil || afterDelete.Submission.Status != "partial" || len(afterDelete.Items) != 2 {
		t.Fatalf("expected deletion to recalculate partial status, got %#v", afterDelete)
	}

	afterDelete = deleteSubmissionFile(itemByName(afterDelete.Items, "first.txt").ID)
	afterDelete = deleteSubmissionFile(itemByName(afterDelete.Items, "third.txt").ID)
	if afterDelete.Submission != nil || len(afterDelete.Items) != 0 {
		t.Fatalf("expected empty response after deleting last file, got %#v", afterDelete)
	}
	if countSubmissionsByAssignmentAndStudent(t, app.db, assignment.ID, currentStudent.ID) != 0 {
		t.Fatalf("expected submission row removed after deleting last file")
	}
}

func TestStudentCannotDeleteSubmissionFileAfterDeadline(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:      1,
		Title:        "删除截止保护",
		Description:  "用于测试截止后不能删除",
		DueAt:        time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:       "published",
		MinFileCount: 1,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	body, contentType := multipartFileBody(t, map[string]string{}, "files", "answer.txt", []byte("answer"))
	submitRecorder := httptest.NewRecorder()
	submitRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", body)
	submitRequest.Header.Set("Content-Type", contentType)
	submitRequest.AddCookie(studentCookie)
	app.ServeHTTP(submitRecorder, submitRequest)
	if submitRecorder.Code != http.StatusOK {
		t.Fatalf("expected submission 200, got %d: %s", submitRecorder.Code, submitRecorder.Body.String())
	}
	var submitPayload studentSubmissionResponse
	decodeJSON(t, submitRecorder.Body, &submitPayload)
	if len(submitPayload.Items) != 1 {
		t.Fatalf("expected one submitted file, got %#v", submitPayload)
	}

	if _, err := app.db.Exec(`update assignments set due_at = ? where id = ?`, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), assignment.ID); err != nil {
		t.Fatalf("expire assignment: %v", err)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/student/assignments/"+itoa(assignment.ID)+"/submission/files/"+itoa(submitPayload.Items[0].ID), nil)
	deleteRequest.AddCookie(studentCookie)
	app.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusConflict {
		t.Fatalf("expected delete after deadline 409, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}

func TestStudentSubmissionSupportsFolderUploadAndTeacherArchive(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "文件夹提交",
		Description: "保留目录层级",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	body, contentType := multipartFilesBody(t, map[string]string{}, []multipartFileSpec{
		{
			FieldName:    "files",
			FileName:     "one.txt",
			Contents:     []byte("first image placeholder"),
			RelativePath: "20260001-张小明/photos/one.txt",
		},
		{
			FieldName:    "files",
			FileName:     "two.txt",
			Contents:     []byte("second image placeholder"),
			RelativePath: "20260001-张小明/two.txt",
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", body)
	request.Header.Set("Content-Type", contentType)
	request.AddCookie(studentCookie)
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected folder submission 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload studentSubmissionResponse
	decodeJSON(t, recorder.Body, &payload)
	if len(payload.Items) != 1 || payload.Items[0].Kind != "dir" || payload.Items[0].Name != "20260001-张小明" {
		t.Fatalf("expected root folder in submission payload, got %#v", payload.Items)
	}
	if payload.Items[0].FileCount != 2 || payload.Items[0].FolderCount != 1 {
		t.Fatalf("expected folder file stats, got %#v", payload.Items[0])
	}
	if payload.Submission == nil || countSubmissionFileEntries(t, app.db, payload.Submission.ID, 1) != 4 {
		t.Fatalf("expected folder, subfolder and 2 files in submission entries")
	}

	folderDownloadRecorder := httptest.NewRecorder()
	folderDownloadRequest := httptest.NewRequest(http.MethodGet, payload.Items[0].DownloadURL, nil)
	folderDownloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(folderDownloadRecorder, folderDownloadRequest)
	if folderDownloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected folder archive download 200, got %d: %s", folderDownloadRecorder.Code, folderDownloadRecorder.Body.String())
	}
	folderEntries := unzipEntries(t, folderDownloadRecorder.Body.Bytes())
	if folderEntries["20260001-张小明/photos/one.txt"] != "first image placeholder" || folderEntries["20260001-张小明/two.txt"] != "second image placeholder" {
		t.Fatalf("unexpected folder archive entries %#v", folderEntries)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/submissions?classId=1", nil)
	listRequest.AddCookie(teacherCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submissions 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload teacherAssignmentSubmissionsResponse
	decodeJSON(t, listRecorder.Body, &listPayload)
	if len(listPayload.Submissions) != 1 || len(listPayload.Submissions[0].Items) != 1 || listPayload.Submissions[0].Items[0].FileCount != 2 {
		t.Fatalf("expected teacher to see submitted folder stats, got %#v", listPayload.Submissions)
	}
	rootFolder := listPayload.Submissions[0].Items[0]
	if len(rootFolder.Children) != 2 {
		t.Fatalf("expected teacher folder payload to include child entries, got %#v", rootFolder)
	}
	if rootFolder.Children[0].Name != "photos" || rootFolder.Children[0].Kind != "dir" || len(rootFolder.Children[0].Children) != 1 {
		t.Fatalf("expected nested photos folder in teacher payload, got %#v", rootFolder.Children)
	}
	if rootFolder.Children[0].Children[0].Name != "one.txt" || rootFolder.Children[0].Children[0].PreviewURL == "" {
		t.Fatalf("expected previewable nested file in teacher payload, got %#v", rootFolder.Children[0].Children)
	}
	if rootFolder.Children[1].Name != "two.txt" || rootFolder.Children[1].PreviewURL == "" {
		t.Fatalf("expected root-level submitted file in teacher payload, got %#v", rootFolder.Children[1])
	}

	archiveRecorder := httptest.NewRecorder()
	archiveRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/submissions/archive?classId=1&assignmentIds="+itoa(assignment.ID), nil)
	archiveRequest.AddCookie(teacherCookie)
	app.ServeHTTP(archiveRecorder, archiveRequest)
	if archiveRecorder.Code != http.StatusOK {
		t.Fatalf("expected assignment submissions archive 200, got %d: %s", archiveRecorder.Code, archiveRecorder.Body.String())
	}
	archiveEntries := unzipEntries(t, archiveRecorder.Body.Bytes())
	if archiveEntries["文件夹提交/20260001-张小明/photos/one.txt"] != "first image placeholder" || archiveEntries["文件夹提交/20260001-张小明/two.txt"] != "second image placeholder" {
		t.Fatalf("unexpected assignment archive entries %#v", archiveEntries)
	}
}

func TestTeacherAssignmentStatisticsAggregatesFullRosterAndSelectedAssignments(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	students := make([]*studentResult, 0, 49)
	for index := 1; index <= 49; index++ {
		student, err := app.createStudent(createStudentRequest{
			ClassID:     1,
			StudentNo:   fmt.Sprintf("20261%03d", index),
			DisplayName: fmt.Sprintf("学生%d", index),
		})
		if err != nil {
			t.Fatalf("create student %d: %v", index, err)
		}
		students = append(students, student)
	}

	firstAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID: 1,
		Title:   "第一课统计",
		DueAt:   time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:  "published",
	})
	if err != nil {
		t.Fatalf("create first assignment: %v", err)
	}
	secondAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID: 1,
		Title:   "第二课统计",
		DueAt:   time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:  "published",
	})
	if err != nil {
		t.Fatalf("create second assignment: %v", err)
	}
	otherClass, err := app.createClass("统计隔壁班")
	if err != nil {
		t.Fatalf("create other class: %v", err)
	}
	otherAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID: otherClass.ID,
		Title:   "隔壁班统计",
		DueAt:   time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:  "published",
	})
	if err != nil {
		t.Fatalf("create other assignment: %v", err)
	}

	insertSubmission := func(assignmentID int64, student *studentResult) {
		t.Helper()
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := app.db.Exec(`
insert into assignment_submissions (assignment_id, student_id, status, submitted_at, updated_at, review_status, teacher_comment)
values (?, ?, ?, ?, ?, ?, ?)`,
			assignmentID,
			student.ID,
			"submitted",
			now,
			now,
			assignmentSubmissionReviewPending,
			"",
		); err != nil {
			t.Fatalf("insert submission assignment %d student %d: %v", assignmentID, student.ID, err)
		}
	}
	for index := 0; index < 22; index++ {
		insertSubmission(firstAssignment.ID, students[index])
	}
	for index := 0; index < 10; index++ {
		insertSubmission(secondAssignment.ID, students[index])
	}
	insertSubmission(secondAssignment.ID, students[22])

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/assignments/statistics?classId=1&assignmentIds="+itoa(firstAssignment.ID)+","+itoa(secondAssignment.ID),
		nil,
	)
	request.AddCookie(teacherCookie)
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected statistics 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload assignmentStatisticsResponse
	decodeJSON(t, recorder.Body, &payload)
	if payload.ClassID != 1 || payload.RosterTotal != 49 || payload.AssignmentTotal != 2 || payload.ExpectedTotal != 98 {
		t.Fatalf("unexpected statistics totals %#v", payload)
	}
	if payload.SubmittedTotal != 33 || payload.MissingTotal != 65 {
		t.Fatalf("expected submitted/missing 33/65, got %#v", payload)
	}
	if len(payload.Rows) != 49 {
		t.Fatalf("expected one row per roster student, got %d", len(payload.Rows))
	}

	rowsByName := make(map[string]assignmentStatisticsRowPayload, len(payload.Rows))
	for _, row := range payload.Rows {
		rowsByName[row.DisplayName] = row
	}
	if rowsByName["学生1"].SubmittedCount != 2 || rowsByName["学生1"].MissingCount != 0 {
		t.Fatalf("expected 学生1 submitted 2 missing 0, got %#v", rowsByName["学生1"])
	}
	if rowsByName["学生23"].SubmittedCount != 1 || rowsByName["学生23"].MissingCount != 1 {
		t.Fatalf("expected 学生23 submitted 1 missing 1, got %#v", rowsByName["学生23"])
	}
	if rowsByName["学生49"].SubmittedCount != 0 || rowsByName["学生49"].MissingCount != 2 {
		t.Fatalf("expected 学生49 submitted 0 missing 2, got %#v", rowsByName["学生49"])
	}

	crossClassRecorder := httptest.NewRecorder()
	crossClassRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/statistics?classId=1&assignmentIds="+itoa(otherAssignment.ID), nil)
	crossClassRequest.AddCookie(teacherCookie)
	app.ServeHTTP(crossClassRecorder, crossClassRequest)
	if crossClassRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected cross-class statistics 404, got %d: %s", crossClassRecorder.Code, crossClassRecorder.Body.String())
	}

	invalidRecorder := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/statistics?classId=1&assignmentIds=bad", nil)
	invalidRequest.AddCookie(teacherCookie)
	app.ServeHTTP(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid assignment IDs 422, got %d: %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}
}

func TestTeacherAssignmentSubmissionsArchiveSupportsSelectedAndAllAssignments(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	if _, err := app.createStudent(createStudentRequest{
		ClassID:     1,
		StudentNo:   "20260002",
		DisplayName: "李待交",
	}); err != nil {
		t.Fatalf("create missing student: %v", err)
	}

	firstAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID: 1,
		Title:   "第一课续做",
		DueAt:   time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:  "published",
	})
	if err != nil {
		t.Fatalf("create first assignment: %v", err)
	}
	secondAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID: 1,
		Title:   "第二课续做",
		DueAt:   time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:  "published",
	})
	if err != nil {
		t.Fatalf("create second assignment: %v", err)
	}
	partialAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:      1,
		Title:        "第三课待补齐",
		DueAt:        time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:       "published",
		MinFileCount: 2,
	})
	if err != nil {
		t.Fatalf("create partial assignment: %v", err)
	}
	otherClass, err := app.createClass("隔壁归档班")
	if err != nil {
		t.Fatalf("create other class: %v", err)
	}
	if _, err := app.db.Exec(`update classes set join_code = ?, registration_enabled = 1 where id = ?`, "EFGH5678", otherClass.ID); err != nil {
		t.Fatalf("set other class join code: %v", err)
	}
	if _, err := app.createStudent(createStudentRequest{
		ClassID:     otherClass.ID,
		StudentNo:   "20269999",
		DisplayName: "隔壁同学",
	}); err != nil {
		t.Fatalf("create other class student: %v", err)
	}
	otherStudentCookie := activateAndLoginStudent(t, app, "EFGH5678", "20269999", "student123")
	otherAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID: otherClass.ID,
		Title:   "隔壁班作业",
		DueAt:   time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:  "published",
	})
	if err != nil {
		t.Fatalf("create other class assignment: %v", err)
	}

	submitFiles := func(cookie *http.Cookie, assignmentID int64, files []multipartFileSpec) {
		t.Helper()
		body, contentType := multipartFilesBody(t, map[string]string{}, files)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignmentID)+"/submission", body)
		request.Header.Set("Content-Type", contentType)
		request.AddCookie(cookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("submit assignment %d status = %d, body = %s", assignmentID, recorder.Code, recorder.Body.String())
		}
	}

	submitFiles(studentCookie, firstAssignment.ID, []multipartFileSpec{
		{FieldName: "files", FileName: "draft.txt", Contents: []byte("first assignment body"), RelativePath: "draft.txt"},
	})
	submitFiles(studentCookie, secondAssignment.ID, []multipartFileSpec{
		{FieldName: "files", FileName: "final.txt", Contents: []byte("second assignment body"), RelativePath: "20260001-张小明/final.txt"},
	})
	submitFiles(studentCookie, partialAssignment.ID, []multipartFileSpec{
		{FieldName: "files", FileName: "partial.txt", Contents: []byte("partial assignment body"), RelativePath: "partial.txt"},
	})
	var firstSubmissionID int64
	if err := app.db.QueryRow(`select id from assignment_submissions where assignment_id = ?`, firstAssignment.ID).Scan(&firstSubmissionID); err != nil {
		t.Fatalf("find first submission: %v", err)
	}
	if _, err := app.db.Exec(`update assignment_submissions set review_status = ?, teacher_comment = ? where id = ?`, assignmentSubmissionReviewReviewed, "书写清晰", firstSubmissionID); err != nil {
		t.Fatalf("review first submission: %v", err)
	}
	submitFiles(otherStudentCookie, otherAssignment.ID, []multipartFileSpec{
		{FieldName: "files", FileName: "other.txt", Contents: []byte("other class body"), RelativePath: "other.txt"},
	})

	selectedRecorder := httptest.NewRecorder()
	selectedRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/submissions/archive?classId=1&assignmentIds="+itoa(firstAssignment.ID), nil)
	selectedRequest.AddCookie(teacherCookie)
	app.ServeHTTP(selectedRecorder, selectedRequest)
	if selectedRecorder.Code != http.StatusOK {
		t.Fatalf("expected selected archive 200, got %d: %s", selectedRecorder.Code, selectedRecorder.Body.String())
	}
	if selectedRecorder.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("expected selected archive zip content type, got %q", selectedRecorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(selectedRecorder.Header().Get("Content-Disposition"), ".zip") {
		t.Fatalf("expected selected archive zip filename, got %q", selectedRecorder.Header().Get("Content-Disposition"))
	}
	selectedEntries := unzipEntries(t, selectedRecorder.Body.Bytes())
	if selectedEntries["第一课续做/20260001-张小明/draft.txt"] != "first assignment body" {
		t.Fatalf("expected selected archive to include first assignment submission, got %#v", selectedEntries)
	}
	selectedManifest := selectedEntries["提交清单.csv"]
	if !strings.Contains(selectedManifest, "作业,学号,姓名,提交时间,文件数,文件路径,批改状态,教师评语") {
		t.Fatalf("expected selected archive manifest header, got %q", selectedManifest)
	}
	if !strings.Contains(selectedManifest, "第一课续做,20260001,张小明,") || !strings.Contains(selectedManifest, ",1,draft.txt") {
		t.Fatalf("expected selected archive manifest to include first assignment file, got %q", selectedManifest)
	}
	if !strings.Contains(selectedManifest, ",已批改,书写清晰") {
		t.Fatalf("expected selected archive manifest to include review summary, got %q", selectedManifest)
	}
	selectedMissingManifest := selectedEntries["未提交清单.csv"]
	if !strings.Contains(selectedMissingManifest, "作业,学号,姓名,状态") || !strings.Contains(selectedMissingManifest, "第一课续做,20260002,李待交,未提交") {
		t.Fatalf("expected selected archive missing manifest, got %q", selectedMissingManifest)
	}
	if _, exists := selectedEntries["第二课续做/20260001-张小明/final.txt"]; exists {
		t.Fatalf("selected archive should not include unselected assignment: %#v", selectedEntries)
	}
	if _, exists := selectedEntries["隔壁班作业/20269999-隔壁同学/other.txt"]; exists {
		t.Fatalf("selected archive should not include another class submission: %#v", selectedEntries)
	}

	allRecorder := httptest.NewRecorder()
	allRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/submissions/archive?classId=1", nil)
	allRequest.AddCookie(teacherCookie)
	app.ServeHTTP(allRecorder, allRequest)
	if allRecorder.Code != http.StatusOK {
		t.Fatalf("expected all archive 200, got %d: %s", allRecorder.Code, allRecorder.Body.String())
	}
	allEntries := unzipEntries(t, allRecorder.Body.Bytes())
	if allEntries["第一课续做/20260001-张小明/draft.txt"] != "first assignment body" {
		t.Fatalf("expected all archive to include first assignment submission, got %#v", allEntries)
	}
	if allEntries["第二课续做/20260001-张小明/final.txt"] != "second assignment body" {
		t.Fatalf("expected all archive to include second assignment submission, got %#v", allEntries)
	}
	if _, exists := allEntries["第三课待补齐/20260001-张小明/partial.txt"]; exists {
		t.Fatalf("all archive should not include partial submission files: %#v", allEntries)
	}
	allManifest := allEntries["提交清单.csv"]
	if !strings.Contains(allManifest, "第一课续做,20260001,张小明,") || !strings.Contains(allManifest, ",1,draft.txt") {
		t.Fatalf("expected all archive manifest to include first assignment file, got %q", allManifest)
	}
	if !strings.Contains(allManifest, "第二课续做,20260001,张小明,") || !strings.Contains(allManifest, ",1,final.txt") {
		t.Fatalf("expected all archive manifest to include second assignment file, got %q", allManifest)
	}
	if strings.Contains(allManifest, "第三课待补齐") {
		t.Fatalf("expected partial assignment to be excluded from submitted manifest, got %q", allManifest)
	}
	allMissingManifest := allEntries["未提交清单.csv"]
	if !strings.Contains(allMissingManifest, "第一课续做,20260002,李待交,未提交") || !strings.Contains(allMissingManifest, "第二课续做,20260002,李待交,未提交") {
		t.Fatalf("expected all archive missing manifest, got %q", allMissingManifest)
	}
	if !strings.Contains(allMissingManifest, "第三课待补齐,20260001,张小明,待补齐") {
		t.Fatalf("expected all archive missing manifest to mark partial submissions, got %q", allMissingManifest)
	}
	if _, exists := allEntries["隔壁班作业/20269999-隔壁同学/other.txt"]; exists {
		t.Fatalf("all archive should not include another class submission: %#v", allEntries)
	}

	invalidRecorder := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/submissions/archive?classId=1&assignmentIds=bad", nil)
	invalidRequest.AddCookie(teacherCookie)
	app.ServeHTTP(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid assignmentIds 422, got %d: %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}

	crossClassRecorder := httptest.NewRecorder()
	crossClassRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/submissions/archive?classId=1&assignmentIds="+itoa(otherAssignment.ID), nil)
	crossClassRequest.AddCookie(teacherCookie)
	app.ServeHTTP(crossClassRecorder, crossClassRequest)
	if crossClassRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected cross-class assignment archive 404, got %d: %s", crossClassRecorder.Code, crossClassRecorder.Body.String())
	}
}

func TestAssignmentSubmissionRulesArePersistedAndReturned(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":        1,
		"title":          "文件夹图片作业",
		"description":    "按学号姓名建文件夹",
		"dueAt":          "",
		"status":         "published",
		"submissionMode": "folder",
		"minFileCount":   5,
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(teacherCookie)
	app.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected assignment create 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)
	if created.SubmissionMode != "folder" || created.MinFileCount != 5 {
		t.Fatalf("expected created submission rule folder/5, got %#v", created)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments?classId=1", nil)
	listRequest.AddCookie(teacherCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected assignments list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listPayload assignmentsResponse
	decodeJSON(t, listRecorder.Body, &listPayload)
	if len(listPayload.Assignments) == 0 || listPayload.Assignments[0].SubmissionMode != "folder" || listPayload.Assignments[0].MinFileCount != 5 {
		t.Fatalf("expected list to include submission rule folder/5, got %#v", listPayload.Assignments)
	}

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/assignments/"+itoa(created.ID)+"?classId=1", jsonBody(t, map[string]any{
		"submissionMode": "files",
		"minFileCount":   2,
	}))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(teacherCookie)
	app.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected assignment update 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated assignmentPayload
	decodeJSON(t, updateRecorder.Body, &updated)
	if updated.Title != "文件夹图片作业" || updated.SubmissionMode != "files" || updated.MinFileCount != 2 {
		t.Fatalf("expected updated submission rule files/2 and preserved title, got %#v", updated)
	}
}

func TestStudentSubmissionRulesRejectInvalidShapeAndTrackPartialCount(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:        1,
		Title:          "五图文件夹作业",
		Description:    "提交学号姓名文件夹",
		DueAt:          time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:         "published",
		SubmissionMode: "folder",
		MinFileCount:   5,
	})
	if err != nil {
		t.Fatalf("create folder rule assignment: %v", err)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments/"+itoa(assignment.ID), nil)
	detailRequest.AddCookie(studentCookie)
	app.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected student detail 200, got %d: %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detailPayload studentAssignmentPayload
	decodeJSON(t, detailRecorder.Body, &detailPayload)
	if detailPayload.SubmissionMode != "folder" || detailPayload.MinFileCount != 5 {
		t.Fatalf("expected student detail to expose folder/5 rule, got %#v", detailPayload)
	}

	singleBody, singleContentType := multipartFileBody(t, map[string]string{}, "files", "single.txt", []byte("single"))
	singleRecorder := httptest.NewRecorder()
	singleRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", singleBody)
	singleRequest.Header.Set("Content-Type", singleContentType)
	singleRequest.AddCookie(studentCookie)
	app.ServeHTTP(singleRecorder, singleRequest)
	if singleRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected plain file submission 422, got %d: %s", singleRecorder.Code, singleRecorder.Body.String())
	}
	var singleError apiErrorResponse
	decodeJSON(t, singleRecorder.Body, &singleError)
	if !strings.Contains(singleError.Error.Message, "提交一个文件夹") {
		t.Fatalf("expected folder rule error, got %#v", singleError)
	}

	shortBody, shortContentType := multipartFilesBody(t, map[string]string{}, []multipartFileSpec{
		{
			FieldName:    "files",
			FileName:     "one.txt",
			Contents:     []byte("one"),
			RelativePath: "20260001-张小明/one.txt",
		},
	})
	shortRecorder := httptest.NewRecorder()
	shortRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", shortBody)
	shortRequest.Header.Set("Content-Type", shortContentType)
	shortRequest.AddCookie(studentCookie)
	app.ServeHTTP(shortRecorder, shortRequest)
	if shortRecorder.Code != http.StatusOK {
		t.Fatalf("expected partial folder submission 200, got %d: %s", shortRecorder.Code, shortRecorder.Body.String())
	}
	var shortPayload studentSubmissionResponse
	decodeJSON(t, shortRecorder.Body, &shortPayload)
	if shortPayload.Submission == nil || shortPayload.Submission.Status != "partial" || len(shortPayload.Items) != 1 || shortPayload.Items[0].FileCount != 1 {
		t.Fatalf("expected partial folder submission with one saved file, got %#v", shortPayload)
	}

	otherRootBody, otherRootContentType := multipartFilesBody(t, map[string]string{}, []multipartFileSpec{
		{
			FieldName:    "files",
			FileName:     "other.txt",
			Contents:     []byte("other root"),
			RelativePath: "另一个文件夹/other.txt",
		},
	})
	otherRootRecorder := httptest.NewRecorder()
	otherRootRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", otherRootBody)
	otherRootRequest.Header.Set("Content-Type", otherRootContentType)
	otherRootRequest.AddCookie(studentCookie)
	app.ServeHTTP(otherRootRecorder, otherRootRequest)
	if otherRootRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected different folder root 422, got %d: %s", otherRootRecorder.Code, otherRootRecorder.Body.String())
	}
	var otherRootError apiErrorResponse
	decodeJSON(t, otherRootRecorder.Body, &otherRootError)
	if !strings.Contains(otherRootError.Error.Message, "只能提交一个文件夹") {
		t.Fatalf("expected existing folder rule error, got %#v", otherRootError)
	}

	validFiles := make([]multipartFileSpec, 0, 4)
	for index := 2; index <= 5; index++ {
		validFiles = append(validFiles, multipartFileSpec{
			FieldName:    "files",
			FileName:     fmt.Sprintf("%d.txt", index),
			Contents:     []byte(fmt.Sprintf("file %d", index)),
			RelativePath: fmt.Sprintf("20260001-张小明/%d.txt", index),
		})
	}
	validBody, validContentType := multipartFilesBody(t, map[string]string{}, validFiles)
	validRecorder := httptest.NewRecorder()
	validRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", validBody)
	validRequest.Header.Set("Content-Type", validContentType)
	validRequest.AddCookie(studentCookie)
	app.ServeHTTP(validRecorder, validRequest)
	if validRecorder.Code != http.StatusOK {
		t.Fatalf("expected valid folder submission 200, got %d: %s", validRecorder.Code, validRecorder.Body.String())
	}
	var validPayload studentSubmissionResponse
	decodeJSON(t, validRecorder.Body, &validPayload)
	if validPayload.Submission == nil || validPayload.Submission.Status != "submitted" || len(validPayload.Items) != 1 || validPayload.Items[0].Kind != "dir" || validPayload.Items[0].FileCount != 5 {
		t.Fatalf("expected accepted submitted folder with 5 files, got %#v", validPayload)
	}
}

func TestStudentCannotAccessOtherClassAssignmentOrOtherStudentFile(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	otherClassAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     2,
		Title:       "二班作业",
		Description: "当前学生不可访问",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create other class assignment: %v", err)
	}

	sharedAssignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "本班作业",
		Description: "用于测试他人提交下载限制",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create shared assignment: %v", err)
	}

	prepareStudentAuthFixture(t, app, "ABCD1234", "20260002", "李小红")
	otherStudentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260002", "student456")

	otherBody, otherContentType := multipartFileBody(t, map[string]string{}, "files", "other.txt", []byte("other student submission"))
	otherRecorder := httptest.NewRecorder()
	otherRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(sharedAssignment.ID)+"/submission", otherBody)
	otherRequest.Header.Set("Content-Type", otherContentType)
	otherRequest.AddCookie(otherStudentCookie)
	app.ServeHTTP(otherRecorder, otherRequest)
	if otherRecorder.Code != http.StatusOK {
		t.Fatalf("expected other student submission 200, got %d: %s", otherRecorder.Code, otherRecorder.Body.String())
	}

	var otherPayload studentSubmissionResponse
	decodeJSON(t, otherRecorder.Body, &otherPayload)
	if len(otherPayload.Items) != 1 {
		t.Fatalf("expected other student uploaded file, got %#v", otherPayload)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments/"+itoa(otherClassAssignment.ID), nil)
	detailRequest.AddCookie(studentCookie)
	app.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected cross-class detail 404, got %d: %s", detailRecorder.Code, detailRecorder.Body.String())
	}

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments/"+itoa(sharedAssignment.ID)+"/submission/files/"+itoa(otherPayload.Items[0].ID)+"/download", nil)
	downloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected other student file download 404, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
}

func TestTeacherAssignmentDetailCanViewCurrentStudentSubmissions(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260002", "李小红")

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "老师查看提交",
		Description: "用于老师端查看当前学生提交",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	studentOneCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	studentTwoCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260002", "student456")

	firstBody, firstContentType := multipartFileBody(t, map[string]string{}, "files", "old-one.txt", []byte("old one"))
	firstRecorder := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", firstBody)
	firstRequest.Header.Set("Content-Type", firstContentType)
	firstRequest.AddCookie(studentOneCookie)
	app.ServeHTTP(firstRecorder, firstRequest)
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("expected first student submission 200, got %d: %s", firstRecorder.Code, firstRecorder.Body.String())
	}

	appendBody, appendContentType := multipartFileBody(t, map[string]string{}, "files", "new-one.txt", []byte("new one"))
	appendRecorder := httptest.NewRecorder()
	appendRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", appendBody)
	appendRequest.Header.Set("Content-Type", appendContentType)
	appendRequest.AddCookie(studentOneCookie)
	app.ServeHTTP(appendRecorder, appendRequest)
	if appendRecorder.Code != http.StatusOK {
		t.Fatalf("expected appended submission 200, got %d: %s", appendRecorder.Code, appendRecorder.Body.String())
	}

	secondBody, secondContentType := multipartFileBody(t, map[string]string{}, "files", "two.txt", []byte("student two"))
	secondRecorder := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", secondBody)
	secondRequest.Header.Set("Content-Type", secondContentType)
	secondRequest.AddCookie(studentTwoCookie)
	app.ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("expected second student submission 200, got %d: %s", secondRecorder.Code, secondRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/submissions?classId=1", nil)
	listRequest.AddCookie(teacherCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submissions list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var payload teacherAssignmentSubmissionsResponse
	decodeJSON(t, listRecorder.Body, &payload)
	if len(payload.Submissions) != 2 {
		t.Fatalf("expected 2 current submissions, got %#v", payload.Submissions)
	}
	if payload.Submissions[0].DisplayName != "李小红" && payload.Submissions[1].DisplayName != "李小红" {
		t.Fatalf("expected 李小红 submission in payload %#v", payload.Submissions)
	}

	var foundStudentOne bool
	for _, submission := range payload.Submissions {
		if submission.DisplayName != "张小明" {
			continue
		}
		foundStudentOne = true
		if len(submission.Items) != 2 {
			t.Fatalf("expected merged current files for 张小明, got %#v", submission.Items)
		}
		var newItem fileSummary
		var foundNewItem bool
		var foundOldItem bool
		for _, item := range submission.Items {
			if item.Name == "new-one.txt" {
				newItem = item
				foundNewItem = true
			}
			if item.Name == "old-one.txt" {
				foundOldItem = true
			}
		}
		if !foundNewItem || !foundOldItem {
			t.Fatalf("expected old and new files after incremental submission, got %#v", submission.Items)
		}
		if newItem.DownloadURL != "/api/assignments/"+itoa(assignment.ID)+"/submissions/files/"+itoa(newItem.ID)+"/download?classId=1" {
			t.Fatalf("unexpected teacher submission download url %#v", newItem)
		}
		expectedPreviewURL := "/api/assignments/" + itoa(assignment.ID) + "/submissions/files/" + itoa(newItem.ID) + "/preview?classId=1"
		if newItem.PreviewURL != expectedPreviewURL {
			t.Fatalf("unexpected teacher submission preview url %#v", newItem)
		}
		previewRecorder := httptest.NewRecorder()
		previewRequest := httptest.NewRequest(http.MethodGet, newItem.PreviewURL, nil)
		previewRequest.AddCookie(teacherCookie)
		app.ServeHTTP(previewRecorder, previewRequest)
		if previewRecorder.Code != http.StatusOK || previewRecorder.Body.String() != "new one" {
			t.Fatalf("expected teacher submission preview 200 with current file, got %d: %s", previewRecorder.Code, previewRecorder.Body.String())
		}
	}
	if !foundStudentOne {
		t.Fatalf("expected 张小明 submission in payload %#v", payload.Submissions)
	}
}

func TestTeacherCanPreviewAndDownloadNestedSubmissionFiles(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "嵌套提交文件",
		Description: "用于老师端预览嵌套提交文件",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	body, contentType := multipartFilesBody(t, map[string]string{}, []multipartFileSpec{
		{FieldName: "files", FileName: "nested.txt", Contents: []byte("nested body"), RelativePath: "任务/nested.txt"},
	})
	submitRecorder := httptest.NewRecorder()
	submitRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", body)
	submitRequest.Header.Set("Content-Type", contentType)
	submitRequest.AddCookie(studentCookie)
	app.ServeHTTP(submitRecorder, submitRequest)
	if submitRecorder.Code != http.StatusOK {
		t.Fatalf("expected student submission 200, got %d: %s", submitRecorder.Code, submitRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/submissions?classId=1", nil)
	listRequest.AddCookie(teacherCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submissions list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var payload teacherAssignmentSubmissionsResponse
	decodeJSON(t, listRecorder.Body, &payload)
	if len(payload.Submissions) != 1 || len(payload.Submissions[0].Items) != 1 {
		t.Fatalf("expected one submission folder, got %#v", payload.Submissions)
	}
	folder := payload.Submissions[0].Items[0]
	if folder.Kind != "dir" || len(folder.Children) != 1 {
		t.Fatalf("expected folder with nested file, got %#v", folder)
	}
	nestedFile := folder.Children[0]
	if nestedFile.Name != "nested.txt" {
		t.Fatalf("expected nested.txt, got %#v", nestedFile)
	}

	previewRecorder := httptest.NewRecorder()
	previewRequest := httptest.NewRequest(http.MethodGet, nestedFile.PreviewURL, nil)
	previewRequest.AddCookie(teacherCookie)
	app.ServeHTTP(previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK || previewRecorder.Body.String() != "nested body" {
		t.Fatalf("expected nested preview 200, got %d: %s", previewRecorder.Code, previewRecorder.Body.String())
	}

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, nestedFile.DownloadURL, nil)
	downloadRequest.AddCookie(teacherCookie)
	app.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK || downloadRecorder.Body.String() != "nested body" {
		t.Fatalf("expected nested download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
}

func TestTeacherAssignmentSubmissionsSupportSearchSortAndPagination(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	fixtures := []struct {
		StudentNo   string
		DisplayName string
		Password    string
		FileName    string
	}{
		{StudentNo: "20260003", DisplayName: "分页提交-C", Password: "student-c", FileName: "c.txt"},
		{StudentNo: "20260001", DisplayName: "无关提交", Password: "student-other", FileName: "other.txt"},
		{StudentNo: "20260002", DisplayName: "分页提交-A", Password: "student-a", FileName: "a.txt"},
	}
	for _, fixture := range fixtures {
		prepareStudentAuthFixture(t, app, "ABCD1234", fixture.StudentNo, fixture.DisplayName)
	}

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "提交分页",
		Description: "用于验证老师端提交列表分页",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	for _, fixture := range fixtures {
		studentCookie := activateAndLoginStudent(t, app, "ABCD1234", fixture.StudentNo, fixture.Password)
		body, contentType := multipartFileBody(t, map[string]string{}, "files", fixture.FileName, []byte(fixture.DisplayName))
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", body)
		request.Header.Set("Content-Type", contentType)
		request.AddCookie(studentCookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected submission upload 200 for %s, got %d: %s", fixture.StudentNo, recorder.Code, recorder.Body.String())
		}
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/submissions?classId=1&q=分页&page=2&pageSize=1&sort=studentNo-asc", nil)
	listRequest.AddCookie(teacherCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submissions list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var payload teacherAssignmentSubmissionsResponse
	decodeJSON(t, listRecorder.Body, &payload)
	if payload.Pagination.Page != 2 || payload.Pagination.PageSize != 1 {
		t.Fatalf("unexpected submissions pagination page/pageSize %#v", payload.Pagination)
	}
	if payload.Pagination.Total != 2 || payload.Pagination.TotalPages != 2 {
		t.Fatalf("expected submissions total 2/2, got %#v", payload.Pagination)
	}
	if len(payload.Submissions) != 1 || payload.Submissions[0].DisplayName != "分页提交-C" {
		t.Fatalf("unexpected submissions page %#v", payload.Submissions)
	}
}

func TestStudentSubmissionValidationRejectsUnsupportedTypeAndOversize(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	currentStudent, err := app.findStudentByClassAndNo(1, "20260001")
	if err != nil || currentStudent == nil {
		t.Fatalf("find current student: %v, student=%#v", err, currentStudent)
	}

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "提交校验",
		Description: "校验文件类型和大小",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	invalidBody, invalidContentType := multipartFileBody(t, map[string]string{}, "files", "virus.exe", []byte("bad"))
	invalidRecorder := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", invalidBody)
	invalidRequest.Header.Set("Content-Type", invalidContentType)
	invalidRequest.AddCookie(studentCookie)
	app.ServeHTTP(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid type 422, got %d: %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}

	var invalidPayload apiErrorResponse
	decodeJSON(t, invalidRecorder.Body, &invalidPayload)
	if invalidPayload.Error.Message != "仅支持 PDF、Word、Excel、PPT、TXT、HTML、Markdown、CSV、JSON、图片、音视频、压缩包 文件" {
		t.Fatalf("unexpected invalid type error %#v", invalidPayload.Error)
	}
	if countSubmissionsByAssignmentAndStudent(t, app.db, assignment.ID, currentStudent.ID) != 0 {
		t.Fatalf("expected no submission row after invalid type")
	}

	err = validateStudentSubmissionHeaders([]*multipart.FileHeader{{
		Filename: "large.pdf",
		Size:     studentSubmissionMaxFileSize + 1,
	}})
	if err == nil {
		t.Fatalf("expected oversize validation error")
	}
	var domainErr domainError
	if !errors.As(err, &domainErr) || domainErr.Message != "单个文件不能超过 100 MB" {
		t.Fatalf("unexpected oversize error %#v", err)
	}
}

func TestStudentSubmissionCategoryValidationUsesAssignmentType(t *testing.T) {
	t.Parallel()

	err := validateStudentSubmissionHeadersForCategory([]*multipart.FileHeader{{
		Filename: "作业.docx",
		Size:     1024,
	}}, assignmentSubmissionTypeImage)
	if err == nil {
		t.Fatalf("expected image assignment to reject word document")
	}
	var domainErr domainError
	if !errors.As(err, &domainErr) || !strings.Contains(domainErr.Message, "图片文件") {
		t.Fatalf("expected image category error, got %#v", err)
	}

	err = validateStudentSubmissionHeadersForCategory([]*multipart.FileHeader{{
		Filename: "效果图.png",
		Size:     1024,
	}}, assignmentSubmissionTypeImage)
	if err != nil {
		t.Fatalf("expected png image accepted, got %v", err)
	}

	err = validateStudentSubmissionHeadersForCategory([]*multipart.FileHeader{{
		Filename: "压缩包.rar",
		Size:     1024,
	}}, assignmentSubmissionTypeArchive)
	if err != nil {
		t.Fatalf("expected rar archive accepted, got %v", err)
	}
}

func TestTeacherAssignmentSubmissionsIncludeReviewSummaryAndCanUpdateReview(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")

	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "批改摘要",
		Description: "老师查看和更新批改摘要",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	submitBody, submitContentType := multipartFileBody(t, map[string]string{}, "files", "answer.txt", []byte("answer"))
	submitRecorder := httptest.NewRecorder()
	submitRequest := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", submitBody)
	submitRequest.Header.Set("Content-Type", submitContentType)
	submitRequest.AddCookie(studentCookie)
	app.ServeHTTP(submitRecorder, submitRequest)
	if submitRecorder.Code != http.StatusOK {
		t.Fatalf("expected submission 200, got %d: %s", submitRecorder.Code, submitRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/submissions?classId=1", nil)
	listRequest.AddCookie(teacherCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submissions list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var beforePayload teacherAssignmentSubmissionsResponse
	decodeJSON(t, listRecorder.Body, &beforePayload)
	if len(beforePayload.Submissions) != 1 {
		t.Fatalf("expected one submission before review, got %#v", beforePayload.Submissions)
	}
	if beforePayload.Submissions[0].ReviewStatus != "pending" || beforePayload.Submissions[0].TeacherCommentSummary != "" || beforePayload.Submissions[0].ReviewedAt != "" || beforePayload.Submissions[0].ReviewerName != "" {
		t.Fatalf("expected default review summary, got %#v", beforePayload.Submissions[0])
	}

	reviewRecorder := httptest.NewRecorder()
	reviewRequest := httptest.NewRequest(http.MethodPatch, "/api/assignments/"+itoa(assignment.ID)+"/submissions/"+itoa(beforePayload.Submissions[0].ID)+"?classId=1", jsonBody(t, map[string]any{
		"reviewStatus":   "reviewed",
		"teacherComment": "书写清晰，继续保持",
	}))
	reviewRequest.Header.Set("Content-Type", "application/json")
	reviewRequest.AddCookie(teacherCookie)
	app.ServeHTTP(reviewRecorder, reviewRequest)
	if reviewRecorder.Code != http.StatusOK {
		t.Fatalf("expected review patch 200, got %d: %s", reviewRecorder.Code, reviewRecorder.Body.String())
	}

	reloadRecorder := httptest.NewRecorder()
	reloadRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/submissions?classId=1", nil)
	reloadRequest.AddCookie(teacherCookie)
	app.ServeHTTP(reloadRecorder, reloadRequest)
	if reloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submissions reload 200, got %d: %s", reloadRecorder.Code, reloadRecorder.Body.String())
	}

	var afterPayload teacherAssignmentSubmissionsResponse
	decodeJSON(t, reloadRecorder.Body, &afterPayload)
	if afterPayload.Submissions[0].ReviewStatus != "reviewed" || afterPayload.Submissions[0].TeacherCommentSummary != "书写清晰，继续保持" {
		t.Fatalf("expected updated review summary, got %#v", afterPayload.Submissions[0])
	}
	if strings.TrimSpace(afterPayload.Submissions[0].ReviewedAt) == "" {
		t.Fatalf("expected reviewedAt to be set after review, got %#v", afterPayload.Submissions[0])
	}
	if afterPayload.Submissions[0].ReviewerName != "示例老师" {
		t.Fatalf("expected reviewerName to be teacher display name, got %#v", afterPayload.Submissions[0])
	}
}

func TestTeacherSettingsProfileGetAndPatch(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/settings/profile", nil)
	getRequest.AddCookie(cookie)
	app.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected profile get 200, got %d: %s", getRecorder.Code, getRecorder.Body.String())
	}

	var initial testTeacherProfileResponse
	decodeJSON(t, getRecorder.Body, &initial)
	if initial.Profile.Username != "admin" || initial.Profile.DisplayName != "示例老师" || initial.Profile.Role != "owner" {
		t.Fatalf("unexpected initial profile %#v", initial.Profile)
	}
	if initial.Profile.Preferences.CompactListEnabled {
		t.Fatalf("expected compactListEnabled default false")
	}

	patchRecorder := httptest.NewRecorder()
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/settings/profile", jsonBody(t, map[string]any{
		"displayName": "王老师",
		"preferences": map[string]any{
			"compactListEnabled": true,
		},
	}))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.AddCookie(cookie)
	app.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("expected profile patch 200, got %d: %s", patchRecorder.Code, patchRecorder.Body.String())
	}

	var patched testTeacherProfileResponse
	decodeJSON(t, patchRecorder.Body, &patched)
	if patched.Profile.DisplayName != "王老师" || !patched.Profile.Preferences.CompactListEnabled {
		t.Fatalf("unexpected patched profile %#v", patched.Profile)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	sessionRequest.AddCookie(cookie)
	app.ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected session 200 after patch, got %d: %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}

	var sessionPayload sessionResponse
	decodeJSON(t, sessionRecorder.Body, &sessionPayload)
	if sessionPayload.User.DisplayName != "王老师" || sessionPayload.User.Role != "owner" {
		t.Fatalf("unexpected session payload %#v", sessionPayload.User)
	}
}

func TestTeacherSettingsPasswordChange(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	changeRecorder := httptest.NewRecorder()
	changeRequest := httptest.NewRequest(http.MethodPost, "/api/settings/password", jsonBody(t, map[string]any{
		"currentPassword": "demo123",
		"newPassword":     "newpass123",
	}))
	changeRequest.Header.Set("Content-Type", "application/json")
	changeRequest.AddCookie(cookie)
	app.ServeHTTP(changeRecorder, changeRequest)
	if changeRecorder.Code != http.StatusOK {
		t.Fatalf("expected password change 200, got %d: %s", changeRecorder.Code, changeRecorder.Body.String())
	}

	oldLoginRecorder := httptest.NewRecorder()
	oldLoginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]any{
		"username": "admin",
		"password": "demo123",
	}))
	oldLoginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(oldLoginRecorder, oldLoginRequest)
	if oldLoginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected old password login 401, got %d: %s", oldLoginRecorder.Code, oldLoginRecorder.Body.String())
	}

	newLoginRecorder := httptest.NewRecorder()
	newLoginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]any{
		"username": "admin",
		"password": "newpass123",
	}))
	newLoginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(newLoginRecorder, newLoginRequest)
	if newLoginRecorder.Code != http.StatusOK {
		t.Fatalf("expected new password login 200, got %d: %s", newLoginRecorder.Code, newLoginRecorder.Body.String())
	}
}

func TestTeacherSettingsSystemOwnerCanReadAndUpdate(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/settings/system", nil)
	getRequest.AddCookie(cookie)
	app.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected system settings get 200, got %d: %s", getRecorder.Code, getRecorder.Body.String())
	}

	var initial testSystemSettingsResponse
	decodeJSON(t, getRecorder.Body, &initial)
	if !initial.Settings.UploadPanelEnabled {
		t.Fatalf("unexpected system settings default %#v", initial.Settings)
	}
	if !initial.Settings.SingleAccountLoginEnabled {
		t.Fatalf("expected single-account login enabled by default, got %#v", initial.Settings)
	}
	if initial.Settings.ServerPort != "80" || initial.Settings.ServerHost == "" {
		t.Fatalf("expected default access host and port, got %#v", initial.Settings)
	}

	patchRecorder := httptest.NewRecorder()
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/settings/system", jsonBody(t, map[string]any{
		"uploadPanelEnabled":        false,
		"singleAccountLoginEnabled": false,
		"serverPort":                "777",
	}))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.AddCookie(cookie)
	app.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("expected system settings patch 200, got %d: %s", patchRecorder.Code, patchRecorder.Body.String())
	}

	var patched testSystemSettingsResponse
	decodeJSON(t, patchRecorder.Body, &patched)
	if patched.Settings.UploadPanelEnabled {
		t.Fatalf("unexpected patched system settings %#v", patched.Settings)
	}
	if patched.Settings.SingleAccountLoginEnabled {
		t.Fatalf("expected single-account login disabled after patch, got %#v", patched.Settings)
	}
	if patched.Settings.ServerPort != "777" || patched.Settings.ServerHost == "" {
		t.Fatalf("unexpected patched access settings %#v", patched.Settings)
	}
}

func TestTeacherSettingsSystemStaffForbidden(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	createTeacherAccount(t, app, "assistant", "助教老师", "assistant123", "staff", false)
	cookie := loginAndGetCookieAs(t, app, "assistant", "assistant123")

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/settings/system", nil)
	getRequest.AddCookie(cookie)
	app.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected system settings get 200 for staff, got %d: %s", getRecorder.Code, getRecorder.Body.String())
	}
	var getPayload testSystemSettingsResponse
	decodeJSON(t, getRecorder.Body, &getPayload)
	if !getPayload.Settings.UploadPanelEnabled {
		t.Fatalf("unexpected system settings for staff %#v", getPayload.Settings)
	}
	if getPayload.Settings.ServerPort != "80" || getPayload.Settings.ServerHost == "" {
		t.Fatalf("expected staff to read access host and default port, got %#v", getPayload.Settings)
	}

	patchRecorder := httptest.NewRecorder()
	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/settings/system", jsonBody(t, map[string]any{
		"uploadPanelEnabled": false,
		"serverPort":         "777",
	}))
	patchRequest.Header.Set("Content-Type", "application/json")
	patchRequest.AddCookie(cookie)
	app.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected system settings patch 403 for staff, got %d: %s", patchRecorder.Code, patchRecorder.Body.String())
	}
}

func TestInitializeMigratesLegacyShareSchema(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	dbDir := filepath.Join(baseDir, "var", "data")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}

	dsn := filepath.Join(dbDir, "classdrive.db")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	legacySchema := `
create table system_settings (
  id integer primary key check (id = 1),
  public_share_enabled integer not null default 1,
  public_share_default_hours integer not null default 72,
  upload_panel_enabled integer not null default 1
);
insert into system_settings (id, public_share_enabled, public_share_default_hours, upload_panel_enabled)
values (1, 0, 12, 0);
create table file_shares (
  id integer primary key autoincrement,
  file_entry_id integer not null
);
create table public_shares (
  id integer primary key autoincrement,
  file_entry_id integer not null
);
`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	handler, err := New(Config{
		BaseDir: baseDir,
		Seed:    false,
	})
	if err != nil {
		t.Fatalf("new app with legacy schema: %v", err)
	}
	app := handler.(*App)
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("close app: %v", err)
		}
	})

	rows, err := app.db.Query(`pragma table_info(system_settings)`)
	if err != nil {
		t.Fatalf("pragma system_settings: %v", err)
	}
	defer rows.Close()

	columns := make([]string, 0)
	for rows.Next() {
		var cid int
		var columnName string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan pragma row: %v", err)
		}
		columns = append(columns, columnName)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read pragma rows: %v", err)
	}
	if len(columns) != 5 || columns[0] != "id" || columns[1] != "upload_panel_enabled" || columns[2] != "single_account_login_enabled" || columns[3] != "server_port" || columns[4] != "default_share_expires_days" {
		t.Fatalf("unexpected migrated system_settings columns %#v", columns)
	}

	settings, err := app.loadSystemSettings()
	if err != nil {
		t.Fatalf("load system settings after migration: %v", err)
	}
	if settings.UploadPanelEnabled {
		t.Fatalf("expected upload panel setting to preserve false, got %#v", settings)
	}
	if !settings.SingleAccountLoginEnabled {
		t.Fatalf("expected migrated system settings to enable single-account login, got %#v", settings)
	}
	if settings.ServerPort != "80" {
		t.Fatalf("expected migrated system settings to use default port, got %#v", settings)
	}
	if settings.DefaultShareExpiresDays != 7 {
		t.Fatalf("expected migrated default share expiry 7, got %#v", settings)
	}

	for _, tableName := range []string{"file_shares", "public_shares"} {
		var count int
		if err := app.db.QueryRow(`select count(*) from sqlite_master where type = 'table' and name = ?`, tableName).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", tableName, err)
		}
		if count != 0 {
			t.Fatalf("expected legacy table %s to be removed, count=%d", tableName, count)
		}
	}
}

func TestEnsureColumnsAddsOnlyMissingColumns(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	if _, err := app.db.Exec(`create table schema_ensure_test (
  id integer primary key,
  existing text not null default 'old'
)`); err != nil {
		t.Fatalf("create schema ensure fixture: %v", err)
	}
	if _, err := app.db.Exec(`insert into schema_ensure_test (id, existing) values (1, 'keep')`); err != nil {
		t.Fatalf("seed schema ensure fixture: %v", err)
	}

	columns := []columnDef{
		{Name: "existing", AlterSQL: `alter table schema_ensure_test add column existing text not null default 'duplicate'`},
		{Name: "added", AlterSQL: `alter table schema_ensure_test add column added integer not null default 7`},
	}
	if err := app.ensureColumns("schema_ensure_test", columns); err != nil {
		t.Fatalf("ensure columns: %v", err)
	}
	if err := app.ensureColumns("schema_ensure_test", columns); err != nil {
		t.Fatalf("ensure columns idempotently: %v", err)
	}

	var existing string
	var added int
	if err := app.db.QueryRow(`select existing, added from schema_ensure_test where id = 1`).Scan(&existing, &added); err != nil {
		t.Fatalf("query ensured columns: %v", err)
	}
	if existing != "keep" || added != 7 {
		t.Fatalf("unexpected ensured values existing=%q added=%d", existing, added)
	}
}

func TestEnsureColumnsRejectsInvalidTableIdentifier(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	err := app.ensureColumns("schema_ensure_test;drop table teachers", []columnDef{
		{Name: "added", AlterSQL: `alter table teachers add column added integer not null default 0`},
	})
	if err == nil {
		t.Fatalf("expected invalid table identifier error")
	}
	if !strings.Contains(err.Error(), "invalid sqlite table identifier") {
		t.Fatalf("expected invalid identifier error, got %v", err)
	}
}

func TestTeacherUsersOwnerCanCreateUpdateAndDisableTeacher(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	ownerCookie := loginAndGetCookie(t, app)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/teachers", nil)
	listRequest.AddCookie(ownerCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected teachers list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var initial testTeacherUsersResponse
	decodeJSON(t, listRecorder.Body, &initial)
	if len(initial.Teachers) != 1 || initial.Teachers[0].Role != "owner" {
		t.Fatalf("unexpected initial teachers %#v", initial.Teachers)
	}

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/teachers", jsonBody(t, map[string]any{
		"username":    "math",
		"displayName": "数学老师",
		"password":    "math12345",
		"role":        "staff",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(ownerCookie)
	app.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected teacher create 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created testTeacherUserResponse
	decodeJSON(t, createRecorder.Body, &created)
	if created.Teacher.Username != "math" || created.Teacher.Role != "staff" || created.Teacher.Disabled {
		t.Fatalf("unexpected created teacher %#v", created.Teacher)
	}

	loginAndGetCookieAs(t, app, "math", "math12345")

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/teachers/"+itoa(created.Teacher.ID), nil)
	detailRequest.AddCookie(ownerCookie)
	app.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher detail 200, got %d: %s", detailRecorder.Code, detailRecorder.Body.String())
	}

	var detail testTeacherUserResponse
	decodeJSON(t, detailRecorder.Body, &detail)
	if detail.Teacher.DisplayName != "数学老师" || detail.Teacher.Role != "staff" {
		t.Fatalf("unexpected teacher detail %#v", detail.Teacher)
	}

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/teachers/"+itoa(created.Teacher.ID), jsonBody(t, map[string]any{
		"displayName": "数学组老师",
		"role":        "owner",
	}))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(ownerCookie)
	app.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher update 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	var updated testTeacherUserResponse
	decodeJSON(t, updateRecorder.Body, &updated)
	if updated.Teacher.DisplayName != "数学组老师" || updated.Teacher.Role != "owner" {
		t.Fatalf("unexpected updated teacher %#v", updated.Teacher)
	}

	disableRecorder := httptest.NewRecorder()
	disableRequest := httptest.NewRequest(http.MethodPatch, "/api/teachers/"+itoa(created.Teacher.ID), jsonBody(t, map[string]any{
		"disabled": true,
	}))
	disableRequest.Header.Set("Content-Type", "application/json")
	disableRequest.AddCookie(ownerCookie)
	app.ServeHTTP(disableRecorder, disableRequest)
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher disable 200, got %d: %s", disableRecorder.Code, disableRecorder.Body.String())
	}

	var disabled testTeacherUserResponse
	decodeJSON(t, disableRecorder.Body, &disabled)
	if !disabled.Teacher.Disabled {
		t.Fatalf("expected teacher disabled, got %#v", disabled.Teacher)
	}

	disabledLoginRecorder := httptest.NewRecorder()
	disabledLoginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]any{
		"username": "math",
		"password": "math12345",
	}))
	disabledLoginRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(disabledLoginRecorder, disabledLoginRequest)
	if disabledLoginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected disabled teacher login 401, got %d: %s", disabledLoginRecorder.Code, disabledLoginRecorder.Body.String())
	}
}

func TestTeacherUsersStaffForbidden(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	createTeacherAccount(t, app, "assistant", "助教老师", "assistant123", "staff", false)
	cookie := loginAndGetCookieAs(t, app, "assistant", "assistant123")

	for _, testCase := range []struct {
		name   string
		method string
		target string
		body   io.Reader
	}{
		{
			name:   "list",
			method: http.MethodGet,
			target: "/api/teachers",
		},
		{
			name:   "create",
			method: http.MethodPost,
			target: "/api/teachers",
			body: jsonBody(t, map[string]any{
				"username":    "newone",
				"displayName": "新老师",
				"password":    "newpass123",
				"role":        "staff",
			}),
		},
		{
			name:   "detail",
			method: http.MethodGet,
			target: "/api/teachers/1",
		},
		{
			name:   "patch",
			method: http.MethodPatch,
			target: "/api/teachers/1",
			body: jsonBody(t, map[string]any{
				"displayName": "改名",
			}),
		},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(testCase.method, testCase.target, testCase.body)
		if testCase.body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		request.AddCookie(cookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected %s to return 403 for staff, got %d: %s", testCase.name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestTeacherUsersRejectRemovingLastOwner(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	ownerCookie := loginAndGetCookie(t, app)

	for _, testCase := range []struct {
		name string
		body map[string]any
	}{
		{
			name: "demote_last_owner",
			body: map[string]any{
				"role": "staff",
			},
		},
		{
			name: "disable_last_owner",
			body: map[string]any{
				"disabled": true,
			},
		},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPatch, "/api/teachers/1", jsonBody(t, testCase.body))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(ownerCookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("expected %s to return 409, got %d: %s", testCase.name, recorder.Code, recorder.Body.String())
		}
	}

	teacher, err := app.findTeacherByID(1)
	if err != nil {
		t.Fatalf("find teacher by id: %v", err)
	}
	if teacher == nil || teacher.Role != "owner" || teacher.Disabled {
		t.Fatalf("expected seeded owner to remain active owner, got %#v", teacher)
	}
}

func TestTeacherUsersAllowOwnerChangeWhenAnotherActiveOwnerExists(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	ownerCookie := loginAndGetCookie(t, app)
	secondOwnerID := createTeacherAccount(t, app, "owner2", "第二负责人", "owner234", "owner", false)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/teachers/1", jsonBody(t, map[string]any{
		"role": "staff",
	}))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(ownerCookie)
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected demote owner with backup owner to return 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	firstOwner, err := app.findTeacherByID(1)
	if err != nil {
		t.Fatalf("find first owner: %v", err)
	}
	if firstOwner == nil || firstOwner.Role != "staff" {
		t.Fatalf("expected first owner to be demoted, got %#v", firstOwner)
	}

	secondOwner, err := app.findTeacherByID(secondOwnerID)
	if err != nil {
		t.Fatalf("find second owner: %v", err)
	}
	if secondOwner == nil || secondOwner.Role != "owner" || secondOwner.Disabled {
		t.Fatalf("expected second owner to remain active owner, got %#v", secondOwner)
	}
}

func TestTeacherUsersAllowDisableOwnerWhenAnotherActiveOwnerExists(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	ownerCookie := loginAndGetCookie(t, app)
	secondOwnerID := createTeacherAccount(t, app, "owner3", "第三负责人", "owner345", "owner", false)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/api/teachers/1", jsonBody(t, map[string]any{
		"disabled": true,
	}))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(ownerCookie)
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected disable owner with backup owner to return 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	firstOwner, err := app.findTeacherByID(1)
	if err != nil {
		t.Fatalf("find first owner: %v", err)
	}
	if firstOwner == nil || !firstOwner.Disabled {
		t.Fatalf("expected first owner to be disabled, got %#v", firstOwner)
	}

	secondOwner, err := app.findTeacherByID(secondOwnerID)
	if err != nil {
		t.Fatalf("find second owner: %v", err)
	}
	if secondOwner == nil || secondOwner.Role != "owner" || secondOwner.Disabled {
		t.Fatalf("expected backup owner to remain active owner, got %#v", secondOwner)
	}
}

func TestSeededTeacherPasswordUsesStrongHash(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)

	teacher, err := app.findTeacherByUsername("admin")
	if err != nil {
		t.Fatalf("find teacher: %v", err)
	}
	if teacher == nil {
		t.Fatalf("expected seeded teacher")
	}
	if teacher.PasswordHash == "demo123" {
		t.Fatalf("expected stored password to be hashed")
	}
	if !strings.HasPrefix(teacher.PasswordHash, "$2") {
		t.Fatalf("expected bcrypt hash prefix, got %q", teacher.PasswordHash)
	}
	if err := verifyPasswordHash(teacher.PasswordHash, "demo123"); err != nil {
		t.Fatalf("expected bcrypt hash to verify: %v", err)
	}
	if err := verifyPasswordHash(teacher.PasswordHash, "wrong-password"); err == nil {
		t.Fatalf("expected wrong password verification to fail")
	}
}

func TestSeededTeacherPasswordCanBeOverriddenByEnvironment(t *testing.T) {
	t.Setenv("CLASSDRIVE_DEFAULT_TEACHER_PASSWORD", "admin987")

	_, app := newTestServer(t)

	teacher, err := app.findTeacherByUsername("admin")
	if err != nil {
		t.Fatalf("find teacher: %v", err)
	}
	if teacher == nil {
		t.Fatalf("expected seeded teacher")
	}
	if err := verifyPasswordHash(teacher.PasswordHash, "admin987"); err != nil {
		t.Fatalf("expected override password to verify: %v", err)
	}
	if err := verifyPasswordHash(teacher.PasswordHash, "demo123"); err == nil {
		t.Fatalf("expected built-in default password to fail after override")
	}
}

func TestShellClassesAndUnauthorizedErrors(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)

	unauthorizedRecorder := httptest.NewRecorder()
	unauthorizedRequest := httptest.NewRequest(http.MethodGet, "/api/classes", nil)
	handler.ServeHTTP(unauthorizedRecorder, unauthorizedRequest)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized classes status 401, got %d", unauthorizedRecorder.Code)
	}

	var errorPayload apiErrorResponse
	decodeJSON(t, unauthorizedRecorder.Body, &errorPayload)
	if errorPayload.Error.Code != "unauthorized" {
		t.Fatalf("expected unauthorized error code, got %#v", errorPayload.Error)
	}

	cookie := loginAndGetCookie(t, handler)

	shellRecorder := httptest.NewRecorder()
	shellRequest := httptest.NewRequest(http.MethodGet, "/api/shell", nil)
	shellRequest.AddCookie(cookie)
	handler.ServeHTTP(shellRecorder, shellRequest)
	if shellRecorder.Code != http.StatusOK {
		t.Fatalf("expected shell status 200, got %d: %s", shellRecorder.Code, shellRecorder.Body.String())
	}

	var shellPayload shellResponse
	decodeJSON(t, shellRecorder.Body, &shellPayload)
	if len(shellPayload.Items) < 7 {
		t.Fatalf("expected at least 7 nav items, got %d", len(shellPayload.Items))
	}
	for _, item := range shellPayload.Items {
		if (item.Key == "classes" || item.Key == "settings") && item.Placeholder {
			t.Fatalf("expected %s nav item to be a completed module, got placeholder", item.Key)
		}
	}

	classesRecorder := httptest.NewRecorder()
	classesRequest := httptest.NewRequest(http.MethodGet, "/api/classes", nil)
	classesRequest.AddCookie(cookie)
	handler.ServeHTTP(classesRecorder, classesRequest)
	if classesRecorder.Code != http.StatusOK {
		t.Fatalf("expected classes status 200, got %d: %s", classesRecorder.Code, classesRecorder.Body.String())
	}

	var classesPayload classesResponse
	decodeJSON(t, classesRecorder.Body, &classesPayload)
	if len(classesPayload.Classes) == 0 {
		t.Fatalf("expected seeded classes")
	}
}

func TestCreateClassAndGenerateRegistrationCode(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/classes", jsonBody(t, map[string]any{
		"name": "三年级一班",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create class 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created classSummary
	decodeJSON(t, createRecorder.Body, &created)
	if created.Name != "三年级一班" {
		t.Fatalf("unexpected created class: %#v", created)
	}
	if created.JoinCodeHint != "" {
		t.Fatalf("expected join code hint empty before generation, got %#v", created)
	}

	generateRecorder := httptest.NewRecorder()
	generateRequest := httptest.NewRequest(http.MethodPost, "/api/classes/"+itoa(created.ID)+"/join-code", nil)
	generateRequest.AddCookie(cookie)
	handler.ServeHTTP(generateRecorder, generateRequest)
	if generateRecorder.Code != http.StatusOK {
		t.Fatalf("expected generate code 200, got %d: %s", generateRecorder.Code, generateRecorder.Body.String())
	}

	var generated classJoinCodeResult
	decodeJSON(t, generateRecorder.Body, &generated)
	if generated.JoinCode == "" || generated.JoinCodeHint == "" {
		t.Fatalf("expected generated registration code, got %#v", generated)
	}

	classesRecorder := httptest.NewRecorder()
	classesRequest := httptest.NewRequest(http.MethodGet, "/api/classes", nil)
	classesRequest.AddCookie(cookie)
	handler.ServeHTTP(classesRecorder, classesRequest)
	if classesRecorder.Code != http.StatusOK {
		t.Fatalf("expected classes 200, got %d: %s", classesRecorder.Code, classesRecorder.Body.String())
	}

	var payload classesResponse
	decodeJSON(t, classesRecorder.Body, &payload)
	found := false
	for _, item := range payload.Classes {
		if item.ID == created.ID {
			found = true
			if item.JoinCodeStatus != "active" || item.JoinCodeHint == "" {
				t.Fatalf("expected class list to include registration code")
			}
		}
	}
	if !found {
		t.Fatalf("expected created class in list")
	}
}

func TestClassesListSupportsSearchSortAndPagination(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	for _, name := range []string{"分页班级-B", "分页班级-A", "无关班级"} {
		if _, err := app.createClass(name); err != nil {
			t.Fatalf("create class %s: %v", name, err)
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/classes?q=分页班级&page=2&pageSize=1&sort=name-asc", nil)
	request.AddCookie(cookie)
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected classes list status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Classes    []classSummary `json:"classes"`
		Pagination struct {
			Page       int `json:"page"`
			PageSize   int `json:"pageSize"`
			Total      int `json:"total"`
			TotalPages int `json:"totalPages"`
		} `json:"pagination"`
	}
	decodeJSON(t, recorder.Body, &payload)

	if payload.Pagination.Page != 2 || payload.Pagination.PageSize != 1 {
		t.Fatalf("unexpected classes pagination %#v", payload.Pagination)
	}
	if payload.Pagination.Total != 2 || payload.Pagination.TotalPages != 2 {
		t.Fatalf("expected classes total 2/2, got %#v", payload.Pagination)
	}
	if len(payload.Classes) != 1 || payload.Classes[0].Name != "分页班级-B" {
		t.Fatalf("unexpected classes page %#v", payload.Classes)
	}
}

func TestStudentsListSupportsSearchSortAndPagination(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	fixtures := []createStudentRequest{
		{ClassID: 1, StudentNo: "20260003", DisplayName: "筛选学生-C"},
		{ClassID: 1, StudentNo: "20260001", DisplayName: "无关学生"},
		{ClassID: 1, StudentNo: "20260002", DisplayName: "筛选学生-A"},
	}
	for _, payload := range fixtures {
		if _, err := app.createStudent(payload); err != nil {
			t.Fatalf("create student %s: %v", payload.StudentNo, err)
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/students?classId=1&q=筛选&page=2&pageSize=1&sort=displayName-asc", nil)
	request.AddCookie(cookie)
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected students list status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Students   []studentSummary `json:"students"`
		Pagination struct {
			Page       int `json:"page"`
			PageSize   int `json:"pageSize"`
			Total      int `json:"total"`
			TotalPages int `json:"totalPages"`
		} `json:"pagination"`
	}
	decodeJSON(t, recorder.Body, &payload)

	if payload.Pagination.Page != 2 || payload.Pagination.PageSize != 1 {
		t.Fatalf("unexpected students pagination %#v", payload.Pagination)
	}
	if payload.Pagination.Total != 2 || payload.Pagination.TotalPages != 2 {
		t.Fatalf("expected students total 2/2, got %#v", payload.Pagination)
	}
	if len(payload.Students) != 1 || payload.Students[0].DisplayName != "筛选学生-C" {
		t.Fatalf("unexpected students page %#v", payload.Students)
	}
}

func TestStudentsListIncludesRegistrationStateAndSortsByIt(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	registered, err := app.createStudent(createStudentRequest{ClassID: 1, StudentNo: "20260001", DisplayName: "已注册学生"})
	if err != nil {
		t.Fatalf("create registered student: %v", err)
	}
	unregistered, err := app.createStudent(createStudentRequest{ClassID: 1, StudentNo: "20260002", DisplayName: "未注册学生"})
	if err != nil {
		t.Fatalf("create unregistered student: %v", err)
	}
	activatedAt := "2026-04-30T10:00:00Z"
	if _, err := app.db.Exec(`update students set activated_at = ? where id = ?`, activatedAt, registered.ID); err != nil {
		t.Fatalf("activate student fixture: %v", err)
	}

	descRecorder := httptest.NewRecorder()
	descRequest := httptest.NewRequest(http.MethodGet, "/api/students?classId=1&sort=registered-desc", nil)
	descRequest.AddCookie(cookie)
	app.ServeHTTP(descRecorder, descRequest)
	if descRecorder.Code != http.StatusOK {
		t.Fatalf("expected registered-desc status 200, got %d: %s", descRecorder.Code, descRecorder.Body.String())
	}

	var descPayload studentsResponse
	decodeJSON(t, descRecorder.Body, &descPayload)
	if len(descPayload.Students) != 2 {
		t.Fatalf("expected two students, got %#v", descPayload.Students)
	}
	if descPayload.Students[0].ID != registered.ID || descPayload.Students[0].ActivatedAt != activatedAt {
		t.Fatalf("expected registered student first with activatedAt, got %#v", descPayload.Students)
	}
	if descPayload.Students[1].ID != unregistered.ID || descPayload.Students[1].ActivatedAt != "" {
		t.Fatalf("expected unregistered student second without activatedAt, got %#v", descPayload.Students)
	}

	ascRecorder := httptest.NewRecorder()
	ascRequest := httptest.NewRequest(http.MethodGet, "/api/students?classId=1&sort=registered-asc", nil)
	ascRequest.AddCookie(cookie)
	app.ServeHTTP(ascRecorder, ascRequest)
	if ascRecorder.Code != http.StatusOK {
		t.Fatalf("expected registered-asc status 200, got %d: %s", ascRecorder.Code, ascRecorder.Body.String())
	}

	var ascPayload studentsResponse
	decodeJSON(t, ascRecorder.Body, &ascPayload)
	if len(ascPayload.Students) != 2 || ascPayload.Students[0].ID != unregistered.ID || ascPayload.Students[1].ID != registered.ID {
		t.Fatalf("unexpected registered-asc order %#v", ascPayload.Students)
	}
}

func TestAssignmentsListSupportsSearchSortAndPagination(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	fixtures := []createAssignmentRequest{
		{
			ClassID:     1,
			Title:       "分页作业-B",
			Description: "筛选命中的第二条作业",
			DueAt:       "2030-05-03T12:00:00.000Z",
			Status:      "published",
		},
		{
			ClassID:     1,
			Title:       "无关作业",
			Description: "不会命中的作业",
			DueAt:       "2030-05-01T12:00:00.000Z",
			Status:      "draft",
		},
		{
			ClassID:     1,
			Title:       "分页作业-A",
			Description: "筛选命中的第一条作业",
			DueAt:       "2030-05-02T12:00:00.000Z",
			Status:      "published",
		},
	}
	for _, payload := range fixtures {
		if _, err := app.createAssignment(payload); err != nil {
			t.Fatalf("create assignment %s: %v", payload.Title, err)
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/assignments?classId=1&q=分页作业&page=2&pageSize=1&sort=dueAt-asc", nil)
	request.AddCookie(cookie)
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected assignments list status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Assignments []assignmentPayload `json:"assignments"`
		Pagination  struct {
			Page       int `json:"page"`
			PageSize   int `json:"pageSize"`
			Total      int `json:"total"`
			TotalPages int `json:"totalPages"`
		} `json:"pagination"`
	}
	decodeJSON(t, recorder.Body, &payload)

	if payload.Pagination.Page != 2 || payload.Pagination.PageSize != 1 {
		t.Fatalf("unexpected assignments pagination %#v", payload.Pagination)
	}
	if payload.Pagination.Total != 2 || payload.Pagination.TotalPages != 2 {
		t.Fatalf("expected assignments total 2/2, got %#v", payload.Pagination)
	}
	if len(payload.Assignments) != 1 || payload.Assignments[0].Title != "分页作业-B" {
		t.Fatalf("unexpected assignments page %#v", payload.Assignments)
	}
}

func TestClassManagementCreateAndJoinCodeFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/classes", jsonBody(t, map[string]string{
		"name": "测试班级A",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create class 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created classSummary
	decodeJSON(t, createRecorder.Body, &created)
	if created.ID <= 0 {
		t.Fatalf("expected created class id, got %#v", created)
	}
	if created.Name != "测试班级A" {
		t.Fatalf("expected class name 测试班级A, got %#v", created)
	}
	if created.JoinCodeStatus != "inactive" {
		t.Fatalf("expected inactive join code status, got %#v", created)
	}
	if created.JoinCodeHint != "" {
		t.Fatalf("expected empty join code hint, got %#v", created)
	}

	classesBeforeCode := listClasses(t, handler, cookie)
	createdBeforeCode := findClassByID(t, classesBeforeCode.Classes, created.ID)
	if createdBeforeCode.JoinCodeStatus != "inactive" {
		t.Fatalf("expected list status inactive before code generation, got %#v", createdBeforeCode)
	}
	if createdBeforeCode.JoinCodeHint != "" {
		t.Fatalf("expected list hint empty before code generation, got %#v", createdBeforeCode)
	}

	generateRecorder := httptest.NewRecorder()
	generateRequest := httptest.NewRequest(http.MethodPost, "/api/classes/"+itoa(created.ID)+"/join-code", nil)
	generateRequest.AddCookie(cookie)
	handler.ServeHTTP(generateRecorder, generateRequest)
	if generateRecorder.Code != http.StatusOK {
		t.Fatalf("expected generate join code 200, got %d: %s", generateRecorder.Code, generateRecorder.Body.String())
	}

	var generated classJoinCodeResponse
	decodeJSON(t, generateRecorder.Body, &generated)
	if generated.ClassID != created.ID {
		t.Fatalf("expected generated class id %d, got %#v", created.ID, generated)
	}
	if len(generated.JoinCode) != 4 {
		t.Fatalf("expected generated join code length = 4, got %#v", generated)
	}
	if generated.JoinCodeHint == "" {
		t.Fatalf("expected generated join code hint, got %#v", generated)
	}
	if generated.RegistrationExpiresAt == "" {
		t.Fatalf("expected generated join code to include registration expiration, got %#v", generated)
	}
	expiresAt, err := time.Parse(time.RFC3339, generated.RegistrationExpiresAt)
	if err != nil {
		t.Fatalf("expected registration expiration to be RFC3339, got %#v: %v", generated, err)
	}
	if expiresAt.Before(time.Now().UTC().Add(89*time.Minute)) || expiresAt.After(time.Now().UTC().Add(91*time.Minute)) {
		t.Fatalf("expected registration expiration near 90 minutes, got %#v", generated)
	}

	closeRecorder := httptest.NewRecorder()
	closeRequest := httptest.NewRequest(http.MethodPatch, "/api/classes/"+itoa(created.ID)+"/join-code", jsonBody(t, map[string]bool{
		"enabled": false,
	}))
	closeRequest.Header.Set("Content-Type", "application/json")
	closeRequest.AddCookie(cookie)
	handler.ServeHTTP(closeRecorder, closeRequest)
	if closeRecorder.Code != http.StatusOK {
		t.Fatalf("expected close registration 200, got %d: %s", closeRecorder.Code, closeRecorder.Body.String())
	}

	classesAfterCode := listClasses(t, handler, cookie)
	createdAfterCode := findClassByID(t, classesAfterCode.Classes, created.ID)
	if createdAfterCode.JoinCodeStatus != "inactive" {
		t.Fatalf("expected list status inactive after closing registration, got %#v", createdAfterCode)
	}
	if createdAfterCode.JoinCodeHint != "" || createdAfterCode.JoinCode != "" {
		t.Fatalf("expected join code cleared after closing registration, got %#v", createdAfterCode)
	}
	if createdAfterCode.RegistrationExpiresAt != "" {
		t.Fatalf("expected expiration cleared after closing registration, got %#v", createdAfterCode)
	}
}

func TestGenerateJoinCodeUsesCryptoRandIntForEachDigit(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	body := string(source)
	functionStart := strings.Index(body, "func generateJoinCode()")
	if functionStart < 0 {
		t.Fatalf("generateJoinCode function not found")
	}
	functionEnd := strings.Index(body[functionStart:], "\nfunc hashPassword")
	if functionEnd < 0 {
		t.Fatalf("generateJoinCode function end not found")
	}
	functionBody := body[functionStart : functionStart+functionEnd]
	if !strings.Contains(functionBody, "rand.Int(") {
		t.Fatalf("generateJoinCode should use crypto/rand.Int to avoid modulo bias")
	}
	if strings.Contains(functionBody, "%len(alphabet)") || strings.Contains(functionBody, "% len(alphabet)") {
		t.Fatalf("generateJoinCode should not reduce random bytes with modulo")
	}
}

func TestGenerateJoinCodeReturnsFourDigits(t *testing.T) {
	t.Parallel()

	code, err := generateJoinCode()
	if err != nil {
		t.Fatalf("generate join code: %v", err)
	}
	if len(code) != classJoinCodeLen {
		t.Fatalf("expected join code length %d, got %q", classJoinCodeLen, code)
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			t.Fatalf("expected numeric join code, got %q", code)
		}
	}
}

func TestExpiredClassRegistrationClosesInTeacherList(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	if _, err := app.db.Exec(
		`update classes set join_code = ?, registration_enabled = 1, registration_expires_at = ? where id = 1`,
		"EX90",
		time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
	); err != nil {
		t.Fatalf("expire registration: %v", err)
	}

	classesAfterExpiry := listClasses(t, app, cookie)
	expiredClass := findClassByID(t, classesAfterExpiry.Classes, 1)
	if expiredClass.JoinCodeStatus != "inactive" || expiredClass.RegistrationEnabled || expiredClass.JoinCode != "" || expiredClass.RegistrationExpiresAt != "" {
		t.Fatalf("expected expired registration to close in teacher list, got %#v", expiredClass)
	}
}

func TestStudentActivateRejectsExpiredClassRegistration(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")

	if _, err := app.db.Exec(
		`update classes set registration_expires_at = ? where id = 1`,
		time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
	); err != nil {
		t.Fatalf("expire registration: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/student/auth/activate", jsonBody(t, map[string]any{
		"joinCode":  "ABCD1234",
		"studentNo": "20260001",
		"password":  "student123",
	}))
	request.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected expired registration 409, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload apiErrorResponse
	decodeJSON(t, recorder.Body, &payload)
	if payload.Error.Code != "registration_closed" {
		t.Fatalf("expected registration_closed error, got %#v", payload)
	}

	var joinCode string
	var registrationEnabled bool
	var registrationExpiresAt string
	if err := app.db.QueryRow(`select join_code, registration_enabled, registration_expires_at from classes where id = 1`).Scan(&joinCode, &registrationEnabled, &registrationExpiresAt); err != nil {
		t.Fatalf("query expired registration: %v", err)
	}
	if joinCode != "" || registrationEnabled || registrationExpiresAt != "" {
		t.Fatalf("expected expired registration to be cleared, got joinCode=%q enabled=%v expiresAt=%q", joinCode, registrationEnabled, registrationExpiresAt)
	}
}

func TestUpdateAndDeleteClassFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/classes", jsonBody(t, map[string]string{
		"name": "待修改班级",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create class 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created classSummary
	decodeJSON(t, createRecorder.Body, &created)

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/classes/"+itoa(created.ID), jsonBody(t, map[string]string{
		"name": "已修改班级",
	}))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(cookie)
	handler.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update class 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	var updated classSummary
	decodeJSON(t, updateRecorder.Body, &updated)
	if updated.Name != "已修改班级" {
		t.Fatalf("expected updated class name, got %#v", updated)
	}

	classesAfterUpdate := listClasses(t, handler, cookie)
	updatedClass := findClassByID(t, classesAfterUpdate.Classes, created.ID)
	if updatedClass.Name != "已修改班级" {
		t.Fatalf("expected list class name updated, got %#v", updatedClass)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/classes/"+itoa(created.ID), nil)
	deleteRequest.AddCookie(cookie)
	handler.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete class 200, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	classesAfterDelete := listClasses(t, handler, cookie)
	for _, item := range classesAfterDelete.Classes {
		if item.ID == created.ID {
			t.Fatalf("expected deleted class removed from list, got %#v", item)
		}
	}
}

func TestDeleteClassCascadesStudentsAssignmentsFilesAndSubmissions(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	createdClass, err := app.createClass("级联删除班级")
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	student, err := app.createStudent(createStudentRequest{
		ClassID:     createdClass.ID,
		StudentNo:   "20990001",
		DisplayName: "级联学生",
	})
	if err != nil {
		t.Fatalf("create student: %v", err)
	}
	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:        createdClass.ID,
		Title:          "级联作业",
		Description:    "用于删除级联测试",
		DueAt:          "",
		Status:         "published",
		SubmissionMode: "any",
		MinFileCount:   1,
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := app.db.Exec(`
insert into assignment_submissions (assignment_id, student_id, status, submitted_at, updated_at, review_status, teacher_comment)
values (?, ?, 'submitted', ?, ?, 'pending', '')`,
		assignment.ID,
		student.ID,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("insert assignment submission: %v", err)
	}
	submissionID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read submission id: %v", err)
	}

	if _, err := app.createSeedFile("class", optionalInt64(createdClass.ID), "/", "班级资料.txt", []byte("class file")); err != nil {
		t.Fatalf("create class file: %v", err)
	}
	if _, err := app.createSeedFile("assignment", optionalInt64(createdClass.ID), "/"+itoa(assignment.ID), "作业附件.txt", []byte("assignment file")); err != nil {
		t.Fatalf("create assignment file: %v", err)
	}
	if _, err := app.createSeedFile("submission", optionalInt64(createdClass.ID), "/"+itoa(submissionID), "学生提交.txt", []byte("submission file")); err != nil {
		t.Fatalf("create submission file: %v", err)
	}

	storageDirs := []string{
		filepath.Join(app.storageRoot, "classes", itoa(createdClass.ID)),
		filepath.Join(app.storageRoot, "assignments", itoa(createdClass.ID)),
		filepath.Join(app.storageRoot, "submissions", itoa(createdClass.ID)),
	}
	for _, dir := range storageDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create storage directory %s: %v", dir, err)
		}
	}
	for _, dir := range storageDirs {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("expected directory before class delete %s: %v", dir, err)
		}
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/classes/"+itoa(createdClass.ID), nil)
	deleteRequest.AddCookie(cookie)
	app.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete class 200, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	if countRows(t, app.db, `select count(*) from classes where id = ?`, createdClass.ID) != 0 {
		t.Fatalf("expected class removed")
	}
	if countRows(t, app.db, `select count(*) from students where class_id = ?`, createdClass.ID) != 0 {
		t.Fatalf("expected class students removed")
	}
	if countRows(t, app.db, `select count(*) from assignments where class_id = ?`, createdClass.ID) != 0 {
		t.Fatalf("expected class assignments removed")
	}
	if countRows(t, app.db, `select count(*) from assignment_submissions where assignment_id = ?`, assignment.ID) != 0 {
		t.Fatalf("expected assignment submissions removed")
	}
	if countRows(t, app.db, `select count(*) from file_entries where class_id = ? and space in ('class', 'assignment', 'submission')`, createdClass.ID) != 0 {
		t.Fatalf("expected class-related file entries removed")
	}

	for _, dir := range storageDirs {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected directory removed after class delete %s, got %v", dir, err)
		}
	}
}

func TestRecentCopyTargetsPreferenceFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	initialRecorder := httptest.NewRecorder()
	initialRequest := httptest.NewRequest(http.MethodGet, "/api/preferences/recent-copy-targets", nil)
	initialRequest.AddCookie(cookie)
	handler.ServeHTTP(initialRecorder, initialRequest)
	if initialRecorder.Code != http.StatusOK {
		t.Fatalf("expected initial preference 200, got %d: %s", initialRecorder.Code, initialRecorder.Body.String())
	}

	var initial recentCopyTargetsResponse
	decodeJSON(t, initialRecorder.Body, &initial)
	if len(initial.Items) != 0 {
		t.Fatalf("expected empty recent targets, got %#v", initial)
	}

	classID := int64(1)
	saveRecorder := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(http.MethodPut, "/api/preferences/recent-copy-targets", jsonBody(t, map[string]any{
		"items": []map[string]any{
			{
				"space":   "class",
				"classId": classID,
				"path":    "/课件归档",
				"label":   "课件归档",
				"pinned":  true,
			},
			{
				"space":  "public",
				"path":   "/最近目录",
				"label":  "最近目录",
				"pinned": false,
			},
		},
	}))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveRequest.AddCookie(cookie)
	handler.ServeHTTP(saveRecorder, saveRequest)
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("expected save preference 200, got %d: %s", saveRecorder.Code, saveRecorder.Body.String())
	}

	reloadRecorder := httptest.NewRecorder()
	reloadRequest := httptest.NewRequest(http.MethodGet, "/api/preferences/recent-copy-targets", nil)
	reloadRequest.AddCookie(cookie)
	handler.ServeHTTP(reloadRecorder, reloadRequest)
	if reloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected reload preference 200, got %d: %s", reloadRecorder.Code, reloadRecorder.Body.String())
	}

	var reloaded recentCopyTargetsResponse
	decodeJSON(t, reloadRecorder.Body, &reloaded)
	if len(reloaded.Items) != 2 {
		t.Fatalf("expected 2 recent targets, got %#v", reloaded)
	}
	if reloaded.Items[0].Path != "/课件归档" || reloaded.Items[0].Label != "课件归档" || !reloaded.Items[0].Pinned {
		t.Fatalf("unexpected recent target payload %#v", reloaded.Items[0])
	}
	if reloaded.Items[1].Path != "/最近目录" || reloaded.Items[1].Label != "最近目录" || reloaded.Items[1].Pinned {
		t.Fatalf("unexpected recent target payload %#v", reloaded.Items[1])
	}
}

func TestStudentsRosterListAndCreateFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/students", jsonBody(t, map[string]any{
		"classId":     1,
		"studentNo":   "20260001",
		"displayName": "张小明",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create student 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created studentSummary
	decodeJSON(t, createRecorder.Body, &created)
	if created.StudentNo != "20260001" || created.DisplayName != "张小明" {
		t.Fatalf("unexpected created student %#v", created)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/students?classId=1", nil)
	listRequest.AddCookie(cookie)
	handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list students 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var payload studentsResponse
	decodeJSON(t, listRecorder.Body, &payload)
	if len(payload.Students) == 0 {
		t.Fatalf("expected non-empty student list")
	}
	if payload.Students[0].DisplayName != "张小明" {
		t.Fatalf("unexpected student list payload %#v", payload.Students)
	}
}

func TestStudentNoIsUniqueAcrossAllClasses(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	otherClass, err := app.createClass("跨班重复学号测试班")
	if err != nil {
		t.Fatalf("create other class: %v", err)
	}

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/students", jsonBody(t, map[string]interface{}{
		"classId":     1,
		"studentNo":   "20260001",
		"displayName": "一班学生",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	app.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create student 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	duplicateCreateRecorder := httptest.NewRecorder()
	duplicateCreateRequest := httptest.NewRequest(http.MethodPost, "/api/students", jsonBody(t, map[string]interface{}{
		"classId":     otherClass.ID,
		"studentNo":   "20260001",
		"displayName": "二班学生",
	}))
	duplicateCreateRequest.Header.Set("Content-Type", "application/json")
	duplicateCreateRequest.AddCookie(cookie)
	app.ServeHTTP(duplicateCreateRecorder, duplicateCreateRequest)
	if duplicateCreateRecorder.Code != http.StatusConflict {
		t.Fatalf("expected duplicate create 409, got %d: %s", duplicateCreateRecorder.Code, duplicateCreateRecorder.Body.String())
	}
	var duplicateCreatePayload apiErrorResponse
	decodeJSON(t, duplicateCreateRecorder.Body, &duplicateCreatePayload)
	if duplicateCreatePayload.Error.Message != "学号已存在" {
		t.Fatalf("unexpected duplicate create message %#v", duplicateCreatePayload.Error)
	}
	if _, err := app.db.Exec(`insert into students (class_id, student_no, display_name) values (?, ?, ?)`, otherClass.ID, "20260001", "绕过接口学生"); err == nil {
		t.Fatalf("expected database unique index to reject duplicate student number across classes")
	}

	secondStudent, err := app.createStudent(createStudentRequest{
		ClassID:     otherClass.ID,
		StudentNo:   "20260002",
		DisplayName: "二班学生",
	})
	if err != nil {
		t.Fatalf("create second student: %v", err)
	}
	duplicateUpdateRecorder := httptest.NewRecorder()
	duplicateUpdateRequest := httptest.NewRequest(http.MethodPatch, "/api/students/"+itoa(secondStudent.ID), jsonBody(t, map[string]interface{}{
		"studentNo":   "20260001",
		"displayName": "二班学生",
	}))
	duplicateUpdateRequest.Header.Set("Content-Type", "application/json")
	duplicateUpdateRequest.AddCookie(cookie)
	app.ServeHTTP(duplicateUpdateRecorder, duplicateUpdateRequest)
	if duplicateUpdateRecorder.Code != http.StatusConflict {
		t.Fatalf("expected duplicate update 409, got %d: %s", duplicateUpdateRecorder.Code, duplicateUpdateRecorder.Body.String())
	}

	body, contentType := multipartFileBody(t, map[string]string{
		"classId": itoa(otherClass.ID),
	}, "file", "students.xlsx", buildTestStudentImportWorkbook(t, [][]string{
		{"studentNo", "displayName"},
		{"20260001", "导入重复学生"},
	}))
	duplicateImportRecorder := httptest.NewRecorder()
	duplicateImportRequest := httptest.NewRequest(http.MethodPost, "/api/students/import-file", body)
	duplicateImportRequest.Header.Set("Content-Type", contentType)
	duplicateImportRequest.AddCookie(cookie)
	app.ServeHTTP(duplicateImportRecorder, duplicateImportRequest)
	if duplicateImportRecorder.Code != http.StatusConflict {
		t.Fatalf("expected duplicate import 409, got %d: %s", duplicateImportRecorder.Code, duplicateImportRecorder.Body.String())
	}
}

func TestStudentsRosterUpdateDeleteAndImportFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/students", jsonBody(t, map[string]any{
		"classId":     1,
		"studentNo":   "20260002",
		"displayName": "李小红",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create student 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created studentSummary
	decodeJSON(t, createRecorder.Body, &created)

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/students/"+itoa(created.ID), jsonBody(t, map[string]any{
		"studentNo":   "20260020",
		"displayName": "李小红-更新",
	}))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(cookie)
	handler.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update student 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	var updated studentSummary
	decodeJSON(t, updateRecorder.Body, &updated)
	if updated.StudentNo != "20260020" || updated.DisplayName != "李小红-更新" {
		t.Fatalf("unexpected updated student %#v", updated)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/students?classId=1", nil)
	listRequest.AddCookie(cookie)
	handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list students 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var payload studentsResponse
	decodeJSON(t, listRecorder.Body, &payload)
	if len(payload.Students) != 1 {
		t.Fatalf("expected 1 student after create and update, got %#v", payload.Students)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/students/"+itoa(created.ID), nil)
	deleteRequest.AddCookie(cookie)
	handler.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete student 200, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	reloadRecorder := httptest.NewRecorder()
	reloadRequest := httptest.NewRequest(http.MethodGet, "/api/students?classId=1", nil)
	reloadRequest.AddCookie(cookie)
	handler.ServeHTTP(reloadRecorder, reloadRequest)
	if reloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected reload students 200, got %d: %s", reloadRecorder.Code, reloadRecorder.Body.String())
	}

	var reloaded studentsResponse
	decodeJSON(t, reloadRecorder.Body, &reloaded)
	if len(reloaded.Students) != 0 {
		t.Fatalf("expected 0 students after delete, got %#v", reloaded.Students)
	}
}

func TestStudentsListFiltersByRegistrationState(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)
	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	if _, err := app.createStudent(createStudentRequest{
		ClassID:     1,
		StudentNo:   "20260002",
		DisplayName: "李小红",
	}); err != nil {
		t.Fatalf("create unregistered student: %v", err)
	}
	activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	registeredRecorder := httptest.NewRecorder()
	registeredRequest := httptest.NewRequest(http.MethodGet, "/api/students?classId=1&registration=registered", nil)
	registeredRequest.AddCookie(teacherCookie)
	app.ServeHTTP(registeredRecorder, registeredRequest)
	if registeredRecorder.Code != http.StatusOK {
		t.Fatalf("expected registered students 200, got %d: %s", registeredRecorder.Code, registeredRecorder.Body.String())
	}
	var registered studentsResponse
	decodeJSON(t, registeredRecorder.Body, &registered)
	if len(registered.Students) != 1 || registered.Students[0].StudentNo != "20260001" {
		t.Fatalf("unexpected registered students %#v", registered.Students)
	}

	unregisteredRecorder := httptest.NewRecorder()
	unregisteredRequest := httptest.NewRequest(http.MethodGet, "/api/students?classId=1&registration=unregistered", nil)
	unregisteredRequest.AddCookie(teacherCookie)
	app.ServeHTTP(unregisteredRecorder, unregisteredRequest)
	if unregisteredRecorder.Code != http.StatusOK {
		t.Fatalf("expected unregistered students 200, got %d: %s", unregisteredRecorder.Code, unregisteredRecorder.Body.String())
	}
	var unregistered studentsResponse
	decodeJSON(t, unregisteredRecorder.Body, &unregistered)
	if len(unregistered.Students) != 1 || unregistered.Students[0].StudentNo != "20260002" {
		t.Fatalf("unexpected unregistered students %#v", unregistered.Students)
	}
}

func TestStudentsJSONImportIsRejected(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/students/import", jsonBody(t, map[string]any{
		"classId": 1,
		"students": []map[string]any{
			{"studentNo": "20260021", "displayName": "王小刚"},
			{"studentNo": "20260022", "displayName": "赵小雪"},
		},
	}))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected json import rejected with 422, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload apiErrorResponse
	decodeJSON(t, recorder.Body, &payload)
	if payload.Error.Message != "仅支持通过 XLSX 文件导入学生" {
		t.Fatalf("unexpected json import error %#v", payload.Error)
	}
}

func TestStudentsImportFileRejectsCSV(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	body, contentType := multipartFileBody(t, map[string]string{
		"classId": "1",
	}, "file", "students.csv", []byte("studentNo,displayName\n20260031,陈小雨\n20260032,孙小北\n"))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/students/import-file", body)
	request.Header.Set("Content-Type", contentType)
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected csv import rejected with 422, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload apiErrorResponse
	decodeJSON(t, recorder.Body, &payload)
	if payload.Error.Message != "仅支持导入 XLSX 文件" {
		t.Fatalf("unexpected csv import error %#v", payload.Error)
	}
}

func TestStudentsImportFileFromXLSX(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	body, contentType := multipartFileBody(t, map[string]string{
		"classId": "1",
	}, "file", "students.xlsx", buildTestStudentImportWorkbook(t, [][]string{
		{"studentNo", "displayName"},
		{"20260041", "林小满"},
		{"20260042", "周小禾"},
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/students/import-file", body)
	request.Header.Set("Content-Type", contentType)
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected xlsx import 201, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var imported studentsResponse
	decodeJSON(t, recorder.Body, &imported)
	if len(imported.Students) != 2 {
		t.Fatalf("expected 2 imported students, got %#v", imported.Students)
	}
	if imported.Students[0].DisplayName != "林小满" || imported.Students[1].StudentNo != "20260042" {
		t.Fatalf("unexpected imported students %#v", imported.Students)
	}
}

func TestStudentsImportTemplateDownloadOnlySupportsXLSX(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	csvRecorder := httptest.NewRecorder()
	csvRequest := httptest.NewRequest(http.MethodGet, "/api/students/import-template?format=csv", nil)
	csvRequest.AddCookie(cookie)
	handler.ServeHTTP(csvRecorder, csvRequest)
	if csvRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected csv template rejected with 422, got %d: %s", csvRecorder.Code, csvRecorder.Body.String())
	}
	var csvError apiErrorResponse
	decodeJSON(t, csvRecorder.Body, &csvError)
	if csvError.Error.Message != "仅支持下载 XLSX 模板" {
		t.Fatalf("unexpected csv template error %#v", csvError.Error)
	}

	xlsxRecorder := httptest.NewRecorder()
	xlsxRequest := httptest.NewRequest(http.MethodGet, "/api/students/import-template?format=xlsx", nil)
	xlsxRequest.AddCookie(cookie)
	handler.ServeHTTP(xlsxRecorder, xlsxRequest)
	if xlsxRecorder.Code != http.StatusOK {
		t.Fatalf("expected xlsx template 200, got %d: %s", xlsxRecorder.Code, xlsxRecorder.Body.String())
	}
	if contentType := xlsxRecorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") {
		t.Fatalf("expected xlsx content type, got %q", contentType)
	}
	if !strings.Contains(xlsxRecorder.Header().Get("Content-Disposition"), "classdrive-students-template.xlsx") {
		t.Fatalf("expected xlsx attachment filename, got %q", xlsxRecorder.Header().Get("Content-Disposition"))
	}

	templateEntries := unzipEntries(t, xlsxRecorder.Body.Bytes())
	sheetXML, ok := templateEntries["xl/worksheets/sheet1.xml"]
	if !ok {
		t.Fatalf("expected xlsx template worksheet, got keys %#v", templateEntries)
	}
	if !strings.Contains(sheetXML, "studentNo") || !strings.Contains(sheetXML, "displayName") {
		t.Fatalf("unexpected xlsx sheet xml %q", sheetXML)
	}
}

func TestEmptyStudentsListSerializesAsArray(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/students?classId=1", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list students 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"students":[]`) {
		t.Fatalf("expected empty students array, got %s", recorder.Body.String())
	}
}

func TestAssignmentsEmptyListSerializesAsArray(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/assignments?classId=1", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list assignments 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"assignments":[]`) {
		t.Fatalf("expected empty assignments array, got %s", recorder.Body.String())
	}
}

func TestFilesEmptyListSerializesAsArray(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/files?space=class&classId=2", nil)
	request.AddCookie(cookie)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list files 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("expected empty files array, got %s", recorder.Body.String())
	}
}

func TestAssignmentsListAndCreateFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "第一单元口算练习",
		"description": "完成第 12 页题目",
		"dueAt":       "2026-04-25T12:00:00Z",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)
	if created.ClassID != 1 || created.Title != "第一单元口算练习" || created.Status != "draft" {
		t.Fatalf("unexpected created assignment %#v", created)
	}
	if created.DueAt != "2026-04-25T12:00:00Z" {
		t.Fatalf("unexpected due at %#v", created)
	}

	secondRecorder := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "第二单元默写",
		"description": "",
		"dueAt":       "",
		"status":      "published",
	}))
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRequest.AddCookie(cookie)
	handler.ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusCreated {
		t.Fatalf("expected second create assignment 201, got %d: %s", secondRecorder.Code, secondRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments?classId=1", nil)
	listRequest.AddCookie(cookie)
	handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list assignments 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var payload assignmentsResponse
	decodeJSON(t, listRecorder.Body, &payload)
	if len(payload.Assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %#v", payload.Assignments)
	}
	if payload.Assignments[0].Title != "第二单元默写" || payload.Assignments[1].Title != "第一单元口算练习" {
		t.Fatalf("unexpected assignment order %#v", payload.Assignments)
	}
}

func TestAssignmentsCreateValidation(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	testCases := []struct {
		name           string
		payload        map[string]any
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "invalid class id",
			payload: map[string]any{
				"classId":     0,
				"title":       "练习一",
				"description": "",
				"dueAt":       "",
				"status":      "draft",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
		{
			name: "class not found",
			payload: map[string]any{
				"classId":     99,
				"title":       "练习一",
				"description": "",
				"dueAt":       "",
				"status":      "draft",
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name: "empty title",
			payload: map[string]any{
				"classId":     1,
				"title":       "  ",
				"description": "",
				"dueAt":       "",
				"status":      "draft",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
		{
			name: "invalid due at",
			payload: map[string]any{
				"classId":     1,
				"title":       "练习一",
				"description": "",
				"dueAt":       "tomorrow",
				"status":      "draft",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
		{
			name: "invalid status",
			payload: map[string]any{
				"classId":     1,
				"title":       "练习一",
				"description": "",
				"dueAt":       "",
				"status":      "archived",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, testCase.payload))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(cookie)
			handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedStatus {
				t.Fatalf("expected status %d, got %d: %s", testCase.expectedStatus, recorder.Code, recorder.Body.String())
			}

			var payload apiErrorResponse
			decodeJSON(t, recorder.Body, &payload)
			if payload.Error.Code != testCase.expectedCode {
				t.Fatalf("expected error code %q, got %#v", testCase.expectedCode, payload.Error)
			}
		})
	}
}

func TestAssignmentDetailByID(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "第三单元阅读理解",
		"description": "完成练习册第 8 页",
		"dueAt":       "2026-05-01T12:00:00Z",
		"status":      "published",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(created.ID)+"?classId=1", nil)
	detailRequest.AddCookie(cookie)
	handler.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected detail assignment 200, got %d: %s", detailRecorder.Code, detailRecorder.Body.String())
	}

	var detail assignmentPayload
	decodeJSON(t, detailRecorder.Body, &detail)
	if detail.ID != created.ID || detail.ClassID != 1 {
		t.Fatalf("unexpected assignment detail %#v", detail)
	}
	if detail.Title != "第三单元阅读理解" || detail.Status != "published" {
		t.Fatalf("unexpected assignment detail %#v", detail)
	}
	if detail.DueAt != "2026-05-01T12:00:00Z" {
		t.Fatalf("unexpected assignment detail %#v", detail)
	}
}

func TestAssignmentDetailValidation(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "第四单元抄写",
		"description": "",
		"dueAt":       "",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)

	testCases := []struct {
		name           string
		path           string
		includeCookie  bool
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "requires login",
			path:           "/api/assignments/" + itoa(created.ID) + "?classId=1",
			includeCookie:  false,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "unauthorized",
		},
		{
			name:           "invalid assignment id",
			path:           "/api/assignments/abc?classId=1",
			includeCookie:  true,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "missing class id",
			path:           "/api/assignments/" + itoa(created.ID),
			includeCookie:  true,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
		{
			name:           "mismatched class id",
			path:           "/api/assignments/" + itoa(created.ID) + "?classId=2",
			includeCookie:  true,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "assignment not found",
			path:           "/api/assignments/999?classId=1",
			includeCookie:  true,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			if testCase.includeCookie {
				request.AddCookie(cookie)
			}
			handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedStatus {
				t.Fatalf("expected status %d, got %d: %s", testCase.expectedStatus, recorder.Code, recorder.Body.String())
			}

			var payload apiErrorResponse
			decodeJSON(t, recorder.Body, &payload)
			if payload.Error.Code != testCase.expectedCode {
				t.Fatalf("expected error code %q, got %#v", testCase.expectedCode, payload.Error)
			}
		})
	}
}

func TestAssignmentUpdateAndDeleteFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "第五单元习题",
		"description": "原始说明",
		"dueAt":       "2026-05-02T12:00:00Z",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/assignments/"+itoa(created.ID)+"?classId=1", jsonBody(t, map[string]any{
		"title":       "第五单元习题-修订版",
		"description": "更新后的说明",
		"dueAt":       "2026-05-03T12:00:00Z",
		"status":      "published",
	}))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.AddCookie(cookie)
	handler.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update assignment 200, got %d: %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	var updated assignmentPayload
	decodeJSON(t, updateRecorder.Body, &updated)
	if updated.Title != "第五单元习题-修订版" || updated.Description != "更新后的说明" {
		t.Fatalf("unexpected updated assignment %#v", updated)
	}
	if updated.DueAt != "2026-05-03T12:00:00Z" || updated.Status != "published" {
		t.Fatalf("unexpected updated assignment %#v", updated)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/assignments/"+itoa(created.ID)+"?classId=1", nil)
	deleteRequest.AddCookie(cookie)
	handler.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete assignment 200, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	var deletePayload map[string]bool
	decodeJSON(t, deleteRecorder.Body, &deletePayload)
	if !deletePayload["ok"] {
		t.Fatalf("expected delete ok response, got %#v", deletePayload)
	}

	reloadRecorder := httptest.NewRecorder()
	reloadRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(created.ID)+"?classId=1", nil)
	reloadRequest.AddCookie(cookie)
	handler.ServeHTTP(reloadRecorder, reloadRequest)
	if reloadRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted assignment detail 404, got %d: %s", reloadRecorder.Code, reloadRecorder.Body.String())
	}
}

func TestAssignmentPatchPreservesOmittedFieldsAndAllowsExplicitClearing(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "第七单元作业",
		"description": "保留这段说明",
		"dueAt":       "2026-05-08T08:00:00Z",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)

	partialRecorder := httptest.NewRecorder()
	partialRequest := httptest.NewRequest(http.MethodPatch, "/api/assignments/"+itoa(created.ID)+"?classId=1", jsonBody(t, map[string]any{
		"title":  "第七单元作业-发布版",
		"status": "published",
	}))
	partialRequest.Header.Set("Content-Type", "application/json")
	partialRequest.AddCookie(cookie)
	handler.ServeHTTP(partialRecorder, partialRequest)
	if partialRecorder.Code != http.StatusOK {
		t.Fatalf("expected partial patch 200, got %d: %s", partialRecorder.Code, partialRecorder.Body.String())
	}

	var partiallyUpdated assignmentPayload
	decodeJSON(t, partialRecorder.Body, &partiallyUpdated)
	if partiallyUpdated.Title != "第七单元作业-发布版" || partiallyUpdated.Status != "published" {
		t.Fatalf("unexpected partial patch result %#v", partiallyUpdated)
	}
	if partiallyUpdated.Description != "保留这段说明" {
		t.Fatalf("expected omitted description to be preserved, got %#v", partiallyUpdated)
	}
	if partiallyUpdated.DueAt != "2026-05-08T08:00:00Z" {
		t.Fatalf("expected omitted dueAt to be preserved, got %#v", partiallyUpdated)
	}

	clearRecorder := httptest.NewRecorder()
	clearRequest := httptest.NewRequest(http.MethodPatch, "/api/assignments/"+itoa(created.ID)+"?classId=1", jsonBody(t, map[string]any{
		"description": "",
		"dueAt":       "",
	}))
	clearRequest.Header.Set("Content-Type", "application/json")
	clearRequest.AddCookie(cookie)
	handler.ServeHTTP(clearRecorder, clearRequest)
	if clearRecorder.Code != http.StatusOK {
		t.Fatalf("expected clear patch 200, got %d: %s", clearRecorder.Code, clearRecorder.Body.String())
	}

	var cleared assignmentPayload
	decodeJSON(t, clearRecorder.Body, &cleared)
	if cleared.Title != "第七单元作业-发布版" || cleared.Status != "published" {
		t.Fatalf("expected omitted title/status to be preserved on clear patch, got %#v", cleared)
	}
	if cleared.Description != "" || cleared.DueAt != "" {
		t.Fatalf("expected explicit empty description/dueAt to clear values, got %#v", cleared)
	}
}

func TestCreateAssignmentDefaultsEmptyDueAtToNinetyMinutesFromNow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	startedAt := time.Now().UTC()
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "默认截止时间作业",
		"description": "未手动选择截止时间",
		"dueAt":       "",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	finishedAt := time.Now().UTC()
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)
	if strings.TrimSpace(created.DueAt) == "" {
		t.Fatalf("expected default dueAt, got empty payload %#v", created)
	}
	dueAt, err := time.Parse(time.RFC3339, created.DueAt)
	if err != nil {
		t.Fatalf("expected RFC3339 default dueAt, got %q: %v", created.DueAt, err)
	}
	minDueAt := startedAt.Add(90 * time.Minute).Add(-time.Second)
	maxDueAt := finishedAt.Add(90 * time.Minute).Add(time.Second)
	if dueAt.Before(minDueAt) || dueAt.After(maxDueAt) {
		t.Fatalf("expected default dueAt near now + 90 minutes, got %s, range [%s, %s]", dueAt, minDueAt, maxDueAt)
	}
}

func TestAssignmentUpdateDeleteValidation(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "第六单元作业",
		"description": "",
		"dueAt":       "",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)

	testCases := []struct {
		name           string
		method         string
		path           string
		body           any
		expectedStatus int
		expectedCode   string
	}{
		{
			name:   "update missing class id",
			method: http.MethodPatch,
			path:   "/api/assignments/" + itoa(created.ID),
			body: map[string]any{
				"title":       "更新标题",
				"description": "",
				"dueAt":       "",
				"status":      "draft",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
		{
			name:   "update invalid status",
			method: http.MethodPatch,
			path:   "/api/assignments/" + itoa(created.ID) + "?classId=1",
			body: map[string]any{
				"title":       "更新标题",
				"description": "",
				"dueAt":       "",
				"status":      "archived",
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
		{
			name:           "update not found",
			method:         http.MethodPatch,
			path:           "/api/assignments/999?classId=1",
			body:           map[string]any{"title": "更新标题", "description": "", "dueAt": "", "status": "draft"},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "delete missing class id",
			method:         http.MethodDelete,
			path:           "/api/assignments/" + itoa(created.ID),
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
		{
			name:           "delete not found",
			method:         http.MethodDelete,
			path:           "/api/assignments/999?classId=1",
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var body io.Reader
			if testCase.body != nil {
				body = jsonBody(t, testCase.body)
			}

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, body)
			if testCase.body != nil {
				request.Header.Set("Content-Type", "application/json")
			}
			request.AddCookie(cookie)
			handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedStatus {
				t.Fatalf("expected status %d, got %d: %s", testCase.expectedStatus, recorder.Code, recorder.Body.String())
			}

			var payload apiErrorResponse
			decodeJSON(t, recorder.Body, &payload)
			if payload.Error.Code != testCase.expectedCode {
				t.Fatalf("expected error code %q, got %#v", testCase.expectedCode, payload.Error)
			}
		})
	}
}

func TestAssignmentAttachmentsEmptyListUploadDownloadAndDeleteFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "附件作业",
		"description": "",
		"dueAt":       "",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(created.ID)+"/attachments?classId=1", nil)
	listRequest.AddCookie(cookie)
	handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list attachments 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	if !strings.Contains(listRecorder.Body.String(), `"items":[]`) {
		t.Fatalf("expected empty attachment items array, got %s", listRecorder.Body.String())
	}

	uploadBody, uploadContentType := multipartFileBody(t, map[string]string{}, "files", "attachment.txt", []byte("assignment attachment body"))
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/assignments/"+itoa(created.ID)+"/attachments?classId=1", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected upload attachment 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	var uploaded assignmentAttachmentsResponse
	decodeJSON(t, uploadRecorder.Body, &uploaded)
	if len(uploaded.Items) != 1 {
		t.Fatalf("expected 1 uploaded attachment, got %#v", uploaded.Items)
	}
	if uploaded.Items[0].Name != "attachment.txt" {
		t.Fatalf("unexpected uploaded attachment %#v", uploaded.Items[0])
	}
	if uploaded.Items[0].DownloadURL != "/api/assignments/"+itoa(created.ID)+"/attachments/"+itoa(uploaded.Items[0].ID)+"/download?classId=1" {
		t.Fatalf("unexpected download url %#v", uploaded.Items[0])
	}

	reloadRecorder := httptest.NewRecorder()
	reloadRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(created.ID)+"/attachments?classId=1", nil)
	reloadRequest.AddCookie(cookie)
	handler.ServeHTTP(reloadRecorder, reloadRequest)
	if reloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected reload attachments 200, got %d: %s", reloadRecorder.Code, reloadRecorder.Body.String())
	}

	var reloaded assignmentAttachmentsResponse
	decodeJSON(t, reloadRecorder.Body, &reloaded)
	if len(reloaded.Items) != 1 || reloaded.Items[0].Name != "attachment.txt" {
		t.Fatalf("unexpected reloaded attachments %#v", reloaded.Items)
	}

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, reloaded.Items[0].DownloadURL, nil)
	downloadRequest.AddCookie(cookie)
	handler.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected download attachment 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	if !strings.Contains(downloadRecorder.Body.String(), "assignment attachment body") {
		t.Fatalf("expected attachment body in download response")
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/assignments/"+itoa(created.ID)+"/attachments/"+itoa(reloaded.Items[0].ID)+"?classId=1", nil)
	deleteRequest.AddCookie(cookie)
	handler.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete attachment 200, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	var deletePayload map[string]bool
	decodeJSON(t, deleteRecorder.Body, &deletePayload)
	if !deletePayload["ok"] {
		t.Fatalf("expected ok delete response, got %#v", deletePayload)
	}

	finalRecorder := httptest.NewRecorder()
	finalRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(created.ID)+"/attachments?classId=1", nil)
	finalRequest.AddCookie(cookie)
	handler.ServeHTTP(finalRecorder, finalRequest)
	if finalRecorder.Code != http.StatusOK {
		t.Fatalf("expected final list attachments 200, got %d: %s", finalRecorder.Code, finalRecorder.Body.String())
	}
	if !strings.Contains(finalRecorder.Body.String(), `"items":[]`) {
		t.Fatalf("expected empty attachments after delete, got %s", finalRecorder.Body.String())
	}
}

func TestDeleteAssignmentRemovesAttachmentFilesAndEntries(t *testing.T) {
	t.Parallel()

	baseDir, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "待删除附件作业",
		"description": "",
		"dueAt":       "",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	app.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)

	uploadBody, uploadContentType := multipartFileBody(t, map[string]string{}, "files", "to-delete.txt", []byte("delete me"))
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/assignments/"+itoa(created.ID)+"/attachments?classId=1", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(cookie)
	app.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected upload attachment 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	var uploaded assignmentAttachmentsResponse
	decodeJSON(t, uploadRecorder.Body, &uploaded)
	if len(uploaded.Items) != 1 {
		t.Fatalf("expected uploaded attachment, got %#v", uploaded.Items)
	}

	storageDir := filepath.Join(baseDir, "var", "storage", "assignments", "1", itoa(created.ID))
	if _, err := os.Stat(filepath.Join(storageDir, "to-delete.txt")); err != nil {
		t.Fatalf("expected attachment file to exist before delete: %v", err)
	}

	if count := countAssignmentFileEntries(t, app.db, created.ID, 1); count != 1 {
		t.Fatalf("expected 1 assignment file entry before delete, got %d", count)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/assignments/"+itoa(created.ID)+"?classId=1", nil)
	deleteRequest.AddCookie(cookie)
	app.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete assignment 200, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	if _, err := os.Stat(storageDir); !os.IsNotExist(err) {
		t.Fatalf("expected assignment attachment directory to be removed, stat err = %v", err)
	}

	if count := countAssignmentFileEntries(t, app.db, created.ID, 1); count != 0 {
		t.Fatalf("expected assignment file entries removed, got %d", count)
	}

	if count := countAssignmentsByID(t, app.db, created.ID, 1); count != 0 {
		t.Fatalf("expected assignment row removed, got %d", count)
	}
}

func TestAssignmentAttachmentsValidation(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/assignments", jsonBody(t, map[string]any{
		"classId":     1,
		"title":       "附件校验作业",
		"description": "",
		"dueAt":       "",
		"status":      "draft",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create assignment 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created assignmentPayload
	decodeJSON(t, createRecorder.Body, &created)

	uploadBody, uploadContentType := multipartFileBody(t, map[string]string{}, "files", "attachment.txt", []byte("validation body"))
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/assignments/"+itoa(created.ID)+"/attachments?classId=1", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected upload attachment 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	var uploaded assignmentAttachmentsResponse
	decodeJSON(t, uploadRecorder.Body, &uploaded)
	attachmentID := uploaded.Items[0].ID

	testCases := []struct {
		name           string
		method         string
		path           string
		body           io.Reader
		contentType    string
		includeCookie  bool
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "list requires login",
			method:         http.MethodGet,
			path:           "/api/assignments/" + itoa(created.ID) + "/attachments?classId=1",
			includeCookie:  false,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "unauthorized",
		},
		{
			name:           "list missing class id",
			method:         http.MethodGet,
			path:           "/api/assignments/" + itoa(created.ID) + "/attachments",
			includeCookie:  true,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "invalid_request",
		},
		{
			name:           "list assignment not found",
			method:         http.MethodGet,
			path:           "/api/assignments/999/attachments?classId=1",
			includeCookie:  true,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "list class mismatch",
			method:         http.MethodGet,
			path:           "/api/assignments/" + itoa(created.ID) + "/attachments?classId=2",
			includeCookie:  true,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "download wrong assignment file id",
			method:         http.MethodGet,
			path:           "/api/assignments/" + itoa(created.ID+1) + "/attachments/" + itoa(attachmentID) + "/download?classId=1",
			includeCookie:  true,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
		{
			name:           "delete missing file",
			method:         http.MethodDelete,
			path:           "/api/assignments/" + itoa(created.ID) + "/attachments/999?classId=1",
			includeCookie:  true,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "not_found",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, testCase.body)
			if testCase.contentType != "" {
				request.Header.Set("Content-Type", testCase.contentType)
			}
			if testCase.includeCookie {
				request.AddCookie(cookie)
			}
			handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.expectedStatus {
				t.Fatalf("expected status %d, got %d: %s", testCase.expectedStatus, recorder.Code, recorder.Body.String())
			}

			var payload apiErrorResponse
			decodeJSON(t, recorder.Body, &payload)
			if payload.Error.Code != testCase.expectedCode {
				t.Fatalf("expected error code %q, got %#v", testCase.expectedCode, payload.Error)
			}
		})
	}
}

func TestFilesCRUDDownloadPreviewAndCopy(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	libraryFiles := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	if len(libraryFiles.Items) == 0 {
		t.Fatalf("expected seeded library entries")
	}

	createFolderRecorder := httptest.NewRecorder()
	createFolderRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "新建文件夹",
	}))
	createFolderRequest.Header.Set("Content-Type", "application/json")
	createFolderRequest.AddCookie(cookie)
	handler.ServeHTTP(createFolderRecorder, createFolderRequest)
	if createFolderRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create folder 201, got %d: %s", createFolderRecorder.Code, createFolderRecorder.Body.String())
	}

	uploadRecorder := httptest.NewRecorder()
	uploadBody := &bytes.Buffer{}
	uploadWriter := multipart.NewWriter(uploadBody)
	mustWriteField(t, uploadWriter, "space", "library")
	mustWriteField(t, uploadWriter, "parentPath", "/")
	part, err := uploadWriter.CreateFormFile("files", "lesson-notes.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("hello classdrive")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := uploadWriter.Close(); err != nil {
		t.Fatalf("close upload writer: %v", err)
	}

	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadWriter.FormDataContentType())
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected upload 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	afterUpload := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	uploadedEntry := findFileByName(t, afterUpload.Items, "lesson-notes.txt")
	if strings.TrimSpace(uploadedEntry.UpdatedAt) == "" {
		t.Fatalf("expected uploaded entry updatedAt to be populated")
	}
	if _, err := time.Parse(time.RFC3339, uploadedEntry.UpdatedAt); err != nil {
		t.Fatalf("expected uploaded entry updatedAt to be RFC3339, got %q (%v)", uploadedEntry.UpdatedAt, err)
	}

	renameRecorder := httptest.NewRecorder()
	renameRequest := httptest.NewRequest(http.MethodPatch, "/api/files/"+itoa(uploadedEntry.ID), jsonBody(t, map[string]string{
		"name": "lesson-notes-v2.txt",
	}))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.AddCookie(cookie)
	handler.ServeHTTP(renameRecorder, renameRequest)
	if renameRecorder.Code != http.StatusOK {
		t.Fatalf("expected rename 200, got %d: %s", renameRecorder.Code, renameRecorder.Body.String())
	}

	afterRename := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	renamedEntry := findFileByName(t, afterRename.Items, "lesson-notes-v2.txt")

	copyRecorder := httptest.NewRecorder()
	copyRequest := httptest.NewRequest(http.MethodPost, "/api/files/copy", jsonBody(t, map[string]any{
		"entryId":               renamedEntry.ID,
		"destinationSpace":      "public",
		"destinationParentPath": "/",
	}))
	copyRequest.Header.Set("Content-Type", "application/json")
	copyRequest.AddCookie(cookie)
	handler.ServeHTTP(copyRecorder, copyRequest)
	if copyRecorder.Code != http.StatusCreated {
		t.Fatalf("expected copy 201, got %d: %s", copyRecorder.Code, copyRecorder.Body.String())
	}

	publicFiles := listFiles(t, handler, cookie, url.Values{
		"space": {"public"},
	})
	copiedEntry := findFileByName(t, publicFiles.Items, "lesson-notes-v2.txt")

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, copiedEntry.DownloadURL, nil)
	downloadRequest.AddCookie(cookie)
	handler.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	if !strings.Contains(downloadRecorder.Body.String(), "hello classdrive") {
		t.Fatalf("expected download body to contain uploaded content")
	}

	previewRecorder := httptest.NewRecorder()
	previewRequest := httptest.NewRequest(http.MethodGet, copiedEntry.PreviewURL, nil)
	previewRequest.AddCookie(cookie)
	handler.ServeHTTP(previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("expected preview 200, got %d: %s", previewRecorder.Code, previewRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/files/"+itoa(copiedEntry.ID), nil)
	deleteRequest.AddCookie(cookie)
	handler.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	publicAfterDelete := listFiles(t, handler, cookie, url.Values{
		"space": {"public"},
	})
	assertMissingFile(t, publicAfterDelete.Items, "lesson-notes-v2.txt")
}

func TestRemoveFileTreeFallsBackAfterOpenfdatInvalidArgument(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "素材", "效果示例")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "demo.txt"), []byte("demo"), 0o644); err != nil {
		t.Fatalf("write nested file: %v", err)
	}

	originalRemoveAllPath := removeAllPath
	removeAllPath = func(path string) error {
		return &os.PathError{Op: "openfdat", Path: filepath.Join(path, "效果示例"), Err: syscall.EINVAL}
	}
	t.Cleanup(func() {
		removeAllPath = originalRemoveAllPath
	})

	if err := removeFileTree(filepath.Join(root, "素材")); err != nil {
		t.Fatalf("expected fallback removal to succeed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "素材")); !os.IsNotExist(err) {
		t.Fatalf("expected directory tree to be removed, got %v", err)
	}
}

func TestDeleteSubmissionFilesUsesRemoveFileTreeFallback(t *testing.T) {
	_, app := newTestServer(t)
	submissionID := int64(42)
	classID := int64(1)
	if _, err := app.createSeedFile("submission", optionalInt64(classID), assignmentSubmissionParentPath(submissionID), "效果示例.txt", []byte("demo")); err != nil {
		t.Fatalf("create submission file: %v", err)
	}

	originalRemoveAllPath := removeAllPath
	removeAllCalled := false
	removeAllPath = func(path string) error {
		removeAllCalled = true
		return &os.PathError{Op: "openfdat", Path: filepath.Join(path, "效果示例"), Err: syscall.EINVAL}
	}
	t.Cleanup(func() {
		removeAllPath = originalRemoveAllPath
	})

	if err := app.deleteSubmissionFiles(submissionID, classID); err != nil {
		t.Fatalf("expected submission deletion fallback to succeed, got %v", err)
	}
	if !removeAllCalled {
		t.Fatalf("expected deleteSubmissionFiles to use removeFileTree")
	}
	if countSubmissionFileEntries(t, app.db, submissionID, classID) != 0 {
		t.Fatalf("expected submission file entries removed")
	}
}

func TestDirectoryListingAndRecursiveCopy(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	rootFiles := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	sourceDirectory := findFileByName(t, rootFiles.Items, "课程资料")

	createChildRecorder := httptest.NewRecorder()
	createChildRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/课程资料",
		"name":       "单元一",
	}))
	createChildRequest.Header.Set("Content-Type", "application/json")
	createChildRequest.AddCookie(cookie)
	handler.ServeHTTP(createChildRecorder, createChildRequest)
	if createChildRecorder.Code != http.StatusCreated {
		t.Fatalf("expected nested folder 201, got %d: %s", createChildRecorder.Code, createChildRecorder.Body.String())
	}

	uploadRecorder := httptest.NewRecorder()
	uploadBody := &bytes.Buffer{}
	uploadWriter := multipart.NewWriter(uploadBody)
	mustWriteField(t, uploadWriter, "space", "library")
	mustWriteField(t, uploadWriter, "parentPath", "/课程资料/单元一")
	part, err := uploadWriter.CreateFormFile("files", "讲义.txt")
	if err != nil {
		t.Fatalf("create nested form file: %v", err)
	}
	if _, err := part.Write([]byte("nested lecture")); err != nil {
		t.Fatalf("write nested form file: %v", err)
	}
	if err := uploadWriter.Close(); err != nil {
		t.Fatalf("close nested upload writer: %v", err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadWriter.FormDataContentType())
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected nested upload 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	nestedList := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
		"path":  {"/课程资料/单元一"},
	})
	findFileByName(t, nestedList.Items, "讲义.txt")

	copyRecorder := httptest.NewRecorder()
	copyRequest := httptest.NewRequest(http.MethodPost, "/api/files/copy", jsonBody(t, map[string]any{
		"entryId":               sourceDirectory.ID,
		"destinationSpace":      "public",
		"destinationParentPath": "/",
	}))
	copyRequest.Header.Set("Content-Type", "application/json")
	copyRequest.AddCookie(cookie)
	handler.ServeHTTP(copyRecorder, copyRequest)
	if copyRecorder.Code != http.StatusCreated {
		t.Fatalf("expected directory copy 201, got %d: %s", copyRecorder.Code, copyRecorder.Body.String())
	}

	publicRoot := listFiles(t, handler, cookie, url.Values{
		"space": {"public"},
	})
	findFileByName(t, publicRoot.Items, "课程资料")

	publicNested := listFiles(t, handler, cookie, url.Values{
		"space": {"public"},
		"path":  {"/课程资料/单元一"},
	})
	copiedLecture := findFileByName(t, publicNested.Items, "讲义.txt")

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, copiedLecture.DownloadURL, nil)
	downloadRequest.AddCookie(cookie)
	handler.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected copied nested file download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	if !strings.Contains(downloadRecorder.Body.String(), "nested lecture") {
		t.Fatalf("expected copied nested file content")
	}
}

func TestFileUploadConflictModes(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	initialBody, initialContentType := multipartFileBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, "files", "冲突上传.txt", []byte("first version"))
	initialRecorder := httptest.NewRecorder()
	initialRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", initialBody)
	initialRequest.Header.Set("Content-Type", initialContentType)
	initialRequest.AddCookie(cookie)
	handler.ServeHTTP(initialRecorder, initialRequest)
	if initialRecorder.Code != http.StatusCreated {
		t.Fatalf("expected initial upload 201, got %d: %s", initialRecorder.Code, initialRecorder.Body.String())
	}

	rejectBody, rejectContentType := multipartFileBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, "files", "冲突上传.txt", []byte("reject version"))
	rejectRecorder := httptest.NewRecorder()
	rejectRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", rejectBody)
	rejectRequest.Header.Set("Content-Type", rejectContentType)
	rejectRequest.AddCookie(cookie)
	handler.ServeHTTP(rejectRecorder, rejectRequest)
	if rejectRecorder.Code != http.StatusConflict {
		t.Fatalf("expected reject upload 409, got %d: %s", rejectRecorder.Code, rejectRecorder.Body.String())
	}

	skipBody, skipContentType := multipartFileBody(t, map[string]string{
		"space":        "library",
		"parentPath":   "/",
		"conflictMode": "skip",
	}, "files", "冲突上传.txt", []byte("skip version"))
	skipRecorder := httptest.NewRecorder()
	skipRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", skipBody)
	skipRequest.Header.Set("Content-Type", skipContentType)
	skipRequest.AddCookie(cookie)
	handler.ServeHTTP(skipRecorder, skipRequest)
	if skipRecorder.Code != http.StatusCreated {
		t.Fatalf("expected skip upload 201, got %d: %s", skipRecorder.Code, skipRecorder.Body.String())
	}

	var skipPayload struct {
		Items   []fileSummary `json:"items"`
		Summary struct {
			CreatedCount  int `json:"createdCount"`
			ReplacedCount int `json:"replacedCount"`
			RenamedCount  int `json:"renamedCount"`
			SkippedCount  int `json:"skippedCount"`
		} `json:"summary"`
	}
	decodeJSON(t, skipRecorder.Body, &skipPayload)
	if len(skipPayload.Items) != 0 || skipPayload.Summary.SkippedCount != 1 {
		t.Fatalf("expected skipped upload summary, got %#v", skipPayload)
	}

	renameBody, renameContentType := multipartFileBody(t, map[string]string{
		"space":        "library",
		"parentPath":   "/",
		"conflictMode": "rename",
	}, "files", "冲突上传.txt", []byte("rename version"))
	renameRecorder := httptest.NewRecorder()
	renameRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", renameBody)
	renameRequest.Header.Set("Content-Type", renameContentType)
	renameRequest.AddCookie(cookie)
	handler.ServeHTTP(renameRecorder, renameRequest)
	if renameRecorder.Code != http.StatusCreated {
		t.Fatalf("expected rename upload 201, got %d: %s", renameRecorder.Code, renameRecorder.Body.String())
	}

	var renamePayload struct {
		Items   []fileSummary `json:"items"`
		Summary struct {
			CreatedCount  int `json:"createdCount"`
			ReplacedCount int `json:"replacedCount"`
			RenamedCount  int `json:"renamedCount"`
			SkippedCount  int `json:"skippedCount"`
		} `json:"summary"`
	}
	decodeJSON(t, renameRecorder.Body, &renamePayload)
	if len(renamePayload.Items) != 1 || renamePayload.Items[0].Name != "冲突上传 (1).txt" || renamePayload.Summary.RenamedCount != 1 {
		t.Fatalf("expected renamed upload result, got %#v", renamePayload)
	}

	replaceBody, replaceContentType := multipartFileBody(t, map[string]string{
		"space":        "library",
		"parentPath":   "/",
		"conflictMode": "replace",
	}, "files", "冲突上传.txt", []byte("replace version"))
	replaceRecorder := httptest.NewRecorder()
	replaceRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", replaceBody)
	replaceRequest.Header.Set("Content-Type", replaceContentType)
	replaceRequest.AddCookie(cookie)
	handler.ServeHTTP(replaceRecorder, replaceRequest)
	if replaceRecorder.Code != http.StatusCreated {
		t.Fatalf("expected replace upload 201, got %d: %s", replaceRecorder.Code, replaceRecorder.Body.String())
	}

	libraryFiles := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	replacedEntry := findFileByName(t, libraryFiles.Items, "冲突上传.txt")
	replaceDownloadRecorder := httptest.NewRecorder()
	replaceDownloadRequest := httptest.NewRequest(http.MethodGet, replacedEntry.DownloadURL, nil)
	replaceDownloadRequest.AddCookie(cookie)
	handler.ServeHTTP(replaceDownloadRecorder, replaceDownloadRequest)
	if replaceDownloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected replaced download 200, got %d: %s", replaceDownloadRecorder.Code, replaceDownloadRecorder.Body.String())
	}
	if !strings.Contains(replaceDownloadRecorder.Body.String(), "replace version") {
		t.Fatalf("expected replaced file content, got %s", replaceDownloadRecorder.Body.String())
	}
	findFileByName(t, libraryFiles.Items, "冲突上传 (1).txt")
}

func TestFilesListReturnsMimeType(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	fileName := "mime-list.txt"
	fileContents := []byte("list mime contract")
	expectedMimeType := contentTypeForFile(fileName, fileContents)

	uploadBody, uploadContentType := multipartFileBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, "files", fileName, fileContents)
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(cookie)
	app.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected upload 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/files?space=library", nil)
	listRequest.AddCookie(cookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}

	var payload struct {
		Items []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			MimeType   string `json:"mimeType"`
			PreviewURL string `json:"previewUrl"`
		} `json:"items"`
	}
	decodeJSON(t, listRecorder.Body, &payload)

	for _, item := range payload.Items {
		if item.Name != fileName {
			continue
		}
		if item.MimeType != expectedMimeType {
			t.Fatalf("expected listed mimeType %q, got %#v", expectedMimeType, item)
		}
		expectedPreviewURL := "/api/files/" + itoa(item.ID) + "/preview"
		if item.PreviewURL != expectedPreviewURL {
			t.Fatalf("expected preview url %q, got %#v", expectedPreviewURL, item)
		}
		return
	}

	t.Fatalf("expected file %q in %#v", fileName, payload.Items)
}

func TestFileUploadWritesAndReturnsMimeType(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	fileName := "mime-upload.txt"
	fileContents := []byte("upload mime contract")
	expectedMimeType := contentTypeForFile(fileName, fileContents)

	uploadBody, uploadContentType := multipartFileBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, "files", fileName, fileContents)
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(cookie)
	app.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected upload 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	var payload struct {
		Items []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			MimeType string `json:"mimeType"`
		} `json:"items"`
	}
	decodeJSON(t, uploadRecorder.Body, &payload)
	if len(payload.Items) != 1 {
		t.Fatalf("expected one uploaded item, got %#v", payload.Items)
	}
	if payload.Items[0].MimeType != expectedMimeType {
		t.Fatalf("expected upload response mimeType %q, got %#v", expectedMimeType, payload.Items[0])
	}

	var storedMimeType string
	if err := app.db.QueryRow(`select mime_type from file_entries where id = ?`, payload.Items[0].ID).Scan(&storedMimeType); err != nil {
		t.Fatalf("query uploaded mime type: %v", err)
	}
	if storedMimeType != expectedMimeType {
		t.Fatalf("expected stored mimeType %q, got %q", expectedMimeType, storedMimeType)
	}
}

func TestTeacherTextFileContentReadAndSave(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	initialContent := "# 初稿\n第一行"
	updatedContent := "# 修订稿\n第二行"

	uploadBody, uploadContentType := multipartFileBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, "files", "editor-note.md", []byte(initialContent))
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(cookie)
	app.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected upload 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	var uploaded uploadFilesResult
	decodeJSON(t, uploadRecorder.Body, &uploaded)
	if len(uploaded.Items) != 1 {
		t.Fatalf("expected one uploaded item, got %#v", uploaded.Items)
	}
	fileID := uploaded.Items[0].ID

	readRecorder := httptest.NewRecorder()
	readRequest := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(fileID)+"/content", nil)
	readRequest.AddCookie(cookie)
	app.ServeHTTP(readRecorder, readRequest)
	if readRecorder.Code != http.StatusOK {
		t.Fatalf("expected read content 200, got %d: %s", readRecorder.Code, readRecorder.Body.String())
	}

	var readPayload struct {
		Item    fileSummary `json:"item"`
		Content string      `json:"content"`
	}
	decodeJSON(t, readRecorder.Body, &readPayload)
	if readPayload.Content != initialContent {
		t.Fatalf("expected initial content %q, got %#v", initialContent, readPayload)
	}

	saveRecorder := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(http.MethodPut, "/api/files/"+itoa(fileID)+"/content", jsonBody(t, map[string]string{
		"content": updatedContent,
	}))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveRequest.AddCookie(cookie)
	app.ServeHTTP(saveRecorder, saveRequest)
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("expected save content 200, got %d: %s", saveRecorder.Code, saveRecorder.Body.String())
	}

	var savePayload struct {
		Item    fileSummary `json:"item"`
		Content string      `json:"content"`
	}
	decodeJSON(t, saveRecorder.Body, &savePayload)
	if savePayload.Content != updatedContent {
		t.Fatalf("expected updated content %q, got %#v", updatedContent, savePayload)
	}
	if savePayload.Item.Size != int64(len([]byte(updatedContent))) {
		t.Fatalf("expected updated size %d, got %#v", len([]byte(updatedContent)), savePayload.Item)
	}

	previewRecorder := httptest.NewRecorder()
	previewRequest := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(fileID)+"/preview", nil)
	previewRequest.AddCookie(cookie)
	app.ServeHTTP(previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("expected preview 200 after save, got %d: %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	if previewRecorder.Body.String() != updatedContent {
		t.Fatalf("expected preview body %q, got %q", updatedContent, previewRecorder.Body.String())
	}
}

func TestCreateFileCreatesEmptyTextFileAndAllowsImmediateRead(t *testing.T) {
	t.Parallel()

	baseDir, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/files/file", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "新建文档.md",
		"content":    "",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	app.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create file 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created fileContentResult
	decodeJSON(t, createRecorder.Body, &created)
	if created.Item.Name != "新建文档.md" || created.Item.Path != "/新建文档.md" {
		t.Fatalf("unexpected created item %#v", created.Item)
	}
	if created.Content != "" {
		t.Fatalf("expected empty content, got %#v", created)
	}

	listPayload := listFiles(t, app, cookie, url.Values{
		"space": {"library"},
	})
	createdItem := findFileByName(t, listPayload.Items, "新建文档.md")
	if createdItem.ID != created.Item.ID {
		t.Fatalf("expected created item id %d in list, got %#v", created.Item.ID, createdItem)
	}

	entry, err := app.getEntryByID(created.Item.ID)
	if err != nil {
		t.Fatalf("get created entry: %v", err)
	}
	absolutePath := filepath.Join(baseDir, "var", "storage", filepath.FromSlash(entry.DiskPath))
	info, err := os.Stat(absolutePath)
	if err != nil {
		t.Fatalf("expected created file on disk: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty file size 0, got %d", info.Size())
	}

	readRecorder := httptest.NewRecorder()
	readRequest := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(created.Item.ID)+"/content", nil)
	readRequest.AddCookie(cookie)
	app.ServeHTTP(readRecorder, readRequest)
	if readRecorder.Code != http.StatusOK {
		t.Fatalf("expected immediate read 200, got %d: %s", readRecorder.Code, readRecorder.Body.String())
	}

	var readPayload fileContentResult
	decodeJSON(t, readRecorder.Body, &readPayload)
	if readPayload.Content != "" {
		t.Fatalf("expected immediate read content to be empty, got %#v", readPayload)
	}
}

func TestFileContentSupportsMarkdownSaveUpdate(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)
	initialContent := "# 初稿\n第一行"
	updatedContent := "# 修订稿\n第二行"

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/files/file", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "课堂纪要.md",
		"content":    initialContent,
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	app.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create markdown file 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created fileContentResult
	decodeJSON(t, createRecorder.Body, &created)
	if created.Content != initialContent {
		t.Fatalf("expected initial markdown content %q, got %#v", initialContent, created)
	}

	saveRecorder := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(http.MethodPut, "/api/files/"+itoa(created.Item.ID)+"/content", jsonBody(t, map[string]string{
		"content": updatedContent,
	}))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveRequest.AddCookie(cookie)
	app.ServeHTTP(saveRecorder, saveRequest)
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("expected markdown save 200, got %d: %s", saveRecorder.Code, saveRecorder.Body.String())
	}

	var savePayload fileContentResult
	decodeJSON(t, saveRecorder.Body, &savePayload)
	if savePayload.Content != updatedContent {
		t.Fatalf("expected updated markdown content %q, got %#v", updatedContent, savePayload)
	}
	if savePayload.Item.Size != int64(len([]byte(updatedContent))) {
		t.Fatalf("expected updated markdown size %d, got %#v", len([]byte(updatedContent)), savePayload.Item)
	}

	previewRecorder := httptest.NewRecorder()
	previewRequest := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(created.Item.ID)+"/preview", nil)
	previewRequest.AddCookie(cookie)
	app.ServeHTTP(previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("expected markdown preview 200, got %d: %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	if previewRecorder.Body.String() != updatedContent {
		t.Fatalf("expected markdown preview body %q, got %q", updatedContent, previewRecorder.Body.String())
	}
}

func TestTeacherTextFileContentRejectsUnsupportedFiles(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	uploadBody, uploadContentType := multipartFileBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, "files", "photo.png", []byte("not an editable text file"))
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(cookie)
	app.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected upload 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	var uploaded uploadFilesResult
	decodeJSON(t, uploadRecorder.Body, &uploaded)
	if len(uploaded.Items) != 1 {
		t.Fatalf("expected one uploaded item, got %#v", uploaded.Items)
	}
	fileID := uploaded.Items[0].ID

	readRecorder := httptest.NewRecorder()
	readRequest := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(fileID)+"/content", nil)
	readRequest.AddCookie(cookie)
	app.ServeHTTP(readRecorder, readRequest)
	if readRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unsupported read 422, got %d: %s", readRecorder.Code, readRecorder.Body.String())
	}

	saveRecorder := httptest.NewRecorder()
	saveRequest := httptest.NewRequest(http.MethodPut, "/api/files/"+itoa(fileID)+"/content", jsonBody(t, map[string]string{
		"content": "still unsupported",
	}))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveRequest.AddCookie(cookie)
	app.ServeHTTP(saveRecorder, saveRequest)
	if saveRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unsupported save 422, got %d: %s", saveRecorder.Code, saveRecorder.Body.String())
	}
}

func TestTUSUploadSessionCreateHeadPatchAndComplete(t *testing.T) {
	t.Parallel()

	baseDir, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)
	fileContents := []byte("hello classdrive via tus")

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload/sessions", nil)
	createRequest.Header.Set("Tus-Resumable", "1.0.0")
	createRequest.Header.Set("Upload-Length", strconv.Itoa(len(fileContents)))
	createRequest.Header.Set("Upload-Metadata", tusMetadataHeader(map[string]string{
		"space":        "library",
		"parentPath":   "/",
		"filename":     "tus-notes.txt",
		"conflictMode": "reject",
	}))
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create session 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	sessionPath := createRecorder.Header().Get("Location")
	if !strings.HasPrefix(sessionPath, "/api/files/upload/sessions/") {
		t.Fatalf("expected session location header, got %q", sessionPath)
	}
	sessionID := strings.TrimPrefix(sessionPath, "/api/files/upload/sessions/")
	if strings.TrimSpace(sessionID) == "" {
		t.Fatalf("expected non-empty session id")
	}
	if createRecorder.Header().Get("Upload-Offset") != "0" {
		t.Fatalf("expected create offset 0, got %q", createRecorder.Header().Get("Upload-Offset"))
	}

	headRecorder := httptest.NewRecorder()
	headRequest := httptest.NewRequest(http.MethodHead, sessionPath, nil)
	headRequest.Header.Set("Tus-Resumable", "1.0.0")
	headRequest.AddCookie(cookie)
	handler.ServeHTTP(headRecorder, headRequest)
	if headRecorder.Code != http.StatusOK {
		t.Fatalf("expected head 200, got %d", headRecorder.Code)
	}
	if headRecorder.Header().Get("Upload-Offset") != "0" {
		t.Fatalf("expected head offset 0, got %q", headRecorder.Header().Get("Upload-Offset"))
	}
	if headRecorder.Header().Get("Upload-Length") != strconv.Itoa(len(fileContents)) {
		t.Fatalf("expected head length %d, got %q", len(fileContents), headRecorder.Header().Get("Upload-Length"))
	}

	firstChunk := []byte("hello ")
	firstPatchRecorder := httptest.NewRecorder()
	firstPatchRequest := httptest.NewRequest(http.MethodPatch, sessionPath, bytes.NewReader(firstChunk))
	firstPatchRequest.Header.Set("Tus-Resumable", "1.0.0")
	firstPatchRequest.Header.Set("Content-Type", "application/offset+octet-stream")
	firstPatchRequest.Header.Set("Upload-Offset", "0")
	firstPatchRequest.AddCookie(cookie)
	handler.ServeHTTP(firstPatchRecorder, firstPatchRequest)
	if firstPatchRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected first patch 204, got %d: %s", firstPatchRecorder.Code, firstPatchRecorder.Body.String())
	}
	if firstPatchRecorder.Header().Get("Upload-Offset") != strconv.Itoa(len(firstChunk)) {
		t.Fatalf("expected first patch offset %d, got %q", len(firstChunk), firstPatchRecorder.Header().Get("Upload-Offset"))
	}

	headAfterPatchRecorder := httptest.NewRecorder()
	headAfterPatchRequest := httptest.NewRequest(http.MethodHead, sessionPath, nil)
	headAfterPatchRequest.Header.Set("Tus-Resumable", "1.0.0")
	headAfterPatchRequest.AddCookie(cookie)
	handler.ServeHTTP(headAfterPatchRecorder, headAfterPatchRequest)
	if headAfterPatchRecorder.Code != http.StatusOK {
		t.Fatalf("expected head after patch 200, got %d", headAfterPatchRecorder.Code)
	}
	if headAfterPatchRecorder.Header().Get("Upload-Offset") != strconv.Itoa(len(firstChunk)) {
		t.Fatalf("expected head after patch offset %d, got %q", len(firstChunk), headAfterPatchRecorder.Header().Get("Upload-Offset"))
	}

	secondPatchRecorder := httptest.NewRecorder()
	secondPatchRequest := httptest.NewRequest(http.MethodPatch, sessionPath, bytes.NewReader(fileContents[len(firstChunk):]))
	secondPatchRequest.Header.Set("Tus-Resumable", "1.0.0")
	secondPatchRequest.Header.Set("Content-Type", "application/offset+octet-stream")
	secondPatchRequest.Header.Set("Upload-Offset", strconv.Itoa(len(firstChunk)))
	secondPatchRequest.AddCookie(cookie)
	handler.ServeHTTP(secondPatchRecorder, secondPatchRequest)
	if secondPatchRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected second patch 204, got %d: %s", secondPatchRecorder.Code, secondPatchRecorder.Body.String())
	}
	if secondPatchRecorder.Header().Get("Upload-Offset") != strconv.Itoa(len(fileContents)) {
		t.Fatalf("expected complete offset %d, got %q", len(fileContents), secondPatchRecorder.Header().Get("Upload-Offset"))
	}

	libraryFiles := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	uploadedFile := findFileByName(t, libraryFiles.Items, "tus-notes.txt")

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, uploadedFile.DownloadURL, nil)
	downloadRequest.AddCookie(cookie)
	handler.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected uploaded file download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	if downloadRecorder.Body.String() != string(fileContents) {
		t.Fatalf("expected uploaded file body %q, got %q", string(fileContents), downloadRecorder.Body.String())
	}

	sessionDataPath, sessionMetaPath := tusSessionArtifacts(baseDir, sessionID)
	if _, err := os.Stat(sessionDataPath); !os.IsNotExist(err) {
		t.Fatalf("expected completed session temp data to be removed, err=%v", err)
	}
	if _, err := os.Stat(sessionMetaPath); !os.IsNotExist(err) {
		t.Fatalf("expected completed session metadata to be removed, err=%v", err)
	}
}

func TestTUSUploadSessionDeleteRemovesTemporaryFiles(t *testing.T) {
	t.Parallel()

	baseDir, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)
	fileContents := []byte("delete session body")

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload/sessions", nil)
	createRequest.Header.Set("Tus-Resumable", "1.0.0")
	createRequest.Header.Set("Upload-Length", strconv.Itoa(len(fileContents)))
	createRequest.Header.Set("Upload-Metadata", tusMetadataHeader(map[string]string{
		"space":        "library",
		"parentPath":   "/",
		"filename":     "delete-tus.txt",
		"conflictMode": "reject",
	}))
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create session 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	sessionPath := createRecorder.Header().Get("Location")
	sessionID := strings.TrimPrefix(sessionPath, "/api/files/upload/sessions/")
	sessionDataPath, sessionMetaPath := tusSessionArtifacts(baseDir, sessionID)

	patchRecorder := httptest.NewRecorder()
	patchRequest := httptest.NewRequest(http.MethodPatch, sessionPath, bytes.NewReader(fileContents[:7]))
	patchRequest.Header.Set("Tus-Resumable", "1.0.0")
	patchRequest.Header.Set("Content-Type", "application/offset+octet-stream")
	patchRequest.Header.Set("Upload-Offset", "0")
	patchRequest.AddCookie(cookie)
	handler.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected patch before delete 204, got %d: %s", patchRecorder.Code, patchRecorder.Body.String())
	}

	if _, err := os.Stat(sessionDataPath); err != nil {
		t.Fatalf("expected temp data file to exist before delete: %v", err)
	}
	if _, err := os.Stat(sessionMetaPath); err != nil {
		t.Fatalf("expected temp metadata file to exist before delete: %v", err)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, sessionPath, nil)
	deleteRequest.Header.Set("Tus-Resumable", "1.0.0")
	deleteRequest.AddCookie(cookie)
	handler.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected delete session 204, got %d: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	if _, err := os.Stat(sessionDataPath); !os.IsNotExist(err) {
		t.Fatalf("expected temp data file removed, err=%v", err)
	}
	if _, err := os.Stat(sessionMetaPath); !os.IsNotExist(err) {
		t.Fatalf("expected temp metadata file removed, err=%v", err)
	}

	headRecorder := httptest.NewRecorder()
	headRequest := httptest.NewRequest(http.MethodHead, sessionPath, nil)
	headRequest.Header.Set("Tus-Resumable", "1.0.0")
	headRequest.AddCookie(cookie)
	handler.ServeHTTP(headRecorder, headRequest)
	if headRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted session head 404, got %d", headRecorder.Code)
	}

	libraryFiles := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	assertMissingFile(t, libraryFiles.Items, "delete-tus.txt")
}

func TestDirectoryUploadPreservesStructureAndHandlesRootConflict(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	uploadBody, uploadContentType := multipartFilesBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, []multipartFileSpec{
		{
			FieldName:    "files",
			FileName:     "guide.txt",
			Contents:     []byte("guide body"),
			RelativePath: "目录包/guide.txt",
		},
		{
			FieldName:    "files",
			FileName:     "answer.txt",
			Contents:     []byte("answer body"),
			RelativePath: "目录包/作业/answer.txt",
		},
	})
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadContentType)
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected directory upload 201, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	rootFiles := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	findFileByName(t, rootFiles.Items, "目录包")

	rootNested := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
		"path":  {"/目录包"},
	})
	findFileByName(t, rootNested.Items, "guide.txt")
	findFileByName(t, rootNested.Items, "作业")

	secondBody, secondContentType := multipartFilesBody(t, map[string]string{
		"space":        "library",
		"parentPath":   "/",
		"conflictMode": "rename",
	}, []multipartFileSpec{
		{
			FieldName:    "files",
			FileName:     "guide.txt",
			Contents:     []byte("second guide"),
			RelativePath: "目录包/guide.txt",
		},
	})
	secondRecorder := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", secondBody)
	secondRequest.Header.Set("Content-Type", secondContentType)
	secondRequest.AddCookie(cookie)
	handler.ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusCreated {
		t.Fatalf("expected renamed directory upload 201, got %d: %s", secondRecorder.Code, secondRecorder.Body.String())
	}

	libraryAfterRename := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	findFileByName(t, libraryAfterRename.Items, "目录包")
	findFileByName(t, libraryAfterRename.Items, "目录包 (1)")

	renamedNested := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
		"path":  {"/目录包 (1)"},
	})
	renamedGuide := findFileByName(t, renamedNested.Items, "guide.txt")
	renamedDownloadRecorder := httptest.NewRecorder()
	renamedDownloadRequest := httptest.NewRequest(http.MethodGet, renamedGuide.DownloadURL, nil)
	renamedDownloadRequest.AddCookie(cookie)
	handler.ServeHTTP(renamedDownloadRecorder, renamedDownloadRequest)
	if renamedDownloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected renamed directory file download 200, got %d: %s", renamedDownloadRecorder.Code, renamedDownloadRecorder.Body.String())
	}
	if !strings.Contains(renamedDownloadRecorder.Body.String(), "second guide") {
		t.Fatalf("expected renamed directory file content, got %s", renamedDownloadRecorder.Body.String())
	}
}

func TestFilesMoveArchiveAndSearchFlow(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	initialLibrary := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	seededFile := findFileByName(t, initialLibrary.Items, "教学安排.txt")
	if strings.TrimSpace(seededFile.UpdatedAt) == "" {
		t.Fatalf("expected seeded file updatedAt to be populated")
	}
	if _, err := time.Parse(time.RFC3339, seededFile.UpdatedAt); err != nil {
		t.Fatalf("expected seeded file updatedAt to be RFC3339, got %q (%v)", seededFile.UpdatedAt, err)
	}

	moveFileBody, moveFileContentType := multipartFileBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, "files", "move-me.txt", []byte("move file body"))
	moveFileUploadRecorder := httptest.NewRecorder()
	moveFileUploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", moveFileBody)
	moveFileUploadRequest.Header.Set("Content-Type", moveFileContentType)
	moveFileUploadRequest.AddCookie(cookie)
	handler.ServeHTTP(moveFileUploadRecorder, moveFileUploadRequest)
	if moveFileUploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected move file upload 201, got %d: %s", moveFileUploadRecorder.Code, moveFileUploadRecorder.Body.String())
	}

	afterFileUpload := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	moveSourceFile := findFileByName(t, afterFileUpload.Items, "move-me.txt")

	moveFileRecorder := httptest.NewRecorder()
	moveFileRequest := httptest.NewRequest(http.MethodPost, "/api/files/move", jsonBody(t, map[string]any{
		"entryId":               moveSourceFile.ID,
		"destinationSpace":      "public",
		"destinationParentPath": "/",
	}))
	moveFileRequest.Header.Set("Content-Type", "application/json")
	moveFileRequest.AddCookie(cookie)
	handler.ServeHTTP(moveFileRecorder, moveFileRequest)
	if moveFileRecorder.Code != http.StatusOK {
		t.Fatalf("expected move file 200, got %d: %s", moveFileRecorder.Code, moveFileRecorder.Body.String())
	}

	var movedFile fileSummary
	decodeJSON(t, moveFileRecorder.Body, &movedFile)
	if strings.TrimSpace(movedFile.UpdatedAt) == "" {
		t.Fatalf("expected moved file updatedAt to be populated")
	}

	libraryAfterMove := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	assertMissingFile(t, libraryAfterMove.Items, "move-me.txt")

	publicAfterMove := listFiles(t, handler, cookie, url.Values{
		"space": {"public"},
	})
	movedFile = findFileByName(t, publicAfterMove.Items, "move-me.txt")

	createRootRecorder := httptest.NewRecorder()
	createRootRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "class",
		"classId":    1,
		"parentPath": "/",
		"name":       "待移动目录",
	}))
	createRootRequest.Header.Set("Content-Type", "application/json")
	createRootRequest.AddCookie(cookie)
	handler.ServeHTTP(createRootRecorder, createRootRequest)
	if createRootRecorder.Code != http.StatusCreated {
		t.Fatalf("expected class root folder 201, got %d: %s", createRootRecorder.Code, createRootRecorder.Body.String())
	}

	createChildRecorder := httptest.NewRecorder()
	createChildRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "class",
		"classId":    1,
		"parentPath": "/待移动目录",
		"name":       "子目录",
	}))
	createChildRequest.Header.Set("Content-Type", "application/json")
	createChildRequest.AddCookie(cookie)
	handler.ServeHTTP(createChildRecorder, createChildRequest)
	if createChildRecorder.Code != http.StatusCreated {
		t.Fatalf("expected class child folder 201, got %d: %s", createChildRecorder.Code, createChildRecorder.Body.String())
	}

	moveDirBody, moveDirContentType := multipartFileBody(t, map[string]string{
		"space":      "class",
		"classId":    "1",
		"parentPath": "/待移动目录/子目录",
	}, "files", "report.txt", []byte("nested report body"))
	moveDirUploadRecorder := httptest.NewRecorder()
	moveDirUploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", moveDirBody)
	moveDirUploadRequest.Header.Set("Content-Type", moveDirContentType)
	moveDirUploadRequest.AddCookie(cookie)
	handler.ServeHTTP(moveDirUploadRecorder, moveDirUploadRequest)
	if moveDirUploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected nested upload 201, got %d: %s", moveDirUploadRecorder.Code, moveDirUploadRecorder.Body.String())
	}

	classOneRoot := listFiles(t, handler, cookie, url.Values{
		"space":   {"class"},
		"classId": {"1"},
	})
	moveSourceDir := findFileByName(t, classOneRoot.Items, "待移动目录")

	moveDirRecorder := httptest.NewRecorder()
	moveDirRequest := httptest.NewRequest(http.MethodPost, "/api/files/move", jsonBody(t, map[string]any{
		"entryId":               moveSourceDir.ID,
		"destinationSpace":      "class",
		"destinationClassId":    2,
		"destinationParentPath": "/",
	}))
	moveDirRequest.Header.Set("Content-Type", "application/json")
	moveDirRequest.AddCookie(cookie)
	handler.ServeHTTP(moveDirRecorder, moveDirRequest)
	if moveDirRecorder.Code != http.StatusOK {
		t.Fatalf("expected move directory 200, got %d: %s", moveDirRecorder.Code, moveDirRecorder.Body.String())
	}

	var movedDir fileSummary
	decodeJSON(t, moveDirRecorder.Body, &movedDir)

	classOneAfterMove := listFiles(t, handler, cookie, url.Values{
		"space":   {"class"},
		"classId": {"1"},
	})
	assertMissingFile(t, classOneAfterMove.Items, "待移动目录")

	classTwoRoot := listFiles(t, handler, cookie, url.Values{
		"space":   {"class"},
		"classId": {"2"},
	})
	movedDir = findFileByName(t, classTwoRoot.Items, "待移动目录")

	classTwoNested := listFiles(t, handler, cookie, url.Values{
		"space":   {"class"},
		"classId": {"2"},
		"path":    {"/待移动目录/子目录"},
	})
	nestedReport := findFileByName(t, classTwoNested.Items, "report.txt")

	searchRecorder := httptest.NewRecorder()
	searchRequest := httptest.NewRequest(http.MethodGet, "/api/files/search?space=class&classId=2&path=/待移动目录&q=report", nil)
	searchRequest.AddCookie(cookie)
	handler.ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusOK {
		t.Fatalf("expected search 200, got %d: %s", searchRecorder.Code, searchRecorder.Body.String())
	}

	var searchPayload filesResponse
	decodeJSON(t, searchRecorder.Body, &searchPayload)
	if searchPayload.Space != "class" || searchPayload.ClassID != 2 || searchPayload.CurrentPath != "/待移动目录" {
		t.Fatalf("unexpected search payload %#v", searchPayload)
	}
	if len(searchPayload.Items) != 1 {
		t.Fatalf("expected one search result, got %#v", searchPayload.Items)
	}
	if searchPayload.Items[0].Path != "/待移动目录/子目录/report.txt" {
		t.Fatalf("unexpected search result path %#v", searchPayload.Items[0])
	}
	if strings.TrimSpace(searchPayload.Items[0].UpdatedAt) == "" {
		t.Fatalf("expected search result updatedAt to be populated")
	}

	fileArchiveRecorder := httptest.NewRecorder()
	fileArchiveRequest := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(movedFile.ID)+"/archive", nil)
	fileArchiveRequest.AddCookie(cookie)
	handler.ServeHTTP(fileArchiveRecorder, fileArchiveRequest)
	if fileArchiveRecorder.Code != http.StatusOK {
		t.Fatalf("expected file archive 200, got %d: %s", fileArchiveRecorder.Code, fileArchiveRecorder.Body.String())
	}
	if !strings.Contains(fileArchiveRecorder.Header().Get("Content-Disposition"), "move-me.txt") {
		t.Fatalf("expected file archive content disposition to contain filename, got %q", fileArchiveRecorder.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(fileArchiveRecorder.Body.String(), "move file body") {
		t.Fatalf("expected file archive body to contain moved file content")
	}

	dirArchiveRecorder := httptest.NewRecorder()
	dirArchiveRequest := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(movedDir.ID)+"/archive", nil)
	dirArchiveRequest.AddCookie(cookie)
	handler.ServeHTTP(dirArchiveRecorder, dirArchiveRequest)
	if dirArchiveRecorder.Code != http.StatusOK {
		t.Fatalf("expected directory archive 200, got %d: %s", dirArchiveRecorder.Code, dirArchiveRecorder.Body.String())
	}
	if !strings.Contains(dirArchiveRecorder.Header().Get("Content-Type"), "application/zip") {
		t.Fatalf("expected zip content type, got %q", dirArchiveRecorder.Header().Get("Content-Type"))
	}

	archiveEntries := unzipEntries(t, dirArchiveRecorder.Body.Bytes())
	if archiveEntries["待移动目录/子目录/report.txt"] != "nested report body" {
		t.Fatalf("expected zipped nested file, got %#v", archiveEntries)
	}
	if nestedReport.DownloadURL == "" {
		t.Fatalf("expected nested report download url to be populated")
	}
}

func TestFilesBatchArchiveIncludesFilesDirectoriesAndValidatesIDs(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	cookie := loginAndGetCookie(t, app)

	rootFile, err := app.createFile("library", nil, "/", "multi-root.txt", strings.NewReader("root body"))
	if err != nil {
		t.Fatalf("create root file: %v", err)
	}
	rootDir, err := app.createFolder("library", nil, "/", "资料目录")
	if err != nil {
		t.Fatalf("create root directory: %v", err)
	}
	if _, err := app.createFolder("library", nil, "/资料目录", "子目录"); err != nil {
		t.Fatalf("create child directory: %v", err)
	}
	if _, err := app.createFile("library", nil, "/资料目录/子目录", "nested.txt", strings.NewReader("nested body")); err != nil {
		t.Fatalf("create nested file: %v", err)
	}
	sameNameDir, err := app.createFolder("public", nil, "/", "资料目录")
	if err != nil {
		t.Fatalf("create same-name public directory: %v", err)
	}
	if _, err := app.createFile("public", nil, "/资料目录", "公开.txt", strings.NewReader("public body")); err != nil {
		t.Fatalf("create same-name directory file: %v", err)
	}

	params := url.Values{
		"entryIds": {strings.Join([]string{itoa(rootFile.ID), itoa(rootDir.ID), itoa(sameNameDir.ID)}, ",")},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/files/batch/archive?"+params.Encode(), nil)
	request.AddCookie(cookie)
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected batch archive 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "application/zip") {
		t.Fatalf("expected batch archive zip content type, got %q", recorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "资料下载.zip") {
		t.Fatalf("expected batch archive filename, got %q", recorder.Header().Get("Content-Disposition"))
	}

	entries := unzipEntries(t, recorder.Body.Bytes())
	if entries["multi-root.txt"] != "root body" {
		t.Fatalf("expected root file in batch archive, got %#v", entries)
	}
	if entries["资料目录/子目录/nested.txt"] != "nested body" {
		t.Fatalf("expected nested directory file in batch archive, got %#v", entries)
	}
	if entries["资料目录 (2)/公开.txt"] != "public body" {
		t.Fatalf("expected same-name root to be de-duplicated in batch archive, got %#v", entries)
	}

	validationFixtures := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{name: "empty IDs", target: "/api/files/batch/archive", wantStatus: http.StatusUnprocessableEntity},
		{name: "invalid ID", target: "/api/files/batch/archive?entryIds=bad", wantStatus: http.StatusUnprocessableEntity},
		{name: "missing ID", target: "/api/files/batch/archive?entryIds=999999", wantStatus: http.StatusNotFound},
	}
	for _, fixture := range validationFixtures {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, fixture.target, nil)
		request.AddCookie(cookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != fixture.wantStatus {
			t.Fatalf("%s: expected status %d, got %d: %s", fixture.name, fixture.wantStatus, recorder.Code, recorder.Body.String())
		}
	}
}

func TestFileMoveValidationAndConflict(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	sourceBody, sourceContentType := multipartFileBody(t, map[string]string{
		"space":      "library",
		"parentPath": "/",
	}, "files", "冲突文件.txt", []byte("source"))
	sourceUploadRecorder := httptest.NewRecorder()
	sourceUploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", sourceBody)
	sourceUploadRequest.Header.Set("Content-Type", sourceContentType)
	sourceUploadRequest.AddCookie(cookie)
	handler.ServeHTTP(sourceUploadRecorder, sourceUploadRequest)
	if sourceUploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected source upload 201, got %d: %s", sourceUploadRecorder.Code, sourceUploadRecorder.Body.String())
	}

	targetBody, targetContentType := multipartFileBody(t, map[string]string{
		"space":      "public",
		"parentPath": "/",
	}, "files", "冲突文件.txt", []byte("target"))
	targetUploadRecorder := httptest.NewRecorder()
	targetUploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", targetBody)
	targetUploadRequest.Header.Set("Content-Type", targetContentType)
	targetUploadRequest.AddCookie(cookie)
	handler.ServeHTTP(targetUploadRecorder, targetUploadRequest)
	if targetUploadRecorder.Code != http.StatusCreated {
		t.Fatalf("expected target upload 201, got %d: %s", targetUploadRecorder.Code, targetUploadRecorder.Body.String())
	}

	libraryFiles := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	conflictSource := findFileByName(t, libraryFiles.Items, "冲突文件.txt")

	conflictRecorder := httptest.NewRecorder()
	conflictRequest := httptest.NewRequest(http.MethodPost, "/api/files/move", jsonBody(t, map[string]any{
		"entryId":               conflictSource.ID,
		"destinationSpace":      "public",
		"destinationParentPath": "/",
	}))
	conflictRequest.Header.Set("Content-Type", "application/json")
	conflictRequest.AddCookie(cookie)
	handler.ServeHTTP(conflictRecorder, conflictRequest)
	if conflictRecorder.Code != http.StatusConflict {
		t.Fatalf("expected move conflict 409, got %d: %s", conflictRecorder.Code, conflictRecorder.Body.String())
	}

	var conflictPayload apiErrorResponse
	decodeJSON(t, conflictRecorder.Body, &conflictPayload)
	if conflictPayload.Error.Code != "conflict" {
		t.Fatalf("expected conflict error code, got %#v", conflictPayload.Error)
	}

	createRootRecorder := httptest.NewRecorder()
	createRootRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "父目录",
	}))
	createRootRequest.Header.Set("Content-Type", "application/json")
	createRootRequest.AddCookie(cookie)
	handler.ServeHTTP(createRootRecorder, createRootRequest)
	if createRootRecorder.Code != http.StatusCreated {
		t.Fatalf("expected root dir 201, got %d: %s", createRootRecorder.Code, createRootRecorder.Body.String())
	}

	createChildRecorder := httptest.NewRecorder()
	createChildRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/父目录",
		"name":       "子目录",
	}))
	createChildRequest.Header.Set("Content-Type", "application/json")
	createChildRequest.AddCookie(cookie)
	handler.ServeHTTP(createChildRecorder, createChildRequest)
	if createChildRecorder.Code != http.StatusCreated {
		t.Fatalf("expected child dir 201, got %d: %s", createChildRecorder.Code, createChildRecorder.Body.String())
	}

	libraryAfterFolders := listFiles(t, handler, cookie, url.Values{
		"space": {"library"},
	})
	parentDir := findFileByName(t, libraryAfterFolders.Items, "父目录")

	invalidRecorder := httptest.NewRecorder()
	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/files/move", jsonBody(t, map[string]any{
		"entryId":               parentDir.ID,
		"destinationSpace":      "library",
		"destinationParentPath": "/父目录/子目录",
	}))
	invalidRequest.Header.Set("Content-Type", "application/json")
	invalidRequest.AddCookie(cookie)
	handler.ServeHTTP(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid move 422, got %d: %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}

	var invalidPayload apiErrorResponse
	decodeJSON(t, invalidRecorder.Body, &invalidPayload)
	if invalidPayload.Error.Code != "invalid_request" {
		t.Fatalf("expected invalid_request code, got %#v", invalidPayload.Error)
	}
}

func TestFileConflictReturnsStructuredError(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	request := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "课程资料",
	}))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected conflict status 409, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var payload apiErrorResponse
	decodeJSON(t, recorder.Body, &payload)
	if payload.Error.Code != "conflict" {
		t.Fatalf("expected conflict error code, got %#v", payload.Error)
	}
}

func TestServerTestSourceDoesNotKeepLegacyShareCommentBlock(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("server_test.go")
	if err != nil {
		t.Fatalf("read server_test.go: %v", err)
	}

	legacyMarkers := []string{
		"func " + "legacy" + "TestFileSharesCreateListDeleteAndStudentVisibility",
		"func " + "legacy" + "TestFileShareSupportsDirectoryAndExpiredVisibilityAndUnauthorizedAccess",
		"func " + "legacy" + "TestPublicShareCreateListAndAnonymousAccess",
		"func " + "legacy" + "TestPublicShareExpiredAndRevokedBecomeUnavailable",
		"func " + "legacy" + "TestPublicShareDisabledInSystemSettingsBlocksCreateListAndAccess",
	}
	for _, marker := range legacyMarkers {
		if bytes.Contains(data, []byte(marker)) {
			t.Fatalf("expected retired route comment block removed, found marker %q", marker)
		}
	}
}

func TestLegacyShareEndpointsAreUnavailable(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)
	prepareStudentAuthFixture(t, app, "LEGACY01", "20260101", "旧入口学生")
	studentCookie := activateAndLoginStudent(t, app, "LEGACY01", "20260101", "student123")

	entry, err := app.createFile("library", nil, "/", "retired-route.txt", strings.NewReader("retired route body"))
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	fileShareRecorder := httptest.NewRecorder()
	fileShareRequest := httptest.NewRequest(http.MethodPost, "/api/files/"+itoa(entry.ID)+"/shares", jsonBody(t, map[string]any{
		"targetType": "class",
		"classId":    1,
	}))
	fileShareRequest.Header.Set("Content-Type", "application/json")
	fileShareRequest.AddCookie(teacherCookie)
	app.ServeHTTP(fileShareRecorder, fileShareRequest)
	if fileShareRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected legacy file share endpoint 405, got %d: %s", fileShareRecorder.Code, fileShareRecorder.Body.String())
	}

	publicShareRecorder := httptest.NewRecorder()
	publicShareRequest := httptest.NewRequest(http.MethodGet, "/api/public-shares?fileId="+itoa(entry.ID), nil)
	publicShareRequest.AddCookie(teacherCookie)
	app.ServeHTTP(publicShareRecorder, publicShareRequest)
	if publicShareRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected legacy public share endpoint 404, got %d: %s", publicShareRecorder.Code, publicShareRecorder.Body.String())
	}

	studentShareRecorder := httptest.NewRecorder()
	studentShareRequest := httptest.NewRequest(http.MethodGet, "/api/student/files/shared", nil)
	studentShareRequest.AddCookie(studentCookie)
	app.ServeHTTP(studentShareRecorder, studentShareRequest)
	if studentShareRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected legacy student shared files endpoint 404, got %d: %s", studentShareRecorder.Code, studentShareRecorder.Body.String())
	}
}

func TestFrontendShareRouteReturnsNotFound(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/share/example-token", nil)
	app.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected retired route 404, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func newTestServer(t *testing.T) (string, *App) {
	t.Helper()
	baseDir, app := newStrictCSRFTestServer(t)
	app.disableCSRFForTests = true
	return baseDir, app
}

func newStrictCSRFTestServer(t *testing.T) (string, *App) {
	t.Helper()
	baseDir := t.TempDir()
	handler, err := New(Config{
		BaseDir: baseDir,
		Seed:    true,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	app, ok := handler.(*App)
	if !ok {
		t.Fatalf("expected *App handler, got %T", handler)
	}
	t.Cleanup(func() {
		if err := app.Close(); err != nil {
			t.Fatalf("close app: %v", err)
		}
	})
	return baseDir, app
}

func loginAndGetCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": "admin",
		"password": "demo123",
	}))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected login cookie")
	}
	return cookies[0]
}

func loginAndGetCookieAs(t *testing.T, handler http.Handler, username, password string) *http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", jsonBody(t, map[string]string{
		"username": username,
		"password": password,
	}))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login %s status = %d, body = %s", username, recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected login cookie for %s", username)
	}
	return cookies[0]
}

func createTeacherAccount(t *testing.T, app *App, username, displayName, password, role string, disabled bool) int64 {
	t.Helper()

	passwordHash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	result, err := app.db.Exec(`insert into teachers (username, display_name, password_hash, role, disabled) values (?, ?, ?, ?, ?)`, username, displayName, passwordHash, role, disabled)
	if err != nil {
		t.Fatalf("create teacher %s: %v", username, err)
	}
	teacherID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("teacher last insert id: %v", err)
	}
	return teacherID
}

func prepareStudentAuthFixture(t *testing.T, app *App, joinCode, studentNo, displayName string) {
	t.Helper()

	if _, err := app.db.Exec(`update classes set join_code = ?, registration_enabled = 1 where id = 1`, joinCode); err != nil {
		t.Fatalf("set class join code: %v", err)
	}

	if _, err := app.createStudent(createStudentRequest{
		ClassID:     1,
		StudentNo:   studentNo,
		DisplayName: displayName,
	}); err != nil {
		t.Fatalf("create test student: %v", err)
	}
}

func studentAuthState(t *testing.T, db *sql.DB, studentID int64) (string, string) {
	t.Helper()

	var passwordHash string
	var activatedAt string
	if err := db.QueryRow(`select password_hash, activated_at from students where id = ?`, studentID).Scan(&passwordHash, &activatedAt); err != nil {
		t.Fatalf("query student auth state: %v", err)
	}
	return passwordHash, activatedAt
}

func activateAndLoginStudent(t *testing.T, handler http.Handler, joinCode, studentNo, password string) *http.Cookie {
	t.Helper()

	activateRecorder := httptest.NewRecorder()
	activateRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/activate", jsonBody(t, map[string]any{
		"joinCode":  joinCode,
		"studentNo": studentNo,
		"password":  password,
	}))
	activateRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(activateRecorder, activateRequest)
	if activateRecorder.Code != http.StatusOK {
		t.Fatalf("activate status = %d, body = %s", activateRecorder.Code, activateRecorder.Body.String())
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]any{
		"studentNo": studentNo,
		"password":  password,
	}))
	loginRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("student login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	cookies := loginRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected student login cookie")
	}
	return cookies[0]
}

func studentLoginAndGetCookie(t *testing.T, handler http.Handler, studentNo, password string) *http.Cookie {
	t.Helper()

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/student/auth/login", jsonBody(t, map[string]any{
		"studentNo": studentNo,
		"password":  password,
	}))
	loginRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("student login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	cookies := loginRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected student login cookie")
	}
	return cookies[0]
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("expected cookie %q, got %#v", name, cookies)
	return nil
}

func listFiles(t *testing.T, handler http.Handler, cookie *http.Cookie, query url.Values) filesResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/files?"+query.Encode(), nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list files 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response filesResponse
	decodeJSON(t, recorder.Body, &response)
	return response
}

func searchFiles(t *testing.T, handler http.Handler, cookie *http.Cookie, query url.Values) filesResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/files/search?"+query.Encode(), nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected search files 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response filesResponse
	decodeJSON(t, recorder.Body, &response)
	return response
}

func listClasses(t *testing.T, handler http.Handler, cookie *http.Cookie) classesResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/classes", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list classes 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response classesResponse
	decodeJSON(t, recorder.Body, &response)
	return response
}

func findClassByID(t *testing.T, classes []classSummary, classID int64) classSummary {
	t.Helper()
	for _, item := range classes {
		if item.ID == classID {
			return item
		}
	}
	t.Fatalf("class %d not found in %#v", classID, classes)
	return classSummary{}
}

func findFileByName(t *testing.T, files []fileSummary, name string) fileSummary {
	t.Helper()
	for _, item := range files {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("file %q not found in %#v", name, files)
	return fileSummary{}
}

func assertMissingFile(t *testing.T, files []fileSummary, name string) {
	t.Helper()
	for _, item := range files {
		if item.Name == name {
			t.Fatalf("did not expect file %q in %#v", name, files)
		}
	}
}

func mustWriteField(t *testing.T, writer *multipart.Writer, key, value string) {
	t.Helper()
	if err := writer.WriteField(key, value); err != nil {
		t.Fatalf("write field %s: %v", key, err)
	}
}

func multipartFileBody(t *testing.T, fields map[string]string, fieldName, fileName string, contents []byte) (*bytes.Buffer, string) {
	t.Helper()

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for key, value := range fields {
		mustWriteField(t, writer, key, value)
	}

	fileWriter, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fileWriter.Write(contents); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return &buffer, writer.FormDataContentType()
}

type multipartFileSpec struct {
	FieldName    string
	FileName     string
	Contents     []byte
	RelativePath string
}

func multipartFilesBody(t *testing.T, fields map[string]string, files []multipartFileSpec) (*bytes.Buffer, string) {
	t.Helper()

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for key, value := range fields {
		mustWriteField(t, writer, key, value)
	}

	for _, file := range files {
		fileWriter, err := writer.CreateFormFile(file.FieldName, file.FileName)
		if err != nil {
			t.Fatalf("create form file %s: %v", file.FileName, err)
		}
		if _, err := fileWriter.Write(file.Contents); err != nil {
			t.Fatalf("write form file %s: %v", file.FileName, err)
		}
		mustWriteField(t, writer, "relativePaths", file.RelativePath)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return &buffer, writer.FormDataContentType()
}

func tusMetadataHeader(values map[string]string) string {
	parts := make([]string, 0, len(values))
	for key, value := range values {
		parts = append(parts, key+" "+base64.StdEncoding.EncodeToString([]byte(value)))
	}
	return strings.Join(parts, ",")
}

func tusSessionArtifacts(baseDir, sessionID string) (string, string) {
	sessionDir := filepath.Join(baseDir, "var", "storage", ".upload-sessions")
	return filepath.Join(sessionDir, sessionID+".bin"), filepath.Join(sessionDir, sessionID+".json")
}

func buildTestStudentImportWorkbook(t *testing.T, rows [][]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, item := range []struct {
		name string
		body string
	}{
		{
			name: "[Content_Types].xml",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`,
		},
		{
			name: "_rels/.rels",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		},
		{
			name: "xl/workbook.xml",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`,
		},
		{
			name: "xl/_rels/workbook.xml.rels",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
		},
		{
			name: "xl/worksheets/sheet1.xml",
			body: buildTestInlineWorksheetXML(rows),
		},
	} {
		fileWriter, err := archive.Create(item.name)
		if err != nil {
			t.Fatalf("create xlsx entry %s: %v", item.name, err)
		}
		if _, err := io.WriteString(fileWriter, item.body); err != nil {
			t.Fatalf("write xlsx entry %s: %v", item.name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close xlsx archive: %v", err)
	}

	return buffer.Bytes()
}

func buildTestInlineWorksheetXML(rows [][]string) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for rowIndex, row := range rows {
		builder.WriteString(fmt.Sprintf(`<row r="%d">`, rowIndex+1))
		for columnIndex, cell := range row {
			builder.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>`, testSpreadsheetColumnName(columnIndex), rowIndex+1, xmlEscape(cell)))
		}
		builder.WriteString(`</row>`)
	}
	builder.WriteString(`</sheetData></worksheet>`)
	return builder.String()
}

func testSpreadsheetColumnName(index int) string {
	value := index + 1
	var result string
	for value > 0 {
		value--
		result = string(rune('A'+(value%26))) + result
		value /= 26
	}
	return result
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func unzipEntries(t *testing.T, contents []byte) map[string]string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		t.Fatalf("open zip reader: %v", err)
	}

	items := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatalf("open zip file %s: %v", file.Name, err)
		}
		body, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			t.Fatalf("read zip file %s: %v", file.Name, err)
		}
		items[file.Name] = string(body)
	}

	return items
}

func decodeJSON(t *testing.T, reader io.Reader, target any) {
	t.Helper()
	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

func jsonBody(t *testing.T, payload any) io.Reader {
	t.Helper()
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(payload); err != nil {
		t.Fatalf("encode json: %v", err)
	}
	return &buffer
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func countAssignmentFileEntries(t *testing.T, db *sql.DB, assignmentID, classID int64) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`
select count(*)
from file_entries
where space = 'assignment'
  and class_id = ?
  and (parent_path = ? or item_path like ?)`,
		classID,
		"/"+itoa(assignmentID),
		"/"+itoa(assignmentID)+"/%",
	).Scan(&count); err != nil {
		t.Fatalf("count assignment file entries: %v", err)
	}
	return count
}

func countAssignmentsByID(t *testing.T, db *sql.DB, assignmentID, classID int64) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`select count(*) from assignments where id = ? and class_id = ?`, assignmentID, classID).Scan(&count); err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	return count
}

func countSubmissionFileEntries(t *testing.T, db *sql.DB, submissionID, classID int64) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`
select count(*)
from file_entries
where space = 'submission'
  and class_id = ?
  and (parent_path = ? or item_path like ?)`,
		classID,
		"/"+itoa(submissionID),
		"/"+itoa(submissionID)+"/%",
	).Scan(&count); err != nil {
		t.Fatalf("count submission file entries: %v", err)
	}
	return count
}

func countSubmissionsByAssignmentAndStudent(t *testing.T, db *sql.DB, assignmentID, studentID int64) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`select count(*) from assignment_submissions where assignment_id = ? and student_id = ?`, assignmentID, studentID).Scan(&count); err != nil {
		t.Fatalf("count assignment submissions: %v", err)
	}
	return count
}
