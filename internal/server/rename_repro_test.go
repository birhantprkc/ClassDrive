package server

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestFolderRenameWithDescendants(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createFolderRecorder := httptest.NewRecorder()
	createFolderRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "待重命名文件夹",
	}))
	createFolderRequest.Header.Set("Content-Type", "application/json")
	createFolderRequest.AddCookie(cookie)
	handler.ServeHTTP(createFolderRecorder, createFolderRequest)
	if createFolderRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create folder 201, got %d: %s", createFolderRecorder.Code, createFolderRecorder.Body.String())
	}

	uploadBody := &bytes.Buffer{}
	uploadWriter := multipart.NewWriter(uploadBody)
	mustWriteField(t, uploadWriter, "space", "library")
	mustWriteField(t, uploadWriter, "parentPath", "/待重命名文件夹")
	part, err := uploadWriter.CreateFormFile("files", "inside.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("inside content")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := uploadWriter.Close(); err != nil {
		t.Fatalf("close upload writer: %v", err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", uploadBody)
	uploadRequest.Header.Set("Content-Type", uploadWriter.FormDataContentType())
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), uploadRequest)

	subRecorder := httptest.NewRecorder()
	subRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/待重命名文件夹",
		"name":       "子文件夹",
	}))
	subRequest.Header.Set("Content-Type", "application/json")
	subRequest.AddCookie(cookie)
	handler.ServeHTTP(subRecorder, subRequest)
	if subRecorder.Code != http.StatusCreated {
		t.Fatalf("expected subfolder 201, got %d: %s", subRecorder.Code, subRecorder.Body.String())
	}

	folderList := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	folder := findFileByName(t, folderList.Items, "待重命名文件夹")
	if folder.Kind != "dir" {
		t.Fatalf("expected dir entry, got %#v", folder)
	}

	renameRecorder := httptest.NewRecorder()
	renameRequest := httptest.NewRequest(http.MethodPatch, "/api/files/"+itoa(folder.ID), jsonBody(t, map[string]string{
		"name": "已重命名文件夹",
	}))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.AddCookie(cookie)
	handler.ServeHTTP(renameRecorder, renameRequest)
	if renameRecorder.Code != http.StatusOK {
		t.Fatalf("expected folder rename 200, got %d: %s", renameRecorder.Code, renameRecorder.Body.String())
	}

	afterRename := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	findFileByName(t, afterRename.Items, "已重命名文件夹")
	nested := listFiles(t, handler, cookie, url.Values{"space": {"library"}, "path": {"/已重命名文件夹"}})
	childDir := findFileByName(t, nested.Items, "子文件夹")
	if childDir.Path != "/已重命名文件夹/子文件夹" {
		t.Fatalf("expected nested dir path to be updated, got %q", childDir.Path)
	}
	childFile := findFileByName(t, nested.Items, "inside.txt")

	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, childFile.DownloadURL, nil)
	downloadRequest.AddCookie(cookie)
	handler.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK || !bytes.Contains(downloadRecorder.Body.Bytes(), []byte("inside content")) {
		t.Fatalf("expected renamed child file download 200 with original content, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
}

func TestFolderRenameWithSpecialCharacters(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createFolder := func(parent, name string) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
			"space":      "library",
			"parentPath": parent,
			"name":       name,
		}))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected create folder %q 201, got %d: %s", name, recorder.Code, recorder.Body.String())
		}
	}
	uploadFile := func(parent, name string) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		mustWriteField(t, writer, "space", "library")
		mustWriteField(t, writer, "parentPath", parent)
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write([]byte("content-" + name)); err != nil {
			t.Fatalf("write form file: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close upload writer: %v", err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.AddCookie(cookie)
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	rename := func(id int64, name string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPatch, "/api/files/"+itoa(id), jsonBody(t, map[string]string{"name": name}))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	// 名称含 % 与 _ 的文件夹，重命名不能误伤兄弟目录（LIKE 通配符转义）
	createFolder("/", "进度100%")
	createFolder("/", "进度100X")
	uploadFile("/进度100%", "a.txt")
	uploadFile("/进度100X", "b.txt")

	rootItems := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	percentFolder := findFileByName(t, rootItems.Items, "进度100%")
	recorder := rename(percentFolder.ID, "进度200%")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected rename with %% 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	renamed := listFiles(t, handler, cookie, url.Values{"space": {"library"}, "path": {"/进度200%"}})
	findFileByName(t, renamed.Items, "a.txt")
	// 兄弟目录不受影响
	sibling := listFiles(t, handler, cookie, url.Values{"space": {"library"}, "path": {"/进度100X"}})
	findFileByName(t, sibling.Items, "b.txt")
}

func TestFolderRenameRecreatesMissingPhysicalDirectory(t *testing.T) {
	t.Parallel()

	baseDir, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "物理目录缺失",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create folder 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	rootItems := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	folder := findFileByName(t, rootItems.Items, "物理目录缺失")

	// 模拟历史数据：物理目录不存在，只有数据库记录
	physicalPath := filepath.Join(baseDir, "var", "storage", "library", "物理目录缺失")
	if err := os.RemoveAll(physicalPath); err != nil {
		t.Fatalf("remove physical dir: %v", err)
	}
	if _, err := os.Lstat(physicalPath); !os.IsNotExist(err) {
		t.Fatalf("expected physical dir missing, got %v", err)
	}

	renameRecorder := httptest.NewRecorder()
	renameRequest := httptest.NewRequest(http.MethodPatch, "/api/files/"+itoa(folder.ID), jsonBody(t, map[string]string{"name": "物理目录已恢复"}))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.AddCookie(cookie)
	handler.ServeHTTP(renameRecorder, renameRequest)
	if renameRecorder.Code != http.StatusOK {
		t.Fatalf("expected rename 200 despite missing physical dir, got %d: %s", renameRecorder.Code, renameRecorder.Body.String())
	}

	afterRename := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	findFileByName(t, afterRename.Items, "物理目录已恢复")
}

func TestFolderRenameAdoptsDiskStateWhenSourceMissing(t *testing.T) {
	t.Parallel()

	baseDir, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	// 建文件夹并放入子文件
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "残留文件夹",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create folder 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	mustWriteField(t, writer, "space", "library")
	mustWriteField(t, writer, "parentPath", "/残留文件夹")
	part, err := writer.CreateFormFile("files", "资料.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("residue content")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close upload writer: %v", err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), uploadRequest)

	rootItems := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	folder := findFileByName(t, rootItems.Items, "残留文件夹")

	// 模拟旧版本 bug 残留：磁盘上目录已被改名，但数据库仍是旧名
	oldPhysical := filepath.Join(baseDir, "var", "storage", "library", "残留文件夹")
	newPhysical := filepath.Join(baseDir, "var", "storage", "library", "残留文件夹-已改")
	if err := os.Rename(oldPhysical, newPhysical); err != nil {
		t.Fatalf("simulate disk rename: %v", err)
	}

	// 重命名为与磁盘一致的名字：应直接采用磁盘现状并成功
	renameRecorder := httptest.NewRecorder()
	renameRequest := httptest.NewRequest(http.MethodPatch, "/api/files/"+itoa(folder.ID), jsonBody(t, map[string]string{"name": "残留文件夹-已改"}))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.AddCookie(cookie)
	handler.ServeHTTP(renameRecorder, renameRequest)
	if renameRecorder.Code != http.StatusOK {
		t.Fatalf("expected rename 200 adopting disk state, got %d: %s", renameRecorder.Code, renameRecorder.Body.String())
	}

	afterRename := listFiles(t, handler, cookie, url.Values{"space": {"library"}, "path": {"/残留文件夹-已改"}})
	child := findFileByName(t, afterRename.Items, "资料.txt")
	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, child.DownloadURL, nil)
	downloadRequest.AddCookie(cookie)
	handler.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK || !bytes.Contains(downloadRecorder.Body.Bytes(), []byte("residue content")) {
		t.Fatalf("expected adopted child download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}

	// 旧名位置不应出现空目录残留
	rootAfter := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	assertMissingFile(t, rootAfter.Items, "残留文件夹")
}

func TestFolderRenameFallsBackToCopyWhenDiskRenameBlocked(t *testing.T) {
	// 注意：本用例不 t.Parallel()——它临时替换全局 renameWithRetryFunc，
	// 需要排在其他并行用例之外运行。
	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "library",
		"parentPath": "/",
		"name":       "被占用文件夹",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create folder 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	mustWriteField(t, writer, "space", "library")
	mustWriteField(t, writer, "parentPath", "/被占用文件夹")
	part, err := writer.CreateFormFile("files", "占用内文件.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("locked folder content")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close upload writer: %v", err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), uploadRequest)

	rootItems := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	folder := findFileByName(t, rootItems.Items, "被占用文件夹")

	// 模拟 Windows 目录被外部程序占用：直接改名始终 Access is denied
	originalRename := renameWithRetryFunc
	renameWithRetryFunc = func(oldPath, newPath string) error {
		return errors.New("Access is denied.")
	}
	defer func() { renameWithRetryFunc = originalRename }()

	renameRecorder := httptest.NewRecorder()
	renameRequest := httptest.NewRequest(http.MethodPatch, "/api/files/"+itoa(folder.ID), jsonBody(t, map[string]string{"name": "被占用文件夹-已改名"}))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.AddCookie(cookie)
	handler.ServeHTTP(renameRecorder, renameRequest)
	if renameRecorder.Code != http.StatusOK {
		t.Fatalf("expected rename 200 via copy fallback, got %d: %s", renameRecorder.Code, renameRecorder.Body.String())
	}

	// 复制通道后：新位置有完整数据、旧名字不再出现在列表
	nested := listFiles(t, handler, cookie, url.Values{"space": {"library"}, "path": {"/被占用文件夹-已改名"}})
	child := findFileByName(t, nested.Items, "占用内文件.txt")
	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, child.DownloadURL, nil)
	downloadRequest.AddCookie(cookie)
	handler.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK || !bytes.Contains(downloadRecorder.Body.Bytes(), []byte("locked folder content")) {
		t.Fatalf("expected copied child download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	rootAfter := listFiles(t, handler, cookie, url.Values{"space": {"library"}})
	assertMissingFile(t, rootAfter.Items, "被占用文件夹")
}

func TestFolderRenameInClassSpace(t *testing.T) {
	t.Parallel()

	_, handler := newTestServer(t)
	cookie := loginAndGetCookie(t, handler)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/files/folder", jsonBody(t, map[string]any{
		"space":      "class",
		"classId":    1,
		"parentPath": "/",
		"name":       "班级文件夹",
	}))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create class folder 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	mustWriteField(t, writer, "space", "class")
	mustWriteField(t, writer, "classId", "1")
	mustWriteField(t, writer, "parentPath", "/班级文件夹")
	part, err := writer.CreateFormFile("files", "班内文件.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("class child")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close upload writer: %v", err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRequest.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), uploadRequest)

	classItems := listFiles(t, handler, cookie, url.Values{"space": {"class"}, "classId": {"1"}})
	folder := findFileByName(t, classItems.Items, "班级文件夹")

	renameRecorder := httptest.NewRecorder()
	renameRequest := httptest.NewRequest(http.MethodPatch, "/api/files/"+itoa(folder.ID), jsonBody(t, map[string]string{"name": "班级文件夹-已改名"}))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.AddCookie(cookie)
	handler.ServeHTTP(renameRecorder, renameRequest)
	if renameRecorder.Code != http.StatusOK {
		t.Fatalf("expected class folder rename 200, got %d: %s", renameRecorder.Code, renameRecorder.Body.String())
	}

	nested := listFiles(t, handler, cookie, url.Values{"space": {"class"}, "classId": {"1"}, "path": {"/班级文件夹-已改名"}})
	child := findFileByName(t, nested.Items, "班内文件.txt")
	downloadRecorder := httptest.NewRecorder()
	downloadRequest := httptest.NewRequest(http.MethodGet, child.DownloadURL, nil)
	downloadRequest.AddCookie(cookie)
	handler.ServeHTTP(downloadRecorder, downloadRequest)
	if downloadRecorder.Code != http.StatusOK || !bytes.Contains(downloadRecorder.Body.Bytes(), []byte("class child")) {
		t.Fatalf("expected class child download 200, got %d: %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
}
