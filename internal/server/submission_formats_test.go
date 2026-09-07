package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStudentSubmissionAcceptsHtmlAndMarkdownWithSafePreview(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "AIGC 作品作业",
		Description: "提交 AI 生成的网页或 Markdown",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")

	submit := func(name, content string) studentSubmissionResponse {
		body, contentType := multipartFileBody(t, map[string]string{}, "files", name, []byte(content))
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/student/assignments/"+itoa(assignment.ID)+"/submission", body)
		request.Header.Set("Content-Type", contentType)
		request.AddCookie(studentCookie)
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected submission of %s 200, got %d: %s", name, recorder.Code, recorder.Body.String())
		}
		var payload studentSubmissionResponse
		decodeJSON(t, recorder.Body, &payload)
		return payload
	}

	htmlPayload := submit("index.html", "<!doctype html><html><body><h1>AI 作品</h1><script>document.title='xss'</script></body></html>")
	mdPayload := submit("实验报告.md", "# 实验报告\n\n**AIGC 生成**")
	_ = submit("数据.csv", "name,score\n张三,95\n")
	_ = submit("矢量图.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"><script>alert(1)</script><circle r=\"10\"/></svg>")

	var htmlItem, mdItem fileSummary
	for _, item := range append(htmlPayload.Items, mdPayload.Items...) {
		switch item.Name {
		case "index.html":
			htmlItem = item
		case "实验报告.md":
			mdItem = item
		}
	}
	if htmlItem.ID == 0 || mdItem.ID == 0 {
		t.Fatalf("expected html and md items in submission, got %#v", htmlPayload.Items)
	}

	// 学生侧提交文件应带内联预览地址
	wantPreviewURL := "/api/student/assignments/" + itoa(assignment.ID) + "/submission/files/" + itoa(htmlItem.ID) + "/preview"
	if htmlItem.PreviewURL != wantPreviewURL {
		t.Fatalf("expected student submission preview URL %q, got %q", wantPreviewURL, htmlItem.PreviewURL)
	}

	// 学生预览 HTML：内联、CSP 沙箱
	previewRecorder := httptest.NewRecorder()
	previewRequest := httptest.NewRequest(http.MethodGet, htmlItem.PreviewURL, nil)
	previewRequest.AddCookie(studentCookie)
	app.ServeHTTP(previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("expected student html preview 200, got %d: %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	if got := previewRecorder.Header().Get("Content-Disposition"); !strings.Contains(got, "inline") {
		t.Fatalf("expected inline disposition for preview, got %q", got)
	}
	if got := previewRecorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "sandbox") {
		t.Fatalf("expected sandbox CSP for html preview, got %q", got)
	}
	if got := previewRecorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("expected text/html content type, got %q", got)
	}

	// 下载仍为 attachment，保证不以内联方式执行
	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, htmlItem.DownloadURL, nil)
	downloadRequest.AddCookie(studentCookie)
	app.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("expected student html download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	if got := downloadRecorder.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") {
		t.Fatalf("expected attachment disposition for download, got %q", got)
	}

	// 老师侧列表与预览
	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/submissions?classId=1", nil)
	listRequest.AddCookie(teacherCookie)
	app.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher submissions list 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var submissions teacherAssignmentSubmissionsResponse
	decodeJSON(t, listRecorder.Body, &submissions)
	if len(submissions.Submissions) != 1 {
		t.Fatalf("expected one submission, got %#v", submissions)
	}
	var teacherHTMLItem fileSummary
	for _, item := range submissions.Submissions[0].Items {
		if item.Name == "index.html" {
			teacherHTMLItem = item
		}
	}
	if teacherHTMLItem.ID == 0 {
		t.Fatalf("expected teacher-visible index.html, got %#v", submissions.Submissions[0].Items)
	}
	wantTeacherPreviewURL := "/api/assignments/" + itoa(assignment.ID) + "/submissions/files/" + itoa(teacherHTMLItem.ID) + "/preview?classId=1"
	if teacherHTMLItem.PreviewURL != wantTeacherPreviewURL {
		t.Fatalf("expected teacher submission preview URL %q, got %q", wantTeacherPreviewURL, teacherHTMLItem.PreviewURL)
	}
	teacherPreviewRecorder := httptest.NewRecorder()
	teacherPreviewRequest := httptest.NewRequest(http.MethodGet, teacherHTMLItem.PreviewURL, nil)
	teacherPreviewRequest.AddCookie(teacherCookie)
	app.ServeHTTP(teacherPreviewRecorder, teacherPreviewRequest)
	if teacherPreviewRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher html preview 200, got %d: %s", teacherPreviewRecorder.Code, teacherPreviewRecorder.Body.String())
	}
	if got := teacherPreviewRecorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "sandbox") {
		t.Fatalf("expected sandbox CSP for teacher html preview, got %q", got)
	}

	// SVG 预览也应带沙箱 CSP（禁脚本）
	svgItem := fileSummary{}
	for _, item := range submissions.Submissions[0].Items {
		if item.Name == "矢量图.svg" {
			svgItem = item
		}
	}
	if svgItem.ID == 0 {
		t.Fatalf("expected svg item in teacher submission, got %#v", submissions.Submissions[0].Items)
	}
	svgRecorder := httptest.NewRecorder()
	svgRequest := httptest.NewRequest(http.MethodGet, svgItem.PreviewURL, nil)
	svgRequest.AddCookie(teacherCookie)
	app.ServeHTTP(svgRecorder, svgRequest)
	if svgRecorder.Code != http.StatusOK {
		t.Fatalf("expected svg preview 200, got %d: %s", svgRecorder.Code, svgRecorder.Body.String())
	}
	if got := svgRecorder.Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Fatalf("expected strict sandbox CSP for svg preview, got %q", got)
	}
}

func TestAssignmentAttachmentPreviewRoutes(t *testing.T) {
	t.Parallel()

	_, app := newTestServer(t)
	teacherCookie := loginAndGetCookie(t, app)

	prepareStudentAuthFixture(t, app, "ABCD1234", "20260001", "张小明")
	assignment, err := app.createAssignment(createAssignmentRequest{
		ClassID:     1,
		Title:       "网页附件作业",
		DueAt:       time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
		Status:      "published",
	})
	if err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	attachment, err := app.createFile("assignment", optionalInt64(1), assignmentAttachmentParentPath(assignment.ID), "作业说明.html", strings.NewReader("<html><body><p>说明</p></body></html>"))
	if err != nil {
		t.Fatalf("create assignment attachment: %v", err)
	}

	// 老师侧附件列表应带预览地址
	teacherListRecorder := httptest.NewRecorder()
	teacherListRequest := httptest.NewRequest(http.MethodGet, "/api/assignments/"+itoa(assignment.ID)+"/attachments?classId=1", nil)
	teacherListRequest.AddCookie(teacherCookie)
	app.ServeHTTP(teacherListRecorder, teacherListRequest)
	if teacherListRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher attachments list 200, got %d: %s", teacherListRecorder.Code, teacherListRecorder.Body.String())
	}
	var teacherAttachments assignmentAttachmentsResult
	decodeJSON(t, teacherListRecorder.Body, &teacherAttachments)
	if len(teacherAttachments.Items) != 1 {
		t.Fatalf("expected one attachment, got %#v", teacherAttachments)
	}
	wantTeacherPreview := "/api/assignments/" + itoa(assignment.ID) + "/attachments/" + itoa(attachment.ID) + "/preview?classId=1"
	if teacherAttachments.Items[0].PreviewURL != wantTeacherPreview {
		t.Fatalf("expected teacher attachment preview URL %q, got %q", wantTeacherPreview, teacherAttachments.Items[0].PreviewURL)
	}
	teacherPreviewRecorder := httptest.NewRecorder()
	teacherPreviewRequest := httptest.NewRequest(http.MethodGet, wantTeacherPreview, nil)
	teacherPreviewRequest.AddCookie(teacherCookie)
	app.ServeHTTP(teacherPreviewRecorder, teacherPreviewRequest)
	if teacherPreviewRecorder.Code != http.StatusOK {
		t.Fatalf("expected teacher attachment preview 200, got %d: %s", teacherPreviewRecorder.Code, teacherPreviewRecorder.Body.String())
	}
	if got := teacherPreviewRecorder.Header().Get("Content-Security-Policy"); !strings.Contains(got, "sandbox") {
		t.Fatalf("expected sandbox CSP for teacher attachment preview, got %q", got)
	}

	// 学生侧附件列表与预览
	studentCookie := activateAndLoginStudent(t, app, "ABCD1234", "20260001", "student123")
	studentDetailRecorder := httptest.NewRecorder()
	studentDetailRequest := httptest.NewRequest(http.MethodGet, "/api/student/assignments/"+itoa(assignment.ID), nil)
	studentDetailRequest.AddCookie(studentCookie)
	app.ServeHTTP(studentDetailRecorder, studentDetailRequest)
	if studentDetailRecorder.Code != http.StatusOK {
		t.Fatalf("expected student assignment detail 200, got %d: %s", studentDetailRecorder.Code, studentDetailRecorder.Body.String())
	}
	var studentDetail studentAssignmentPayload
	decodeJSON(t, studentDetailRecorder.Body, &studentDetail)
	if len(studentDetail.AssignmentAttachments) != 1 {
		t.Fatalf("expected one student attachment, got %#v", studentDetail.AssignmentAttachments)
	}
	wantStudentPreview := "/api/student/assignments/" + itoa(assignment.ID) + "/attachments/" + itoa(attachment.ID) + "/preview"
	if studentDetail.AssignmentAttachments[0].PreviewURL != wantStudentPreview {
		t.Fatalf("expected student attachment preview URL %q, got %q", wantStudentPreview, studentDetail.AssignmentAttachments[0].PreviewURL)
	}
	studentPreviewRecorder := httptest.NewRecorder()
	studentPreviewRequest := httptest.NewRequest(http.MethodGet, wantStudentPreview, nil)
	studentPreviewRequest.AddCookie(studentCookie)
	app.ServeHTTP(studentPreviewRecorder, studentPreviewRequest)
	if studentPreviewRecorder.Code != http.StatusOK {
		t.Fatalf("expected student attachment preview 200, got %d: %s", studentPreviewRecorder.Code, studentPreviewRecorder.Body.String())
	}
	if got := studentPreviewRecorder.Header().Get("Content-Disposition"); !strings.Contains(got, "inline") {
		t.Fatalf("expected inline disposition for student attachment preview, got %q", got)
	}
}
