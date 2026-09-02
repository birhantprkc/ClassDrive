package server

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const (
	sessionCookieName                   = "classdrive_session"
	studentSessionCookieName            = "classdrive_student_session"
	csrfCookieName                      = "classdrive_csrf"
	csrfHeaderName                      = "X-CSRF-Token"
	defaultTeacherPasswordEnv           = "CLASSDRIVE_DEFAULT_TEACHER_PASSWORD"
	defaultStudentResetPasswordEnv      = "CLASSDRIVE_DEFAULT_STUDENT_RESET_PASSWORD"
	serverErrorMessage                  = "服务器内部错误"
	sessionDuration                     = 24 * time.Hour
	DefaultTeacherUsername              = "admin"
	DefaultTeacherPassword              = "demo123"
	DefaultStudentResetPassword         = "123456"
	classJoinCodeLen                    = 4
	classRegistrationDuration           = 90 * time.Minute
	preferenceRecentCopyTargets         = "recent_copy_targets"
	studentSubmissionMaxFileSize        = 100 * 1024 * 1024
	studentSubmissionMaxFileSizeLabel   = "100 MB"
	assignmentSubmissionModeAny         = "any"
	assignmentSubmissionModeFiles       = "files"
	assignmentSubmissionModeFolder      = "folder"
	assignmentSubmissionTypeMixed       = "mixed"
	assignmentSubmissionTypeImage       = "image"
	assignmentSubmissionTypeWord        = "word"
	assignmentSubmissionTypePDF         = "pdf"
	assignmentSubmissionTypeArchive     = "archive"
	assignmentSubmissionStatusPartial   = "partial"
	assignmentSubmissionStatusSubmitted = "submitted"
	assignmentSubmissionReviewPending   = "pending"
	assignmentSubmissionReviewReviewed  = "reviewed"
	defaultServerPort                   = "80"
	tusResumableVersion                 = "1.0.0"
	editableTextFileMaxSize             = 1 * 1024 * 1024
	teacherRoleOwner                    = "owner"
	teacherRoleStaff                    = "staff"
	preferenceTeacherProfileSettings    = "profile_settings"
	defaultTeacherListPageSize          = 30
	maxTeacherListPageSize              = 100
)

func configuredDefaultPassword(envName string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return fallback
}

func EffectiveDefaultTeacherPassword() string {
	return configuredDefaultPassword(defaultTeacherPasswordEnv, DefaultTeacherPassword)
}

func EffectiveDefaultStudentResetPassword() string {
	return configuredDefaultPassword(defaultStudentResetPasswordEnv, DefaultStudentResetPassword)
}

func DefaultTeacherPasswordStartupText() string {
	if strings.TrimSpace(os.Getenv(defaultTeacherPasswordEnv)) != "" {
		return "已通过 " + defaultTeacherPasswordEnv + " 配置"
	}
	return DefaultTeacherPassword
}

var studentSubmissionAllowedExtensions = []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".jpg", ".jpeg", ".png", ".zip", ".rar", ".7z"}
var studentSubmissionTypeExtensions = map[string][]string{
	assignmentSubmissionTypeMixed:   studentSubmissionAllowedExtensions,
	assignmentSubmissionTypeImage:   {".jpg", ".jpeg", ".png"},
	assignmentSubmissionTypeWord:    {".doc", ".docx"},
	assignmentSubmissionTypePDF:     {".pdf"},
	assignmentSubmissionTypeArchive: {".zip", ".rar", ".7z"},
}
var studentSubmissionTypeLabels = map[string]string{
	assignmentSubmissionTypeMixed:   studentSubmissionAllowedTypesLabel,
	assignmentSubmissionTypeImage:   "图片文件（JPG、JPEG、PNG）",
	assignmentSubmissionTypeWord:    "Word 文档（DOC、DOCX）",
	assignmentSubmissionTypePDF:     "PDF 文件",
	assignmentSubmissionTypeArchive: "压缩包（ZIP、RAR、7Z）",
}
var editableTextExtensions = map[string]struct{}{
	".txt":      {},
	".md":       {},
	".markdown": {},
	".json":     {},
	".csv":      {},
	".tsv":      {},
	".yml":      {},
	".yaml":     {},
	".js":       {},
	".ts":       {},
	".tsx":      {},
	".jsx":      {},
	".html":     {},
	".css":      {},
	".vue":      {},
}

const studentSubmissionAllowedTypesLabel = "PDF、Word、Excel、PPT、TXT、JPG、PNG、ZIP"

//go:embed dist
var frontendAssets embed.FS

type Config struct {
	BaseDir                 string
	Seed                    bool
	PrepareServerPortChange func(port string) (PreparedServerPortChange, error)
}

type PreparedServerPortChange struct {
	Commit func()
	Cancel func()
}

type App struct {
	db                      *sql.DB
	mux                     *http.ServeMux
	baseDir                 string
	storageRoot             string
	prepareServerPortChange func(port string) (PreparedServerPortChange, error)
	disableCSRFForTests     bool

	shareAttemptsMu sync.Mutex
	shareAttempts   map[string]*shareAttempt
}

type shareAttempt struct {
	failures  int
	lockUntil time.Time
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type sessionUser struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type studentSessionUser struct {
	ID                 int64  `json:"id"`
	ClassID            int64  `json:"classId"`
	StudentNo          string `json:"studentNo"`
	DisplayName        string `json:"displayName"`
	ClassName          string `json:"className"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

type apiResponse[T any] struct {
	Data T `json:"data,omitempty"`
	User T `json:"user,omitempty"`
}

type apiErrorEnvelope struct {
	Error apiError `json:"error"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type teacherProfilePreferences struct {
	CompactListEnabled bool `json:"compactListEnabled"`
}

type teacherProfileResult struct {
	Profile teacherProfilePayload `json:"profile"`
}

type teacherProfilePayload struct {
	ID          int64                     `json:"id"`
	Username    string                    `json:"username"`
	DisplayName string                    `json:"displayName"`
	Role        string                    `json:"role"`
	Preferences teacherProfilePreferences `json:"preferences"`
}

type updateTeacherProfileRequest struct {
	DisplayName *string                    `json:"displayName"`
	Preferences *teacherProfilePreferences `json:"preferences"`
}

type updateTeacherPasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type systemSettings struct {
	UploadPanelEnabled        bool   `json:"uploadPanelEnabled"`
	SingleAccountLoginEnabled bool   `json:"singleAccountLoginEnabled"`
	ServerPort                string `json:"serverPort"`
	ServerHost                string `json:"serverHost"`
	DefaultShareExpiresDays   int    `json:"defaultShareExpiresDays"`
}

type updateSystemSettingsRequest struct {
	UploadPanelEnabled        bool   `json:"uploadPanelEnabled"`
	SingleAccountLoginEnabled *bool  `json:"singleAccountLoginEnabled"`
	ServerPort                string `json:"serverPort"`
	DefaultShareExpiresDays   *int   `json:"defaultShareExpiresDays"`
}

type systemSettingsResult struct {
	Settings systemSettings `json:"settings"`
}

type loginLogPayload struct {
	ID         int64  `json:"id"`
	OccurredAt string `json:"occurredAt"`
	ActorType  string `json:"actorType"`
	ActorName  string `json:"actorName"`
	Username   string `json:"username"`
	Status     string `json:"status"`
	IPAddress  string `json:"ipAddress"`
	Message    string `json:"message"`
}

type loginLogsResult struct {
	Logs       []loginLogPayload `json:"logs"`
	Pagination paginationPayload `json:"pagination"`
}

type operationLogPayload struct {
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

type operationLogsResult struct {
	Logs       []operationLogPayload `json:"logs"`
	Pagination paginationPayload     `json:"pagination"`
}

type auditLogPayload struct {
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

type auditLogsResult struct {
	Logs       []auditLogPayload `json:"logs"`
	Pagination paginationPayload `json:"pagination"`
}

type clearAuditLogsResult struct {
	DeletedLoginLogs     int64 `json:"deletedLoginLogs"`
	DeletedOperationLogs int64 `json:"deletedOperationLogs"`
}

type auditLogFilters struct {
	LogType   string
	ActorType string
	Result    string
	Method    string
	Query     string
	IP        string
	From      string
	To        string
	Page      int
	PageSize  int
	Offset    int
}

type loginLogFilters struct {
	ActorType string
	Status    string
	Query     string
	IP        string
	From      string
	To        string
	Limit     int
	Page      int
	PageSize  int
	Offset    int
}

type operationLogFilters struct {
	ActorType string
	Method    string
	Query     string
	IP        string
	From      string
	To        string
	Limit     int
	Page      int
	PageSize  int
	Offset    int
}

type teacherUserPayload struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Disabled    bool   `json:"disabled"`
}

type teacherUsersResult struct {
	Teachers []teacherUserPayload `json:"teachers"`
}

type teacherUserResult struct {
	Teacher teacherUserPayload `json:"teacher"`
}

type createTeacherUserRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type updateTeacherUserRequest struct {
	DisplayName *string `json:"displayName"`
	Password    *string `json:"password"`
	Role        *string `json:"role"`
	Disabled    *bool   `json:"disabled"`
}

type studentActivateRequest struct {
	JoinCode  string `json:"joinCode"`
	StudentNo string `json:"studentNo"`
	Password  string `json:"password"`
}

type studentLoginRequest struct {
	StudentNo string `json:"studentNo"`
	Password  string `json:"password"`
}

type studentPasswordChangeRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type loginResult struct {
	User sessionUser `json:"user"`
}

type studentSessionResult struct {
	User studentSessionUser `json:"user"`
}

type studentPasswordResetResult struct {
	Student         studentResult `json:"student"`
	DefaultPassword string        `json:"defaultPassword"`
}

type sessionResponse struct {
	User sessionUser `json:"user"`
}

type shellResult struct {
	User  sessionUser `json:"user"`
	Items []shellItem `json:"items"`
}

type shellItem struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Href        string `json:"href"`
	Placeholder bool   `json:"placeholder"`
}

type classesResult struct {
	Classes    []classSummary    `json:"classes"`
	Pagination paginationPayload `json:"pagination"`
}

type paginationPayload struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type teacherListQuery struct {
	Keyword            string
	Sort               string
	Page               int
	PageSize           int
	Offset             int
	Paged              bool
	RegistrationFilter string
}

type createClassRequest struct {
	Name string `json:"name"`
}

type updateClassRequest struct {
	Name string `json:"name"`
}

type updateClassRegistrationRequest struct {
	Enabled bool `json:"enabled"`
}

type updateClassExamLockRequest struct {
	Enabled bool `json:"enabled"`
}

type classJoinCodeResult struct {
	ClassID               int64  `json:"classId"`
	JoinCode              string `json:"joinCode"`
	JoinCodeHint          string `json:"joinCodeHint"`
	JoinCodeStatus        string `json:"joinCodeStatus"`
	RegistrationEnabled   bool   `json:"registrationEnabled"`
	RegistrationExpiresAt string `json:"registrationExpiresAt"`
}

type fileListResult struct {
	Space       string            `json:"space"`
	ClassID     int64             `json:"classId,omitempty"`
	CurrentPath string            `json:"currentPath"`
	Items       []fileSummary     `json:"items"`
	Pagination  paginationPayload `json:"pagination"`
}

type uploadSummary struct {
	CreatedCount  int `json:"createdCount"`
	ReplacedCount int `json:"replacedCount"`
	RenamedCount  int `json:"renamedCount"`
	SkippedCount  int `json:"skippedCount"`
}

type uploadFilesResult struct {
	Items   []fileSummary `json:"items"`
	Summary uploadSummary `json:"summary"`
}

type fileContentResult struct {
	Item    fileSummary `json:"item"`
	Content string      `json:"content"`
}

type updateFileContentRequest struct {
	Content string `json:"content"`
}

type uploadSession struct {
	ID           string             `json:"id"`
	Space        string             `json:"space"`
	ClassID      *int64             `json:"classId,omitempty"`
	ParentPath   string             `json:"parentPath"`
	Filename     string             `json:"filename"`
	ConflictMode uploadConflictMode `json:"conflictMode"`
	UploadLength int64              `json:"uploadLength"`
	UploadOffset int64              `json:"uploadOffset"`
	CreatedAt    string             `json:"createdAt"`
}

type createFolderRequest struct {
	Space      string `json:"space"`
	ClassID    int64  `json:"classId,omitempty"`
	ParentPath string `json:"parentPath"`
	Name       string `json:"name"`
}

type createFileRequest struct {
	Space      string `json:"space"`
	ClassID    int64  `json:"classId,omitempty"`
	ParentPath string `json:"parentPath"`
	Name       string `json:"name"`
	Content    string `json:"content"`
}

type saveRecentCopyTargetsRequest struct {
	Items []recentCopyTargetPayload `json:"items"`
}

type recentCopyTargetsResult struct {
	Items []recentCopyTargetPayload `json:"items"`
}

type recentCopyTargetPayload struct {
	Space   string `json:"space"`
	ClassID *int64 `json:"classId,omitempty"`
	Path    string `json:"path"`
	Label   string `json:"label"`
	Pinned  bool   `json:"pinned"`
}

type copyFileRequest struct {
	EntryID               int64  `json:"entryId"`
	DestinationSpace      string `json:"destinationSpace"`
	DestinationClassID    int64  `json:"destinationClassId,omitempty"`
	DestinationParentPath string `json:"destinationParentPath"`
}

type renameRequest struct {
	Name string `json:"name"`
}

type createStudentRequest struct {
	ClassID     int64  `json:"classId"`
	StudentNo   string `json:"studentNo"`
	DisplayName string `json:"displayName"`
}

type createAssignmentRequest struct {
	ClassID                int64  `json:"classId"`
	Title                  string `json:"title"`
	Description            string `json:"description"`
	DueAt                  string `json:"dueAt"`
	Status                 string `json:"status"`
	SubmissionMode         string `json:"submissionMode"`
	SubmissionTypeCategory string `json:"submissionTypeCategory"`
	MinFileCount           int    `json:"minFileCount"`
}

type updateAssignmentRequest struct {
	Title                  *string `json:"title"`
	Description            *string `json:"description"`
	DueAt                  *string `json:"dueAt"`
	Status                 *string `json:"status"`
	SubmissionMode         *string `json:"submissionMode"`
	SubmissionTypeCategory *string `json:"submissionTypeCategory"`
	MinFileCount           *int    `json:"minFileCount"`
}

type updateStudentRequest struct {
	StudentNo   string `json:"studentNo"`
	DisplayName string `json:"displayName"`
}

type importStudentsRequest struct {
	ClassID  int64                  `json:"classId"`
	Students []createStudentRequest `json:"students"`
}

type classSummary struct {
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	JoinCode              string `json:"joinCode"`
	JoinCodeStatus        string `json:"joinCodeStatus"`
	JoinCodeHint          string `json:"joinCodeHint"`
	RegistrationEnabled   bool   `json:"registrationEnabled"`
	RegistrationExpiresAt string `json:"registrationExpiresAt"`
	ExamLockEnabled       bool   `json:"examLockEnabled"`
}

type fileSummary struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	Kind        string        `json:"kind"`
	MimeType    string        `json:"mimeType"`
	Size        int64         `json:"size"`
	UpdatedAt   string        `json:"updatedAt"`
	DownloadURL string        `json:"downloadUrl"`
	ArchiveURL  string        `json:"archiveUrl"`
	PreviewURL  string        `json:"previewUrl"`
	FileCount   int           `json:"fileCount,omitempty"`
	FolderCount int           `json:"folderCount,omitempty"`
	Children    []fileSummary `json:"children,omitempty"`
}

type fileEntry struct {
	ID         int64
	Space      string
	ClassID    sql.NullInt64
	ParentPath string
	ItemPath   string
	Name       string
	Kind       string
	MimeType   string
	Size       int64
	DiskPath   string
	CreatedAt  string
	UpdatedAt  string
}

type fileSearchQuery struct {
	Keyword       string
	TypeFilter    string
	CaseSensitive bool
}

var fileSearchTypeExtensions = map[string]map[string]struct{}{
	"image": {
		".png":  {},
		".jpg":  {},
		".jpeg": {},
		".gif":  {},
		".webp": {},
	},
	"audio": {
		".mp3": {},
		".wav": {},
		".ogg": {},
		".m4a": {},
	},
	"video": {
		".mp4":  {},
		".webm": {},
		".mov":  {},
		".mkv":  {},
	},
}

type studentsResult struct {
	Students   []studentResult   `json:"students"`
	Pagination paginationPayload `json:"pagination"`
}

type assignmentsResult struct {
	Assignments []assignmentSummary `json:"assignments"`
	Pagination  paginationPayload   `json:"pagination"`
}

type assignmentAttachmentsResult struct {
	Items []fileSummary `json:"items"`
}

type assignmentSubmissionsResult struct {
	Submissions           []teacherAssignmentSubmissionItem `json:"submissions"`
	SubmissionConstraints studentSubmissionConstraints      `json:"submissionConstraints"`
	Pagination            paginationPayload                 `json:"pagination"`
}

type assignmentStatisticsResult struct {
	ClassID         int64                         `json:"classId"`
	AssignmentIDs   []int64                       `json:"assignmentIds"`
	RosterTotal     int                           `json:"rosterTotal"`
	AssignmentTotal int                           `json:"assignmentTotal"`
	ExpectedTotal   int                           `json:"expectedTotal"`
	SubmittedTotal  int                           `json:"submittedTotal"`
	MissingTotal    int                           `json:"missingTotal"`
	Rows            []assignmentStatisticsRowItem `json:"rows"`
}

type assignmentStatisticsRowItem struct {
	StudentID      int64  `json:"studentId"`
	StudentNo      string `json:"studentNo"`
	DisplayName    string `json:"displayName"`
	SubmittedCount int    `json:"submittedCount"`
	MissingCount   int    `json:"missingCount"`
}

type teacherAssignmentSubmissionItem struct {
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

type studentAssignmentsResult struct {
	Assignments           []studentAssignmentSummary   `json:"assignments"`
	Pagination            paginationPayload            `json:"pagination"`
	SubmissionConstraints studentSubmissionConstraints `json:"submissionConstraints"`
}

type studentAssignmentSummary struct {
	ID                     int64                        `json:"id"`
	ClassID                int64                        `json:"classId"`
	Title                  string                       `json:"title"`
	Description            string                       `json:"description"`
	DueAt                  string                       `json:"dueAt"`
	Status                 string                       `json:"status"`
	SubmissionMode         string                       `json:"submissionMode"`
	SubmissionTypeCategory string                       `json:"submissionTypeCategory"`
	MinFileCount           int                          `json:"minFileCount"`
	CreatedAt              string                       `json:"createdAt"`
	UpdatedAt              string                       `json:"updatedAt"`
	Overdue                bool                         `json:"overdue"`
	Submission             *assignmentSubmissionSummary `json:"submission"`
}

type studentAssignmentDetail struct {
	ID                     int64                        `json:"id"`
	ClassID                int64                        `json:"classId"`
	Title                  string                       `json:"title"`
	Description            string                       `json:"description"`
	DueAt                  string                       `json:"dueAt"`
	Status                 string                       `json:"status"`
	SubmissionMode         string                       `json:"submissionMode"`
	SubmissionTypeCategory string                       `json:"submissionTypeCategory"`
	MinFileCount           int                          `json:"minFileCount"`
	CreatedAt              string                       `json:"createdAt"`
	UpdatedAt              string                       `json:"updatedAt"`
	Overdue                bool                         `json:"overdue"`
	Submission             *assignmentSubmissionSummary `json:"submission"`
	SubmissionConstraints  studentSubmissionConstraints `json:"submissionConstraints"`
	AssignmentAttachments  []fileSummary                `json:"assignmentAttachments"`
	Items                  []fileSummary                `json:"items"`
	ExamLocked             bool                         `json:"examLocked"`
}

type assignmentSubmissionSummary struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	SubmittedAt string `json:"submittedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type studentSubmissionConstraints struct {
	AllowedTypesLabel string `json:"allowedTypesLabel"`
	MaxFileSizeBytes  int64  `json:"maxFileSizeBytes"`
	MaxFileSizeLabel  string `json:"maxFileSizeLabel"`
}

type updateAssignmentSubmissionReviewRequest struct {
	ReviewStatus   string `json:"reviewStatus"`
	TeacherComment string `json:"teacherComment"`
}

type studentSubmissionResult struct {
	Submission *assignmentSubmissionSummary `json:"submission"`
	Items      []fileSummary                `json:"items"`
}

type studentResult struct {
	ID          int64  `json:"id"`
	ClassID     int64  `json:"classId"`
	StudentNo   string `json:"studentNo"`
	DisplayName string `json:"displayName"`
	ActivatedAt string `json:"activatedAt"`
}

type assignmentSummary struct {
	ID                     int64  `json:"id"`
	ClassID                int64  `json:"classId"`
	Title                  string `json:"title"`
	Description            string `json:"description"`
	DueAt                  string `json:"dueAt"`
	Status                 string `json:"status"`
	SubmissionMode         string `json:"submissionMode"`
	SubmissionTypeCategory string `json:"submissionTypeCategory"`
	MinFileCount           int    `json:"minFileCount"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
}

type studentImportRecord struct {
	StudentNo   string
	DisplayName string
}

type xlsxWorksheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Ref          string           `xml:"r,attr"`
	Type         string           `xml:"t,attr"`
	Value        string           `xml:"v"`
	InlineString xlsxInlineString `xml:"is"`
}

type xlsxInlineString struct {
	Text string        `xml:"t"`
	Runs []xlsxTextRun `xml:"r"`
}

type xlsxTextRun struct {
	Text string `xml:"t"`
}

type xlsxSharedStringTable struct {
	Items []xlsxSharedStringItem `xml:"si"`
}

type xlsxSharedStringItem struct {
	Text string        `xml:"t"`
	Runs []xlsxTextRun `xml:"r"`
}

func New(config Config) (http.Handler, error) {
	baseDir := config.BaseDir
	if strings.TrimSpace(baseDir) == "" {
		baseDir = "."
	}

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}

	varDir := filepath.Join(baseDir, "var")
	dbDir := filepath.Join(varDir, "data")
	storageRoot := filepath.Join(varDir, "storage")

	for _, dir := range []string{dbDir, storageRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	dsn := filepath.Join(dbDir, "classdrive.db") + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Configure connection pool for better concurrency handling
	db.SetMaxOpenConns(25)                  // Maximum number of open connections to the database
	db.SetMaxIdleConns(5)                   // Maximum number of idle connections in the pool
	db.SetConnMaxLifetime(5 * time.Minute)  // Maximum amount of time a connection may be reused

	app := &App{
		db:                      db,
		mux:                     http.NewServeMux(),
		baseDir:                 baseDir,
		storageRoot:             storageRoot,
		prepareServerPortChange: config.PrepareServerPortChange,
		shareAttempts:           make(map[string]*shareAttempt),
	}

	if err := app.initialize(config.Seed); err != nil {
		return nil, err
	}

	app.registerRoutes()
	return app, nil
}

func (app *App) SetServerPortChangeHandler(handler func(port string) (PreparedServerPortChange, error)) {
	app.prepareServerPortChange = handler
}

func (app *App) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !app.allowRequestOrigin(writer, request) {
		return
	}
	app.mux.ServeHTTP(writer, request)
}

func (app *App) Close() error {
	return app.db.Close()
}

func (app *App) setAuthCookie(writer http.ResponseWriter, request *http.Request, name, value string, expires time.Time) {
	http.SetCookie(writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookieForRequest(request),
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
	})
}

func (app *App) clearAuthCookie(writer http.ResponseWriter, request *http.Request, name string) {
	http.SetCookie(writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookieForRequest(request),
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func (app *App) setCSRFCookie(writer http.ResponseWriter, request *http.Request, value string, expires time.Time) {
	http.SetCookie(writer, &http.Cookie{
		Name:     csrfCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: false,
		Secure:   secureCookieForRequest(request),
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
	})
}

func (app *App) clearCSRFCookie(writer http.ResponseWriter, request *http.Request) {
	http.SetCookie(writer, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   secureCookieForRequest(request),
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func (app *App) ensureCSRFCookie(writer http.ResponseWriter, request *http.Request) error {
	cookie, err := request.Cookie(csrfCookieName)
	if err == nil && strings.TrimSpace(cookie.Value) != "" {
		return nil
	}
	token, err := generateToken()
	if err != nil {
		return err
	}
	app.setCSRFCookie(writer, request, token, time.Now().Add(sessionDuration))
	return nil
}

func secureCookieForRequest(request *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CLASSDRIVE_COOKIE_SECURE"))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return request.TLS != nil
	}
}

func needsCSRFProtection(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func requestScheme(request *http.Request) string {
	if request.URL != nil && request.URL.Scheme != "" {
		return strings.ToLower(request.URL.Scheme)
	}
	if request.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(request *http.Request) string {
	if strings.TrimSpace(request.Host) != "" {
		return strings.ToLower(strings.TrimSpace(request.Host))
	}
	if request.URL != nil {
		return strings.ToLower(strings.TrimSpace(request.URL.Host))
	}
	return ""
}

func sameOrigin(request *http.Request, rawOrigin string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawOrigin))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, requestScheme(request)) &&
		strings.EqualFold(parsed.Host, requestHost(request))
}

func (app *App) allowRequestOrigin(writer http.ResponseWriter, request *http.Request) bool {
	if !needsCSRFProtection(request.Method) {
		return true
	}
	if origin := strings.TrimSpace(request.Header.Get("Origin")); origin != "" {
		if sameOrigin(request, origin) {
			return true
		}
		app.writeError(writer, http.StatusForbidden, "csrf_rejected", "请求来源不可信")
		return false
	}
	if referer := strings.TrimSpace(request.Header.Get("Referer")); referer != "" {
		if sameOrigin(request, referer) {
			return true
		}
		app.writeError(writer, http.StatusForbidden, "csrf_rejected", "请求来源不可信")
		return false
	}
	return true
}

func (app *App) validateCSRF(writer http.ResponseWriter, request *http.Request) bool {
	if !needsCSRFProtection(request.Method) {
		return true
	}
	if app.disableCSRFForTests {
		return true
	}
	headerValue := strings.TrimSpace(request.Header.Get(csrfHeaderName))
	cookie, err := request.Cookie(csrfCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" || headerValue == "" {
		app.writeError(writer, http.StatusForbidden, "csrf_rejected", "请求校验失败")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(cookie.Value)), []byte(headerValue)) != 1 {
		app.writeError(writer, http.StatusForbidden, "csrf_rejected", "请求校验失败")
		return false
	}
	return true
}

func (app *App) initialize(seed bool) error {
	if err := app.runSchema(); err != nil {
		return err
	}
	if seed {
		return app.seed()
	}
	return nil
}

func (app *App) registerRoutes() {
	app.mux.HandleFunc("/api/auth/login", app.handleLogin)
	app.mux.HandleFunc("/api/auth/logout", app.withSession(app.handleLogout))
	app.mux.HandleFunc("/api/session", app.handleSession)
	app.mux.HandleFunc("/api/student/auth/activate", app.handleStudentActivate)
	app.mux.HandleFunc("/api/student/auth/login", app.handleStudentLogin)
	app.mux.HandleFunc("/api/student/auth/logout", app.withStudentSessionAccess(app.handleStudentLogout, false))
	app.mux.HandleFunc("/api/student/session", app.handleStudentSession)
	app.mux.HandleFunc("/api/student/password", app.withStudentSessionAccess(app.handleStudentPassword, false))
	app.mux.HandleFunc("/api/student/assignments", app.withStudentSession(app.handleStudentAssignments))
	app.mux.HandleFunc("/api/student/assignments/", app.withStudentSession(app.handleStudentAssignmentByID))
	app.mux.HandleFunc("/api/student/files", app.withStudentSession(app.handleStudentFiles))
	app.mux.HandleFunc("/api/student/files/", app.withStudentSession(app.handleStudentFileByID))
	app.mux.HandleFunc("/api/shell", app.withSession(app.handleShell))
	app.mux.HandleFunc("/api/settings/profile", app.withSession(app.handleTeacherProfileSettings))
	app.mux.HandleFunc("/api/settings/password", app.withSession(app.handleTeacherPasswordSettings))
	app.mux.HandleFunc("/api/settings/system", app.withSession(app.handleSystemSettings))
	app.mux.HandleFunc("/api/audit/logs", app.withSession(app.handleAuditLogs))
	app.mux.HandleFunc("/api/audit/login-logs", app.withSession(app.handleLoginLogs))
	app.mux.HandleFunc("/api/audit/operation-logs", app.withSession(app.handleOperationLogs))
	app.mux.HandleFunc("/api/teachers", app.withSession(app.handleTeacherUsers))
	app.mux.HandleFunc("/api/teachers/", app.withSession(app.handleTeacherUserByID))
	app.mux.HandleFunc("/api/classes", app.withSession(app.handleClasses))
	app.mux.HandleFunc("/api/classes/", app.withSession(app.handleClassByID))
	app.mux.HandleFunc("/api/preferences/recent-copy-targets", app.withSession(app.handleRecentCopyTargets))
	app.mux.HandleFunc("/api/assignments", app.withSession(app.handleAssignments))
	app.mux.HandleFunc("/api/assignments/statistics", app.withSession(app.handleAssignmentStatistics))
	app.mux.HandleFunc("/api/assignments/submissions/archive", app.withSession(app.handleAssignmentSubmissionsArchive))
	app.mux.HandleFunc("/api/assignments/", app.withSession(app.handleAssignmentByID))
	app.mux.HandleFunc("/api/students", app.withSession(app.handleStudents))
	app.mux.HandleFunc("/api/students/import", app.withSession(app.handleImportStudents))
	app.mux.HandleFunc("/api/students/import-file", app.withSession(app.handleImportStudentsFile))
	app.mux.HandleFunc("/api/students/import-template", app.withSession(app.handleStudentImportTemplate))
	app.mux.HandleFunc("/api/students/", app.withSession(app.handleStudentByID))
	app.mux.HandleFunc("/api/files", app.withSession(app.handleFiles))
	app.mux.HandleFunc("/api/files/search", app.withSession(app.handleFileSearch))
	app.mux.HandleFunc("/api/files/batch/archive", app.withSession(app.handleFilesBatchArchive))
	app.mux.HandleFunc("/api/files/move", app.withSession(app.handleMoveFile))
	app.mux.HandleFunc("/api/files/upload/sessions", app.withSession(app.handleUploadSessions))
	app.mux.HandleFunc("/api/files/upload/sessions/", app.withSession(app.handleUploadSessionByID))
	app.mux.HandleFunc("/api/files/upload", app.withSession(app.handleUpload))
	app.mux.HandleFunc("/api/files/file", app.withSession(app.handleCreateFile))
	app.mux.HandleFunc("/api/files/folder", app.withSession(app.handleCreateFolder))
	app.mux.HandleFunc("/api/files/copy", app.withSession(app.handleCopyFile))
	app.mux.HandleFunc("/api/files/", app.withSession(app.handleFileByID))
	app.mux.HandleFunc("/api/library-shares", app.withSession(app.handleLibraryShares))
	app.mux.HandleFunc("/api/library-shares/", app.withSession(app.handleLibraryShareByID))
	app.mux.HandleFunc("/api/share/", app.handlePublicShare)
	app.mux.HandleFunc("/", app.handleFrontend)
}

func (app *App) handleLogin(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	var payload loginRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}

	teacher, err := app.findTeacherByUsername(payload.Username)
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	if teacher == nil || teacher.Disabled || verifyPasswordHash(teacher.PasswordHash, payload.Password) != nil {
		app.recordLoginLog(request, "teacher", teacherLoginLogID(teacher), strings.TrimSpace(payload.Username), teacherLoginLogName(teacher), "failure", "账号或密码错误")
		app.writeError(writer, http.StatusUnauthorized, "unauthorized", "账号或密码错误")
		return
	}

	token, err := generateToken()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	settings, err := app.loadSystemSettings()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	invalidatedSessions := int64(0)
	if settings.SingleAccountLoginEnabled {
		deleted, err := app.deleteSessionsForTeacher(teacher.ID)
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		invalidatedSessions = deleted
	}
	if err := app.createSession(token, teacher.ID); err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	loginMessage := "登录成功"
	if invalidatedSessions > 0 {
		loginMessage = "登录成功，旧登录已失效"
	}
	app.recordLoginLog(request, "teacher", teacher.ID, teacher.Username, teacher.DisplayName, "success", loginMessage)

	csrfToken, err := generateToken()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	expires := time.Now().Add(sessionDuration)
	app.setAuthCookie(writer, request, sessionCookieName, token, expires)
	app.setCSRFCookie(writer, request, csrfToken, expires)

	app.writeJSON(writer, http.StatusOK, loginResult{
		User: teacher.toSessionUser(),
	})
}

func (app *App) handleLogout(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	if cookie, err := request.Cookie(sessionCookieName); err == nil {
		_ = app.deleteSession(cookie.Value)
	}
	app.recordLoginLog(request, "teacher", teacher.ID, teacher.Username, teacher.DisplayName, "success", "老师退出登录")

	app.clearAuthCookie(writer, request, sessionCookieName)
	app.clearCSRFCookie(writer, request)

	app.writeJSON(writer, http.StatusOK, map[string]any{
		"ok":   true,
		"user": teacher.toSessionUser(),
	})
}

func (app *App) handleSession(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	teacher, err := app.requireTeacher(request)
	if err != nil {
		app.writeError(writer, http.StatusUnauthorized, "unauthorized", "未登录")
		return
	}
	if err := app.ensureCSRFCookie(writer, request); err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	app.writeJSON(writer, http.StatusOK, sessionResponse{User: teacher.toSessionUser()})
}

func (app *App) handleStudentActivate(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	var payload studentActivateRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}

	student, err := app.activateStudent(payload)
	if err != nil {
		app.recordLoginLog(request, "student", 0, strings.TrimSpace(payload.StudentNo), "", "failure", "学生激活失败")
		app.writeDomainError(writer, err)
		return
	}

	token, err := generateToken()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	settings, err := app.loadSystemSettings()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	invalidatedSessions := int64(0)
	if settings.SingleAccountLoginEnabled {
		deleted, err := app.deleteSessionsForStudent(student.ID)
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		invalidatedSessions = deleted
	}
	if err := app.createStudentSession(token, student.ID); err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	loginMessage := "学生激活并登录成功"
	if invalidatedSessions > 0 {
		loginMessage = "学生激活并登录成功，旧登录已失效"
	}
	app.recordLoginLog(request, "student", student.ID, student.StudentNo, student.DisplayName, "success", loginMessage)

	csrfToken, err := generateToken()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	expires := time.Now().Add(sessionDuration)
	app.setAuthCookie(writer, request, studentSessionCookieName, token, expires)
	app.setCSRFCookie(writer, request, csrfToken, expires)

	app.writeJSON(writer, http.StatusOK, studentSessionResult{User: student.toSessionUser()})
}

func (app *App) handleStudentLogin(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	var payload studentLoginRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}

	student, err := app.authenticateStudent(payload)
	if err != nil {
		actorID, actorName := app.studentLoginLogIdentity(payload.StudentNo)
		app.recordLoginLog(request, "student", actorID, strings.TrimSpace(payload.StudentNo), actorName, "failure", "学生登录失败")
		app.writeDomainError(writer, err)
		return
	}

	token, err := generateToken()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	settings, err := app.loadSystemSettings()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	invalidatedSessions := int64(0)
	if settings.SingleAccountLoginEnabled {
		deleted, err := app.deleteSessionsForStudent(student.ID)
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		invalidatedSessions = deleted
	}
	if err := app.createStudentSession(token, student.ID); err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	loginMessage := "学生登录成功"
	if invalidatedSessions > 0 {
		loginMessage = "学生登录成功，旧登录已失效"
	}
	app.recordLoginLog(request, "student", student.ID, student.StudentNo, student.DisplayName, "success", loginMessage)

	csrfToken, err := generateToken()
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	expires := time.Now().Add(sessionDuration)
	app.setAuthCookie(writer, request, studentSessionCookieName, token, expires)
	app.setCSRFCookie(writer, request, csrfToken, expires)

	app.writeJSON(writer, http.StatusOK, studentSessionResult{User: student.toSessionUser()})
}

func (app *App) handleStudentLogout(writer http.ResponseWriter, request *http.Request, student studentRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	if cookie, err := request.Cookie(studentSessionCookieName); err == nil {
		_ = app.deleteStudentSession(cookie.Value)
	}
	app.recordLoginLog(request, "student", student.ID, student.StudentNo, student.DisplayName, "success", "学生退出登录")

	app.clearAuthCookie(writer, request, studentSessionCookieName)
	app.clearCSRFCookie(writer, request)

	app.writeJSON(writer, http.StatusOK, map[string]any{
		"ok": true,
	})
}

func (app *App) handleStudentSession(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	student, err := app.requireStudent(request)
	if err != nil {
		app.writeError(writer, http.StatusUnauthorized, "unauthorized", "未登录")
		return
	}
	if err := app.ensureCSRFCookie(writer, request); err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	app.writeJSON(writer, http.StatusOK, studentSessionResult{User: student.toSessionUser()})
}

func (app *App) handleStudentPassword(writer http.ResponseWriter, request *http.Request, student studentRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	var payload studentPasswordChangeRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}

	updated, err := app.changeStudentPassword(student, payload)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusOK, studentSessionResult{User: updated.toSessionUser()})
}

func (app *App) handleStudentAssignments(writer http.ResponseWriter, request *http.Request, student studentRecord) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	query, err := parseTeacherListQuery(request.URL.Query(), "updated-desc", map[string]struct{}{
		"updated-desc": {},
	})
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	assignments, pagination, err := app.listStudentAssignmentsPage(student, query)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusOK, studentAssignmentsResult{
		Assignments:           assignments,
		Pagination:            pagination,
		SubmissionConstraints: studentSubmissionConstraintsPayload(),
	})
}

func (app *App) handleStudentAssignmentByID(writer http.ResponseWriter, request *http.Request, student studentRecord) {
	trimmed := strings.Trim(strings.TrimPrefix(request.URL.Path, "/api/student/assignments/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		app.writeError(writer, http.StatusNotFound, "not_found", "作业不存在")
		return
	}

	assignmentID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || assignmentID <= 0 {
		app.writeError(writer, http.StatusNotFound, "not_found", "作业不存在")
		return
	}

	switch {
	case len(parts) == 1:
		if request.Method != http.MethodGet {
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
			return
		}
		detail, err := app.getStudentAssignmentDetail(assignmentID, student)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, detail)
	case len(parts) == 2 && parts[1] == "submission":
		if request.Method != http.MethodPost {
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
			return
		}
		if err := request.ParseMultipartForm(32 << 20); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "解析上传失败")
			return
		}
		headers := request.MultipartForm.File["files"]
		if len(headers) == 0 {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "请先选择文件")
			return
		}
		relativePaths := request.MultipartForm.Value["relativePaths"]
		result, err := app.submitStudentAssignment(assignmentID, student, headers, relativePaths)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, result)
	case len(parts) == 4 && parts[1] == "attachments" && parts[3] == "download":
		if request.Method != http.MethodGet {
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
			return
		}
		fileID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || fileID <= 0 {
			app.writeError(writer, http.StatusNotFound, "not_found", "附件不存在")
			return
		}
		entry, err := app.findStudentAssignmentAttachmentByID(assignmentID, student, fileID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.serveSubmissionEntryDownload(writer, request, entry)
	case len(parts) == 5 && parts[1] == "submission" && parts[2] == "files" && parts[4] == "download":
		if request.Method != http.MethodGet {
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
			return
		}
		fileID, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil || fileID <= 0 {
			app.writeError(writer, http.StatusNotFound, "not_found", "附件不存在")
			return
		}
		locked, err := app.isClassExamLocked(student.ClassID)
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "internal_error", "服务异常")
			return
		}
		if locked {
			app.writeError(writer, http.StatusForbidden, "exam_locked", "考试期间已禁止下载和预览历史提交")
			return
		}
		entry, err := app.findStudentSubmissionFileByID(assignmentID, student, fileID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.serveSubmissionEntryDownload(writer, request, entry)
	case len(parts) == 4 && parts[1] == "submission" && parts[2] == "files":
		if request.Method != http.MethodDelete {
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
			return
		}
		fileID, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil || fileID <= 0 {
			app.writeError(writer, http.StatusNotFound, "not_found", "附件不存在")
			return
		}
		result, err := app.deleteStudentSubmissionFile(assignmentID, student, fileID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, result)
	default:
		app.writeError(writer, http.StatusNotFound, "not_found", "作业不存在")
	}
}

func (app *App) handleStudentFiles(writer http.ResponseWriter, request *http.Request, student studentRecord) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	space, classID, err := studentFileSpace(request.URL.Query().Get("space"), student)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	query, err := parseFileListQuery(request.URL.Query())
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	currentPath, err := normalizeListPath(request.URL.Query().Get("path"))
	if err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	var items []fileEntry
	if strings.TrimSpace(query.Keyword) != "" {
		items, err = app.searchFileEntries(space, classID, currentPath, query.Keyword)
	} else {
		items, err = app.listFileEntries(space, classID, currentPath)
	}
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if err := app.applyFileEntryFolderSizes(items); err != nil {
		app.writeDomainError(writer, err)
		return
	}
	items, pagination := sortAndPaginateFileEntries(items, query)
	result := fileListResult{
		Space:       space,
		CurrentPath: currentPath,
		Items:       make([]fileSummary, 0, len(items)),
		Pagination:  pagination,
	}
	if classID != nil {
		result.ClassID = *classID
	}
	for _, item := range items {
		result.Items = append(result.Items, buildStudentFileSummary(item))
	}
	app.writeJSON(writer, http.StatusOK, result)
}

func (app *App) handleStudentFileByID(writer http.ResponseWriter, request *http.Request, student studentRecord) {
	trimmed := strings.TrimPrefix(request.URL.Path, "/api/student/files/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}
	entryID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || entryID <= 0 {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}
	entry, err := app.getStudentVisibleFileEntry(entryID, student)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}

	switch {
	case parts[1] == "download" && request.Method == http.MethodGet:
		app.serveFileEntryContent(writer, request, entry, true)
	case parts[1] == "preview" && request.Method == http.MethodGet:
		app.serveFileEntryContent(writer, request, entry, false)
	case parts[1] == "archive" && request.Method == http.MethodGet:
		app.serveStudentFileArchive(writer, request, entry)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleShell(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	app.writeJSON(writer, http.StatusOK, shellResult{
		User: teacher.toSessionUser(),
		Items: []shellItem{
			{Key: "library", Label: "老师资料", Href: "/files/library"},
			{Key: "public", Label: "公共资料", Href: "/files/public"},
			{Key: "classes-files", Label: "班级资料", Href: "/files/classes/1"},
			{Key: "classes", Label: "班级管理", Href: "/classes", Placeholder: false},
			{Key: "assignments", Label: "作业管理", Href: "/assignments", Placeholder: false},
			{Key: "students", Label: "学生管理", Href: "/students", Placeholder: false},
			{Key: "settings", Label: "设置", Href: "/settings", Placeholder: false},
		},
	})
}

func (app *App) handleTeacherProfileSettings(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	switch request.Method {
	case http.MethodGet:
		preferences, err := app.loadTeacherProfilePreferences(teacher.ID)
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		app.writeJSON(writer, http.StatusOK, teacherProfileResult{
			Profile: buildTeacherProfilePayload(teacher, preferences),
		})
	case http.MethodPatch:
		var payload updateTeacherProfileRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		updatedTeacher, preferences, err := app.updateTeacherProfile(teacher, payload)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, teacherProfileResult{
			Profile: buildTeacherProfilePayload(*updatedTeacher, preferences),
		})
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleTeacherPasswordSettings(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	var payload updateTeacherPasswordRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	if err := app.updateTeacherPassword(teacher.ID, payload); err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleSystemSettings(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	switch request.Method {
	case http.MethodGet:
		settings, err := app.loadSystemSettings()
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		app.writeJSON(writer, http.StatusOK, systemSettingsResult{Settings: settings.withServerHost(request)})
	case http.MethodPatch:
		if err := requireOwnerTeacher(teacher); err != nil {
			app.writeDomainError(writer, err)
			return
		}
		var payload updateSystemSettingsRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		updated, err := app.updateSystemSettings(payload)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, systemSettingsResult{Settings: updated.withServerHost(request)})
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleAuditLogs(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	if err := requireOwnerTeacher(teacher); err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if request.Method == http.MethodGet {
		logs, pagination, err := app.listAuditLogsPage(parseAuditLogFilters(request.URL.Query()))
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		app.writeJSON(writer, http.StatusOK, auditLogsResult{Logs: logs, Pagination: pagination})
		return
	}
	if request.Method != http.MethodDelete {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	before, err := normalizeAuditClearBefore(request.URL.Query().Get("before"))
	if err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	result, err := app.clearAuditLogsBefore(before)
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	app.writeJSON(writer, http.StatusOK, result)
}

func (app *App) handleLoginLogs(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	if err := requireOwnerTeacher(teacher); err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	logs, pagination, err := app.listLoginLogsPage(parseLoginLogFilters(request.URL.Query()))
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	app.writeJSON(writer, http.StatusOK, loginLogsResult{Logs: logs, Pagination: pagination})
}

func (app *App) handleOperationLogs(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	if err := requireOwnerTeacher(teacher); err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	logs, pagination, err := app.listOperationLogsPage(parseOperationLogFilters(request.URL.Query()))
	if err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	app.writeJSON(writer, http.StatusOK, operationLogsResult{Logs: logs, Pagination: pagination})
}

func (app *App) handleTeacherUsers(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	if err := requireOwnerTeacher(teacher); err != nil {
		app.writeDomainError(writer, err)
		return
	}

	switch request.Method {
	case http.MethodGet:
		teachers, err := app.listTeacherUsers()
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		app.writeJSON(writer, http.StatusOK, teacherUsersResult{Teachers: teachers})
	case http.MethodPost:
		var payload createTeacherUserRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		created, err := app.createTeacherUser(payload)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusCreated, teacherUserResult{Teacher: created.toTeacherUserPayload()})
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleTeacherUserByID(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	if err := requireOwnerTeacher(teacher); err != nil {
		app.writeDomainError(writer, err)
		return
	}

	trimmed := strings.Trim(strings.TrimPrefix(request.URL.Path, "/api/teachers/"), "/")
	teacherID, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || teacherID <= 0 {
		app.writeError(writer, http.StatusNotFound, "not_found", "老师不存在")
		return
	}

	switch request.Method {
	case http.MethodGet:
		record, err := app.findTeacherByID(teacherID)
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		if record == nil {
			app.writeError(writer, http.StatusNotFound, "not_found", "老师不存在")
			return
		}
		app.writeJSON(writer, http.StatusOK, teacherUserResult{Teacher: record.toTeacherUserPayload()})
	case http.MethodPatch:
		var payload updateTeacherUserRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		record, err := app.updateTeacherUser(teacherID, payload)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, teacherUserResult{Teacher: record.toTeacherUserPayload()})
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func parseTeacherListQuery(values url.Values, defaultSort string, allowedSorts map[string]struct{}) (teacherListQuery, error) {
	query := teacherListQuery{
		Keyword:  strings.TrimSpace(values.Get("q")),
		Sort:     defaultSort,
		Page:     1,
		PageSize: defaultTeacherListPageSize,
		Paged:    true,
	}

	if registration := strings.TrimSpace(values.Get("registration")); registration != "" {
		switch registration {
		case "registered", "unregistered":
			query.RegistrationFilter = registration
		default:
			return teacherListQuery{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "registration 非法"}
		}
	}

	if requestedSort := strings.TrimSpace(values.Get("sort")); requestedSort != "" {
		query.Sort = requestedSort
	}
	if _, ok := allowedSorts[query.Sort]; !ok {
		return teacherListQuery{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "sort 非法"}
	}

	pageSizeRaw := strings.TrimSpace(values.Get("pageSize"))
	pageRaw := strings.TrimSpace(values.Get("page"))
	if pageRaw != "" {
		page, err := strconv.Atoi(pageRaw)
		if err != nil || page <= 0 {
			return teacherListQuery{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "page 非法"}
		}
		query.Page = page
	}

	if pageSizeRaw != "" {
		pageSize, err := strconv.Atoi(pageSizeRaw)
		if err != nil || pageSize <= 0 || pageSize > maxTeacherListPageSize {
			return teacherListQuery{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "pageSize 非法"}
		}
		query.PageSize = pageSize
	}
	query.Offset = (query.Page - 1) * query.PageSize
	return query, nil
}

func buildTeacherListPagination(total int, query teacherListQuery, currentCount int) paginationPayload {
	if total < 0 {
		total = 0
	}
	if !query.Paged {
		return paginationPayload{
			Page:       1,
			PageSize:   currentCount,
			Total:      total,
			TotalPages: 1,
		}
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + query.PageSize - 1) / query.PageSize
	}
	return paginationPayload{
		Page:       query.Page,
		PageSize:   query.PageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}

func parseFileListQuery(values url.Values) (teacherListQuery, error) {
	return parseTeacherListQuery(values, "name-asc", map[string]struct{}{
		"name-asc":     {},
		"name-desc":    {},
		"updated-desc": {},
		"updated-asc":  {},
		"size-desc":    {},
		"size-asc":     {},
	})
}

func sortAndPaginateFileEntries(entries []fileEntry, query teacherListQuery) ([]fileEntry, paginationPayload) {
	sorted := append([]fileEntry(nil), entries...)
	sort.SliceStable(sorted, func(leftIndex, rightIndex int) bool {
		left := sorted[leftIndex]
		right := sorted[rightIndex]
		switch query.Sort {
		case "name-desc":
			if left.Name != right.Name {
				return strings.Compare(left.Name, right.Name) > 0
			}
		case "updated-desc":
			if left.UpdatedAt != right.UpdatedAt {
				return left.UpdatedAt > right.UpdatedAt
			}
		case "updated-asc":
			if left.UpdatedAt != right.UpdatedAt {
				return left.UpdatedAt < right.UpdatedAt
			}
		case "size-desc":
			if left.Size != right.Size {
				return left.Size > right.Size
			}
		case "size-asc":
			if left.Size != right.Size {
				return left.Size < right.Size
			}
		}
		return strings.Compare(left.Name, right.Name) < 0
	})

	total := len(sorted)
	if !query.Paged {
		return sorted, buildTeacherListPagination(total, query, len(sorted))
	}

	start := query.Offset
	if start > total {
		start = total
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}
	paged := sorted[start:end]
	return paged, buildTeacherListPagination(total, query, len(paged))
}

func (app *App) handleClasses(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	switch request.Method {
	case http.MethodGet:
		query, err := parseTeacherListQuery(request.URL.Query(), "name-asc", map[string]struct{}{
			"name-asc":          {},
			"name-desc":         {},
			"registration-desc": {},
			"registration-asc":  {},
		})
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		classes, pagination, err := app.listClassesPage(query)
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		app.writeJSON(writer, http.StatusOK, classesResult{Classes: classes, Pagination: pagination})
	case http.MethodPost:
		var payload createClassRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		created, err := app.createClass(payload.Name)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusCreated, created)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleClassByID(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	trimmed := strings.TrimPrefix(request.URL.Path, "/api/classes/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}

	classID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || classID <= 0 {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}

	if len(parts) == 1 {
		switch request.Method {
		case http.MethodPatch:
			var payload updateClassRequest
			if err := decodeJSONBody(request, &payload); err != nil {
				app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
				return
			}
			updated, updateErr := app.updateClass(classID, payload.Name)
			if updateErr != nil {
				app.writeDomainError(writer, updateErr)
				return
			}
			app.writeJSON(writer, http.StatusOK, updated)
		case http.MethodDelete:
			if err := app.deleteClass(classID); err != nil {
				app.writeDomainError(writer, err)
				return
			}
			app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
		default:
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		}
		return
	}

	if len(parts) == 2 && parts[1] == "join-code" {
		switch request.Method {
		case http.MethodPost:
			result, refreshErr := app.refreshClassJoinCode(classID)
			if refreshErr != nil {
				app.writeDomainError(writer, refreshErr)
				return
			}
			app.writeJSON(writer, http.StatusOK, result)
		case http.MethodPatch:
			var payload updateClassRegistrationRequest
			if err := decodeJSONBody(request, &payload); err != nil {
				app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
				return
			}
			result, updateErr := app.updateClassRegistration(classID, payload.Enabled)
			if updateErr != nil {
				app.writeDomainError(writer, updateErr)
				return
			}
			app.writeJSON(writer, http.StatusOK, result)
		default:
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		}
		return
	}

	if len(parts) == 2 && parts[1] == "exam-lock" {
		if request.Method != http.MethodPatch {
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
			return
		}
		var payload updateClassExamLockRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		result, updateErr := app.updateClassExamLock(classID, payload.Enabled)
		if updateErr != nil {
			app.writeDomainError(writer, updateErr)
			return
		}
		app.writeJSON(writer, http.StatusOK, result)
		return
	}

	app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
}

func (app *App) handleRecentCopyTargets(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	switch request.Method {
	case http.MethodGet:
		items, err := app.loadRecentCopyTargets(teacher.ID)
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		app.writeJSON(writer, http.StatusOK, recentCopyTargetsResult{Items: items})
	case http.MethodPut:
		var payload saveRecentCopyTargetsRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		if err := app.saveRecentCopyTargets(teacher.ID, payload.Items); err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, recentCopyTargetsResult{Items: payload.Items})
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleAssignments(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	switch request.Method {
	case http.MethodGet:
		classID, err := parseOptionalInt64(request.URL.Query().Get("classId"))
		if err != nil || classID == nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
			return
		}
		query, err := parseTeacherListQuery(request.URL.Query(), "updated-desc", map[string]struct{}{
			"updated-desc": {},
			"updated-asc":  {},
			"dueAt-asc":    {},
			"dueAt-desc":   {},
			"title-asc":    {},
			"title-desc":   {},
		})
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		assignments, pagination, listErr := app.listAssignmentsPage(*classID, query)
		if listErr != nil {
			app.writeDomainError(writer, listErr)
			return
		}
		app.writeJSON(writer, http.StatusOK, assignmentsResult{Assignments: assignments, Pagination: pagination})
	case http.MethodPost:
		var payload createAssignmentRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		created, createErr := app.createAssignment(payload)
		if createErr != nil {
			app.writeDomainError(writer, createErr)
			return
		}
		app.writeJSON(writer, http.StatusCreated, created)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleAssignmentStatistics(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	classID, err := parseOptionalInt64(request.URL.Query().Get("classId"))
	if err != nil || classID == nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
		return
	}
	assignmentIDs, err := parseAssignmentArchiveIDs(request.URL.Query().Get("assignmentIds"))
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	result, err := app.buildAssignmentStatistics(*classID, assignmentIDs)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusOK, result)
}

func (app *App) handleAssignmentByID(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	trimmed := strings.Trim(strings.TrimPrefix(request.URL.Path, "/api/assignments/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		app.writeError(writer, http.StatusNotFound, "not_found", "作业不存在")
		return
	}

	assignmentID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || assignmentID <= 0 {
		app.writeError(writer, http.StatusNotFound, "not_found", "作业不存在")
		return
	}

	classID, parseErr := parseOptionalInt64(request.URL.Query().Get("classId"))
	if parseErr != nil || classID == nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
		return
	}

	switch {
	case len(parts) == 1:
		switch request.Method {
		case http.MethodGet:
			assignment, findErr := app.findAssignmentByID(assignmentID, *classID)
			if findErr != nil {
				app.writeDomainError(writer, findErr)
				return
			}
			app.writeJSON(writer, http.StatusOK, assignment)
		case http.MethodPatch:
			var payload updateAssignmentRequest
			if err := decodeJSONBody(request, &payload); err != nil {
				app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
				return
			}
			updated, updateErr := app.updateAssignment(assignmentID, *classID, payload)
			if updateErr != nil {
				app.writeDomainError(writer, updateErr)
				return
			}
			app.writeJSON(writer, http.StatusOK, updated)
		case http.MethodDelete:
			if err := app.deleteAssignment(assignmentID, *classID); err != nil {
				app.writeDomainError(writer, err)
				return
			}
			app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
		default:
			app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		}
	case len(parts) == 2 && parts[1] == "attachments":
		app.handleAssignmentAttachments(writer, request, assignmentID, *classID)
	case len(parts) == 3 && parts[1] == "attachments":
		fileID, parseErr := strconv.ParseInt(parts[2], 10, 64)
		if parseErr != nil || fileID <= 0 {
			app.writeError(writer, http.StatusNotFound, "not_found", "附件不存在")
			return
		}
		app.handleAssignmentAttachmentByID(writer, request, assignmentID, *classID, fileID, false)
	case len(parts) == 4 && parts[1] == "attachments" && parts[3] == "download":
		fileID, parseErr := strconv.ParseInt(parts[2], 10, 64)
		if parseErr != nil || fileID <= 0 {
			app.writeError(writer, http.StatusNotFound, "not_found", "附件不存在")
			return
		}
		app.handleAssignmentAttachmentByID(writer, request, assignmentID, *classID, fileID, true)
	case len(parts) == 2 && parts[1] == "submissions":
		app.handleAssignmentSubmissions(writer, request, assignmentID, *classID)
	case len(parts) == 3 && parts[1] == "submissions":
		submissionID, parseErr := strconv.ParseInt(parts[2], 10, 64)
		if parseErr != nil || submissionID <= 0 {
			app.writeError(writer, http.StatusNotFound, "not_found", "提交不存在")
			return
		}
		app.handleAssignmentSubmissionByID(writer, request, assignmentID, *classID, submissionID, teacher)
	case len(parts) == 5 && parts[1] == "submissions" && parts[2] == "files" && parts[4] == "download":
		fileID, parseErr := strconv.ParseInt(parts[3], 10, 64)
		if parseErr != nil || fileID <= 0 {
			app.writeError(writer, http.StatusNotFound, "not_found", "附件不存在")
			return
		}
		app.handleAssignmentSubmissionFileByID(writer, request, assignmentID, *classID, fileID, false)
	case len(parts) == 5 && parts[1] == "submissions" && parts[2] == "files" && parts[4] == "preview":
		fileID, parseErr := strconv.ParseInt(parts[3], 10, 64)
		if parseErr != nil || fileID <= 0 {
			app.writeError(writer, http.StatusNotFound, "not_found", "附件不存在")
			return
		}
		app.handleAssignmentSubmissionFileByID(writer, request, assignmentID, *classID, fileID, true)
	default:
		app.writeError(writer, http.StatusNotFound, "not_found", "作业不存在")
	}
}

func (app *App) handleAssignmentAttachments(writer http.ResponseWriter, request *http.Request, assignmentID, classID int64) {
	switch request.Method {
	case http.MethodGet:
		items, err := app.listAssignmentAttachments(assignmentID, classID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, assignmentAttachmentsResult{Items: items})
	case http.MethodPost:
		if err := request.ParseMultipartForm(32 << 20); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "解析上传失败")
			return
		}
		headers := request.MultipartForm.File["files"]
		if len(headers) == 0 {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "缺少上传文件")
			return
		}
		items, err := app.createAssignmentAttachments(assignmentID, classID, headers)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusCreated, assignmentAttachmentsResult{Items: items})
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleAssignmentAttachmentByID(writer http.ResponseWriter, request *http.Request, assignmentID, classID, fileID int64, download bool) {
	switch {
	case download && request.Method == http.MethodGet:
		entry, err := app.findAssignmentAttachmentByID(assignmentID, classID, fileID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.serveFileEntryContent(writer, request, entry, true)
	case !download && request.Method == http.MethodDelete:
		if err := app.deleteAssignmentAttachment(assignmentID, classID, fileID); err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleAssignmentSubmissions(writer http.ResponseWriter, request *http.Request, assignmentID, classID int64) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	query, err := parseTeacherListQuery(request.URL.Query(), "updatedAt-desc", map[string]struct{}{
		"updatedAt-desc":   {},
		"updatedAt-asc":    {},
		"studentNo-asc":    {},
		"studentNo-desc":   {},
		"displayName-asc":  {},
		"displayName-desc": {},
	})
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	items, pagination, err := app.listAssignmentSubmissions(assignmentID, classID, query)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusOK, assignmentSubmissionsResult{
		Submissions:           items,
		SubmissionConstraints: studentSubmissionConstraintsPayload(),
		Pagination:            pagination,
	})
}

func (app *App) handleAssignmentSubmissionsArchive(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	classID, err := parseOptionalInt64(request.URL.Query().Get("classId"))
	if err != nil || classID == nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
		return
	}
	assignmentIDs, err := parseAssignmentArchiveIDs(request.URL.Query().Get("assignmentIds"))
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	archiveData, fileName, err := app.buildAssignmentSubmissionsArchive(*classID, assignmentIDs)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(archiveData)
}

func (app *App) handleAssignmentSubmissionFileByID(writer http.ResponseWriter, request *http.Request, assignmentID, classID, fileID int64, preview bool) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	entry, err := app.findAssignmentSubmissionFileByID(assignmentID, classID, fileID)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if preview {
		app.serveFileEntryContent(writer, request, entry, false)
		return
	}
	app.serveSubmissionEntryDownload(writer, request, entry)
}

func (app *App) handleAssignmentSubmissionByID(writer http.ResponseWriter, request *http.Request, assignmentID, classID, submissionID int64, teacher teacherRecord) {
	if request.Method != http.MethodPatch {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	var payload updateAssignmentSubmissionReviewRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	item, err := app.updateAssignmentSubmissionReview(assignmentID, classID, submissionID, payload, teacher)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusOK, item)
}

func (app *App) handleStudents(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	switch request.Method {
	case http.MethodGet:
		classID, err := parseOptionalInt64(request.URL.Query().Get("classId"))
		if err != nil || classID == nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
			return
		}
		query, err := parseTeacherListQuery(request.URL.Query(), "studentNo-asc", map[string]struct{}{
			"studentNo-asc":    {},
			"studentNo-desc":   {},
			"displayName-asc":  {},
			"displayName-desc": {},
			"registered-desc":  {},
			"registered-asc":   {},
		})
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		students, pagination, listErr := app.listStudentsPage(*classID, query)
		if listErr != nil {
			app.writeDomainError(writer, listErr)
			return
		}
		app.writeJSON(writer, http.StatusOK, studentsResult{Students: students, Pagination: pagination})
	case http.MethodPost:
		var payload createStudentRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		created, createErr := app.createStudent(payload)
		if createErr != nil {
			app.writeDomainError(writer, createErr)
			return
		}
		app.writeJSON(writer, http.StatusCreated, created)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleStudentByID(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	trimmed := strings.Trim(strings.TrimPrefix(request.URL.Path, "/api/students/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		app.writeError(writer, http.StatusNotFound, "not_found", "学生不存在")
		return
	}
	studentID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || studentID <= 0 {
		app.writeError(writer, http.StatusNotFound, "not_found", "学生不存在")
		return
	}

	switch {
	case len(parts) == 1 && request.Method == http.MethodPatch:
		var payload updateStudentRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		updated, updateErr := app.updateStudent(studentID, payload)
		if updateErr != nil {
			app.writeDomainError(writer, updateErr)
			return
		}
		app.writeJSON(writer, http.StatusOK, updated)
	case len(parts) == 1 && request.Method == http.MethodDelete:
		if err := app.deleteStudent(studentID); err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
	case len(parts) == 2 && parts[1] == "reset-password" && request.Method == http.MethodPost:
		result, resetErr := app.resetStudentPassword(studentID)
		if resetErr != nil {
			app.writeDomainError(writer, resetErr)
			return
		}
		app.writeJSON(writer, http.StatusOK, result)
	case len(parts) == 2 && parts[1] == "reset-password":
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	case len(parts) == 1:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	default:
		app.writeError(writer, http.StatusNotFound, "not_found", "学生不存在")
	}
}

func (app *App) handleImportStudents(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "仅支持通过 XLSX 文件导入学生")
}

func (app *App) handleImportStudentsFile(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	if err := request.ParseMultipartForm(8 << 20); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "解析导入文件失败")
		return
	}
	classID, err := parseOptionalInt64(request.FormValue("classId"))
	if err != nil || classID == nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
		return
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "缺少导入文件")
		return
	}
	defer file.Close()

	imported, importErr := app.importStudentsFromFile(*classID, header.Filename, file)
	if importErr != nil {
		app.writeDomainError(writer, importErr)
		return
	}
	app.writeJSON(writer, http.StatusCreated, studentsResult{Students: imported})
}

func (app *App) handleStudentImportTemplate(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	contents, contentType, fileName, err := buildStudentImportTemplate(request.URL.Query().Get("format"))
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}

	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(contents)
}

func (app *App) handleFiles(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	switch request.Method {
	case http.MethodGet:
		space := request.URL.Query().Get("space")
		query, err := parseFileListQuery(request.URL.Query())
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		classID, err := parseOptionalInt64(request.URL.Query().Get("classId"))
		if err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
			return
		}
		currentPath, err := normalizeListPath(request.URL.Query().Get("path"))
		if err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		items, pagination, err := app.listFileEntriesPage(space, classID, currentPath, query)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		result := fileListResult{
			Space:       space,
			CurrentPath: currentPath,
			Items:       make([]fileSummary, 0, len(items)),
			Pagination:  pagination,
		}
		if classID != nil {
			result.ClassID = *classID
		}
		for _, item := range items {
			result.Items = append(result.Items, buildDefaultFileSummary(item))
		}
		app.writeJSON(writer, http.StatusOK, result)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleFileSearch(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	space := request.URL.Query().Get("space")
	classID, err := parseOptionalInt64(request.URL.Query().Get("classId"))
	if err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
		return
	}
	currentPath, err := normalizeListPath(request.URL.Query().Get("path"))
	if err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("q"))
	if query == "" {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "q 不能为空")
		return
	}
	listQuery, err := parseFileListQuery(request.URL.Query())
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	items, err := app.searchFileEntries(space, classID, currentPath, query)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if err := app.applyFileEntryFolderSizes(items); err != nil {
		app.writeDomainError(writer, err)
		return
	}
	items, pagination := sortAndPaginateFileEntries(items, listQuery)
	result := fileListResult{
		Space:       space,
		CurrentPath: currentPath,
		Items:       make([]fileSummary, 0, len(items)),
		Pagination:  pagination,
	}
	if classID != nil {
		result.ClassID = *classID
	}
	for _, item := range items {
		result.Items = append(result.Items, buildDefaultFileSummary(item))
	}
	app.writeJSON(writer, http.StatusOK, result)
}

func (app *App) handleCreateFolder(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	var payload createFolderRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	entry, err := app.createFolder(payload.Space, optionalInt64(payload.ClassID), payload.ParentPath, payload.Name)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusCreated, buildDefaultFileSummary(*entry))
}

func (app *App) handleCreateFile(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	var payload createFileRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	result, err := app.createEditableTextFile(payload.Space, optionalInt64(payload.ClassID), payload.ParentPath, payload.Name, payload.Content)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusCreated, result)
}

func (app *App) handleUpload(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	if err := request.ParseMultipartForm(32 << 20); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "解析上传失败")
		return
	}

	space := request.FormValue("space")
	parentPath := request.FormValue("parentPath")
	classID, err := parseOptionalInt64(request.FormValue("classId"))
	if err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "classId 非法")
		return
	}
	conflictMode, err := normalizeUploadConflictMode(request.FormValue("conflictMode"))
	if err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	headers := request.MultipartForm.File["files"]
	if len(headers) == 0 {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "缺少上传文件")
		return
	}
	relativePaths := request.MultipartForm.Value["relativePaths"]

	created := make([]fileSummary, 0, len(headers))
	summary := uploadSummary{}
	rootReplacements := make(map[string]*string)

	for index, header := range headers {
		relativePath := ""
		if index < len(relativePaths) {
			relativePath = relativePaths[index]
		}
		targetParentPath, targetName, action, resolveErr := app.resolveUploadTarget(space, classID, parentPath, header.Filename, relativePath, conflictMode, rootReplacements)
		if resolveErr != nil {
			app.writeDomainError(writer, resolveErr)
			return
		}
		if action == uploadActionSkipped {
			summary.SkippedCount++
			continue
		}
		file, openErr := header.Open()
		if openErr != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", openErr.Error())
			return
		}
		entry, createAction, createErr := app.createFileWithConflict(space, classID, targetParentPath, targetName, file, conflictMode)
		_ = file.Close()
		if createErr != nil {
			app.writeDomainError(writer, createErr)
			return
		}
		if entry != nil {
			created = append(created, buildDefaultFileSummary(*entry))
		}
		switch createAction {
		case uploadActionCreated:
			summary.CreatedCount++
		case uploadActionReplaced:
			summary.CreatedCount++
			summary.ReplacedCount++
		case uploadActionRenamed:
			summary.CreatedCount++
			summary.RenamedCount++
		case uploadActionSkipped:
			summary.SkippedCount++
		}
	}
	app.writeJSON(writer, http.StatusCreated, uploadFilesResult{
		Items:   created,
		Summary: summary,
	})
}

func (app *App) handleUploadSessions(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if err := requireTUSResumable(request); err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}

	session, err := app.createUploadSession(request)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}

	writer.Header().Set("Location", "/api/files/upload/sessions/"+session.ID)
	app.writeUploadSessionHeaders(writer, session)
	writer.WriteHeader(http.StatusCreated)
}

func (app *App) handleUploadSessionByID(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if err := requireTUSResumable(request); err != nil {
		app.writeDomainError(writer, err)
		return
	}

	trimmed := strings.TrimPrefix(request.URL.Path, "/api/files/upload/sessions/")
	sessionID := strings.Trim(trimmed, "/")
	if sessionID == "" || strings.Contains(sessionID, "/") {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}

	switch request.Method {
	case http.MethodHead:
		session, err := app.loadUploadSession(sessionID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeUploadSessionHeaders(writer, session)
		writer.WriteHeader(http.StatusOK)
	case http.MethodPatch:
		session, err := app.appendUploadSession(sessionID, request)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeUploadSessionHeaders(writer, session)
		writer.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := app.removeUploadSession(sessionID); err != nil {
			app.writeDomainError(writer, err)
			return
		}
		writer.Header().Set("Tus-Resumable", tusResumableVersion)
		writer.WriteHeader(http.StatusNoContent)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleCopyFile(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	var payload copyFileRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	entry, err := app.copyEntry(payload.EntryID, payload.DestinationSpace, optionalInt64(payload.DestinationClassID), payload.DestinationParentPath)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusCreated, buildDefaultFileSummary(*entry))
}

func (app *App) handleMoveFile(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodPost {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	var payload copyFileRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	entry, err := app.moveEntry(payload.EntryID, payload.DestinationSpace, optionalInt64(payload.DestinationClassID), payload.DestinationParentPath)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.writeJSON(writer, http.StatusOK, buildDefaultFileSummary(*entry))
}

func (app *App) handleFileByID(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	trimmed := strings.TrimPrefix(request.URL.Path, "/api/files/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}
	entryID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}

	switch {
	case len(parts) == 1 && request.Method == http.MethodDelete:
		if err := app.deleteEntry(entryID); err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
	case len(parts) == 1 && request.Method == http.MethodPatch:
		var payload renameRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		entry, err := app.renameEntry(entryID, payload.Name)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, buildDefaultFileSummary(*entry))
	case len(parts) == 2 && parts[1] == "download" && request.Method == http.MethodGet:
		app.serveEntryContent(writer, request, entryID, true)
	case len(parts) == 2 && parts[1] == "archive" && request.Method == http.MethodGet:
		app.serveEntryArchive(writer, request, entryID)
	case len(parts) == 2 && parts[1] == "preview" && request.Method == http.MethodGet:
		app.serveEntryContent(writer, request, entryID, false)
	case len(parts) == 2 && parts[1] == "content":
		app.handleFileContent(writer, request, entryID)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleFileContent(writer http.ResponseWriter, request *http.Request, entryID int64) {
	switch request.Method {
	case http.MethodGet:
		result, err := app.readFileContent(entryID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, result)
	case http.MethodPut:
		var payload updateFileContentRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		result, err := app.saveFileContent(entryID, payload.Content)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, result)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) serveEntryContent(writer http.ResponseWriter, request *http.Request, entryID int64, attachment bool) {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.serveFileEntryContent(writer, request, entry, attachment)
}

func (app *App) serveFileEntryContent(writer http.ResponseWriter, request *http.Request, entry *fileEntry, attachment bool) {
	if entry.Kind != "file" {
		app.writeError(writer, http.StatusNotFound, "not_found", "仅文件可预览")
		return
	}
	absolutePath := filepath.Join(app.storageRoot, filepath.FromSlash(entry.DiskPath))
	if attachment {
		writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", entry.Name))
	} else {
		writer.Header().Set("Content-Disposition", "inline")
	}
	if entry.MimeType != "" {
		writer.Header().Set("Content-Type", entry.MimeType)
	}
	http.ServeFile(writer, request, absolutePath)
}

func (app *App) readFileContent(entryID int64) (fileContentResult, error) {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		return fileContentResult{}, err
	}
	content, err := app.readEditableTextContent(entry)
	if err != nil {
		return fileContentResult{}, err
	}
	return fileContentResult{
		Item:    buildDefaultFileSummary(*entry),
		Content: content,
	}, nil
}

func (app *App) saveFileContent(entryID int64, content string) (fileContentResult, error) {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		return fileContentResult{}, err
	}
	if err := validateEditableTextEntry(entry); err != nil {
		return fileContentResult{}, err
	}

	data := []byte(content)
	if len(data) > editableTextFileMaxSize {
		return fileContentResult{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "文本文件超过 1 MB，暂不支持在线编辑"}
	}

	absolutePath := filepath.Join(app.storageRoot, filepath.FromSlash(entry.DiskPath))
	if err := os.WriteFile(absolutePath, data, 0o644); err != nil {
		return fileContentResult{}, err
	}

	entry.Size = int64(len(data))
	entry.MimeType = contentTypeForFile(entry.Name, data)
	entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := app.db.Exec(`update file_entries set size = ?, mime_type = ?, updated_at = ? where id = ?`, entry.Size, entry.MimeType, entry.UpdatedAt, entry.ID); err != nil {
		return fileContentResult{}, err
	}

	return fileContentResult{
		Item:    buildDefaultFileSummary(*entry),
		Content: content,
	}, nil
}

func (app *App) createEditableTextFile(space string, classID *int64, parentPath, name, content string) (fileContentResult, error) {
	data := []byte(content)
	if len(data) > editableTextFileMaxSize {
		return fileContentResult{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "文本文件超过 1 MB，暂不支持在线编辑"}
	}
	if !isEditableTextFile(name, contentTypeForFile(name, data)) {
		return fileContentResult{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "当前文件不支持在线编辑"}
	}

	entry, err := app.createFile(space, classID, parentPath, name, strings.NewReader(content))
	if err != nil {
		return fileContentResult{}, err
	}

	return fileContentResult{
		Item:    buildDefaultFileSummary(*entry),
		Content: content,
	}, nil
}

func (app *App) serveEntryArchive(writer http.ResponseWriter, request *http.Request, entryID int64) {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if entry.Kind == "file" {
		app.serveFileEntryContent(writer, request, entry, true)
		return
	}
	if entry.Kind != "dir" {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}

	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", entry.Name+".zip"))
	writer.WriteHeader(http.StatusOK)
	if err := app.writeDirectoryArchive(writer, entry); err != nil {
		log.Printf("stream directory archive %d: %v", entry.ID, err)
	}
}

func (app *App) handleFilesBatchArchive(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	if request.Method != http.MethodGet {
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
		return
	}
	entryIDs, err := parseFileArchiveEntryIDs(request.URL.Query().Get("entryIds"))
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}

	entries := make([]fileEntry, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		entry, err := app.getEntryByID(entryID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		entries = append(entries, *entry)
	}

	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", `attachment; filename="资料下载.zip"`)
	writer.WriteHeader(http.StatusOK)
	if err := app.writeFileEntriesArchive(writer, entries); err != nil {
		log.Printf("stream batch file archive: %v", err)
	}
}

func studentFileSpace(raw string, student studentRecord) (string, *int64, error) {
	switch strings.TrimSpace(raw) {
	case "", "public":
		return "public", nil, nil
	case "class":
		classID := student.ClassID
		return "class", &classID, nil
	case "library":
		return "", nil, domainError{Status: http.StatusForbidden, Code: "forbidden", Message: "学生端不能访问老师资料"}
	default:
		return "", nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "未知资料空间"}
	}
}

func (app *App) getStudentVisibleFileEntry(entryID int64, student studentRecord) (*fileEntry, error) {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		return nil, err
	}
	if entry.Space == "public" && !entry.ClassID.Valid {
		return entry, nil
	}
	if entry.Space == "class" && entry.ClassID.Valid && entry.ClassID.Int64 == student.ClassID {
		return entry, nil
	}
	return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "资源不存在"}
}

func (app *App) serveStudentFileArchive(writer http.ResponseWriter, request *http.Request, entry *fileEntry) {
	if entry.Kind == "file" {
		app.serveFileEntryContent(writer, request, entry, true)
		return
	}
	if entry.Kind != "dir" {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}

	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", entry.Name+".zip"))
	writer.WriteHeader(http.StatusOK)
	if err := app.writeDirectoryArchive(writer, entry); err != nil {
		log.Printf("stream student directory archive %d: %v", entry.ID, err)
	}
}

func (app *App) serveSubmissionEntryDownload(writer http.ResponseWriter, request *http.Request, entry *fileEntry) {
	if entry.Kind == "file" {
		app.serveFileEntryContent(writer, request, entry, true)
		return
	}
	if entry.Kind != "dir" {
		app.writeError(writer, http.StatusNotFound, "not_found", "附件不存在")
		return
	}
	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", entry.Name+".zip"))
	writer.WriteHeader(http.StatusOK)
	if err := app.writeDirectoryArchive(writer, entry); err != nil {
		log.Printf("stream submission directory archive %d: %v", entry.ID, err)
	}
}

func (app *App) writeDirectoryArchive(output io.Writer, entry *fileEntry) error {
	descendants, err := app.listDescendants(entry)
	if err != nil {
		return err
	}

	archive := zip.NewWriter(output)
	if _, err := archive.Create(entry.Name + "/"); err != nil {
		return err
	}
	for _, item := range descendants {
		relativePath := strings.TrimPrefix(item.ItemPath, entry.ItemPath+"/")
		archivePath := path.Join(entry.Name, relativePath)
		if item.Kind == "dir" {
			if _, err := archive.Create(archivePath + "/"); err != nil {
				_ = archive.Close()
				return err
			}
			continue
		}
		if item.Kind != "file" {
			_ = archive.Close()
			return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "目录归档包含未知资源类型"}
		}
		header := &zip.FileHeader{
			Name:   archivePath,
			Method: zip.Deflate,
		}
		if modifiedAt, parseErr := time.Parse(time.RFC3339, item.UpdatedAt); parseErr == nil {
			header.SetModTime(modifiedAt)
		}
		writer, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			return err
		}
		absolutePath := filepath.Join(app.storageRoot, filepath.FromSlash(item.DiskPath))
		file, err := os.Open(absolutePath)
		if err != nil {
			_ = archive.Close()
			return err
		}
		_, copyErr := io.Copy(writer, file)
		_ = file.Close()
		if copyErr != nil {
			_ = archive.Close()
			return copyErr
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	return nil
}

func (app *App) writeFileEntriesArchive(output io.Writer, entries []fileEntry) error {
	if len(entries) == 0 {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "请选择要下载的资料"}
	}

	archive := zip.NewWriter(output)
	createdDirs := make(map[string]struct{})
	usedRootNames := make(map[string]int, len(entries))
	for _, entry := range entries {
		rootName := uniqueArchiveSegment(safeArchiveSegment(entry.Name, "未命名资料"), usedRootNames)
		switch entry.Kind {
		case "file":
			if err := app.writeArchiveFile(archive, entry, rootName); err != nil {
				_ = archive.Close()
				return err
			}
		case "dir":
			if err := app.writeDirectoryEntryArchive(archive, createdDirs, entry, rootName); err != nil {
				_ = archive.Close()
				return err
			}
		default:
			_ = archive.Close()
			return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "资料归档包含未知资源类型"}
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	return nil
}

func (app *App) writeDirectoryEntryArchive(archive *zip.Writer, createdDirs map[string]struct{}, entry fileEntry, rootName string) error {
	if err := ensureZipDirectory(archive, createdDirs, rootName); err != nil {
		return err
	}
	descendants, err := app.listDescendants(&entry)
	if err != nil {
		return err
	}
	for _, item := range descendants {
		relativePath := strings.TrimPrefix(item.ItemPath, entry.ItemPath+"/")
		archivePath := path.Join(rootName, relativePath)
		switch item.Kind {
		case "dir":
			if err := ensureZipDirectory(archive, createdDirs, archivePath); err != nil {
				return err
			}
		case "file":
			if err := app.writeArchiveFile(archive, item, archivePath); err != nil {
				return err
			}
		default:
			return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "目录归档包含未知资源类型"}
		}
	}
	return nil
}

func (app *App) handleFrontend(writer http.ResponseWriter, request *http.Request) {
	sub, err := fs.Sub(frontendAssets, "dist")
	if err != nil {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(writer, "<!doctype html><html><body><h1>ClassDrive frontend not built</h1></body></html>")
		return
	}

	if request.URL.Path != "/" && request.URL.Path != "/login" && !strings.HasPrefix(request.URL.Path, "/student") && !strings.HasPrefix(request.URL.Path, "/files") && !strings.HasPrefix(request.URL.Path, "/classes") && !strings.HasPrefix(request.URL.Path, "/assignments") && !strings.HasPrefix(request.URL.Path, "/students") && !strings.HasPrefix(request.URL.Path, "/settings") {
		trimmed := strings.TrimPrefix(request.URL.Path, "/")
		if file, openErr := sub.Open(trimmed); openErr == nil {
			_ = file.Close()
			http.FileServer(http.FS(sub)).ServeHTTP(writer, request)
			return
		}
		http.NotFound(writer, request)
		return
	}

	index, readErr := fs.ReadFile(sub, "index.html")
	if readErr != nil {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(writer, "<!doctype html><html><body><h1>ClassDrive frontend not built</h1></body></html>")
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write(index)
}

type teacherRecord struct {
	ID           int64
	Username     string
	DisplayName  string
	PasswordHash string
	Role         string
	Disabled     bool
}

type classRecord struct {
	ID                    int64
	Name                  string
	JoinCode              string
	RegistrationEnabled   bool
	RegistrationExpiresAt string
	ExamLockEnabled       bool
}

type studentRecord struct {
	ID                 int64
	ClassID            int64
	StudentNo          string
	DisplayName        string
	PasswordHash       string
	ActivatedAt        string
	ClassName          string
	MustChangePassword bool
}

type assignmentSubmissionRecord struct {
	ID             int64
	AssignmentID   int64
	StudentID      int64
	Status         string
	SubmittedAt    string
	UpdatedAt      string
	ReviewStatus   string
	TeacherComment string
	ReviewedAt     string
	ReviewerName   string
}

func (record teacherRecord) toSessionUser() sessionUser {
	return sessionUser{
		ID:          record.ID,
		Username:    record.Username,
		DisplayName: record.DisplayName,
		Role:        record.Role,
	}
}

func (record studentRecord) toSessionUser() studentSessionUser {
	return studentSessionUser{
		ID:                 record.ID,
		ClassID:            record.ClassID,
		StudentNo:          record.StudentNo,
		DisplayName:        record.DisplayName,
		ClassName:          record.ClassName,
		MustChangePassword: record.MustChangePassword,
	}
}

func (record assignmentSubmissionRecord) toSummary() assignmentSubmissionSummary {
	return assignmentSubmissionSummary{
		ID:          record.ID,
		Status:      record.Status,
		SubmittedAt: record.SubmittedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

func (record teacherRecord) toTeacherUserPayload() teacherUserPayload {
	return teacherUserPayload{
		ID:          record.ID,
		Username:    record.Username,
		DisplayName: record.DisplayName,
		Role:        record.Role,
		Disabled:    record.Disabled,
	}
}

func buildTeacherProfilePayload(record teacherRecord, preferences teacherProfilePreferences) teacherProfilePayload {
	return teacherProfilePayload{
		ID:          record.ID,
		Username:    record.Username,
		DisplayName: record.DisplayName,
		Role:        record.Role,
		Preferences: preferences,
	}
}

type sessionHandler func(http.ResponseWriter, *http.Request, teacherRecord)

type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (writer *auditResponseWriter) WriteHeader(statusCode int) {
	writer.statusCode = statusCode
	writer.ResponseWriter.WriteHeader(statusCode)
}

func (app *App) withSession(next sessionHandler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		teacher, err := app.requireTeacher(request)
		if err != nil {
			app.writeError(writer, http.StatusUnauthorized, "unauthorized", "未登录")
			return
		}
		if !app.validateCSRF(writer, request) {
			return
		}
		recorder := &auditResponseWriter{ResponseWriter: writer, statusCode: http.StatusOK}
		next(recorder, request, *teacher)
		app.recordOperationLog(request, "teacher", teacher.ID, teacher.DisplayName, recorder.statusCode)
	}
}

type studentSessionHandler func(http.ResponseWriter, *http.Request, studentRecord)

func (app *App) withStudentSession(next studentSessionHandler) http.HandlerFunc {
	return app.withStudentSessionAccess(next, true)
}

func (app *App) withStudentSessionAccess(next studentSessionHandler, blockPasswordChangeRequired bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		student, err := app.requireStudent(request)
		if err != nil {
			app.writeError(writer, http.StatusUnauthorized, "unauthorized", "未登录")
			return
		}
		if blockPasswordChangeRequired && student.MustChangePassword {
			app.writeError(writer, http.StatusConflict, "password_change_required", "请先修改初始密码")
			return
		}
		if !app.validateCSRF(writer, request) {
			return
		}
		recorder := &auditResponseWriter{ResponseWriter: writer, statusCode: http.StatusOK}
		next(recorder, request, *student)
		app.recordOperationLog(request, "student", student.ID, student.DisplayName, recorder.statusCode)
	}
}

func (app *App) requireTeacher(request *http.Request) (*teacherRecord, error) {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return nil, errors.New("missing token")
	}
	teacher, err := app.findTeacherBySession(token)
	if err != nil {
		return nil, err
	}
	if teacher.Disabled {
		return nil, errors.New("teacher disabled")
	}
	return teacher, nil
}

func (app *App) requireStudent(request *http.Request) (*studentRecord, error) {
	cookie, err := request.Cookie(studentSessionCookieName)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(cookie.Value)
	if token == "" {
		return nil, errors.New("missing token")
	}
	return app.findStudentBySession(token)
}

func decodeJSONBody(request *http.Request, target any) error {
	defer request.Body.Close()
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func requireOwnerTeacher(teacher teacherRecord) error {
	if teacher.Role != teacherRoleOwner {
		return domainError{Status: http.StatusForbidden, Code: "forbidden", Message: "当前账号无权访问"}
	}
	return nil
}

func generateToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func generateJoinCode() (string, error) {
	const alphabet = "0123456789"
	max := big.NewInt(int64(len(alphabet)))
	var builder strings.Builder
	builder.Grow(classJoinCodeLen)
	for range classJoinCodeLen {
		item, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[item.Int64()])
	}
	return builder.String(), nil
}

const (
	sharePermissionView     = "view"
	sharePermissionDownload = "download"
	shareAccessCodeLength   = 6

	shareSessionCookieName = "classdrive_share_session"
	shareSessionDuration   = 2 * time.Hour

	// 内存级安全码防爆破（LAN 场景，可一处关闭）。
	shareBruteForceGuardEnabled = true
	shareMaxFailedAttempts      = 5
	shareCooldown               = 30 * time.Second

	defaultShareExpiresDaysFallback = 7
	maxShareExpiresDays             = 3650
)

// generateShareAccessCode 生成无歧义大写字母数字安全码（去掉 0/O/1/I/L），逐位使用 crypto/rand。
func generateShareAccessCode() (string, error) {
	const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	max := big.NewInt(int64(len(alphabet)))
	var builder strings.Builder
	builder.Grow(shareAccessCodeLength)
	for range shareAccessCodeLength {
		item, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[item.Int64()])
	}
	return builder.String(), nil
}

type libraryShareRecord struct {
	ID                 int64
	Token              string
	AccessCodeHash     string
	EntryID            int64
	Permission         string
	ExpiresAt          string
	Disabled           bool
	CreatedByTeacherID int64
	CreatedAt          string
	UpdatedAt          string
	LastAccessedAt     string
	AccessCount        int64
}

type libraryShareItem struct {
	ID                 int64  `json:"id"`
	Token              string `json:"token"`
	EntryID            int64  `json:"entryId"`
	EntryName          string `json:"entryName"`
	EntryKind          string `json:"entryKind"`
	Permission         string `json:"permission"`
	RequiresAccessCode bool   `json:"requiresAccessCode"`
	ExpiresAt          string `json:"expiresAt"`
	Disabled           bool   `json:"disabled"`
	Status             string `json:"status"`
	AccessCount        int64  `json:"accessCount"`
	CreatedByName      string `json:"createdByName"`
	CreatedAt          string `json:"createdAt"`
	LastAccessedAt     string `json:"lastAccessedAt"`
}

type librarySharesResult struct {
	Shares []libraryShareItem `json:"shares"`
}

type libraryShareResult struct {
	Share libraryShareItem `json:"share"`
}

type createLibraryShareRequest struct {
	EntryID           int64  `json:"entryId"`
	Permission        string `json:"permission"`
	RequireAccessCode bool   `json:"requireAccessCode"`
	ExpiresAt         string `json:"expiresAt"`
}

type updateLibraryShareRequest struct {
	Permission *string `json:"permission"`
	ExpiresAt  *string `json:"expiresAt"`
	Disabled   *bool   `json:"disabled"`
}

type createLibraryShareResult struct {
	Share      libraryShareItem `json:"share"`
	AccessCode string           `json:"accessCode"`
}

func normalizeSharePermission(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "", sharePermissionView:
		return sharePermissionView, nil
	case sharePermissionDownload:
		return sharePermissionDownload, nil
	default:
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "分享权限只能是仅查看或允许下载"}
	}
}

func normalizeShareExpiresAt(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "有效期格式不正确"}
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func libraryShareExpired(rec libraryShareRecord) bool {
	if strings.TrimSpace(rec.ExpiresAt) == "" {
		return false
	}
	expires, err := time.Parse(time.RFC3339, rec.ExpiresAt)
	if err != nil {
		return false
	}
	return time.Now().UTC().After(expires)
}

func (app *App) teacherDisplayName(teacherID int64) string {
	var name string
	if err := app.db.QueryRow(`select display_name from teachers where id = ?`, teacherID).Scan(&name); err != nil {
		return ""
	}
	return name
}

// shareEntry 返回分享指向的有效老师资料条目；条目被删除或已移出 library 时返回 false。
func (app *App) shareEntry(rec libraryShareRecord) (*fileEntry, bool) {
	entry, err := app.getEntryByID(rec.EntryID)
	if err != nil {
		return nil, false
	}
	if entry.Space != "library" {
		return nil, false
	}
	return entry, true
}

func (app *App) buildLibraryShareItem(rec libraryShareRecord) libraryShareItem {
	item := libraryShareItem{
		ID:                 rec.ID,
		Token:              rec.Token,
		EntryID:            rec.EntryID,
		Permission:         rec.Permission,
		RequiresAccessCode: strings.TrimSpace(rec.AccessCodeHash) != "",
		ExpiresAt:          rec.ExpiresAt,
		Disabled:           rec.Disabled,
		AccessCount:        rec.AccessCount,
		CreatedByName:      app.teacherDisplayName(rec.CreatedByTeacherID),
		CreatedAt:          rec.CreatedAt,
		LastAccessedAt:     rec.LastAccessedAt,
	}
	entry, valid := app.shareEntry(rec)
	if valid {
		item.EntryName = entry.Name
		item.EntryKind = entry.Kind
	}
	switch {
	case rec.Disabled:
		item.Status = "disabled"
	case libraryShareExpired(rec):
		item.Status = "expired"
	case !valid:
		item.Status = "invalid"
	default:
		item.Status = "active"
	}
	return item
}

func (app *App) getLibraryShareByID(id int64) (*libraryShareRecord, error) {
	row := app.db.QueryRow(`
select id, token, access_code_hash, entry_id, permission, expires_at, disabled, created_by_teacher_id, created_at, updated_at, last_accessed_at, access_count
from library_shares where id = ?`, id)
	return scanLibraryShareRecord(row)
}

func scanLibraryShareRecord(row interface{ Scan(...any) error }) (*libraryShareRecord, error) {
	var rec libraryShareRecord
	if err := row.Scan(&rec.ID, &rec.Token, &rec.AccessCodeHash, &rec.EntryID, &rec.Permission, &rec.ExpiresAt, &rec.Disabled, &rec.CreatedByTeacherID, &rec.CreatedAt, &rec.UpdatedAt, &rec.LastAccessedAt, &rec.AccessCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "分享不存在"}
		}
		return nil, err
	}
	return &rec, nil
}

func (app *App) listLibraryShares() ([]libraryShareItem, error) {
	rows, err := app.db.Query(`
select id, token, access_code_hash, entry_id, permission, expires_at, disabled, created_by_teacher_id, created_at, updated_at, last_accessed_at, access_count
from library_shares order by created_at desc, id desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]libraryShareItem, 0)
	for rows.Next() {
		rec, err := scanLibraryShareRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, app.buildLibraryShareItem(*rec))
	}
	return items, rows.Err()
}

func (app *App) createLibraryShare(teacher teacherRecord, req createLibraryShareRequest) (createLibraryShareResult, error) {
	entry, err := app.getEntryByID(req.EntryID)
	if err != nil {
		return createLibraryShareResult{}, err
	}
	if entry.Space != "library" {
		return createLibraryShareResult{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "只能分享老师资料"}
	}
	permission, err := normalizeSharePermission(req.Permission)
	if err != nil {
		return createLibraryShareResult{}, err
	}
	expiresAt, err := normalizeShareExpiresAt(req.ExpiresAt)
	if err != nil {
		return createLibraryShareResult{}, err
	}

	accessCode := ""
	accessCodeHash := ""
	if req.RequireAccessCode {
		accessCode, err = generateShareAccessCode()
		if err != nil {
			return createLibraryShareResult{}, err
		}
		accessCodeHash, err = hashPassword(accessCode)
		if err != nil {
			return createLibraryShareResult{}, err
		}
	}

	token, err := generateToken()
	if err != nil {
		return createLibraryShareResult{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := app.db.Exec(`
insert into library_shares (token, access_code_hash, entry_id, permission, expires_at, disabled, created_by_teacher_id, created_at, updated_at, last_accessed_at, access_count)
values (?, ?, ?, ?, ?, 0, ?, ?, ?, '', 0)`,
		token, accessCodeHash, entry.ID, permission, expiresAt, teacher.ID, now, now)
	if err != nil {
		return createLibraryShareResult{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return createLibraryShareResult{}, err
	}
	rec, err := app.getLibraryShareByID(id)
	if err != nil {
		return createLibraryShareResult{}, err
	}
	return createLibraryShareResult{Share: app.buildLibraryShareItem(*rec), AccessCode: accessCode}, nil
}

func (app *App) updateLibraryShare(id int64, req updateLibraryShareRequest) (libraryShareItem, error) {
	rec, err := app.getLibraryShareByID(id)
	if err != nil {
		return libraryShareItem{}, err
	}
	if req.Permission != nil {
		permission, err := normalizeSharePermission(*req.Permission)
		if err != nil {
			return libraryShareItem{}, err
		}
		rec.Permission = permission
	}
	if req.ExpiresAt != nil {
		expiresAt, err := normalizeShareExpiresAt(*req.ExpiresAt)
		if err != nil {
			return libraryShareItem{}, err
		}
		rec.ExpiresAt = expiresAt
	}
	if req.Disabled != nil {
		rec.Disabled = *req.Disabled
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := app.db.Exec(`update library_shares set permission = ?, expires_at = ?, disabled = ?, updated_at = ? where id = ?`,
		rec.Permission, rec.ExpiresAt, boolToInt(rec.Disabled), now, rec.ID); err != nil {
		return libraryShareItem{}, err
	}
	rec.UpdatedAt = now
	return app.buildLibraryShareItem(*rec), nil
}

func (app *App) deleteLibraryShare(id int64) error {
	rec, err := app.getLibraryShareByID(id)
	if err != nil {
		return err
	}
	if _, err := app.db.Exec(`delete from library_share_sessions where share_id = ?`, rec.ID); err != nil {
		return err
	}
	_, err = app.db.Exec(`delete from library_shares where id = ?`, rec.ID)
	return err
}

func (app *App) resetLibraryShareCode(id int64) (createLibraryShareResult, error) {
	rec, err := app.getLibraryShareByID(id)
	if err != nil {
		return createLibraryShareResult{}, err
	}
	accessCode, err := generateShareAccessCode()
	if err != nil {
		return createLibraryShareResult{}, err
	}
	accessCodeHash, err := hashPassword(accessCode)
	if err != nil {
		return createLibraryShareResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := app.db.Exec(`update library_shares set access_code_hash = ?, updated_at = ? where id = ?`, accessCodeHash, now, rec.ID); err != nil {
		return createLibraryShareResult{}, err
	}
	if _, err := app.db.Exec(`delete from library_share_sessions where share_id = ?`, rec.ID); err != nil {
		return createLibraryShareResult{}, err
	}
	rec.AccessCodeHash = accessCodeHash
	rec.UpdatedAt = now
	return createLibraryShareResult{Share: app.buildLibraryShareItem(*rec), AccessCode: accessCode}, nil
}

func (app *App) handleLibraryShares(writer http.ResponseWriter, request *http.Request, teacher teacherRecord) {
	switch request.Method {
	case http.MethodGet:
		shares, err := app.listLibraryShares()
		if err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		app.writeJSON(writer, http.StatusOK, librarySharesResult{Shares: shares})
	case http.MethodPost:
		var payload createLibraryShareRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		result, err := app.createLibraryShare(teacher, payload)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusCreated, result)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

func (app *App) handleLibraryShareByID(writer http.ResponseWriter, request *http.Request, _ teacherRecord) {
	trimmed := strings.TrimPrefix(request.URL.Path, "/api/library-shares/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		app.writeError(writer, http.StatusNotFound, "not_found", "分享不存在")
		return
	}
	shareID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		app.writeError(writer, http.StatusNotFound, "not_found", "分享不存在")
		return
	}

	switch {
	case len(parts) == 1 && request.Method == http.MethodPatch:
		var payload updateLibraryShareRequest
		if err := decodeJSONBody(request, &payload); err != nil {
			app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
			return
		}
		item, err := app.updateLibraryShare(shareID, payload)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, libraryShareResult{Share: item})
	case len(parts) == 1 && request.Method == http.MethodDelete:
		if err := app.deleteLibraryShare(shareID); err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
	case len(parts) == 2 && parts[1] == "reset-code" && request.Method == http.MethodPost:
		result, err := app.resetLibraryShareCode(shareID)
		if err != nil {
			app.writeDomainError(writer, err)
			return
		}
		app.writeJSON(writer, http.StatusOK, result)
	default:
		app.writeError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
	}
}

type shareInfo struct {
	EntryID            int64  `json:"entryId"`
	EntryName          string `json:"entryName"`
	EntryKind          string `json:"entryKind"`
	Permission         string `json:"permission"`
	RequiresAccessCode bool   `json:"requiresAccessCode"`
	ExpiresAt          string `json:"expiresAt"`
	Status             string `json:"status"`
}

type shareInfoResult struct {
	Info shareInfo `json:"info"`
}

type shareVerifyRequest struct {
	AccessCode string `json:"accessCode"`
}

type shareFileItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Size        int64  `json:"size"`
	MimeType    string `json:"mimeType"`
	UpdatedAt   string `json:"updatedAt"`
	PreviewURL  string `json:"previewUrl"`
	DownloadURL string `json:"downloadUrl"`
	ArchiveURL  string `json:"archiveUrl"`
}

type shareItemsResult struct {
	Items []shareFileItem `json:"items"`
	Path  string          `json:"path"`
}

var errShareNotFound = domainError{Status: http.StatusNotFound, Code: "not_found", Message: "分享不存在或已失效"}

func (app *App) getLibraryShareByToken(token string) (*libraryShareRecord, error) {
	row := app.db.QueryRow(`
select id, token, access_code_hash, entry_id, permission, expires_at, disabled, created_by_teacher_id, created_at, updated_at, last_accessed_at, access_count
from library_shares where token = ?`, strings.TrimSpace(token))
	rec, err := scanLibraryShareRecord(row)
	if err != nil {
		if domain, ok := err.(domainError); ok && domain.Status == http.StatusNotFound {
			return nil, errShareNotFound
		}
		return nil, err
	}
	return rec, nil
}

// resolveShareTarget 校验分享存在、未停用、未过期，且指向的老师资料条目仍有效。
func (app *App) resolveShareTarget(token string) (*libraryShareRecord, *fileEntry, error) {
	rec, err := app.getLibraryShareByToken(token)
	if err != nil {
		return nil, nil, err
	}
	if rec.Disabled || libraryShareExpired(*rec) {
		return nil, nil, errShareNotFound
	}
	entry, ok := app.shareEntry(*rec)
	if !ok {
		return nil, nil, errShareNotFound
	}
	return rec, entry, nil
}

// findLibraryShareByID 用于操作日志摘要查询，返回轻量结构
func (app *App) findLibraryShareByID(id int64, teacherID int64) (*struct{ entryName string }, error) {
	rec, err := app.getLibraryShareByID(id)
	if err != nil {
		return nil, err
	}
	entry, ok := app.shareEntry(*rec)
	if !ok {
		return nil, err
	}
	return &struct{ entryName string }{entryName: entry.Name}, nil
}

// findShareByToken 用于操作日志摘要查询，返回轻量结构
func (app *App) findShareByToken(token string) (*struct{ entryName string }, error) {
	rec, entry, err := app.resolveShareTarget(token)
	if err != nil {
		// 分享已失效/删除时仍返回基本信息用于日志
		rec, err = app.getLibraryShareByToken(token)
		if err != nil {
			return nil, err
		}
		// 尝试获取entry，即使已删除也尝试从数据库读取名称
		var entryName string
		if err := app.db.QueryRow(`select name from files where id = ?`, rec.EntryID).Scan(&entryName); err != nil {
			entryName = "(已删除)"
		}
		return &struct{ entryName string }{entryName: entryName}, nil
	}
	return &struct{ entryName string }{entryName: entry.Name}, nil
}

// authorizeShareEntry 把访问限制在分享根（文件分享=仅该文件；文件夹分享=根及其子孙）。
func (app *App) authorizeShareEntry(root *fileEntry, entryID int64) (*fileEntry, error) {
	forbidden := domainError{Status: http.StatusForbidden, Code: "forbidden", Message: "无权访问该资源"}
	if entryID == root.ID {
		return root, nil
	}
	if root.Kind != "dir" {
		return nil, forbidden
	}
	target, err := app.getEntryByID(entryID)
	if err != nil {
		return nil, errShareNotFound
	}
	if target.Space != "library" || classIDKey(target.ClassID) != classIDKey(root.ClassID) {
		return nil, forbidden
	}
	if !strings.HasPrefix(target.ItemPath, root.ItemPath+"/") {
		return nil, forbidden
	}
	return target, nil
}

func (app *App) issueShareSession(writer http.ResponseWriter, request *http.Request, rec *libraryShareRecord) error {
	token, err := generateToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(shareSessionDuration)
	if _, err := app.db.Exec(`insert into library_share_sessions (token, share_id, expires_at) values (?, ?, ?)`,
		token, rec.ID, expires.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	app.setAuthCookie(writer, request, shareSessionCookieName, token, expires)
	return nil
}

func (app *App) shareSessionValid(request *http.Request, shareID int64) bool {
	cookie, err := request.Cookie(shareSessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return false
	}
	var sessionShareID int64
	var expiresAt string
	if err := app.db.QueryRow(`select share_id, expires_at from library_share_sessions where token = ?`, cookie.Value).Scan(&sessionShareID, &expiresAt); err != nil {
		return false
	}
	if sessionShareID != shareID {
		return false
	}
	if expires, err := time.Parse(time.RFC3339, expiresAt); err == nil && time.Now().After(expires) {
		return false
	}
	return true
}

// authorizeShareView 判定访客是否已可查看内容：免码分享放行，否则需有效分享会话。
func (app *App) authorizeShareView(request *http.Request, rec *libraryShareRecord) bool {
	if rec.AccessCodeHash == "" {
		return true
	}
	return app.shareSessionValid(request, rec.ID)
}

func (app *App) touchShareAccess(shareID int64) {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := app.db.Exec(`update library_shares set access_count = access_count + 1, last_accessed_at = ? where id = ?`, now, shareID); err != nil {
		log.Printf("touch share access %d: %v", shareID, err)
	}
}

func (app *App) shareLocked(token string) bool {
	if !shareBruteForceGuardEnabled {
		return false
	}
	app.shareAttemptsMu.Lock()
	defer app.shareAttemptsMu.Unlock()
	attempt := app.shareAttempts[token]
	if attempt == nil {
		return false
	}
	return time.Now().Before(attempt.lockUntil)
}

func (app *App) recordShareFailure(token string) {
	if !shareBruteForceGuardEnabled {
		return
	}
	app.shareAttemptsMu.Lock()
	defer app.shareAttemptsMu.Unlock()

	// Clean up expired entries to prevent unbounded growth
	now := time.Now()
	if len(app.shareAttempts) > 100 {
		for key, attempt := range app.shareAttempts {
			// Remove entries that have been unlocked for more than 1 hour
			if attempt.lockUntil.Before(now) && now.Sub(attempt.lockUntil) > time.Hour {
				delete(app.shareAttempts, key)
			}
		}
	}

	attempt := app.shareAttempts[token]
	if attempt == nil {
		attempt = &shareAttempt{}
		app.shareAttempts[token] = attempt
	}
	attempt.failures++
	if attempt.failures >= shareMaxFailedAttempts {
		attempt.lockUntil = time.Now().Add(shareCooldown)
	}
}

func (app *App) clearShareAttempts(token string) {
	app.shareAttemptsMu.Lock()
	defer app.shareAttemptsMu.Unlock()
	delete(app.shareAttempts, token)
}

func (app *App) handlePublicShare(writer http.ResponseWriter, request *http.Request) {
	recorder := &auditResponseWriter{ResponseWriter: writer, statusCode: http.StatusOK}
	app.routePublicShare(recorder, request)
	app.recordOperationLog(request, "visitor", 0, "", recorder.statusCode)
}

func (app *App) routePublicShare(writer http.ResponseWriter, request *http.Request) {
	trimmed := strings.TrimPrefix(request.URL.Path, "/api/share/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		app.writeError(writer, http.StatusNotFound, "not_found", "分享不存在")
		return
	}
	token := parts[0]

	switch {
	case len(parts) == 1 && request.Method == http.MethodGet:
		app.handleShareInfo(writer, request, token)
	case len(parts) == 2 && parts[1] == "verify" && request.Method == http.MethodPost:
		app.handleShareVerify(writer, request, token)
	case len(parts) == 2 && parts[1] == "items" && request.Method == http.MethodGet:
		app.handleShareItems(writer, request, token)
	case len(parts) == 4 && parts[1] == "files" && parts[3] == "preview" && request.Method == http.MethodGet:
		app.handleShareContent(writer, request, token, parts[2], "preview")
	case len(parts) == 4 && parts[1] == "files" && parts[3] == "download" && request.Method == http.MethodGet:
		app.handleShareContent(writer, request, token, parts[2], "download")
	case len(parts) == 4 && parts[1] == "files" && parts[3] == "archive" && request.Method == http.MethodGet:
		app.handleShareContent(writer, request, token, parts[2], "archive")
	default:
		app.writeError(writer, http.StatusNotFound, "not_found", "分享不存在")
	}
}

func (app *App) handleShareInfo(writer http.ResponseWriter, request *http.Request, token string) {
	if err := app.ensureCSRFCookie(writer, request); err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	rec, root, err := app.resolveShareTarget(token)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if rec.AccessCodeHash == "" {
		if err := app.issueShareSession(writer, request, rec); err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
	}
	app.writeJSON(writer, http.StatusOK, shareInfoResult{Info: shareInfo{
		EntryID:            root.ID,
		EntryName:          root.Name,
		EntryKind:          root.Kind,
		Permission:         rec.Permission,
		RequiresAccessCode: rec.AccessCodeHash != "",
		ExpiresAt:          rec.ExpiresAt,
		Status:             "active",
	}})
}

func (app *App) handleShareVerify(writer http.ResponseWriter, request *http.Request, token string) {
	rec, _, err := app.resolveShareTarget(token)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if app.shareLocked(token) {
		app.writeError(writer, http.StatusTooManyRequests, "too_many_requests", "尝试过于频繁，请稍后再试")
		return
	}
	var payload shareVerifyRequest
	if err := decodeJSONBody(request, &payload); err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	if rec.AccessCodeHash == "" {
		if err := app.issueShareSession(writer, request, rec); err != nil {
			app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if verifyPasswordHash(rec.AccessCodeHash, strings.TrimSpace(payload.AccessCode)) != nil {
		app.recordShareFailure(token)
		app.writeError(writer, http.StatusForbidden, "invalid_access_code", "安全码不正确")
		return
	}
	app.clearShareAttempts(token)
	if err := app.issueShareSession(writer, request, rec); err != nil {
		app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	app.touchShareAccess(rec.ID)
	app.writeJSON(writer, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleShareItems(writer http.ResponseWriter, request *http.Request, token string) {
	rec, root, err := app.resolveShareTarget(token)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if !app.authorizeShareView(request, rec) {
		app.writeError(writer, http.StatusUnauthorized, "unauthorized", "请先输入安全码")
		return
	}
	if root.Kind != "dir" {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", "该分享不是文件夹")
		return
	}
	relativePath, err := normalizeRelativePath(request.URL.Query().Get("path"))
	if err != nil {
		app.writeError(writer, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	currentPath := root.ItemPath
	if relativePath != "/" {
		currentPath = root.ItemPath + relativePath
	}
	if currentPath != root.ItemPath && !strings.HasPrefix(currentPath, root.ItemPath+"/") {
		app.writeError(writer, http.StatusForbidden, "forbidden", "无权访问该目录")
		return
	}
	entries, err := app.listFileEntries("library", nil, currentPath)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	items := make([]shareFileItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildShareFileItem(token, rec.Permission, entry))
	}
	app.writeJSON(writer, http.StatusOK, shareItemsResult{Items: items, Path: relativePath})
}

func buildShareFileItem(token, permission string, entry fileEntry) shareFileItem {
	base := "/api/share/" + token + "/files/" + strconv.FormatInt(entry.ID, 10)
	item := shareFileItem{
		ID:         entry.ID,
		Name:       entry.Name,
		Kind:       entry.Kind,
		Size:       entry.Size,
		MimeType:   entry.MimeType,
		UpdatedAt:  entry.UpdatedAt,
		PreviewURL: base + "/preview",
	}
	if permission == sharePermissionDownload {
		if entry.Kind == "dir" {
			item.ArchiveURL = base + "/archive"
		} else {
			item.DownloadURL = base + "/download"
		}
	}
	return item
}

func (app *App) handleShareContent(writer http.ResponseWriter, request *http.Request, token, rawEntryID, action string) {
	rec, root, err := app.resolveShareTarget(token)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	if !app.authorizeShareView(request, rec) {
		app.writeError(writer, http.StatusUnauthorized, "unauthorized", "请先输入安全码")
		return
	}
	if (action == "download" || action == "archive") && rec.Permission != sharePermissionDownload {
		app.writeError(writer, http.StatusForbidden, "forbidden", "该分享不允许下载")
		return
	}
	entryID, err := strconv.ParseInt(rawEntryID, 10, 64)
	if err != nil {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}
	target, err := app.authorizeShareEntry(root, entryID)
	if err != nil {
		app.writeDomainError(writer, err)
		return
	}
	app.touchShareAccess(rec.ID)
	switch action {
	case "archive":
		app.serveShareArchive(writer, request, target)
	case "download":
		app.serveFileEntryContent(writer, request, target, true)
	default:
		app.serveFileEntryContent(writer, request, target, false)
	}
}

func (app *App) serveShareArchive(writer http.ResponseWriter, request *http.Request, entry *fileEntry) {
	if entry.Kind == "file" {
		app.serveFileEntryContent(writer, request, entry, true)
		return
	}
	if entry.Kind != "dir" {
		app.writeError(writer, http.StatusNotFound, "not_found", "资源不存在")
		return
	}
	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", entry.Name+".zip"))
	writer.WriteHeader(http.StatusOK)
	if err := app.writeDirectoryArchive(writer, entry); err != nil {
		log.Printf("stream share directory archive %d: %v", entry.ID, err)
	}
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func verifyPasswordHash(hashedPassword string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func optionalInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func parseOptionalInt64(raw string) (*int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	if parsed <= 0 {
		return nil, errors.New("value must be positive")
	}
	return &parsed, nil
}

type uploadConflictMode string

const (
	uploadConflictModeReject  uploadConflictMode = "reject"
	uploadConflictModeSkip    uploadConflictMode = "skip"
	uploadConflictModeReplace uploadConflictMode = "replace"
	uploadConflictModeRename  uploadConflictMode = "rename"
)

type uploadAction string

const (
	uploadActionCreated  uploadAction = "created"
	uploadActionReplaced uploadAction = "replaced"
	uploadActionRenamed  uploadAction = "renamed"
	uploadActionSkipped  uploadAction = "skipped"
)

func normalizeUploadConflictMode(raw string) (uploadConflictMode, error) {
	switch uploadConflictMode(strings.TrimSpace(raw)) {
	case "":
		return uploadConflictModeReject, nil
	case uploadConflictModeReject, uploadConflictModeSkip, uploadConflictModeReplace, uploadConflictModeRename:
		return uploadConflictMode(strings.TrimSpace(raw)), nil
	default:
		return "", errors.New("conflictMode 非法")
	}
}

func requireTUSResumable(request *http.Request) error {
	if strings.TrimSpace(request.Header.Get("Tus-Resumable")) != tusResumableVersion {
		return domainError{Status: http.StatusPreconditionFailed, Code: "invalid_request", Message: "Tus-Resumable 必须为 1.0.0"}
	}
	return nil
}

func parseUploadLength(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "缺少 Upload-Length"}
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value <= 0 {
		return 0, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "Upload-Length 非法"}
	}
	return value, nil
}

func parseUploadOffset(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "缺少 Upload-Offset"}
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value < 0 {
		return 0, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "Upload-Offset 非法"}
	}
	return value, nil
}

func parseTUSMetadata(raw string) (map[string]string, error) {
	result := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return result, nil
	}

	for _, item := range strings.Split(raw, ",") {
		part := strings.TrimSpace(item)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, " ", 2)
		if len(pieces) != 2 {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "Upload-Metadata 非法"}
		}
		value, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pieces[1]))
		if err != nil {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "Upload-Metadata 非法"}
		}
		result[strings.TrimSpace(pieces[0])] = string(value)
	}

	return result, nil
}

func encodeTUSMetadata(values map[string]string) string {
	parts := make([]string, 0, len(values))
	for _, key := range []string{"space", "parentPath", "filename", "classId", "conflictMode"} {
		value := values[key]
		if strings.TrimSpace(value) == "" {
			continue
		}
		parts = append(parts, key+" "+base64.StdEncoding.EncodeToString([]byte(value)))
	}
	return strings.Join(parts, ",")
}

func normalizeListPath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "/", nil
	}
	return normalizeRelativePath(raw)
}

func normalizeRelativePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "/" {
		return "/", nil
	}

	// 在清理前检查明显的路径遍历尝试
	if strings.Contains(trimmed, "..") {
		return "", errors.New("非法路径：包含父目录引用")
	}

	// 检查 URL 编码的路径遍历尝试
	// %2e = '.' , %2f = '/' , %5c = '\'
	lowered := strings.ToLower(trimmed)
	if strings.Contains(lowered, "%2e") || strings.Contains(lowered, "%5c") {
		return "", errors.New("非法路径：包含编码的特殊字符")
	}

	// 使用 path.Clean 标准化路径（注意：这里使用 path 而非 filepath，因为我们处理的是 URL 路径）
	cleaned := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))

	// 二次检查：确保清理后的路径不包含父目录引用
	if strings.Contains(cleaned, "..") {
		return "", errors.New("非法路径：清理后仍包含父目录引用")
	}

	// 确保路径以 / 开头且不等于根路径时不以 / 结尾
	if cleaned != "/" && strings.HasSuffix(cleaned, "/") {
		cleaned = strings.TrimSuffix(cleaned, "/")
	}

	// 最终验证：路径必须以 / 开头
	if !strings.HasPrefix(cleaned, "/") {
		return "", errors.New("非法路径：必须是绝对路径")
	}

	return cleaned, nil
}

func sanitizeName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("名称不能为空")
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return "", errors.New("名称不能包含路径分隔符")
	}
	return trimmed, nil
}

func sanitizeRequiredText(value, fieldName string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New(fieldName + "不能为空")
	}
	return trimmed, nil
}

const weakPasswordMessage = "密码过于简单，请换一个不连续、不重复的密码"

var commonTrivialPasswords = map[string]struct{}{
	"000000":   {},
	"111111":   {},
	"123123":   {},
	"123456":   {},
	"1234567":  {},
	"12345678": {},
	"654321":   {},
	"abcdef":   {},
	"abc123":   {},
	"password": {},
	"qwerty":   {},
}

func validatePasswordComplexity(password string) (string, error) {
	trimmed := strings.TrimSpace(password)
	if utf8.RuneCountInString(trimmed) < 6 {
		return "", errors.New("密码至少 6 位")
	}
	if isTrivialPassword(trimmed) {
		return "", errors.New(weakPasswordMessage)
	}
	return trimmed, nil
}

func isTrivialPassword(value string) bool {
	normalized := strings.ToLower(value)
	if _, ok := commonTrivialPasswords[normalized]; ok {
		return true
	}
	return isRepeatedPasswordValue(value) || isSingleStepPasswordSequence(normalized)
}

func isRepeatedPasswordValue(value string) bool {
	runes := []rune(value)
	if len(runes) <= 1 {
		return false
	}
	for _, item := range runes[1:] {
		if item != runes[0] {
			return false
		}
	}
	return true
}

func isSingleStepPasswordSequence(value string) bool {
	runes := []rune(value)
	if len(runes) < 3 {
		return false
	}
	step := runes[1] - runes[0]
	if step != 1 && step != -1 {
		return false
	}
	for index := 1; index < len(runes); index += 1 {
		if runes[index]-runes[index-1] != step {
			return false
		}
	}
	return true
}

func validateStudentPassword(password string) (string, error) {
	return validatePasswordComplexity(password)
}

func studentSubmissionConstraintsPayload() studentSubmissionConstraints {
	return studentSubmissionConstraintsPayloadForCategory(assignmentSubmissionTypeMixed)
}

func studentSubmissionConstraintsPayloadForCategory(rawCategory string) studentSubmissionConstraints {
	category := normalizeAssignmentSubmissionTypeCategory(rawCategory)
	return studentSubmissionConstraints{
		AllowedTypesLabel: studentSubmissionTypeLabels[category],
		MaxFileSizeBytes:  studentSubmissionMaxFileSize,
		MaxFileSizeLabel:  studentSubmissionMaxFileSizeLabel,
	}
}

func validateStudentSubmissionHeaders(headers []*multipart.FileHeader) error {
	return validateStudentSubmissionHeadersForCategory(headers, assignmentSubmissionTypeMixed)
}

func validateStudentSubmissionHeadersForCategory(headers []*multipart.FileHeader, rawCategory string) error {
	category := normalizeAssignmentSubmissionTypeCategory(rawCategory)
	allowedExtensions := studentSubmissionTypeExtensions[category]
	allowedLabel := studentSubmissionTypeLabels[category]
	for _, header := range headers {
		extension := strings.ToLower(path.Ext(strings.TrimSpace(header.Filename)))
		if !isAllowedStudentSubmissionExtension(extension, allowedExtensions) {
			return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "仅支持 " + allowedLabel + " 文件"}
		}
		if header.Size > studentSubmissionMaxFileSize {
			return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "单个文件不能超过 " + studentSubmissionMaxFileSizeLabel}
		}
	}
	return nil
}

func validateStudentSubmissionRule(assignment assignmentSummary, headers []*multipart.FileHeader, relativePaths []string) error {
	mode, err := normalizeAssignmentSubmissionMode(assignment.SubmissionMode)
	if err != nil {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	if _, err := validateAssignmentMinFileCount(assignment.MinFileCount); err != nil {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}

	switch mode {
	case assignmentSubmissionModeFiles:
		for index := range headers {
			segments, err := submissionRelativePathSegments(index, relativePaths)
			if err != nil {
				return err
			}
			if len(segments) > 1 {
				return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "本次作业要求提交文件，不要选择文件夹"}
			}
		}
	case assignmentSubmissionModeFolder:
		rootName := ""
		for index := range headers {
			segments, err := submissionRelativePathSegments(index, relativePaths)
			if err != nil {
				return err
			}
			if len(segments) < 2 {
				return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "本次作业要求提交一个文件夹"}
			}
			if rootName == "" {
				rootName = segments[0]
				continue
			}
			if segments[0] != rootName {
				return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "本次作业只能提交一个文件夹"}
			}
		}
	}

	return nil
}

func submissionRelativePathSegments(index int, relativePaths []string) ([]string, error) {
	relativePath := ""
	if index < len(relativePaths) {
		relativePath = relativePaths[index]
	}
	return normalizeUploadRelativePath(relativePath)
}

func isAllowedStudentSubmissionExtension(extension string, allowedExtensions []string) bool {
	for _, candidate := range allowedExtensions {
		if extension == candidate {
			return true
		}
	}
	return false
}

func normalizeAssignmentSubmissionTypeCategory(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case assignmentSubmissionTypeImage:
		return assignmentSubmissionTypeImage
	case assignmentSubmissionTypeWord:
		return assignmentSubmissionTypeWord
	case assignmentSubmissionTypePDF:
		return assignmentSubmissionTypePDF
	case assignmentSubmissionTypeArchive:
		return assignmentSubmissionTypeArchive
	default:
		return assignmentSubmissionTypeMixed
	}
}

func normalizeAssignmentSubmissionReviewStatus(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", assignmentSubmissionReviewPending:
		return assignmentSubmissionReviewPending, nil
	case assignmentSubmissionReviewReviewed:
		return assignmentSubmissionReviewReviewed, nil
	default:
		return "", errors.New("批改状态非法")
	}
}

func normalizeAssignmentSubmissionMode(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", assignmentSubmissionModeAny:
		return assignmentSubmissionModeAny, nil
	case assignmentSubmissionModeFiles:
		return assignmentSubmissionModeFiles, nil
	case assignmentSubmissionModeFolder:
		return assignmentSubmissionModeFolder, nil
	default:
		return "", errors.New("提交方式非法")
	}
}

func normalizeAssignmentMinFileCount(raw int) (int, error) {
	if raw == 0 {
		return 1, nil
	}
	return validateAssignmentMinFileCount(raw)
}

func validateAssignmentMinFileCount(raw int) (int, error) {
	if raw < 1 {
		return 0, errors.New("最少文件数不能小于 1")
	}
	if raw > 500 {
		return 0, errors.New("最少文件数不能超过 500")
	}
	return raw, nil
}

func normalizeOptionalRFC3339(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return "", err
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func normalizeAssignmentStatus(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "draft":
		return "draft", nil
	case "published":
		return "published", nil
	default:
		return "", errors.New("作业状态非法")
	}
}

func joinChild(parentPath, name string) string {
	if parentPath == "/" {
		return "/" + name
	}
	return path.Join(parentPath, name)
}

func contentTypeForFile(name string, data []byte) string {
	byExtension := mime.TypeByExtension(filepath.Ext(name))
	if byExtension != "" {
		return byExtension
	}
	return http.DetectContentType(data)
}

func validateEditableTextEntry(entry *fileEntry) error {
	if entry.Kind != "file" {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "当前文件不支持在线编辑"}
	}
	if !isEditableTextFile(entry.Name, entry.MimeType) {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "当前文件不支持在线编辑"}
	}
	return nil
}

func isEditableTextFile(name, mimeType string) bool {
	extension := strings.ToLower(path.Ext(strings.TrimSpace(name)))
	if _, ok := editableTextExtensions[extension]; ok {
		return true
	}
	if extension != "" {
		return false
	}
	normalizedMimeType := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	switch normalizedMimeType {
	case "text/plain", "text/markdown", "application/json", "application/xml", "application/javascript":
		return true
	default:
		return false
	}
}

func (app *App) readEditableTextContent(entry *fileEntry) (string, error) {
	if err := validateEditableTextEntry(entry); err != nil {
		return "", err
	}
	absolutePath := filepath.Join(app.storageRoot, filepath.FromSlash(entry.DiskPath))
	data, err := os.ReadFile(absolutePath)
	if err != nil {
		return "", err
	}
	if len(data) > editableTextFileMaxSize {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "文本文件超过 1 MB，暂不支持在线编辑"}
	}
	if !utf8.Valid(data) {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "当前仅支持 UTF-8 文本文件在线编辑"}
	}
	return string(data), nil
}

type domainError struct {
	Status  int
	Code    string
	Message string
}

func (err domainError) Error() string {
	return err.Message
}

func (app *App) writeDomainError(writer http.ResponseWriter, err error) {
	var typed domainError
	if errors.As(err, &typed) {
		app.writeError(writer, typed.Status, typed.Code, typed.Message)
		return
	}
	app.writeError(writer, http.StatusInternalServerError, "server_error", err.Error())
}

func (app *App) writeError(writer http.ResponseWriter, status int, code, message string) {
	if status >= http.StatusInternalServerError {
		log.Printf("server error response: status=%d code=%s message=%s", status, code, message)
		message = serverErrorMessage
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(apiErrorEnvelope{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}

func (app *App) writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func (app *App) uploadSessionDir() string {
	return filepath.Join(app.storageRoot, ".upload-sessions")
}

func (app *App) uploadSessionPaths(sessionID string) (string, string) {
	sessionDir := app.uploadSessionDir()
	return filepath.Join(sessionDir, sessionID+".bin"), filepath.Join(sessionDir, sessionID+".json")
}

func (app *App) writeUploadSessionHeaders(writer http.ResponseWriter, session *uploadSession) {
	metadata := map[string]string{
		"space":        session.Space,
		"parentPath":   session.ParentPath,
		"filename":     session.Filename,
		"conflictMode": string(session.ConflictMode),
	}
	if session.ClassID != nil {
		metadata["classId"] = strconv.FormatInt(*session.ClassID, 10)
	}

	writer.Header().Set("Tus-Resumable", tusResumableVersion)
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Upload-Offset", strconv.FormatInt(session.UploadOffset, 10))
	writer.Header().Set("Upload-Length", strconv.FormatInt(session.UploadLength, 10))
	writer.Header().Set("Upload-Metadata", encodeTUSMetadata(metadata))
}

func (app *App) createUploadSession(request *http.Request) (*uploadSession, error) {
	uploadLength, err := parseUploadLength(request.Header.Get("Upload-Length"))
	if err != nil {
		return nil, err
	}
	metadata, err := parseTUSMetadata(request.Header.Get("Upload-Metadata"))
	if err != nil {
		return nil, err
	}

	classID, err := parseOptionalInt64(metadata["classId"])
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}
	conflictMode, err := normalizeUploadConflictMode(metadata["conflictMode"])
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	if conflictMode != uploadConflictModeReject {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "resumable 上传仅支持 conflictMode=reject"}
	}

	parentPath, fileName, err := app.validateUploadSessionTarget(metadata["space"], classID, metadata["parentPath"], metadata["filename"], conflictMode)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(app.uploadSessionDir(), 0o755); err != nil {
		return nil, err
	}

	sessionID, err := generateToken()
	if err != nil {
		return nil, err
	}
	dataPath, _ := app.uploadSessionPaths(sessionID)
	dataFile, err := os.OpenFile(dataPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := dataFile.Close(); err != nil {
		return nil, err
	}

	session := &uploadSession{
		ID:           sessionID,
		Space:        metadata["space"],
		ClassID:      classID,
		ParentPath:   parentPath,
		Filename:     fileName,
		ConflictMode: conflictMode,
		UploadLength: uploadLength,
		UploadOffset: 0,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := app.saveUploadSession(session); err != nil {
		_ = os.Remove(dataPath)
		return nil, err
	}
	return session, nil
}

func (app *App) validateUploadSessionTarget(space string, classID *int64, parentPath, fileName string, conflictMode uploadConflictMode) (string, string, error) {
	normalizedParentPath, err := normalizeRelativePath(parentPath)
	if err != nil {
		return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	cleanName, err := sanitizeName(fileName)
	if err != nil {
		return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	if _, _, err := app.storageDir(space, classID); err != nil {
		return "", "", err
	}
	if conflictMode != uploadConflictModeReject {
		return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "resumable 上传仅支持 conflictMode=reject"}
	}
	if err := app.ensurePathAvailable(space, classID, joinChild(normalizedParentPath, cleanName)); err != nil {
		return "", "", err
	}
	return normalizedParentPath, cleanName, nil
}

func (app *App) saveUploadSession(session *uploadSession) error {
	_, metaPath := app.uploadSessionPaths(session.ID)
	contents, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, contents, 0o600)
}

func (app *App) loadUploadSession(sessionID string) (*uploadSession, error) {
	_, metaPath := app.uploadSessionPaths(sessionID)
	contents, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "资源不存在"}
		}
		return nil, err
	}

	var session uploadSession
	if err := json.Unmarshal(contents, &session); err != nil {
		return nil, err
	}
	if session.ID == "" {
		session.ID = sessionID
	}
	return &session, nil
}

func (app *App) appendUploadSession(sessionID string, request *http.Request) (*uploadSession, error) {
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/offset+octet-stream") {
		return nil, domainError{Status: http.StatusUnsupportedMediaType, Code: "invalid_request", Message: "PATCH 必须使用 application/offset+octet-stream"}
	}

	session, err := app.loadUploadSession(sessionID)
	if err != nil {
		return nil, err
	}
	uploadOffset, err := parseUploadOffset(request.Header.Get("Upload-Offset"))
	if err != nil {
		return nil, err
	}
	if uploadOffset != session.UploadOffset {
		return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "Upload-Offset 与服务端不一致"}
	}

	remaining := session.UploadLength - session.UploadOffset
	body, err := io.ReadAll(io.LimitReader(request.Body, remaining+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > remaining {
		return nil, domainError{Status: http.StatusRequestEntityTooLarge, Code: "invalid_request", Message: "上传分片超过剩余大小"}
	}

	dataPath, _ := app.uploadSessionPaths(sessionID)
	file, err := os.OpenFile(dataPath, os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "资源不存在"}
		}
		return nil, err
	}
	defer file.Close()

	if _, err := file.Seek(session.UploadOffset, io.SeekStart); err != nil {
		return nil, err
	}
	if _, err := file.Write(body); err != nil {
		return nil, err
	}

	session.UploadOffset += int64(len(body))
	if session.UploadOffset < session.UploadLength {
		if err := app.saveUploadSession(session); err != nil {
			return nil, err
		}
		return session, nil
	}

	if err := file.Close(); err != nil {
		return nil, err
	}

	if _, err := app.completeUploadSession(session); err != nil {
		return nil, err
	}
	return session, nil
}

func (app *App) completeUploadSession(session *uploadSession) (*fileEntry, error) {
	parentPath, fileName, err := app.validateUploadSessionTarget(session.Space, session.ClassID, session.ParentPath, session.Filename, session.ConflictMode)
	if err != nil {
		return nil, err
	}

	baseDir, relativeRoot, err := app.storageDir(session.Space, session.ClassID)
	if err != nil {
		return nil, err
	}
	itemPath := joinChild(parentPath, fileName)
	absolutePath := filepath.Join(baseDir, filepath.FromSlash(strings.TrimPrefix(itemPath, "/")))
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return nil, err
	}

	dataPath, metaPath := app.uploadSessionPaths(session.ID)
	if err := os.Rename(dataPath, absolutePath); err != nil {
		return nil, err
	}

	mimeType, size, err := detectStoredFile(absolutePath, fileName)
	if err != nil {
		return nil, err
	}

	entry := &fileEntry{
		Space:      session.Space,
		ParentPath: parentPath,
		ItemPath:   itemPath,
		Name:       fileName,
		Kind:       "file",
		MimeType:   mimeType,
		Size:       size,
		DiskPath:   path.Join(relativeRoot, strings.TrimPrefix(itemPath, "/")),
	}
	if session.ClassID != nil {
		entry.ClassID = sql.NullInt64{Int64: *session.ClassID, Valid: true}
	}
	if err := app.insertEntry(entry); err != nil {
		_ = os.Remove(absolutePath)
		return nil, err
	}
	_ = os.Remove(metaPath)
	return entry, nil
}

func (app *App) removeUploadSession(sessionID string) error {
	if _, err := app.loadUploadSession(sessionID); err != nil {
		return err
	}
	dataPath, metaPath := app.uploadSessionPaths(sessionID)
	if err := os.Remove(dataPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func detectStoredFile(absolutePath, fileName string) (string, int64, error) {
	file, err := os.Open(absolutePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}
	head := make([]byte, 512)
	count, err := file.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", 0, err
	}
	return contentTypeForFile(fileName, head[:count]), info.Size(), nil
}

func (app *App) runSchema() error {
	schema := `
create table if not exists teachers (
  id integer primary key autoincrement,
  username text not null unique,
  display_name text not null,
  password_hash text not null,
  role text not null default 'staff',
  disabled integer not null default 0
);

create table if not exists classes (
  id integer primary key autoincrement,
  name text not null unique,
  join_code text not null default '',
  registration_enabled integer not null default 0,
  registration_expires_at text not null default '',
  exam_lock_enabled integer not null default 0
);

create table if not exists sessions (
  token text primary key,
  teacher_id integer not null,
  expires_at text not null
);

create table if not exists student_sessions (
  token text primary key,
  student_id integer not null,
  expires_at text not null
);

create table if not exists login_logs (
  id integer primary key autoincrement,
  occurred_at text not null,
  actor_type text not null,
  actor_id integer,
  actor_name text not null default '',
  username text not null default '',
  status text not null,
  ip_address text not null default '',
  user_agent text not null default '',
  message text not null default ''
);

create table if not exists operation_logs (
  id integer primary key autoincrement,
  occurred_at text not null,
  actor_type text not null,
  actor_id integer,
  actor_name text not null default '',
  method text not null,
  path text not null,
  status_code integer not null,
  ip_address text not null default '',
  user_agent text not null default '',
  summary text not null default ''
);

create table if not exists teacher_preferences (
  teacher_id integer not null,
  preference_key text not null,
  preference_json text not null default '',
  primary key (teacher_id, preference_key)
);

create table if not exists system_settings (
  id integer primary key check (id = 1),
  upload_panel_enabled integer not null default 1,
  single_account_login_enabled integer not null default 1,
  server_port text not null default '80',
  default_share_expires_days integer not null default 7
);

create table if not exists students (
  id integer primary key autoincrement,
  class_id integer not null,
  student_no text not null,
  display_name text not null,
  password_hash text not null default '',
  activated_at text not null default '',
  must_change_password integer not null default 0
);

create table if not exists assignments (
  id integer primary key autoincrement,
  class_id integer not null,
  title text not null,
  description text not null default '',
  due_at text not null default '',
  status text not null default 'draft',
  submission_mode text not null default 'any',
  submission_type_category text not null default 'mixed',
  min_file_count integer not null default 1,
  created_at text not null,
  updated_at text not null
);

create table if not exists assignment_submissions (
  id integer primary key autoincrement,
  assignment_id integer not null,
  student_id integer not null,
  status text not null,
  submitted_at text not null,
  updated_at text not null,
  review_status text not null default 'pending',
  teacher_comment text not null default '',
  reviewed_at text not null default '',
  reviewer_name text not null default ''
);

create table if not exists file_entries (
  id integer primary key autoincrement,
  space text not null,
  class_id integer,
  parent_path text not null,
  item_path text not null,
  name text not null,
  kind text not null,
  mime_type text not null,
  size integer not null,
  disk_path text not null,
  created_at text not null,
  updated_at text not null
);

create table if not exists library_shares (
  id integer primary key autoincrement,
  token text not null unique,
  access_code_hash text not null default '',
  entry_id integer not null,
  permission text not null default 'view',
  expires_at text not null default '',
  disabled integer not null default 0,
  created_by_teacher_id integer not null,
  created_at text not null,
  updated_at text not null,
  last_accessed_at text not null default '',
  access_count integer not null default 0
);

create table if not exists library_share_sessions (
  token text primary key,
  share_id integer not null,
  expires_at text not null
);

create unique index if not exists idx_file_entries_scope_path on file_entries (space, ifnull(class_id, 0), item_path);
create index if not exists idx_assignments_class_created on assignments (class_id, created_at desc, id desc);
create unique index if not exists idx_assignment_submissions_assignment_student on assignment_submissions (assignment_id, student_id);
create index if not exists idx_login_logs_occurred on login_logs (occurred_at desc, id desc);
create index if not exists idx_operation_logs_occurred on operation_logs (occurred_at desc, id desc);
create index if not exists idx_library_shares_token on library_shares (token);
create index if not exists idx_library_shares_entry on library_shares (entry_id);
`
	if _, err := app.db.Exec(schema); err != nil {
		return err
	}
	if err := app.ensureClassesJoinCodeColumn(); err != nil {
		return err
	}
	if err := app.ensureClassesRegistrationEnabledColumn(); err != nil {
		return err
	}
	if err := app.ensureClassesRegistrationExpiresAtColumn(); err != nil {
		return err
	}
	if err := app.ensureClassesExamLockEnabledColumn(); err != nil {
		return err
	}
	if err := app.ensureStudentsAuthColumns(); err != nil {
		return err
	}
	if err := app.ensureAssignmentSubmissionReviewColumns(); err != nil {
		return err
	}
	if err := app.ensureAssignmentSubmissionRuleColumns(); err != nil {
		return err
	}
	if err := app.ensureTeachersRoleColumns(); err != nil {
		return err
	}
	if err := app.ensureLegacyShareCleanup(); err != nil {
		return err
	}
	if err := app.ensureSystemSettingsRow(); err != nil {
		return err
	}
	if _, err := app.db.Exec(`create unique index if not exists idx_classes_join_code on classes (join_code) where join_code <> ''`); err != nil {
		return err
	}
	if _, err := app.db.Exec(`drop index if exists idx_students_class_no`); err != nil {
		return err
	}
	_, err := app.db.Exec(`create unique index if not exists idx_students_student_no on students (student_no)`)
	return err
}

type columnDef struct {
	Name     string
	AlterSQL string
}

func (app *App) ensureColumns(tableName string, columns []columnDef) error {
	if !isSQLiteIdentifier(tableName) {
		return fmt.Errorf("invalid sqlite table identifier %q", tableName)
	}
	rows, err := app.db.Query(fmt.Sprintf(`pragma table_info(%s)`, tableName))
	if err != nil {
		return err
	}

	existing := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var columnName string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		existing[columnName] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, column := range columns {
		if _, ok := existing[column.Name]; ok {
			continue
		}
		if _, err := app.db.Exec(column.AlterSQL); err != nil {
			return err
		}
	}
	return nil
}

func isSQLiteIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		isLetter := (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
		isDigit := char >= '0' && char <= '9'
		if char != '_' && !isLetter && !isDigit {
			return false
		}
		if index == 0 && isDigit {
			return false
		}
	}
	return true
}

func (app *App) ensureClassesJoinCodeColumn() error {
	return app.ensureColumns("classes", []columnDef{
		{Name: "join_code", AlterSQL: `alter table classes add column join_code text not null default ''`},
	})
}

func (app *App) ensureClassesRegistrationEnabledColumn() error {
	return app.ensureColumns("classes", []columnDef{
		{Name: "registration_enabled", AlterSQL: `alter table classes add column registration_enabled integer not null default 0`},
	})
}

func (app *App) ensureClassesRegistrationExpiresAtColumn() error {
	return app.ensureColumns("classes", []columnDef{
		{Name: "registration_expires_at", AlterSQL: `alter table classes add column registration_expires_at text not null default ''`},
	})
}

func (app *App) ensureClassesExamLockEnabledColumn() error {
	return app.ensureColumns("classes", []columnDef{
		{Name: "exam_lock_enabled", AlterSQL: `alter table classes add column exam_lock_enabled integer not null default 0`},
	})
}

func (app *App) ensureStudentsAuthColumns() error {
	return app.ensureColumns("students", []columnDef{
		{Name: "password_hash", AlterSQL: `alter table students add column password_hash text not null default ''`},
		{Name: "activated_at", AlterSQL: `alter table students add column activated_at text not null default ''`},
		{Name: "must_change_password", AlterSQL: `alter table students add column must_change_password integer not null default 0`},
	})
}

func (app *App) ensureAssignmentSubmissionReviewColumns() error {
	return app.ensureColumns("assignment_submissions", []columnDef{
		{Name: "review_status", AlterSQL: `alter table assignment_submissions add column review_status text not null default 'pending'`},
		{Name: "teacher_comment", AlterSQL: `alter table assignment_submissions add column teacher_comment text not null default ''`},
		{Name: "reviewed_at", AlterSQL: `alter table assignment_submissions add column reviewed_at text not null default ''`},
		{Name: "reviewer_name", AlterSQL: `alter table assignment_submissions add column reviewer_name text not null default ''`},
	})
}

func (app *App) ensureAssignmentSubmissionRuleColumns() error {
	return app.ensureColumns("assignments", []columnDef{
		{Name: "submission_mode", AlterSQL: `alter table assignments add column submission_mode text not null default 'any'`},
		{Name: "submission_type_category", AlterSQL: `alter table assignments add column submission_type_category text not null default 'mixed'`},
		{Name: "min_file_count", AlterSQL: `alter table assignments add column min_file_count integer not null default 1`},
	})
}

func (app *App) ensureTeachersRoleColumns() error {
	return app.ensureColumns("teachers", []columnDef{
		{Name: "role", AlterSQL: `alter table teachers add column role text not null default 'staff'`},
		{Name: "disabled", AlterSQL: `alter table teachers add column disabled integer not null default 0`},
	})
}

func (app *App) ensureSystemSettingsRow() error {
	_, err := app.db.Exec(`
insert into system_settings (id, upload_panel_enabled, single_account_login_enabled, server_port)
values (1, 1, 1, '80')
on conflict(id) do nothing
`)
	return err
}

func (app *App) ensureLegacyShareCleanup() error {
	if err := app.migrateSystemSettingsTable(); err != nil {
		return err
	}
	if _, err := app.db.Exec(`drop table if exists file_shares`); err != nil {
		return err
	}
	if _, err := app.db.Exec(`drop table if exists public_shares`); err != nil {
		return err
	}
	return nil
}

func (app *App) migrateSystemSettingsTable() error {
	rows, err := app.db.Query(`pragma table_info(system_settings)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns := make([]string, 0, 4)
	hasServerPort := false
	hasSingleAccountLoginEnabled := false
	hasDefaultShareExpiresDays := false
	for rows.Next() {
		var cid int
		var columnName string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		columns = append(columns, columnName)
		switch columnName {
		case "server_port":
			hasServerPort = true
		case "single_account_login_enabled":
			hasSingleAccountLoginEnabled = true
		case "default_share_expires_days":
			hasDefaultShareExpiresDays = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if hasServerPort {
		if !hasSingleAccountLoginEnabled {
			if _, err := app.db.Exec(`alter table system_settings add column single_account_login_enabled integer not null default 1`); err != nil {
				return err
			}
		}
		if !hasDefaultShareExpiresDays {
			if _, err := app.db.Exec(`alter table system_settings add column default_share_expires_days integer not null default 7`); err != nil {
				return err
			}
		}
		return nil
	}

	tx, err := app.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`drop table if exists system_settings_next`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
create table system_settings_next (
  id integer primary key check (id = 1),
  upload_panel_enabled integer not null default 1,
  single_account_login_enabled integer not null default 1,
  server_port text not null default '80',
  default_share_expires_days integer not null default 7
)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
insert into system_settings_next (id, upload_panel_enabled, single_account_login_enabled, server_port, default_share_expires_days)
select id, upload_panel_enabled, 1, '80', 7
from system_settings
`); err != nil {
		return err
	}
	if _, err := tx.Exec(`drop table system_settings`); err != nil {
		return err
	}
	if _, err := tx.Exec(`alter table system_settings_next rename to system_settings`); err != nil {
		return err
	}

	return tx.Commit()
}

func (app *App) seed() error {
	var teacherCount int
	if err := app.db.QueryRow(`select count(*) from teachers`).Scan(&teacherCount); err != nil {
		return err
	}
	if teacherCount == 0 {
		passwordHash, err := hashPassword(EffectiveDefaultTeacherPassword())
		if err != nil {
			return err
		}
		if _, err := app.db.Exec(`insert into teachers (username, display_name, password_hash, role, disabled) values (?, ?, ?, ?, 0)`, DefaultTeacherUsername, "示例老师", passwordHash, teacherRoleOwner); err != nil {
			return err
		}
	} else {
		if _, err := app.db.Exec(`
update teachers
set username = ?
where username = 'teacher'
  and display_name = '示例老师'
  and not exists (select 1 from teachers where username = ?)
`, DefaultTeacherUsername, DefaultTeacherUsername); err != nil {
			return err
		}
		if _, err := app.db.Exec(`update teachers set role = ? where username = ? and trim(coalesce(role, '')) = ''`, teacherRoleOwner, DefaultTeacherUsername); err != nil {
			return err
		}
	}

	var classCount int
	if err := app.db.QueryRow(`select count(*) from classes`).Scan(&classCount); err != nil {
		return err
	}
	if classCount == 0 {
		if _, err := app.db.Exec(`insert into classes (name) values (?), (?)`, "一年级一班", "一年级二班"); err != nil {
			return err
		}
	}

	var fileCount int
	if err := app.db.QueryRow(`select count(*) from file_entries`).Scan(&fileCount); err != nil {
		return err
	}
	if fileCount > 0 {
		return nil
	}

	if _, err := app.createFolder("library", nil, "/", "课程资料"); err != nil {
		return err
	}
	if _, err := app.createSeedFile("library", nil, "/", "教学安排.txt", []byte("本周完成课程导入与资料整理。")); err != nil {
		return err
	}
	if _, err := app.createSeedFile("public", nil, "/", "公共通知.txt", []byte("请同学们按时查看公共资料。")); err != nil {
		return err
	}
	if _, err := app.createSeedFile("class", optionalInt64(1), "/", "班级守则.txt", []byte("第一条：保持课堂整洁。")); err != nil {
		return err
	}
	return nil
}

func (app *App) createSeedFile(space string, classID *int64, parentPath, name string, content []byte) (*fileEntry, error) {
	return app.createFile(space, classID, parentPath, name, bytes.NewReader(content))
}

func (app *App) storageDir(space string, classID *int64) (string, string, error) {
	switch space {
	case "library":
		return filepath.Join(app.storageRoot, "library"), "library", nil
	case "public":
		return filepath.Join(app.storageRoot, "public"), "public", nil
	case "class":
		if classID == nil || *classID <= 0 {
			return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "班级资料需要 classId"}
		}
		relative := filepath.ToSlash(filepath.Join("classes", strconv.FormatInt(*classID, 10)))
		return filepath.Join(app.storageRoot, "classes", strconv.FormatInt(*classID, 10)), relative, nil
	case "assignment":
		if classID == nil || *classID <= 0 {
			return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "作业附件需要 classId"}
		}
		relative := filepath.ToSlash(filepath.Join("assignments", strconv.FormatInt(*classID, 10)))
		return filepath.Join(app.storageRoot, "assignments", strconv.FormatInt(*classID, 10)), relative, nil
	case "submission":
		if classID == nil || *classID <= 0 {
			return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "学生提交附件需要 classId"}
		}
		relative := filepath.ToSlash(filepath.Join("submissions", strconv.FormatInt(*classID, 10)))
		return filepath.Join(app.storageRoot, "submissions", strconv.FormatInt(*classID, 10)), relative, nil
	default:
		return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "未知资料空间"}
	}
}

func (app *App) listClasses() ([]classSummary, error) {
	items, _, err := app.listClassesPage(teacherListQuery{Sort: "name-asc", Page: 1})
	return items, err
}

func (app *App) listClassesPage(query teacherListQuery) ([]classSummary, paginationPayload, error) {
	if err := app.closeExpiredClassRegistrations(time.Now().UTC()); err != nil {
		return nil, paginationPayload{}, err
	}

	whereParts := make([]string, 0, 1)
	args := make([]any, 0, 2)
	if query.Keyword != "" {
		whereParts = append(whereParts, `name like ?`)
		args = append(args, "%"+query.Keyword+"%")
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = " where " + strings.Join(whereParts, " and ")
	}

	var total int
	countQuery := `select count(*) from classes` + whereClause
	if err := app.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, paginationPayload{}, err
	}

	orderBy := ` order by name asc, id asc`
	switch query.Sort {
	case "name-desc":
		orderBy = ` order by name desc, id desc`
	case "registration-desc":
		orderBy = ` order by registration_enabled desc, name asc, id asc`
	case "registration-asc":
		orderBy = ` order by registration_enabled asc, name asc, id asc`
	}

	listQuery := `select id, name, join_code, registration_enabled, registration_expires_at, exam_lock_enabled from classes` + whereClause + orderBy
	listArgs := append([]any{}, args...)
	if query.Paged {
		listQuery += ` limit ? offset ?`
		listArgs = append(listArgs, query.PageSize, query.Offset)
	}

	rows, err := app.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()
	classes := make([]classSummary, 0)
	for rows.Next() {
		var id int64
		var name string
		var joinCode string
		var registrationEnabled bool
		var registrationExpiresAt string
		var examLockEnabled bool
		if err := rows.Scan(&id, &name, &joinCode, &registrationEnabled, &registrationExpiresAt, &examLockEnabled); err != nil {
			return nil, paginationPayload{}, err
		}
		classes = append(classes, buildClassSummary(id, name, joinCode, registrationEnabled, registrationExpiresAt, examLockEnabled))
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return classes, buildTeacherListPagination(total, query, len(classes)), nil
}

func (app *App) loadRecentCopyTargets(teacherID int64) ([]recentCopyTargetPayload, error) {
	row := app.db.QueryRow(`select preference_json from teacher_preferences where teacher_id = ? and preference_key = ?`, teacherID, preferenceRecentCopyTargets)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []recentCopyTargetPayload{}, nil
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return []recentCopyTargetPayload{}, nil
	}
	var items []recentCopyTargetPayload
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (app *App) loadTeacherProfilePreferences(teacherID int64) (teacherProfilePreferences, error) {
	var settings teacherProfilePreferences
	found, err := app.loadTeacherPreferenceJSON(teacherID, preferenceTeacherProfileSettings, &settings)
	if err != nil {
		return teacherProfilePreferences{}, err
	}
	if !found {
		return teacherProfilePreferences{}, nil
	}
	return settings, nil
}

func (app *App) loadTeacherPreferenceJSON(teacherID int64, key string, target any) (bool, error) {
	row := app.db.QueryRow(`select preference_json from teacher_preferences where teacher_id = ? and preference_key = ?`, teacherID, key)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if strings.TrimSpace(raw) == "" {
		return false, nil
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return false, err
	}
	return true, nil
}

func (app *App) saveRecentCopyTargets(teacherID int64, items []recentCopyTargetPayload) error {
	if len(items) > 5 {
		items = items[:5]
	}
	for index := range items {
		normalizedPath, err := normalizeRelativePath(items[index].Path)
		if err != nil {
			return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "目标目录路径非法"}
		}
		items[index].Path = normalizedPath
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return app.saveTeacherPreferenceJSON(teacherID, preferenceRecentCopyTargets, encoded)
}

func (app *App) saveTeacherPreferenceJSON(teacherID int64, key string, encoded []byte) error {
	_, err := app.db.Exec(`
insert into teacher_preferences (teacher_id, preference_key, preference_json)
values (?, ?, ?)
on conflict(teacher_id, preference_key) do update set preference_json = excluded.preference_json
`, teacherID, key, string(encoded))
	return err
}

func normalizeTeacherRole(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case teacherRoleOwner:
		return teacherRoleOwner, nil
	case teacherRoleStaff:
		return teacherRoleStaff, nil
	default:
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "老师角色非法"}
	}
}

func sanitizeTeacherUsername(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "登录账号不能为空"}
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "登录账号不能包含空白字符"}
	}
	return trimmed, nil
}

func sanitizeTeacherDisplayName(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "老师姓名不能为空"}
	}
	return trimmed, nil
}

func validateTeacherPassword(value string) (string, error) {
	trimmed, err := validatePasswordComplexity(value)
	if err != nil {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	return trimmed, nil
}

func (app *App) updateTeacherProfile(current teacherRecord, payload updateTeacherProfileRequest) (*teacherRecord, teacherProfilePreferences, error) {
	updated := current
	if payload.DisplayName != nil {
		displayName, err := sanitizeTeacherDisplayName(*payload.DisplayName)
		if err != nil {
			return nil, teacherProfilePreferences{}, err
		}
		if _, err := app.db.Exec(`update teachers set display_name = ? where id = ?`, displayName, current.ID); err != nil {
			return nil, teacherProfilePreferences{}, err
		}
		updated.DisplayName = displayName
	}
	preferences, err := app.loadTeacherProfilePreferences(current.ID)
	if err != nil {
		return nil, teacherProfilePreferences{}, err
	}
	if payload.Preferences != nil {
		preferences = *payload.Preferences
		encoded, err := json.Marshal(preferences)
		if err != nil {
			return nil, teacherProfilePreferences{}, err
		}
		if err := app.saveTeacherPreferenceJSON(current.ID, preferenceTeacherProfileSettings, encoded); err != nil {
			return nil, teacherProfilePreferences{}, err
		}
	}
	return &updated, preferences, nil
}

func (app *App) updateTeacherPassword(teacherID int64, payload updateTeacherPasswordRequest) error {
	currentPassword := strings.TrimSpace(payload.CurrentPassword)
	newPassword, err := validateTeacherPassword(payload.NewPassword)
	if err != nil {
		return err
	}
	if currentPassword == "" {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "当前密码不能为空"}
	}
	record, err := app.findTeacherByID(teacherID)
	if err != nil {
		return err
	}
	if record == nil {
		return domainError{Status: http.StatusNotFound, Code: "not_found", Message: "老师不存在"}
	}
	if verifyPasswordHash(record.PasswordHash, currentPassword) != nil {
		return domainError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "当前密码错误"}
	}
	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = app.db.Exec(`update teachers set password_hash = ? where id = ?`, passwordHash, teacherID)
	return err
}

func (settings systemSettings) withServerHost(request *http.Request) systemSettings {
	settings.ServerHost = serverHostForRequest(request)
	return settings
}

func serverHostForRequest(request *http.Request) string {
	if host, ok := detectServerHost(); ok {
		return host
	}
	if request != nil {
		host := strings.TrimSpace(request.Host)
		if parsedHost, _, err := net.SplitHostPort(host); err == nil && parsedHost != "" {
			return parsedHost
		}
		if host != "" {
			return strings.Split(host, ":")[0]
		}
	}
	return "127.0.0.1"
}

func detectServerHost() (string, bool) {
	conn, err := net.DialTimeout("udp4", "8.8.8.8:80", 200*time.Millisecond)
	if err != nil {
		return "", false
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", false
	}
	ip := localAddr.IP.To4()
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return "", false
	}
	return ip.String(), true
}

func normalizeServerPort(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultServerPort, nil
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil || port < 1 || port > 65535 {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "端口必须为 1-65535 的数字"}
	}
	return strconv.Itoa(port), nil
}

func (app *App) loadSystemSettings() (systemSettings, error) {
	row := app.db.QueryRow(`select upload_panel_enabled, single_account_login_enabled, server_port, default_share_expires_days from system_settings where id = 1`)
	var uploadPanelEnabled bool
	var singleAccountLoginEnabled bool
	var serverPort string
	var defaultShareExpiresDays int
	if err := row.Scan(&uploadPanelEnabled, &singleAccountLoginEnabled, &serverPort, &defaultShareExpiresDays); err != nil {
		return systemSettings{}, err
	}
	normalizedPort, err := normalizeServerPort(serverPort)
	if err != nil {
		normalizedPort = defaultServerPort
	}
	if defaultShareExpiresDays <= 0 {
		defaultShareExpiresDays = defaultShareExpiresDaysFallback
	}
	return systemSettings{
		UploadPanelEnabled:        uploadPanelEnabled,
		SingleAccountLoginEnabled: singleAccountLoginEnabled,
		ServerPort:                normalizedPort,
		DefaultShareExpiresDays:   defaultShareExpiresDays,
	}, nil
}

func (app *App) updateSystemSettings(payload updateSystemSettingsRequest) (systemSettings, error) {
	serverPort, err := normalizeServerPort(payload.ServerPort)
	if err != nil {
		return systemSettings{}, err
	}
	current, err := app.loadSystemSettings()
	if err != nil {
		return systemSettings{}, err
	}
	singleAccountLoginEnabled := current.SingleAccountLoginEnabled
	if payload.SingleAccountLoginEnabled != nil {
		singleAccountLoginEnabled = *payload.SingleAccountLoginEnabled
	}
	defaultShareExpiresDays := current.DefaultShareExpiresDays
	if payload.DefaultShareExpiresDays != nil {
		defaultShareExpiresDays = *payload.DefaultShareExpiresDays
		if defaultShareExpiresDays < 1 || defaultShareExpiresDays > maxShareExpiresDays {
			return systemSettings{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "默认分享有效期需在 1 到 3650 天之间"}
		}
	}
	preparedPortChange := PreparedServerPortChange{}
	hasPreparedPortChange := false
	committedPortChange := false
	if serverPort != current.ServerPort && app.prepareServerPortChange != nil {
		prepared, err := app.prepareServerPortChange(serverPort)
		if err != nil {
			return systemSettings{}, err
		}
		preparedPortChange = prepared
		hasPreparedPortChange = true
		defer func() {
			if hasPreparedPortChange && !committedPortChange && preparedPortChange.Cancel != nil {
				preparedPortChange.Cancel()
			}
		}()
	}
	_, err = app.db.Exec(`
update system_settings
set upload_panel_enabled = ?,
    single_account_login_enabled = ?,
    server_port = ?,
    default_share_expires_days = ?
where id = 1
`, payload.UploadPanelEnabled, singleAccountLoginEnabled, serverPort, defaultShareExpiresDays)
	if err != nil {
		return systemSettings{}, err
	}
	updated, err := app.loadSystemSettings()
	if err != nil {
		return systemSettings{}, err
	}
	committedPortChange = true
	if hasPreparedPortChange && preparedPortChange.Commit != nil {
		preparedPortChange.Commit()
	}
	return updated, nil
}

func (app *App) listTeacherUsers() ([]teacherUserPayload, error) {
	rows, err := app.db.Query(`select id, username, display_name, password_hash, role, disabled from teachers order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teachers := make([]teacherUserPayload, 0)
	for rows.Next() {
		var record teacherRecord
		if err := rows.Scan(&record.ID, &record.Username, &record.DisplayName, &record.PasswordHash, &record.Role, &record.Disabled); err != nil {
			return nil, err
		}
		teachers = append(teachers, record.toTeacherUserPayload())
	}
	return teachers, rows.Err()
}

func (app *App) createTeacherUser(payload createTeacherUserRequest) (*teacherRecord, error) {
	username, err := sanitizeTeacherUsername(payload.Username)
	if err != nil {
		return nil, err
	}
	displayName, err := sanitizeTeacherDisplayName(payload.DisplayName)
	if err != nil {
		return nil, err
	}
	password, err := validateTeacherPassword(payload.Password)
	if err != nil {
		return nil, err
	}
	role, err := normalizeTeacherRole(payload.Role)
	if err != nil {
		return nil, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	result, err := app.db.Exec(`insert into teachers (username, display_name, password_hash, role, disabled) values (?, ?, ?, ?, 0)`, username, displayName, passwordHash, role)
	if err != nil {
		if isUniqueConstraintError(err, "teachers.username") {
			return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "登录账号已存在"}
		}
		return nil, err
	}
	teacherID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return app.findTeacherByID(teacherID)
}

func (app *App) updateTeacherUser(teacherID int64, payload updateTeacherUserRequest) (*teacherRecord, error) {
	record, err := app.findTeacherByID(teacherID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "老师不存在"}
	}
	if payload.DisplayName != nil {
		displayName, err := sanitizeTeacherDisplayName(*payload.DisplayName)
		if err != nil {
			return nil, err
		}
		record.DisplayName = displayName
	}
	if payload.Role != nil {
		role, err := normalizeTeacherRole(*payload.Role)
		if err != nil {
			return nil, err
		}
		record.Role = role
	}
	if payload.Disabled != nil {
		record.Disabled = *payload.Disabled
	}
	if payload.Password != nil {
		password, err := validateTeacherPassword(*payload.Password)
		if err != nil {
			return nil, err
		}
		passwordHash, err := hashPassword(password)
		if err != nil {
			return nil, err
		}
		record.PasswordHash = passwordHash
	}
	if err := app.ensureOwnerRemovalAllowed(*record); err != nil {
		return nil, err
	}
	_, err = app.db.Exec(`update teachers set display_name = ?, password_hash = ?, role = ?, disabled = ? where id = ?`, record.DisplayName, record.PasswordHash, record.Role, record.Disabled, teacherID)
	if err != nil {
		return nil, err
	}
	return app.findTeacherByID(teacherID)
}

func (app *App) ensureOwnerRemovalAllowed(record teacherRecord) error {
	if record.Role == teacherRoleOwner && !record.Disabled {
		return nil
	}
	ownerCount, err := app.countActiveOwnersExcluding(record.ID)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		return domainError{Status: http.StatusConflict, Code: "conflict", Message: "系统至少需要保留一个可用的负责人账号"}
	}
	return nil
}

func (app *App) createClass(name string) (*classSummary, error) {
	cleanName, err := sanitizeName(name)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	result, err := app.db.Exec(`insert into classes (name, join_code, registration_enabled, registration_expires_at) values (?, '', 0, '')`, cleanName)
	if err != nil {
		if isUniqueConstraintError(err, "classes.name") {
			return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "班级名称已存在"}
		}
		return nil, err
	}
	classID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	summary := buildClassSummary(classID, cleanName, "", false, "", false)
	return &summary, nil
}

func (app *App) updateClass(classID int64, name string) (*classSummary, error) {
	if classID <= 0 {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
	}
	cleanName, err := sanitizeName(name)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}

	var joinCode string
	var registrationEnabled bool
	var registrationExpiresAt string
	var examLockEnabled bool
	if err := app.db.QueryRow(`select join_code, registration_enabled, registration_expires_at, exam_lock_enabled from classes where id = ?`, classID).Scan(&joinCode, &registrationEnabled, &registrationExpiresAt, &examLockEnabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
		}
		return nil, err
	}

	if _, err := app.db.Exec(`update classes set name = ? where id = ?`, cleanName, classID); err != nil {
		if isUniqueConstraintError(err, "classes.name") {
			return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "班级名称已存在"}
		}
		return nil, err
	}

	summary := buildClassSummary(classID, cleanName, joinCode, registrationEnabled, registrationExpiresAt, examLockEnabled)
	return &summary, nil
}

func (app *App) updateClassExamLock(classID int64, enabled bool) (*classSummary, error) {
	if classID <= 0 {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
	}

	var name string
	var joinCode string
	var registrationEnabled bool
	var registrationExpiresAt string
	if err := app.db.QueryRow(`select name, join_code, registration_enabled, registration_expires_at from classes where id = ?`, classID).Scan(&name, &joinCode, &registrationEnabled, &registrationExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
		}
		return nil, err
	}

	if _, err := app.db.Exec(`update classes set exam_lock_enabled = ? where id = ?`, boolToInt(enabled), classID); err != nil {
		return nil, err
	}

	summary := buildClassSummary(classID, name, joinCode, registrationEnabled, registrationExpiresAt, enabled)
	return &summary, nil
}

func (app *App) updateClassRegistration(classID int64, enabled bool) (*classJoinCodeResult, error) {
	if classID <= 0 {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
	}

	var joinCode string
	if err := app.db.QueryRow(`select join_code from classes where id = ?`, classID).Scan(&joinCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
		}
		return nil, err
	}

	if enabled {
		return app.refreshClassJoinCode(classID)
	}

	if err := app.closeClassRegistration(classID); err != nil {
		return nil, err
	}

	return &classJoinCodeResult{
		ClassID:               classID,
		JoinCode:              "",
		JoinCodeHint:          "",
		JoinCodeStatus:        "inactive",
		RegistrationEnabled:   false,
		RegistrationExpiresAt: "",
	}, nil
}

func (app *App) deleteClass(classID int64) error {
	if classID <= 0 {
		return domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
	}

	var name string
	if err := app.db.QueryRow(`select name from classes where id = ?`, classID).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
		}
		return err
	}

	classDir, _, err := app.storageDir("class", optionalInt64(classID))
	if err != nil {
		return err
	}
	assignmentDir, _, err := app.storageDir("assignment", optionalInt64(classID))
	if err != nil {
		return err
	}
	submissionDir, _, err := app.storageDir("submission", optionalInt64(classID))
	if err != nil {
		return err
	}

	tx, err := app.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`delete from file_entries where class_id = ? and space in ('class', 'assignment', 'submission')`, classID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
delete from assignment_submissions
where assignment_id in (select id from assignments where class_id = ?)`, classID); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from assignments where class_id = ?`, classID); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from students where class_id = ?`, classID); err != nil {
		return err
	}
	if _, err := tx.Exec(`delete from classes where id = ?`, classID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	for _, dir := range []string{classDir, assignmentDir, submissionDir} {
		if err := removeFileTree(dir); err != nil {
			return err
		}
	}
	return nil
}

func (app *App) ensureNoLinkedClassRecords(classID int64) error {
	checks := []struct {
		query   string
		message string
		args    []any
	}{
		{
			query:   `select count(*) from students where class_id = ?`,
			message: "当前班级下仍有学生，请先清理学生名册",
			args:    []any{classID},
		},
		{
			query:   `select count(*) from assignments where class_id = ?`,
			message: "当前班级下仍有关联作业，请先清理作业",
			args:    []any{classID},
		},
		{
			query:   `select count(*) from file_entries where space = 'class' and class_id = ?`,
			message: "当前班级资料未清空，请先清理班级资料",
			args:    []any{classID},
		},
	}

	for _, check := range checks {
		var count int
		if err := app.db.QueryRow(check.query, check.args...).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return domainError{Status: http.StatusConflict, Code: "conflict", Message: check.message}
		}
	}

	return nil
}

func (app *App) listStudents(classID int64) ([]studentResult, error) {
	items, _, err := app.listStudentsPage(classID, teacherListQuery{Sort: "studentNo-asc", Page: 1})
	return items, err
}

func (app *App) listStudentsPage(classID int64, query teacherListQuery) ([]studentResult, paginationPayload, error) {
	if classID <= 0 {
		return nil, paginationPayload{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}

	whereParts := []string{`class_id = ?`}
	args := []any{classID}
	if query.Keyword != "" {
		whereParts = append(whereParts, `(student_no like ? or display_name like ?)`)
		likeKeyword := "%" + query.Keyword + "%"
		args = append(args, likeKeyword, likeKeyword)
	}
	switch query.RegistrationFilter {
	case "registered":
		whereParts = append(whereParts, `activated_at <> ''`)
	case "unregistered":
		whereParts = append(whereParts, `activated_at = ''`)
	}
	whereClause := " where " + strings.Join(whereParts, " and ")

	var total int
	countQuery := `select count(*) from students` + whereClause
	if err := app.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, paginationPayload{}, err
	}

	orderBy := ` order by student_no asc, id asc`
	switch query.Sort {
	case "studentNo-desc":
		orderBy = ` order by student_no desc, id desc`
	case "displayName-asc":
		orderBy = ` order by display_name asc, student_no asc, id asc`
	case "displayName-desc":
		orderBy = ` order by display_name desc, student_no asc, id asc`
	case "registered-desc":
		orderBy = ` order by case when activated_at = '' then 0 else 1 end desc, student_no asc, id asc`
	case "registered-asc":
		orderBy = ` order by case when activated_at = '' then 0 else 1 end asc, student_no asc, id asc`
	}

	listQuery := `select id, class_id, student_no, display_name, activated_at from students` + whereClause + orderBy
	listArgs := append([]any{}, args...)
	if query.Paged {
		listQuery += ` limit ? offset ?`
		listArgs = append(listArgs, query.PageSize, query.Offset)
	}

	rows, err := app.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()
	students := make([]studentResult, 0)
	for rows.Next() {
		var item studentResult
		if err := rows.Scan(&item.ID, &item.ClassID, &item.StudentNo, &item.DisplayName, &item.ActivatedAt); err != nil {
			return nil, paginationPayload{}, err
		}
		students = append(students, item)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return students, buildTeacherListPagination(total, query, len(students)), nil
}

func (app *App) listAssignments(classID int64) ([]assignmentSummary, error) {
	items, _, err := app.listAssignmentsPage(classID, teacherListQuery{Sort: "updated-desc", Page: 1})
	return items, err
}

func (app *App) listAssignmentsPage(classID int64, query teacherListQuery) ([]assignmentSummary, paginationPayload, error) {
	if classID <= 0 {
		return nil, paginationPayload{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}
	if err := app.ensureClassExists(classID); err != nil {
		return nil, paginationPayload{}, err
	}

	whereParts := []string{`class_id = ?`}
	args := []any{classID}
	if query.Keyword != "" {
		whereParts = append(whereParts, `(title like ? or description like ?)`)
		likeKeyword := "%" + query.Keyword + "%"
		args = append(args, likeKeyword, likeKeyword)
	}
	whereClause := " where " + strings.Join(whereParts, " and ")

	var total int
	countQuery := `select count(*) from assignments` + whereClause
	if err := app.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, paginationPayload{}, err
	}

	orderBy := ` order by updated_at desc, id desc`
	switch query.Sort {
	case "updated-asc":
		orderBy = ` order by updated_at asc, id asc`
	case "dueAt-asc":
		orderBy = ` order by due_at asc, id asc`
	case "dueAt-desc":
		orderBy = ` order by due_at desc, id desc`
	case "title-asc":
		orderBy = ` order by title asc, id asc`
	case "title-desc":
		orderBy = ` order by title desc, id desc`
	}

	listQuery := `
select id, class_id, title, description, due_at, status, submission_mode, submission_type_category, min_file_count, created_at, updated_at
from assignments` + whereClause + orderBy
	listArgs := append([]any{}, args...)
	if query.Paged {
		listQuery += ` limit ? offset ?`
		listArgs = append(listArgs, query.PageSize, query.Offset)
	}

	rows, err := app.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()
	assignments := make([]assignmentSummary, 0)
	for rows.Next() {
		var item assignmentSummary
		if err := rows.Scan(&item.ID, &item.ClassID, &item.Title, &item.Description, &item.DueAt, &item.Status, &item.SubmissionMode, &item.SubmissionTypeCategory, &item.MinFileCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, paginationPayload{}, err
		}
		assignments = append(assignments, item)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return assignments, buildTeacherListPagination(total, query, len(assignments)), nil
}

func (app *App) createAssignment(payload createAssignmentRequest) (*assignmentSummary, error) {
	if payload.ClassID <= 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}
	if err := app.ensureClassExists(payload.ClassID); err != nil {
		return nil, err
	}
	title, err := sanitizeRequiredText(payload.Title, "标题")
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	dueAt, err := normalizeOptionalRFC3339(payload.DueAt)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "截止时间格式非法"}
	}
	status, err := normalizeAssignmentStatus(payload.Status)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	submissionMode, err := normalizeAssignmentSubmissionMode(payload.SubmissionMode)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	submissionTypeCategory := normalizeAssignmentSubmissionTypeCategory(payload.SubmissionTypeCategory)
	minFileCount, err := normalizeAssignmentMinFileCount(payload.MinFileCount)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}

	description := strings.TrimSpace(payload.Description)
	now := time.Now().UTC()
	if dueAt == "" {
		dueAt = now.Add(90 * time.Minute).Format(time.RFC3339)
	}
	nowText := now.Format(time.RFC3339)
	result, err := app.db.Exec(`
insert into assignments (class_id, title, description, due_at, status, submission_mode, submission_type_category, min_file_count, created_at, updated_at)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		payload.ClassID,
		title,
		description,
		dueAt,
		status,
		submissionMode,
		submissionTypeCategory,
		minFileCount,
		nowText,
		nowText,
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &assignmentSummary{
		ID:                     id,
		ClassID:                payload.ClassID,
		Title:                  title,
		Description:            description,
		DueAt:                  dueAt,
		Status:                 status,
		SubmissionMode:         submissionMode,
		SubmissionTypeCategory: submissionTypeCategory,
		MinFileCount:           minFileCount,
		CreatedAt:              nowText,
		UpdatedAt:              nowText,
	}, nil
}

func (app *App) findAssignmentByID(assignmentID, classID int64) (*assignmentSummary, error) {
	if classID <= 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}
	if err := app.ensureClassExists(classID); err != nil {
		return nil, err
	}

	row := app.db.QueryRow(`
select id, class_id, title, description, due_at, status, submission_mode, submission_type_category, min_file_count, created_at, updated_at
from assignments
where id = ? and class_id = ?`, assignmentID, classID)

	var assignment assignmentSummary
	if err := row.Scan(
		&assignment.ID,
		&assignment.ClassID,
		&assignment.Title,
		&assignment.Description,
		&assignment.DueAt,
		&assignment.Status,
		&assignment.SubmissionMode,
		&assignment.SubmissionTypeCategory,
		&assignment.MinFileCount,
		&assignment.CreatedAt,
		&assignment.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "作业不存在"}
		}
		return nil, err
	}
	return &assignment, nil
}

func (app *App) findAssignmentsByIDs(classID int64, assignmentIDs []int64) ([]assignmentSummary, error) {
	if classID <= 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}
	if err := app.ensureClassExists(classID); err != nil {
		return nil, err
	}
	if len(assignmentIDs) == 0 {
		return []assignmentSummary{}, nil
	}

	// Build IN clause with placeholders
	placeholders := make([]string, len(assignmentIDs))
	args := make([]interface{}, len(assignmentIDs)+1)
	args[0] = classID
	for i, id := range assignmentIDs {
		placeholders[i] = "?"
		args[i+1] = id
	}

	query := fmt.Sprintf(`
select id, class_id, title, description, due_at, status, submission_mode, submission_type_category, min_file_count, created_at, updated_at
from assignments
where class_id = ? and id in (%s)`, strings.Join(placeholders, ","))

	rows, err := app.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := make([]assignmentSummary, 0, len(assignmentIDs))
	for rows.Next() {
		var assignment assignmentSummary
		if err := rows.Scan(
			&assignment.ID,
			&assignment.ClassID,
			&assignment.Title,
			&assignment.Description,
			&assignment.DueAt,
			&assignment.Status,
			&assignment.SubmissionMode,
			&assignment.SubmissionTypeCategory,
			&assignment.MinFileCount,
			&assignment.CreatedAt,
			&assignment.UpdatedAt,
		); err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

func (app *App) updateAssignment(assignmentID, classID int64, payload updateAssignmentRequest) (*assignmentSummary, error) {
	current, err := app.findAssignmentByID(assignmentID, classID)
	if err != nil {
		return nil, err
	}
	title := current.Title
	if payload.Title != nil {
		title, err = sanitizeRequiredText(*payload.Title, "标题")
		if err != nil {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
		}
	}

	description := current.Description
	if payload.Description != nil {
		description = strings.TrimSpace(*payload.Description)
	}

	dueAt := current.DueAt
	if payload.DueAt != nil {
		dueAt, err = normalizeOptionalRFC3339(*payload.DueAt)
		if err != nil {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "截止时间格式非法"}
		}
	}

	status := current.Status
	if payload.Status != nil {
		status, err = normalizeAssignmentStatus(*payload.Status)
		if err != nil {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
		}
	}
	submissionMode := current.SubmissionMode
	if payload.SubmissionMode != nil {
		submissionMode, err = normalizeAssignmentSubmissionMode(*payload.SubmissionMode)
		if err != nil {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
		}
	}
	submissionTypeCategory := current.SubmissionTypeCategory
	if payload.SubmissionTypeCategory != nil {
		submissionTypeCategory = normalizeAssignmentSubmissionTypeCategory(*payload.SubmissionTypeCategory)
	}
	minFileCount := current.MinFileCount
	if payload.MinFileCount != nil {
		minFileCount, err = validateAssignmentMinFileCount(*payload.MinFileCount)
		if err != nil {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
		}
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = app.db.Exec(`
update assignments
set title = ?, description = ?, due_at = ?, status = ?, submission_mode = ?, submission_type_category = ?, min_file_count = ?, updated_at = ?
where id = ? and class_id = ?`,
		title,
		description,
		dueAt,
		status,
		submissionMode,
		submissionTypeCategory,
		minFileCount,
		updatedAt,
		assignmentID,
		classID,
	)
	if err != nil {
		return nil, err
	}
	return &assignmentSummary{
		ID:                     current.ID,
		ClassID:                current.ClassID,
		Title:                  title,
		Description:            description,
		DueAt:                  dueAt,
		Status:                 status,
		SubmissionMode:         submissionMode,
		SubmissionTypeCategory: submissionTypeCategory,
		MinFileCount:           minFileCount,
		CreatedAt:              current.CreatedAt,
		UpdatedAt:              updatedAt,
	}, nil
}

func (app *App) deleteAssignment(assignmentID, classID int64) error {
	if _, err := app.findAssignmentByID(assignmentID, classID); err != nil {
		return err
	}

	attachmentsBaseDir, _, err := app.storageDir("assignment", optionalInt64(classID))
	if err != nil {
		return err
	}
	attachmentParentPath := assignmentAttachmentParentPath(assignmentID)
	attachmentDir := filepath.Join(attachmentsBaseDir, filepath.FromSlash(strings.TrimPrefix(attachmentParentPath, "/")))
	if err := removeFileTree(attachmentDir); err != nil {
		return err
	}

	tx, err := app.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
delete from file_entries
where space = 'assignment' and class_id = ? and (parent_path = ? or item_path like ?)`,
		classID,
		attachmentParentPath,
		attachmentParentPath+"/%",
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`delete from assignments where id = ? and class_id = ?`, assignmentID, classID); err != nil {
		return err
	}

	return tx.Commit()
}

func assignmentAttachmentParentPath(assignmentID int64) string {
	return "/" + strconv.FormatInt(assignmentID, 10)
}

func buildAssignmentAttachmentSummary(assignmentID, classID int64, entry fileEntry) fileSummary {
	return buildFileSummary(entry, fmt.Sprintf("/api/assignments/%d/attachments/%d/download?classId=%d", assignmentID, entry.ID, classID), "")
}

func buildStudentAssignmentAttachmentSummary(assignmentID int64, entry fileEntry) fileSummary {
	return buildFileSummary(entry, fmt.Sprintf("/api/student/assignments/%d/attachments/%d/download", assignmentID, entry.ID), "")
}

func (app *App) listAssignmentAttachments(assignmentID, classID int64) ([]fileSummary, error) {
	if _, err := app.findAssignmentByID(assignmentID, classID); err != nil {
		return nil, err
	}
	entries, err := app.listFileEntries("assignment", optionalInt64(classID), assignmentAttachmentParentPath(assignmentID))
	if err != nil {
		return nil, err
	}
	items := make([]fileSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind != "file" {
			continue
		}
		items = append(items, buildAssignmentAttachmentSummary(assignmentID, classID, entry))
	}
	return items, nil
}

func (app *App) listStudentAssignmentAttachments(assignmentID, classID int64) ([]fileSummary, error) {
	if _, err := app.findVisibleStudentAssignmentByID(assignmentID, classID); err != nil {
		return nil, err
	}
	entries, err := app.listFileEntries("assignment", optionalInt64(classID), assignmentAttachmentParentPath(assignmentID))
	if err != nil {
		return nil, err
	}
	items := make([]fileSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind != "file" {
			continue
		}
		items = append(items, buildStudentAssignmentAttachmentSummary(assignmentID, entry))
	}
	return items, nil
}

func (app *App) createAssignmentAttachments(assignmentID, classID int64, headers []*multipart.FileHeader) ([]fileSummary, error) {
	if _, err := app.findAssignmentByID(assignmentID, classID); err != nil {
		return nil, err
	}
	items := make([]fileSummary, 0, len(headers))
	parentPath := assignmentAttachmentParentPath(assignmentID)
	for _, header := range headers {
		file, err := header.Open()
		if err != nil {
			return nil, err
		}
		entry, createErr := app.createFile("assignment", optionalInt64(classID), parentPath, header.Filename, file)
		_ = file.Close()
		if createErr != nil {
			return nil, createErr
		}
		items = append(items, buildAssignmentAttachmentSummary(assignmentID, classID, *entry))
	}
	return items, nil
}

func (app *App) findAssignmentAttachmentByID(assignmentID, classID, fileID int64) (*fileEntry, error) {
	if _, err := app.findAssignmentByID(assignmentID, classID); err != nil {
		return nil, err
	}
	entry, err := app.getEntryByID(fileID)
	if err != nil {
		return nil, err
	}
	if entry.Space != "assignment" || !entry.ClassID.Valid || entry.ClassID.Int64 != classID || entry.ParentPath != assignmentAttachmentParentPath(assignmentID) {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "附件不存在"}
	}
	return entry, nil
}

func (app *App) findStudentAssignmentAttachmentByID(assignmentID int64, student studentRecord, fileID int64) (*fileEntry, error) {
	if _, err := app.findVisibleStudentAssignmentByID(assignmentID, student.ClassID); err != nil {
		return nil, err
	}
	return app.findAssignmentAttachmentByID(assignmentID, student.ClassID, fileID)
}

func (app *App) deleteAssignmentAttachment(assignmentID, classID, fileID int64) error {
	if _, err := app.findAssignmentAttachmentByID(assignmentID, classID, fileID); err != nil {
		return err
	}
	return app.deleteEntry(fileID)
}

func assignmentSubmissionParentPath(submissionID int64) string {
	return "/" + strconv.FormatInt(submissionID, 10)
}

func submissionEntryBelongsToSubmission(entry *fileEntry, submissionID int64) bool {
	if entry == nil {
		return false
	}
	rootPath := assignmentSubmissionParentPath(submissionID)
	return entry.ItemPath == rootPath || strings.HasPrefix(entry.ItemPath, rootPath+"/")
}

func submissionIDFromEntryPath(entry *fileEntry) (int64, bool) {
	if entry == nil {
		return 0, false
	}
	trimmed := strings.TrimPrefix(entry.ItemPath, "/")
	segment, _, _ := strings.Cut(trimmed, "/")
	submissionID, err := strconv.ParseInt(segment, 10, 64)
	return submissionID, err == nil && submissionID > 0
}

func (app *App) buildStudentSubmissionFileSummary(assignmentID int64, entry fileEntry) fileSummary {
	summary := buildFileSummary(entry, fmt.Sprintf("/api/student/assignments/%d/submission/files/%d/download", assignmentID, entry.ID), "")
	if entry.Kind == "dir" {
		summary.ArchiveURL = summary.DownloadURL
	}
	app.applyFolderStats(&summary, entry)
	return summary
}

func (app *App) buildTeacherSubmissionFileSummary(assignmentID, classID int64, entry fileEntry) fileSummary {
	previewURL := ""
	if entry.Kind == "file" {
		previewURL = fmt.Sprintf("/api/assignments/%d/submissions/files/%d/preview?classId=%d", assignmentID, entry.ID, classID)
	}
	summary := buildFileSummary(entry, fmt.Sprintf("/api/assignments/%d/submissions/files/%d/download?classId=%d", assignmentID, entry.ID, classID), previewURL)
	if entry.Kind == "dir" {
		summary.ArchiveURL = summary.DownloadURL
	}
	app.applyFolderStats(&summary, entry)
	return summary
}

func sortSubmissionFileEntries(entries []fileEntry) {
	sort.SliceStable(entries, func(left, right int) bool {
		if entries[left].Kind != entries[right].Kind {
			return entries[left].Kind == "dir"
		}
		return strings.ToLower(entries[left].Name) < strings.ToLower(entries[right].Name)
	})
}

func (app *App) buildTeacherSubmissionFileTree(assignmentID, classID int64, entry fileEntry) (fileSummary, error) {
	summary := app.buildTeacherSubmissionFileSummary(assignmentID, classID, entry)
	if entry.Kind != "dir" {
		return summary, nil
	}
	children, err := app.listFileEntries("submission", optionalInt64(classID), entry.ItemPath)
	if err != nil {
		return fileSummary{}, err
	}
	sortSubmissionFileEntries(children)
	for _, child := range children {
		if child.Kind != "file" && child.Kind != "dir" {
			continue
		}
		childSummary, childErr := app.buildTeacherSubmissionFileTree(assignmentID, classID, child)
		if childErr != nil {
			return fileSummary{}, childErr
		}
		summary.Children = append(summary.Children, childSummary)
	}
	return summary, nil
}

func (app *App) applyFolderStats(summary *fileSummary, entry fileEntry) {
	if entry.Kind != "dir" {
		return
	}
	var fileCount int
	var folderCount int
	var totalSize int64
	err := app.db.QueryRow(`
select
  coalesce(sum(case when kind = 'file' then 1 else 0 end), 0),
  coalesce(sum(case when kind = 'dir' then 1 else 0 end), 0),
  coalesce(sum(case when kind = 'file' then size else 0 end), 0)
from file_entries
where space = ? and ifnull(class_id, 0) = ? and item_path like ?`,
		entry.Space,
		classIDKey(entry.ClassID),
		entry.ItemPath+"/%",
	).Scan(&fileCount, &folderCount, &totalSize)
	if err != nil {
		return
	}
	summary.FileCount = fileCount
	summary.FolderCount = folderCount
	summary.Size = totalSize
}

func (app *App) applyFileEntryFolderSizes(entries []fileEntry) error {
	directoryIDs := make([]any, 0)
	directoryIndexes := make(map[int64]int)
	for index := range entries {
		if entries[index].Kind != "dir" {
			continue
		}
		directoryIDs = append(directoryIDs, entries[index].ID)
		directoryIndexes[entries[index].ID] = index
	}
	if len(directoryIDs) == 0 {
		return nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(directoryIDs)), ",")
	rows, err := app.db.Query(`
select parent.id, coalesce(sum(child.size), 0)
from file_entries parent
left join file_entries child
  on child.space = parent.space
  and ifnull(child.class_id, 0) = ifnull(parent.class_id, 0)
  and child.kind = 'file'
  and child.item_path like parent.item_path || '/%'
where parent.id in (`+placeholders+`)
group by parent.id`,
		directoryIDs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var entryID int64
		var totalSize int64
		if err := rows.Scan(&entryID, &totalSize); err != nil {
			return err
		}
		if index, ok := directoryIndexes[entryID]; ok {
			entries[index].Size = totalSize
		}
	}
	return rows.Err()
}

func assignmentIsOverdue(dueAt string, now time.Time) bool {
	trimmed := strings.TrimSpace(dueAt)
	if trimmed == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return false
	}
	return !parsed.After(now)
}

func toAssignmentSubmissionSummary(record *assignmentSubmissionRecord) *assignmentSubmissionSummary {
	if record == nil {
		return nil
	}
	summary := record.toSummary()
	return &summary
}

func (app *App) findVisibleStudentAssignmentByID(assignmentID, classID int64) (*assignmentSummary, error) {
	assignment, err := app.findAssignmentByID(assignmentID, classID)
	if err != nil {
		return nil, err
	}
	if assignment.Status != "published" {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "作业不存在"}
	}
	return assignment, nil
}

func (app *App) listStudentAssignmentsPage(student studentRecord, query teacherListQuery) ([]studentAssignmentSummary, paginationPayload, error) {
	whereClause := ` where class_id = ? and status = 'published'`
	args := []any{student.ClassID}
	var total int
	if err := app.db.QueryRow(`select count(*) from assignments`+whereClause, args...).Scan(&total); err != nil {
		return nil, paginationPayload{}, err
	}

	listQuery := `select id, class_id, title, description, due_at, status, submission_mode, submission_type_category, min_file_count, created_at, updated_at
from assignments` + whereClause + ` order by updated_at desc, id desc`
	listArgs := append([]any{}, args...)
	if query.Paged {
		listQuery += ` limit ? offset ?`
		listArgs = append(listArgs, query.PageSize, query.Offset)
	}
	rows, err := app.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	items := make([]studentAssignmentSummary, 0)
	for rows.Next() {
		var assignment assignmentSummary
		if err := rows.Scan(
			&assignment.ID,
			&assignment.ClassID,
			&assignment.Title,
			&assignment.Description,
			&assignment.DueAt,
			&assignment.Status,
			&assignment.SubmissionMode,
			&assignment.SubmissionTypeCategory,
			&assignment.MinFileCount,
			&assignment.CreatedAt,
			&assignment.UpdatedAt,
		); err != nil {
			return nil, paginationPayload{}, err
		}
		submission, err := app.findAssignmentSubmissionByAssignmentAndStudent(assignment.ID, student.ID)
		if err != nil {
			return nil, paginationPayload{}, err
		}
		items = append(items, studentAssignmentSummary{
			ID:                     assignment.ID,
			ClassID:                assignment.ClassID,
			Title:                  assignment.Title,
			Description:            assignment.Description,
			DueAt:                  assignment.DueAt,
			Status:                 assignment.Status,
			SubmissionMode:         assignment.SubmissionMode,
			SubmissionTypeCategory: assignment.SubmissionTypeCategory,
			MinFileCount:           assignment.MinFileCount,
			CreatedAt:              assignment.CreatedAt,
			UpdatedAt:              assignment.UpdatedAt,
			Overdue:                assignmentIsOverdue(assignment.DueAt, now),
			Submission:             toAssignmentSubmissionSummary(submission),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return items, buildTeacherListPagination(total, query, len(items)), nil
}

func (app *App) listAssignmentSubmissions(assignmentID, classID int64, query teacherListQuery) ([]teacherAssignmentSubmissionItem, paginationPayload, error) {
	if _, err := app.findAssignmentByID(assignmentID, classID); err != nil {
		return nil, paginationPayload{}, err
	}

	whereParts := []string{`s.assignment_id = ?`, `st.class_id = ?`}
	args := []any{assignmentID, classID}
	if query.Keyword != "" {
		whereParts = append(whereParts, `(st.student_no like ? or st.display_name like ? or s.teacher_comment like ?)`)
		likeKeyword := "%" + query.Keyword + "%"
		args = append(args, likeKeyword, likeKeyword, likeKeyword)
	}
	whereClause := " where " + strings.Join(whereParts, " and ")

	var total int
	countQuery := `select count(*)
from assignment_submissions s
join students st on st.id = s.student_id` + whereClause
	if err := app.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, paginationPayload{}, err
	}

	orderBy := ` order by s.updated_at desc, s.id desc`
	switch query.Sort {
	case "updatedAt-asc":
		orderBy = ` order by s.updated_at asc, s.id asc`
	case "studentNo-asc":
		orderBy = ` order by st.student_no asc, s.id asc`
	case "studentNo-desc":
		orderBy = ` order by st.student_no desc, s.id desc`
	case "displayName-asc":
		orderBy = ` order by st.display_name asc, st.student_no asc, s.id asc`
	case "displayName-desc":
		orderBy = ` order by st.display_name desc, st.student_no asc, s.id asc`
	}

	listQuery := `
select s.id, s.student_id, st.student_no, st.display_name, s.status, s.submitted_at, s.updated_at, s.review_status, s.teacher_comment, s.reviewed_at, s.reviewer_name
from assignment_submissions s
join students st on st.id = s.student_id
` + whereClause + orderBy
	listArgs := append([]any{}, args...)
	if query.Paged {
		listQuery += ` limit ? offset ?`
		listArgs = append(listArgs, query.PageSize, query.Offset)
	}

	rows, err := app.db.Query(listQuery, listArgs...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()

	items := make([]teacherAssignmentSubmissionItem, 0)
	for rows.Next() {
		var item teacherAssignmentSubmissionItem
		if err := rows.Scan(
			&item.ID,
			&item.StudentID,
			&item.StudentNo,
			&item.DisplayName,
			&item.Status,
			&item.SubmittedAt,
			&item.UpdatedAt,
			&item.ReviewStatus,
			&item.TeacherCommentSummary,
			&item.ReviewedAt,
			&item.ReviewerName,
		); err != nil {
			return nil, paginationPayload{}, err
		}
		if item.ReviewStatus == "" {
			item.ReviewStatus = assignmentSubmissionReviewPending
		}
		files, err := app.listTeacherSubmissionFiles(assignmentID, item.ID, classID)
		if err != nil {
			return nil, paginationPayload{}, err
		}
		item.Items = files
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return items, buildTeacherListPagination(total, query, len(items)), nil
}

func (app *App) buildAssignmentStatistics(classID int64, assignmentIDs []int64) (*assignmentStatisticsResult, error) {
	if err := app.ensureClassExists(classID); err != nil {
		return nil, err
	}

	assignments := make([]assignmentSummary, 0)
	if len(assignmentIDs) == 0 {
		items, err := app.listAssignments(classID)
		if err != nil {
			return nil, err
		}
		assignments = items
	} else {
		for _, assignmentID := range assignmentIDs {
			assignment, err := app.findAssignmentByID(assignmentID, classID)
			if err != nil {
				return nil, err
			}
			assignments = append(assignments, *assignment)
		}
	}

	resolvedAssignmentIDs := make([]int64, 0, len(assignments))
	assignmentIndex := make(map[int64]struct{}, len(assignments))
	for _, assignment := range assignments {
		resolvedAssignmentIDs = append(resolvedAssignmentIDs, assignment.ID)
		assignmentIndex[assignment.ID] = struct{}{}
	}

	students, err := app.listArchiveStudents(classID)
	if err != nil {
		return nil, err
	}

	rowsByStudentID := make(map[int64]*assignmentStatisticsRowItem, len(students))
	rows := make([]assignmentStatisticsRowItem, len(students))
	for index, student := range students {
		rows[index] = assignmentStatisticsRowItem{
			StudentID:   student.ID,
			StudentNo:   student.StudentNo,
			DisplayName: student.DisplayName,
		}
		rowsByStudentID[student.ID] = &rows[index]
	}

	submittedTotal := 0
	if len(resolvedAssignmentIDs) > 0 && len(students) > 0 {
		submittedPairs, err := app.queryAssignmentStatisticsSubmittedPairs(classID, resolvedAssignmentIDs)
		if err != nil {
			return nil, err
		}
		defer submittedPairs.Close()

		seenPairs := make(map[string]struct{})
		for submittedPairs.Next() {
			var assignmentID int64
			var studentID int64
			if err := submittedPairs.Scan(&assignmentID, &studentID); err != nil {
				return nil, err
			}
			if _, exists := assignmentIndex[assignmentID]; !exists {
				continue
			}
			row, exists := rowsByStudentID[studentID]
			if !exists {
				continue
			}
			pairKey := strconv.FormatInt(assignmentID, 10) + ":" + strconv.FormatInt(studentID, 10)
			if _, exists := seenPairs[pairKey]; exists {
				continue
			}
			seenPairs[pairKey] = struct{}{}
			row.SubmittedCount++
			submittedTotal++
		}
		if err := submittedPairs.Err(); err != nil {
			return nil, err
		}
	}

	for index := range rows {
		rows[index].MissingCount = len(resolvedAssignmentIDs) - rows[index].SubmittedCount
	}

	expectedTotal := len(students) * len(resolvedAssignmentIDs)
	return &assignmentStatisticsResult{
		ClassID:         classID,
		AssignmentIDs:   resolvedAssignmentIDs,
		RosterTotal:     len(students),
		AssignmentTotal: len(resolvedAssignmentIDs),
		ExpectedTotal:   expectedTotal,
		SubmittedTotal:  submittedTotal,
		MissingTotal:    expectedTotal - submittedTotal,
		Rows:            rows,
	}, nil
}

func (app *App) queryAssignmentStatisticsSubmittedPairs(classID int64, assignmentIDs []int64) (*sql.Rows, error) {
	ids := make([]string, 0, len(assignmentIDs))
	args := []any{classID}
	for _, assignmentID := range assignmentIDs {
		ids = append(ids, "?")
		args = append(args, assignmentID)
	}
	return app.db.Query(`
select distinct s.assignment_id, s.student_id
from assignment_submissions s
join students st on st.id = s.student_id
where st.class_id = ? and s.status = 'submitted' and s.assignment_id in (`+strings.Join(ids, ",")+`)
order by s.assignment_id asc, st.student_no asc, st.id asc`, args...)
}

func parseAssignmentArchiveIDs(raw string) ([]int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, ",")
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{})
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "assignmentIds 非法"}
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseFileArchiveEntryIDs(raw string) ([]int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "entryIds 不能为空"}
	}
	parts := strings.Split(trimmed, ",")
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{})
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "entryIds 非法"}
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func (app *App) buildAssignmentSubmissionsArchive(classID int64, assignmentIDs []int64) ([]byte, string, error) {
	if err := app.ensureClassExists(classID); err != nil {
		return nil, "", err
	}

	assignments := make([]assignmentSummary, 0)
	if len(assignmentIDs) == 0 {
		items, err := app.listAssignments(classID)
		if err != nil {
			return nil, "", err
		}
		assignments = items
	} else {
		// Use batch query instead of N+1 queries
		items, err := app.findAssignmentsByIDs(classID, assignmentIDs)
		if err != nil {
			return nil, "", err
		}
		if len(items) != len(assignmentIDs) {
			return nil, "", domainError{Status: http.StatusNotFound, Code: "not_found", Message: "部分作业不存在"}
		}
		assignments = items
	}
	if len(assignments) == 0 {
		return nil, "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "当前班级没有可下载的作业"}
	}

	assignmentFolders := make(map[int64]string, len(assignments))
	usedAssignmentFolders := make(map[string]int)
	for _, assignment := range assignments {
		folderName := uniqueArchiveSegment(safeArchiveSegment(assignment.Title, "未命名作业"), usedAssignmentFolders)
		assignmentFolders[assignment.ID] = folderName
	}

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	createdDirs := make(map[string]struct{})
	for _, folder := range assignmentFolders {
		if err := ensureZipDirectory(archive, createdDirs, folder); err != nil {
			_ = archive.Close()
			return nil, "", err
		}
	}

	rows, err := app.queryAssignmentSubmissionArchiveRows(classID, assignments)
	if err != nil {
		_ = archive.Close()
		return nil, "", err
	}

	manifestRows := make([]assignmentSubmissionArchiveManifestRow, 0)
	submittedStudents := make(map[int64]map[int64]struct{}, len(assignments))
	for rows.Next() {
		var submissionID int64
		var assignmentID int64
		var studentID int64
		var studentNo string
		var displayName string
		var submittedAt string
		var reviewStatus string
		var teacherComment string
		if err := rows.Scan(&submissionID, &assignmentID, &studentID, &studentNo, &displayName, &submittedAt, &reviewStatus, &teacherComment); err != nil {
			_ = archive.Close()
			return nil, "", err
		}
		if _, exists := submittedStudents[assignmentID]; !exists {
			submittedStudents[assignmentID] = make(map[int64]struct{})
		}
		submittedStudents[assignmentID][studentID] = struct{}{}
		assignmentFolder, exists := assignmentFolders[assignmentID]
		if !exists {
			continue
		}
		studentFolder := safeArchiveSegment(studentNo+"-"+displayName, "未命名学生")
		studentRoot := path.Join(assignmentFolder, studentFolder)
		if err := ensureZipDirectory(archive, createdDirs, studentRoot); err != nil {
			_ = archive.Close()
			return nil, "", err
		}
		files, err := app.listSubmissionArchiveFiles(submissionID, classID)
		if err != nil {
			_ = archive.Close()
			return nil, "", err
		}
		fileCount := len(files)
		for _, file := range files {
			relativePath := strings.TrimPrefix(file.ItemPath, assignmentSubmissionParentPath(submissionID)+"/")
			relativePath = trimDuplicateStudentFolder(relativePath, studentFolder)
			archivePath := path.Join(studentRoot, relativePath)
			if err := app.writeArchiveFile(archive, file, archivePath); err != nil {
				_ = archive.Close()
				return nil, "", err
			}
			manifestRows = append(manifestRows, assignmentSubmissionArchiveManifestRow{
				AssignmentTitle: assignmentFolder,
				StudentNo:       studentNo,
				DisplayName:     displayName,
				SubmittedAt:     submittedAt,
				FileCount:       fileCount,
				FilePath:        relativePath,
				ReviewStatus:    reviewStatus,
				TeacherComment:  teacherComment,
			})
		}
	}
	if err := rows.Err(); err != nil {
		_ = archive.Close()
		_ = rows.Close()
		return nil, "", err
	}
	if err := rows.Close(); err != nil {
		_ = archive.Close()
		return nil, "", err
	}
	if err := writeAssignmentSubmissionArchiveManifest(archive, manifestRows); err != nil {
		_ = archive.Close()
		return nil, "", err
	}
	partialStudents, err := app.buildAssignmentSubmissionPartialStudents(classID, assignments)
	if err != nil {
		_ = archive.Close()
		return nil, "", err
	}
	missingRows, err := app.buildAssignmentSubmissionMissingArchiveRows(classID, assignments, assignmentFolders, submittedStudents, partialStudents)
	if err != nil {
		_ = archive.Close()
		return nil, "", err
	}
	if err := writeAssignmentSubmissionMissingArchiveManifest(archive, missingRows); err != nil {
		_ = archive.Close()
		return nil, "", err
	}
	if err := archive.Close(); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), "作业提交.zip", nil
}

type assignmentSubmissionArchiveManifestRow struct {
	AssignmentTitle string
	StudentNo       string
	DisplayName     string
	SubmittedAt     string
	FileCount       int
	FilePath        string
	ReviewStatus    string
	TeacherComment  string
}

type assignmentSubmissionMissingArchiveRow struct {
	AssignmentTitle string
	StudentNo       string
	DisplayName     string
	Status          string
}

func (app *App) queryAssignmentSubmissionArchiveRows(classID int64, assignments []assignmentSummary) (*sql.Rows, error) {
	ids := make([]string, 0, len(assignments))
	args := []any{classID}
	for _, assignment := range assignments {
		ids = append(ids, "?")
		args = append(args, assignment.ID)
	}
	return app.db.Query(`
select s.id, s.assignment_id, st.id, st.student_no, st.display_name, s.submitted_at, s.review_status, s.teacher_comment
from assignment_submissions s
join students st on st.id = s.student_id
where st.class_id = ? and s.assignment_id in (`+strings.Join(ids, ",")+`)
  and s.status = 'submitted'
order by s.assignment_id asc, st.student_no asc, st.display_name asc`, args...)
}

func (app *App) buildAssignmentSubmissionMissingArchiveRows(classID int64, assignments []assignmentSummary, assignmentFolders map[int64]string, submittedStudents map[int64]map[int64]struct{}, partialStudents map[int64]map[int64]struct{}) ([]assignmentSubmissionMissingArchiveRow, error) {
	students, err := app.listArchiveStudents(classID)
	if err != nil {
		return nil, err
	}
	rows := make([]assignmentSubmissionMissingArchiveRow, 0)
	for _, assignment := range assignments {
		assignmentFolder := assignmentFolders[assignment.ID]
		submittedForAssignment := submittedStudents[assignment.ID]
		for _, student := range students {
			if submittedForAssignment != nil {
				if _, exists := submittedForAssignment[student.ID]; exists {
					continue
				}
			}
			status := "未提交"
			if partialForAssignment := partialStudents[assignment.ID]; partialForAssignment != nil {
				if _, exists := partialForAssignment[student.ID]; exists {
					status = "待补齐"
				}
			}
			rows = append(rows, assignmentSubmissionMissingArchiveRow{
				AssignmentTitle: assignmentFolder,
				StudentNo:       student.StudentNo,
				DisplayName:     student.DisplayName,
				Status:          status,
			})
		}
	}
	return rows, nil
}

func (app *App) buildAssignmentSubmissionPartialStudents(classID int64, assignments []assignmentSummary) (map[int64]map[int64]struct{}, error) {
	if len(assignments) == 0 {
		return map[int64]map[int64]struct{}{}, nil
	}
	ids := make([]string, 0, len(assignments))
	args := []any{classID}
	for _, assignment := range assignments {
		ids = append(ids, "?")
		args = append(args, assignment.ID)
	}
	rows, err := app.db.Query(`
select s.assignment_id, s.student_id
from assignment_submissions s
join students st on st.id = s.student_id
where st.class_id = ? and s.status = 'partial' and s.assignment_id in (`+strings.Join(ids, ",")+`)
order by s.assignment_id asc, st.student_no asc, st.id asc`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]map[int64]struct{}, len(assignments))
	for rows.Next() {
		var assignmentID int64
		var studentID int64
		if err := rows.Scan(&assignmentID, &studentID); err != nil {
			return nil, err
		}
		if _, exists := result[assignmentID]; !exists {
			result[assignmentID] = make(map[int64]struct{})
		}
		result[assignmentID][studentID] = struct{}{}
	}
	return result, rows.Err()
}

func (app *App) listArchiveStudents(classID int64) ([]studentResult, error) {
	rows, err := app.db.Query(`
select id, class_id, student_no, display_name, activated_at
from students
where class_id = ?
order by student_no asc, display_name asc`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := make([]studentResult, 0)
	for rows.Next() {
		var item studentResult
		if err := rows.Scan(&item.ID, &item.ClassID, &item.StudentNo, &item.DisplayName, &item.ActivatedAt); err != nil {
			return nil, err
		}
		students = append(students, item)
	}
	return students, rows.Err()
}

func writeAssignmentSubmissionArchiveManifest(archive *zip.Writer, rows []assignmentSubmissionArchiveManifestRow) error {
	writer, err := archive.Create("提交清单.csv")
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
		return err
	}
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"作业", "学号", "姓名", "提交时间", "文件数", "文件路径", "批改状态", "教师评语"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := csvWriter.Write([]string{
			row.AssignmentTitle,
			row.StudentNo,
			row.DisplayName,
			row.SubmittedAt,
			strconv.Itoa(row.FileCount),
			row.FilePath,
			assignmentSubmissionReviewStatusCSVLabel(row.ReviewStatus),
			row.TeacherComment,
		}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func writeAssignmentSubmissionMissingArchiveManifest(archive *zip.Writer, rows []assignmentSubmissionMissingArchiveRow) error {
	writer, err := archive.Create("未提交清单.csv")
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
		return err
	}
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"作业", "学号", "姓名", "状态"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := csvWriter.Write([]string{
			row.AssignmentTitle,
			row.StudentNo,
			row.DisplayName,
			row.Status,
		}); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func assignmentSubmissionReviewStatusCSVLabel(status string) string {
	switch status {
	case assignmentSubmissionReviewReviewed:
		return "已批改"
	default:
		return "待批改"
	}
}

func (app *App) listSubmissionArchiveFiles(submissionID, classID int64) ([]fileEntry, error) {
	parentPath := assignmentSubmissionParentPath(submissionID)
	rows, err := app.db.Query(`
select id, space, class_id, parent_path, item_path, name, kind, mime_type, size, disk_path, created_at, updated_at
from file_entries
where space = 'submission'
  and class_id = ?
  and kind = 'file'
  and item_path like ?
order by item_path asc`,
		classID,
		parentPath+"/%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]fileEntry, 0)
	for rows.Next() {
		var item fileEntry
		if err := rows.Scan(&item.ID, &item.Space, &item.ClassID, &item.ParentPath, &item.ItemPath, &item.Name, &item.Kind, &item.MimeType, &item.Size, &item.DiskPath, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func safeArchiveSegment(raw, fallback string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = fallback
	}
	replacer := strings.NewReplacer(
		"/", "／",
		"\\", "＼",
		":", "：",
		"*", "＊",
		"?", "？",
		"\"", "＂",
		"<", "＜",
		">", "＞",
		"|", "｜",
	)
	clean := strings.TrimSpace(replacer.Replace(trimmed))
	if clean == "" {
		return fallback
	}
	return clean
}

func trimDuplicateStudentFolder(relativePath, studentFolder string) string {
	normalized := strings.ReplaceAll(strings.Trim(relativePath, "/"), "\\", "/")
	parts := strings.Split(normalized, "/")
	if len(parts) <= 1 {
		return normalized
	}
	if safeArchiveSegment(parts[0], "") != studentFolder {
		return normalized
	}
	return path.Join(parts[1:]...)
}

func uniqueArchiveSegment(base string, used map[string]int) string {
	count := used[base]
	used[base] = count + 1
	if count == 0 {
		return base
	}
	return fmt.Sprintf("%s (%d)", base, count+1)
}

func ensureZipDirectory(archive *zip.Writer, created map[string]struct{}, directory string) error {
	clean := strings.Trim(strings.ReplaceAll(directory, "\\", "/"), "/")
	if clean == "" {
		return nil
	}
	parts := strings.Split(clean, "/")
	current := ""
	for _, part := range parts {
		current = path.Join(current, part)
		key := current + "/"
		if _, exists := created[key]; exists {
			continue
		}
		if _, err := archive.Create(key); err != nil {
			return err
		}
		created[key] = struct{}{}
	}
	return nil
}

func (app *App) writeArchiveFile(archive *zip.Writer, entry fileEntry, archivePath string) error {
	header := &zip.FileHeader{
		Name:   archivePath,
		Method: zip.Deflate,
	}
	if modifiedAt, parseErr := time.Parse(time.RFC3339, entry.UpdatedAt); parseErr == nil {
		header.SetModTime(modifiedAt)
	}
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	absolutePath := filepath.Join(app.storageRoot, filepath.FromSlash(entry.DiskPath))
	file, err := os.Open(absolutePath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(writer, file)
	_ = file.Close()
	return copyErr
}

func (app *App) listTeacherSubmissionFiles(assignmentID, submissionID, classID int64) ([]fileSummary, error) {
	entries, err := app.listFileEntries("submission", optionalInt64(classID), assignmentSubmissionParentPath(submissionID))
	if err != nil {
		return nil, err
	}
	sortSubmissionFileEntries(entries)
	items := make([]fileSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind != "file" && entry.Kind != "dir" {
			continue
		}
		item, buildErr := app.buildTeacherSubmissionFileTree(assignmentID, classID, entry)
		if buildErr != nil {
			return nil, buildErr
		}
		items = append(items, item)
	}
	return items, nil
}

func (app *App) updateAssignmentSubmissionReview(assignmentID, classID, submissionID int64, payload updateAssignmentSubmissionReviewRequest, teacher teacherRecord) (*teacherAssignmentSubmissionItem, error) {
	reviewStatus, err := normalizeAssignmentSubmissionReviewStatus(payload.ReviewStatus)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	teacherComment := strings.TrimSpace(payload.TeacherComment)

	row := app.db.QueryRow(`
select s.id, s.student_id, st.student_no, st.display_name, s.status, s.submitted_at, s.updated_at, s.review_status, s.teacher_comment, s.reviewed_at, s.reviewer_name
from assignment_submissions s
join students st on st.id = s.student_id
where s.id = ? and s.assignment_id = ? and st.class_id = ?`,
		submissionID,
		assignmentID,
		classID,
	)
	var item teacherAssignmentSubmissionItem
	if err := row.Scan(
		&item.ID,
		&item.StudentID,
		&item.StudentNo,
		&item.DisplayName,
		&item.Status,
		&item.SubmittedAt,
		&item.UpdatedAt,
		&item.ReviewStatus,
		&item.TeacherCommentSummary,
		&item.ReviewedAt,
		&item.ReviewerName,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "提交不存在"}
		}
		return nil, err
	}

	reviewedAt := ""
	reviewerName := ""
	if reviewStatus == assignmentSubmissionReviewReviewed {
		reviewedAt = time.Now().UTC().Format(time.RFC3339)
		reviewerName = teacher.DisplayName
	}
	if _, err := app.db.Exec(`update assignment_submissions set review_status = ?, teacher_comment = ?, reviewed_at = ?, reviewer_name = ? where id = ?`, reviewStatus, teacherComment, reviewedAt, reviewerName, submissionID); err != nil {
		return nil, err
	}
	files, err := app.listTeacherSubmissionFiles(assignmentID, submissionID, classID)
	if err != nil {
		return nil, err
	}
	item.ReviewStatus = reviewStatus
	item.TeacherCommentSummary = teacherComment
	item.ReviewedAt = reviewedAt
	item.ReviewerName = reviewerName
	item.Items = files
	return &item, nil
}

func (app *App) getStudentAssignmentDetail(assignmentID int64, student studentRecord) (*studentAssignmentDetail, error) {
	assignment, err := app.findVisibleStudentAssignmentByID(assignmentID, student.ClassID)
	if err != nil {
		return nil, err
	}
	submission, err := app.findAssignmentSubmissionByAssignmentAndStudent(assignmentID, student.ID)
	if err != nil {
		return nil, err
	}
	items := make([]fileSummary, 0)
	if submission != nil {
		items, err = app.listSubmissionFiles(assignmentID, submission.ID, student.ClassID)
		if err != nil {
			return nil, err
		}
	}
	assignmentAttachments, err := app.listStudentAssignmentAttachments(assignmentID, student.ClassID)
	if err != nil {
		return nil, err
	}
	examLocked, err := app.isClassExamLocked(student.ClassID)
	if err != nil {
		return nil, err
	}
	return &studentAssignmentDetail{
		ID:                     assignment.ID,
		ClassID:                assignment.ClassID,
		Title:                  assignment.Title,
		Description:            assignment.Description,
		DueAt:                  assignment.DueAt,
		Status:                 assignment.Status,
		SubmissionMode:         assignment.SubmissionMode,
		SubmissionTypeCategory: assignment.SubmissionTypeCategory,
		MinFileCount:           assignment.MinFileCount,
		CreatedAt:              assignment.CreatedAt,
		UpdatedAt:              assignment.UpdatedAt,
		Overdue:                assignmentIsOverdue(assignment.DueAt, time.Now().UTC()),
		Submission:             toAssignmentSubmissionSummary(submission),
		SubmissionConstraints:  studentSubmissionConstraintsPayloadForCategory(assignment.SubmissionTypeCategory),
		AssignmentAttachments:  assignmentAttachments,
		Items:                  items,
		ExamLocked:             examLocked,
	}, nil
}

func (app *App) submitStudentAssignment(assignmentID int64, student studentRecord, headers []*multipart.FileHeader, relativePaths []string) (*studentSubmissionResult, error) {
	assignment, err := app.findVisibleStudentAssignmentByID(assignmentID, student.ClassID)
	if err != nil {
		return nil, err
	}
	if assignmentIsOverdue(assignment.DueAt, time.Now().UTC()) {
		return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "当前作业已截止，不能再提交"}
	}
	if err := validateStudentSubmissionHeadersForCategory(headers, assignment.SubmissionTypeCategory); err != nil {
		return nil, err
	}
	if err := validateStudentSubmissionRule(*assignment, headers, relativePaths); err != nil {
		return nil, err
	}

	submission, err := app.findAssignmentSubmissionByAssignmentAndStudent(assignmentID, student.ID)
	if err != nil {
		return nil, err
	}
	if submission != nil {
		if err := app.validateExistingSubmissionUploadRoot(*assignment, submission.ID, student.ClassID, headers, relativePaths); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if submission == nil {
		result, err := app.db.Exec(`
insert into assignment_submissions (assignment_id, student_id, status, submitted_at, updated_at, review_status, teacher_comment)
values (?, ?, ?, ?, ?, ?, ?)`,
			assignmentID,
			student.ID,
			assignmentSubmissionStatusPartial,
			"",
			now,
			assignmentSubmissionReviewPending,
			"",
		)
		if err != nil {
			return nil, err
		}
		submissionID, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}
		submission = &assignmentSubmissionRecord{
			ID:             submissionID,
			AssignmentID:   assignmentID,
			StudentID:      student.ID,
			Status:         assignmentSubmissionStatusPartial,
			SubmittedAt:    "",
			UpdatedAt:      now,
			ReviewStatus:   assignmentSubmissionReviewPending,
			TeacherComment: "",
			ReviewedAt:     "",
			ReviewerName:   "",
		}
	}

	parentPath := assignmentSubmissionParentPath(submission.ID)
	for index, header := range headers {
		relativePath := ""
		if index < len(relativePaths) {
			relativePath = relativePaths[index]
		}
		targetParentPath, targetName, resolveErr := app.resolveSubmissionUploadTarget(student.ClassID, parentPath, header.Filename, relativePath)
		if resolveErr != nil {
			return nil, resolveErr
		}
		file, err := header.Open()
		if err != nil {
			return nil, err
		}
		entry, _, createErr := app.createFileWithConflict("submission", optionalInt64(student.ClassID), targetParentPath, targetName, file, uploadConflictModeReplace)
		_ = file.Close()
		if createErr != nil {
			return nil, createErr
		}
		_ = entry
	}

	fileCount, err := app.countSubmissionFiles(submission.ID, student.ClassID)
	if err != nil {
		return nil, err
	}
	summary, err := app.updateAssignmentSubmissionAfterFileMutation(*assignment, submission, fileCount, now)
	if err != nil {
		return nil, err
	}
	items, err := app.listSubmissionFiles(assignmentID, submission.ID, student.ClassID)
	if err != nil {
		return nil, err
	}

	return &studentSubmissionResult{
		Submission: summary,
		Items:      items,
	}, nil
}

func (app *App) validateExistingSubmissionUploadRoot(assignment assignmentSummary, submissionID, classID int64, headers []*multipart.FileHeader, relativePaths []string) error {
	mode, err := normalizeAssignmentSubmissionMode(assignment.SubmissionMode)
	if err != nil {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	if mode != assignmentSubmissionModeFolder {
		return nil
	}
	uploadRoot, err := folderSubmissionUploadRoot(headers, relativePaths)
	if err != nil {
		return err
	}
	if uploadRoot == "" {
		return nil
	}
	existingRoot, err := app.currentSubmissionRootName(submissionID, classID)
	if err != nil {
		return err
	}
	if existingRoot != "" && existingRoot != uploadRoot {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "本次作业只能提交一个文件夹，请继续补充当前提交文件夹"}
	}
	return nil
}

func folderSubmissionUploadRoot(headers []*multipart.FileHeader, relativePaths []string) (string, error) {
	for index := range headers {
		segments, err := submissionRelativePathSegments(index, relativePaths)
		if err != nil {
			return "", err
		}
		if len(segments) >= 2 {
			return segments[0], nil
		}
	}
	return "", nil
}

func (app *App) currentSubmissionRootName(submissionID, classID int64) (string, error) {
	entries, err := app.listFileEntries("submission", optionalInt64(classID), assignmentSubmissionParentPath(submissionID))
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	return entries[0].Name, nil
}

func (app *App) deleteStudentSubmissionFile(assignmentID int64, student studentRecord, fileID int64) (*studentSubmissionResult, error) {
	assignment, err := app.findVisibleStudentAssignmentByID(assignmentID, student.ClassID)
	if err != nil {
		return nil, err
	}
	if assignmentIsOverdue(assignment.DueAt, time.Now().UTC()) {
		return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "当前作业已截止，不能再修改提交"}
	}
	submission, err := app.findAssignmentSubmissionByAssignmentAndStudent(assignmentID, student.ID)
	if err != nil {
		return nil, err
	}
	if submission == nil {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "附件不存在"}
	}
	entry, err := app.findStudentSubmissionFileByID(assignmentID, student, fileID)
	if err != nil {
		return nil, err
	}
	if err := app.deleteEntry(entry.ID); err != nil {
		return nil, err
	}

	fileCount, err := app.countSubmissionFiles(submission.ID, student.ClassID)
	if err != nil {
		return nil, err
	}
	if fileCount == 0 {
		if err := app.removeSubmissionDirectory(submission.ID, student.ClassID); err != nil {
			return nil, err
		}
		if _, err := app.db.Exec(`delete from assignment_submissions where id = ?`, submission.ID); err != nil {
			return nil, err
		}
		return &studentSubmissionResult{
			Submission: nil,
			Items:      []fileSummary{},
		}, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	summary, err := app.updateAssignmentSubmissionAfterFileMutation(*assignment, submission, fileCount, now)
	if err != nil {
		return nil, err
	}
	items, err := app.listSubmissionFiles(assignmentID, submission.ID, student.ClassID)
	if err != nil {
		return nil, err
	}
	return &studentSubmissionResult{
		Submission: summary,
		Items:      items,
	}, nil
}

func (app *App) resolveSubmissionUploadTarget(classID int64, parentPath, fallbackName, relativePath string) (string, string, error) {
	normalizedParentPath, err := normalizeRelativePath(parentPath)
	if err != nil {
		return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	segments, err := normalizeUploadRelativePath(relativePath)
	if err != nil {
		return "", "", err
	}
	if len(segments) == 0 {
		return normalizedParentPath, fallbackName, app.ensureDirectoryHierarchy("submission", optionalInt64(classID), normalizedParentPath)
	}

	targetName := segments[len(segments)-1]
	targetParentPath := normalizedParentPath
	if len(segments) > 1 {
		targetParentPath = joinChild(normalizedParentPath, path.Join(segments[:len(segments)-1]...))
	}
	if err := app.ensureDirectoryHierarchy("submission", optionalInt64(classID), targetParentPath); err != nil {
		return "", "", err
	}
	return targetParentPath, targetName, nil
}

func (app *App) updateAssignmentSubmissionAfterFileMutation(assignment assignmentSummary, submission *assignmentSubmissionRecord, fileCount int, now string) (*assignmentSubmissionSummary, error) {
	status, err := assignmentSubmissionStatusForFileCount(assignment, fileCount)
	if err != nil {
		return nil, err
	}
	submittedAt := submission.SubmittedAt
	if status == assignmentSubmissionStatusSubmitted && submittedAt == "" {
		submittedAt = now
	}
	if status == assignmentSubmissionStatusPartial {
		submittedAt = ""
	}
	if _, err := app.db.Exec(`
update assignment_submissions
set status = ?, submitted_at = ?, updated_at = ?, review_status = ?, teacher_comment = ?, reviewed_at = ?, reviewer_name = ?
where id = ?`,
		status,
		submittedAt,
		now,
		assignmentSubmissionReviewPending,
		"",
		"",
		"",
		submission.ID,
	); err != nil {
		return nil, err
	}
	submission.Status = status
	submission.SubmittedAt = submittedAt
	submission.UpdatedAt = now
	submission.ReviewStatus = assignmentSubmissionReviewPending
	submission.TeacherComment = ""
	submission.ReviewedAt = ""
	submission.ReviewerName = ""
	summary := submission.toSummary()
	return &summary, nil
}

func assignmentSubmissionStatusForFileCount(assignment assignmentSummary, fileCount int) (string, error) {
	minFileCount, err := validateAssignmentMinFileCount(assignment.MinFileCount)
	if err != nil {
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	if fileCount >= minFileCount {
		return assignmentSubmissionStatusSubmitted, nil
	}
	return assignmentSubmissionStatusPartial, nil
}

func (app *App) findStudentSubmissionFileByID(assignmentID int64, student studentRecord, fileID int64) (*fileEntry, error) {
	if _, err := app.findVisibleStudentAssignmentByID(assignmentID, student.ClassID); err != nil {
		return nil, err
	}
	submission, err := app.findAssignmentSubmissionByAssignmentAndStudent(assignmentID, student.ID)
	if err != nil {
		return nil, err
	}
	if submission == nil {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "附件不存在"}
	}
	entry, err := app.getEntryByID(fileID)
	if err != nil {
		return nil, err
	}
	if entry.Space != "submission" || !entry.ClassID.Valid || entry.ClassID.Int64 != student.ClassID || !submissionEntryBelongsToSubmission(entry, submission.ID) {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "附件不存在"}
	}
	return entry, nil
}

func (app *App) findAssignmentSubmissionFileByID(assignmentID, classID, fileID int64) (*fileEntry, error) {
	if _, err := app.findAssignmentByID(assignmentID, classID); err != nil {
		return nil, err
	}
	entry, err := app.getEntryByID(fileID)
	if err != nil {
		return nil, err
	}
	if entry.Space != "submission" || !entry.ClassID.Valid || entry.ClassID.Int64 != classID {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "附件不存在"}
	}
	submissionID, ok := submissionIDFromEntryPath(entry)
	if !ok {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "附件不存在"}
	}

	var count int
	if err := app.db.QueryRow(`
select count(*)
from assignment_submissions s
join students st on st.id = s.student_id
where s.id = ? and s.assignment_id = ? and st.class_id = ?`,
		submissionID,
		assignmentID,
		classID,
	).Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "附件不存在"}
	}
	return entry, nil
}

func (app *App) activateStudent(payload studentActivateRequest) (*studentRecord, error) {
	joinCode, err := sanitizeRequiredText(payload.JoinCode, "班级注册码")
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	studentNo, err := sanitizeRequiredText(payload.StudentNo, "学号")
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	password, err := validateStudentPassword(payload.Password)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}

	class, err := app.findClassByJoinCode(joinCode)
	if err != nil {
		return nil, err
	}
	if class == nil {
		return nil, domainError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "班级注册码不正确"}
	}
	if isClassRegistrationExpired(class.RegistrationExpiresAt, time.Now().UTC()) {
		if err := app.closeClassRegistration(class.ID); err != nil {
			return nil, err
		}
		return nil, domainError{Status: http.StatusConflict, Code: "registration_closed", Message: "当前班级未开放注册"}
	}
	if !class.RegistrationEnabled {
		return nil, domainError{Status: http.StatusConflict, Code: "registration_closed", Message: "当前班级未开放注册"}
	}

	student, err := app.findStudentByClassAndNo(class.ID, studentNo)
	if err != nil {
		return nil, err
	}
	if student == nil {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "未找到对应学生"}
	}
	if strings.TrimSpace(student.PasswordHash) != "" {
		return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "该学生已激活，请直接登录"}
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	activatedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := app.db.Exec(`update students set password_hash = ?, activated_at = ?, must_change_password = 0 where id = ?`, passwordHash, activatedAt, student.ID); err != nil {
		return nil, err
	}
	student.PasswordHash = passwordHash
	student.ActivatedAt = activatedAt
	student.MustChangePassword = false
	return student, nil
}

func (app *App) authenticateStudent(payload studentLoginRequest) (*studentRecord, error) {
	studentNo := strings.TrimSpace(payload.StudentNo)
	if studentNo == "" || strings.TrimSpace(payload.Password) == "" {
		return nil, domainError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "登录信息不正确"}
	}

	students, err := app.findStudentsByNo(studentNo)
	if err != nil {
		return nil, err
	}
	if len(students) == 0 {
		return nil, domainError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "登录信息不正确"}
	}
	if len(students) > 1 {
		return nil, domainError{Status: http.StatusConflict, Code: "duplicate_student_no", Message: "学号重复，请联系老师整理名册"}
	}

	student := students[0]
	if strings.TrimSpace(student.PasswordHash) == "" || verifyPasswordHash(student.PasswordHash, payload.Password) != nil {
		return nil, domainError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "登录信息不正确"}
	}

	return &student, nil
}

func (app *App) studentLoginLogIdentity(studentNo string) (int64, string) {
	students, err := app.findStudentsByNo(studentNo)
	if err != nil || len(students) != 1 {
		return 0, ""
	}
	student := students[0]
	return student.ID, student.DisplayName
}

func (app *App) resetStudentPassword(studentID int64) (*studentPasswordResetResult, error) {
	student, err := app.findStudentByID(studentID)
	if err != nil {
		return nil, err
	}

	defaultPassword := EffectiveDefaultStudentResetPassword()
	passwordHash, err := hashPassword(defaultPassword)
	if err != nil {
		return nil, err
	}
	activatedAt := student.ActivatedAt
	if strings.TrimSpace(activatedAt) == "" {
		activatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := app.db.Exec(`
update students
set password_hash = ?, activated_at = ?, must_change_password = 1
where id = ?`,
		passwordHash,
		activatedAt,
		studentID,
	); err != nil {
		return nil, err
	}
	if _, err := app.deleteSessionsForStudent(studentID); err != nil {
		return nil, err
	}
	student.ActivatedAt = activatedAt
	return &studentPasswordResetResult{
		Student:         *student,
		DefaultPassword: defaultPassword,
	}, nil
}

func (app *App) changeStudentPassword(student studentRecord, payload studentPasswordChangeRequest) (*studentRecord, error) {
	currentPassword := strings.TrimSpace(payload.CurrentPassword)
	newPassword, err := validateStudentPassword(payload.NewPassword)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	if currentPassword == "" || verifyPasswordHash(student.PasswordHash, currentPassword) != nil {
		return nil, domainError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "当前密码错误"}
	}
	if newPassword == currentPassword {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "新密码不能与当前密码相同"}
	}
	if newPassword == EffectiveDefaultStudentResetPassword() {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "新密码不能使用默认密码"}
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return nil, err
	}
	if _, err := app.db.Exec(`update students set password_hash = ?, must_change_password = 0 where id = ?`, passwordHash, student.ID); err != nil {
		return nil, err
	}
	student.PasswordHash = passwordHash
	student.MustChangePassword = false
	return &student, nil
}

func (app *App) createStudent(payload createStudentRequest) (*studentResult, error) {
	if payload.ClassID <= 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}
	studentNo, err := sanitizeName(payload.StudentNo)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "学号不能为空"}
	}
	displayName, err := sanitizeName(payload.DisplayName)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "姓名不能为空"}
	}
	var classCount int
	if err := app.db.QueryRow(`select count(*) from classes where id = ?`, payload.ClassID).Scan(&classCount); err != nil {
		return nil, err
	}
	if classCount == 0 {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
	}
	if err := app.ensureStudentNoAvailable(studentNo, 0); err != nil {
		return nil, err
	}
	result, err := app.db.Exec(`insert into students (class_id, student_no, display_name) values (?, ?, ?)`, payload.ClassID, studentNo, displayName)
	if err != nil {
		if isUniqueConstraintError(err, "students.student_no") {
			return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "学号已存在"}
		}
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &studentResult{
		ID:          id,
		ClassID:     payload.ClassID,
		StudentNo:   studentNo,
		DisplayName: displayName,
	}, nil
}

func (app *App) updateStudent(studentID int64, payload updateStudentRequest) (*studentResult, error) {
	studentNo, err := sanitizeName(payload.StudentNo)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "学号不能为空"}
	}
	displayName, err := sanitizeName(payload.DisplayName)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "姓名不能为空"}
	}
	current, err := app.findStudentByID(studentID)
	if err != nil {
		return nil, err
	}
	if err := app.ensureStudentNoAvailable(studentNo, studentID); err != nil {
		return nil, err
	}
	_, err = app.db.Exec(`update students set student_no = ?, display_name = ? where id = ?`, studentNo, displayName, studentID)
	if err != nil {
		if isUniqueConstraintError(err, "students.student_no") {
			return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "学号已存在"}
		}
		return nil, err
	}
	return &studentResult{
		ID:          studentID,
		ClassID:     current.ClassID,
		StudentNo:   studentNo,
		DisplayName: displayName,
	}, nil
}

func (app *App) deleteStudent(studentID int64) error {
	if _, err := app.findStudentByID(studentID); err != nil {
		return err
	}
	_, err := app.db.Exec(`delete from students where id = ?`, studentID)
	return err
}

func (app *App) importStudents(payload importStudentsRequest) ([]studentResult, error) {
	if payload.ClassID <= 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}
	seenStudentNos := make(map[string]struct{}, len(payload.Students))
	for _, item := range payload.Students {
		studentNo, err := sanitizeName(item.StudentNo)
		if err != nil {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "学号不能为空"}
		}
		if _, exists := seenStudentNos[studentNo]; exists {
			return nil, domainError{Status: http.StatusConflict, Code: "conflict", Message: "学号已存在"}
		}
		seenStudentNos[studentNo] = struct{}{}
		if err := app.ensureStudentNoAvailable(studentNo, 0); err != nil {
			return nil, err
		}
	}
	var imported []studentResult
	for _, item := range payload.Students {
		created, err := app.createStudent(createStudentRequest{
			ClassID:     payload.ClassID,
			StudentNo:   item.StudentNo,
			DisplayName: item.DisplayName,
		})
		if err != nil {
			return nil, err
		}
		imported = append(imported, *created)
	}
	return imported, nil
}

func (app *App) importStudentsFromFile(classID int64, fileName string, reader io.Reader) ([]studentResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "导入文件不能为空"}
	}

	var records []studentImportRecord
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))) {
	case ".xlsx":
		records, err = parseStudentImportRecordsFromXLSX(data)
	default:
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "仅支持导入 XLSX 文件"}
	}
	if err != nil {
		return nil, err
	}

	students := make([]createStudentRequest, 0, len(records))
	for _, item := range records {
		students = append(students, createStudentRequest{
			ClassID:     classID,
			StudentNo:   item.StudentNo,
			DisplayName: item.DisplayName,
		})
	}

	return app.importStudents(importStudentsRequest{
		ClassID:  classID,
		Students: students,
	})
}

func buildStudentImportTemplate(format string) ([]byte, string, string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "xlsx":
		contents, err := buildStudentImportXLSXTemplate()
		if err != nil {
			return nil, "", "", err
		}
		return contents, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "classdrive-students-template.xlsx", nil
	default:
		return nil, "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "仅支持下载 XLSX 模板"}
	}
}

func buildStudentImportXLSXTemplate() ([]byte, error) {
	return buildInlineWorkbook([][]string{
		{"studentNo", "displayName"},
		{"20260021", "王小刚"},
	})
}

func parseStudentImportRecordsFromXLSX(data []byte) ([]studentImportRecord, error) {
	entries, err := unzipBinaryEntries(data)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "Excel 解析失败"}
	}

	sharedStrings, err := parseSharedStrings(entries["xl/sharedStrings.xml"])
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "Excel 共享字符串解析失败"}
	}

	worksheetData, err := firstWorksheet(entries)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "Excel 文件缺少工作表"}
	}

	rows, err := parseWorksheetRows(worksheetData, sharedStrings)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "Excel 表格解析失败"}
	}

	return parseStudentImportTable(rows)
}

func parseStudentImportTable(rows [][]string) ([]studentImportRecord, error) {
	if len(rows) == 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "导入文件不能为空"}
	}

	headerStudentNo := normalizeImportHeader(tableCell(rows[0], 0))
	headerDisplayName := normalizeImportHeader(tableCell(rows[0], 1))
	if headerStudentNo != "studentNo" || headerDisplayName != "displayName" {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "导入模板表头必须为 studentNo, displayName"}
	}

	records := make([]studentImportRecord, 0, len(rows)-1)
	for index, row := range rows[1:] {
		studentNo := strings.TrimSpace(tableCell(row, 0))
		displayName := strings.TrimSpace(tableCell(row, 1))
		if studentNo == "" && displayName == "" {
			continue
		}
		if studentNo == "" || displayName == "" {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: fmt.Sprintf("第 %d 行必须同时填写学号和姓名", index+2)}
		}
		records = append(records, studentImportRecord{
			StudentNo:   studentNo,
			DisplayName: displayName,
		})
	}
	if len(records) == 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "导入文件没有可导入的数据"}
	}
	return records, nil
}

func tableCell(row []string, index int) string {
	if index < len(row) {
		return row[index]
	}
	return ""
}

func normalizeImportHeader(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(value, "\uFEFF"))
}

func unzipBinaryEntries(data []byte) (map[string][]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	entries := make(map[string][]byte, len(archive.File))
	for _, item := range archive.File {
		stream, err := item.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(stream)
		_ = stream.Close()
		if err != nil {
			return nil, err
		}
		entries[item.Name] = body
	}

	return entries, nil
}

func parseSharedStrings(data []byte) ([]string, error) {
	if len(data) == 0 {
		return []string{}, nil
	}

	var table xlsxSharedStringTable
	if err := xml.Unmarshal(data, &table); err != nil {
		return nil, err
	}

	values := make([]string, 0, len(table.Items))
	for _, item := range table.Items {
		values = append(values, extractTextValue(item.Text, item.Runs))
	}
	return values, nil
}

func firstWorksheet(entries map[string][]byte) ([]byte, error) {
	if body, ok := entries["xl/worksheets/sheet1.xml"]; ok {
		return body, nil
	}

	names := make([]string, 0, len(entries))
	for name := range entries {
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, errors.New("worksheet not found")
	}
	return entries[names[0]], nil
}

func parseWorksheetRows(data []byte, sharedStrings []string) ([][]string, error) {
	var worksheet xlsxWorksheet
	if err := xml.Unmarshal(data, &worksheet); err != nil {
		return nil, err
	}

	rows := make([][]string, 0, len(worksheet.Rows))
	for _, row := range worksheet.Rows {
		if len(row.Cells) == 0 {
			rows = append(rows, []string{})
			continue
		}

		values := map[int]string{}
		maxColumn := -1
		for index, cell := range row.Cells {
			columnIndex, err := xlsxColumnIndex(cell.Ref)
			if err != nil {
				columnIndex = index
			}
			value, err := xlsxCellString(cell, sharedStrings)
			if err != nil {
				return nil, err
			}
			values[columnIndex] = value
			if columnIndex > maxColumn {
				maxColumn = columnIndex
			}
		}

		columns := make([]string, maxColumn+1)
		for columnIndex, value := range values {
			columns[columnIndex] = value
		}
		rows = append(rows, columns)
	}

	return rows, nil
}

func xlsxColumnIndex(ref string) (int, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, errors.New("empty cell ref")
	}

	value := 0
	used := 0
	for _, runeValue := range ref {
		switch {
		case runeValue >= 'A' && runeValue <= 'Z':
			value = value*26 + int(runeValue-'A'+1)
			used++
		case runeValue >= 'a' && runeValue <= 'z':
			value = value*26 + int(runeValue-'a'+1)
			used++
		default:
			if used == 0 {
				return 0, errors.New("invalid cell ref")
			}
			return value - 1, nil
		}
	}
	if used == 0 {
		return 0, errors.New("invalid cell ref")
	}
	return value - 1, nil
}

func xlsxCellString(cell xlsxCell, sharedStrings []string) (string, error) {
	switch cell.Type {
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err != nil || index < 0 || index >= len(sharedStrings) {
			return "", errors.New("shared string index invalid")
		}
		return sharedStrings[index], nil
	case "inlineStr":
		return extractTextValue(cell.InlineString.Text, cell.InlineString.Runs), nil
	default:
		if strings.TrimSpace(cell.Value) != "" {
			return strings.TrimSpace(cell.Value), nil
		}
		return extractTextValue(cell.InlineString.Text, cell.InlineString.Runs), nil
	}
}

func extractTextValue(text string, runs []xlsxTextRun) string {
	if text != "" {
		return text
	}
	if len(runs) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, run := range runs {
		builder.WriteString(run.Text)
	}
	return builder.String()
}

func buildInlineWorkbook(rows [][]string) ([]byte, error) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)

	files := []struct {
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
			body: buildInlineWorksheetXML(rows),
		},
	}

	for _, item := range files {
		writer, err := archive.Create(item.name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(writer, item.body); err != nil {
			return nil, err
		}
	}

	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildInlineWorksheetXML(rows [][]string) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	builder.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for rowIndex, row := range rows {
		builder.WriteString(fmt.Sprintf(`<row r="%d">`, rowIndex+1))
		for columnIndex, value := range row {
			builder.WriteString(fmt.Sprintf(`<c r="%s%d" t="inlineStr"><is><t>%s</t></is></c>`, spreadsheetColumnName(columnIndex), rowIndex+1, escapeXMLText(value)))
		}
		builder.WriteString(`</row>`)
	}
	builder.WriteString(`</sheetData></worksheet>`)
	return builder.String()
}

func spreadsheetColumnName(index int) string {
	value := index + 1
	var result string
	for value > 0 {
		value--
		result = string(rune('A'+(value%26))) + result
		value /= 26
	}
	return result
}

func escapeXMLText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func (app *App) findStudentByID(studentID int64) (*studentResult, error) {
	row := app.db.QueryRow(`select id, class_id, student_no, display_name, activated_at from students where id = ?`, studentID)
	var student studentResult
	if err := row.Scan(&student.ID, &student.ClassID, &student.StudentNo, &student.DisplayName, &student.ActivatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "学生不存在"}
		}
		return nil, err
	}
	return &student, nil
}

func (app *App) findClassByJoinCode(joinCode string) (*classRecord, error) {
	row := app.db.QueryRow(`select id, name, join_code, registration_enabled, registration_expires_at, exam_lock_enabled from classes where join_code = ?`, strings.TrimSpace(joinCode))
	var class classRecord
	if err := row.Scan(&class.ID, &class.Name, &class.JoinCode, &class.RegistrationEnabled, &class.RegistrationExpiresAt, &class.ExamLockEnabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &class, nil
}

func (app *App) findStudentByClassAndNo(classID int64, studentNo string) (*studentRecord, error) {
	row := app.db.QueryRow(`
select s.id, s.class_id, s.student_no, s.display_name, s.password_hash, s.activated_at, c.name, s.must_change_password
from students s
join classes c on c.id = s.class_id
where s.class_id = ? and s.student_no = ?`,
		classID,
		strings.TrimSpace(studentNo),
	)
	var student studentRecord
	if err := row.Scan(
		&student.ID,
		&student.ClassID,
		&student.StudentNo,
		&student.DisplayName,
		&student.PasswordHash,
		&student.ActivatedAt,
		&student.ClassName,
		&student.MustChangePassword,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &student, nil
}

func (app *App) findStudentsByNo(studentNo string) ([]studentRecord, error) {
	rows, err := app.db.Query(`
select s.id, s.class_id, s.student_no, s.display_name, s.password_hash, s.activated_at, c.name, s.must_change_password
from students s
join classes c on c.id = s.class_id
where s.student_no = ?
order by s.id asc`,
		strings.TrimSpace(studentNo),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	students := make([]studentRecord, 0)
	for rows.Next() {
		var student studentRecord
		if err := rows.Scan(
			&student.ID,
			&student.ClassID,
			&student.StudentNo,
			&student.DisplayName,
			&student.PasswordHash,
			&student.ActivatedAt,
			&student.ClassName,
			&student.MustChangePassword,
		); err != nil {
			return nil, err
		}
		students = append(students, student)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return students, nil
}

func (app *App) ensureStudentNoAvailable(studentNo string, excludedStudentID int64) error {
	var count int
	if err := app.db.QueryRow(`select count(*) from students where student_no = ? and id <> ?`, strings.TrimSpace(studentNo), excludedStudentID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return domainError{Status: http.StatusConflict, Code: "conflict", Message: "学号已存在"}
	}
	return nil
}

func (app *App) findAssignmentSubmissionByAssignmentAndStudent(assignmentID, studentID int64) (*assignmentSubmissionRecord, error) {
	row := app.db.QueryRow(`
select id, assignment_id, student_id, status, submitted_at, updated_at, review_status, teacher_comment, reviewed_at, reviewer_name
from assignment_submissions
where assignment_id = ? and student_id = ?`,
		assignmentID,
		studentID,
	)
	var submission assignmentSubmissionRecord
	if err := row.Scan(
		&submission.ID,
		&submission.AssignmentID,
		&submission.StudentID,
		&submission.Status,
		&submission.SubmittedAt,
		&submission.UpdatedAt,
		&submission.ReviewStatus,
		&submission.TeacherComment,
		&submission.ReviewedAt,
		&submission.ReviewerName,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if submission.ReviewStatus == "" {
		submission.ReviewStatus = assignmentSubmissionReviewPending
	}
	return &submission, nil
}

func (app *App) listSubmissionFiles(assignmentID, submissionID, classID int64) ([]fileSummary, error) {
	entries, err := app.listFileEntries("submission", optionalInt64(classID), assignmentSubmissionParentPath(submissionID))
	if err != nil {
		return nil, err
	}
	items := make([]fileSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind != "file" && entry.Kind != "dir" {
			continue
		}
		items = append(items, app.buildStudentSubmissionFileSummary(assignmentID, entry))
	}
	return items, nil
}

func (app *App) countSubmissionFiles(submissionID, classID int64) (int, error) {
	parentPath := assignmentSubmissionParentPath(submissionID)
	var count int
	if err := app.db.QueryRow(`
select count(*)
from file_entries
where space = 'submission'
  and class_id = ?
  and kind = 'file'
  and item_path like ?`,
		classID,
		parentPath+"/%",
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (app *App) removeSubmissionDirectory(submissionID, classID int64) error {
	baseDir, _, err := app.storageDir("submission", optionalInt64(classID))
	if err != nil {
		return err
	}
	parentPath := assignmentSubmissionParentPath(submissionID)
	submissionDir := filepath.Join(baseDir, filepath.FromSlash(strings.TrimPrefix(parentPath, "/")))
	return removeFileTree(submissionDir)
}

func (app *App) deleteSubmissionFiles(submissionID, classID int64) error {
	parentPath := assignmentSubmissionParentPath(submissionID)
	if err := app.removeSubmissionDirectory(submissionID, classID); err != nil {
		return err
	}
	_, err := app.db.Exec(`
delete from file_entries
where space = 'submission' and class_id = ? and (parent_path = ? or item_path like ?)`,
		classID,
		parentPath,
		parentPath+"/%",
	)
	return err
}

func (app *App) ensureClassExists(classID int64) error {
	var classCount int
	if err := app.db.QueryRow(`select count(*) from classes where id = ?`, classID).Scan(&classCount); err != nil {
		return err
	}
	if classCount == 0 {
		return domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
	}
	return nil
}

func (app *App) refreshClassJoinCode(classID int64) (*classJoinCodeResult, error) {
	if classID <= 0 {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "classId 非法"}
	}
	var classCount int
	if err := app.db.QueryRow(`select count(*) from classes where id = ?`, classID).Scan(&classCount); err != nil {
		return nil, err
	}
	if classCount == 0 {
		return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "班级不存在"}
	}

	for attempt := 0; attempt < 5; attempt++ {
		joinCode, err := generateJoinCode()
		if err != nil {
			return nil, err
		}
		expiresAt := time.Now().UTC().Add(classRegistrationDuration).Format(time.RFC3339)
		_, err = app.db.Exec(`update classes set join_code = ?, registration_enabled = 1, registration_expires_at = ? where id = ?`, joinCode, expiresAt, classID)
		if err != nil {
			if isUniqueConstraintError(err, "classes.join_code") {
				continue
			}
			return nil, err
		}
		return &classJoinCodeResult{
			ClassID:               classID,
			JoinCode:              joinCode,
			JoinCodeHint:          buildJoinCodeHint(joinCode),
			JoinCodeStatus:        "active",
			RegistrationEnabled:   true,
			RegistrationExpiresAt: expiresAt,
		}, nil
	}
	return nil, errors.New("generate class join code failed after retries")
}

func buildClassSummary(classID int64, name string, joinCode string, registrationEnabled bool, registrationExpiresAt string, examLockEnabled bool) classSummary {
	cleanJoinCode := strings.TrimSpace(joinCode)
	cleanRegistrationExpiresAt := strings.TrimSpace(registrationExpiresAt)
	active := cleanJoinCode != "" && registrationEnabled && !isClassRegistrationExpired(cleanRegistrationExpiresAt, time.Now().UTC())
	status := "inactive"
	if active {
		status = "active"
	} else {
		cleanJoinCode = ""
		cleanRegistrationExpiresAt = ""
	}
	return classSummary{
		ID:                    classID,
		Name:                  name,
		JoinCode:              cleanJoinCode,
		JoinCodeStatus:        status,
		JoinCodeHint:          buildJoinCodeHint(cleanJoinCode),
		ExamLockEnabled:       examLockEnabled,
		RegistrationEnabled:   active,
		RegistrationExpiresAt: cleanRegistrationExpiresAt,
	}
}

func (app *App) isClassExamLocked(classID int64) (bool, error) {
	var enabled bool
	if err := app.db.QueryRow(`select exam_lock_enabled from classes where id = ?`, classID).Scan(&enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return enabled, nil
}

func isClassRegistrationExpired(registrationExpiresAt string, now time.Time) bool {
	cleanRegistrationExpiresAt := strings.TrimSpace(registrationExpiresAt)
	if cleanRegistrationExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, cleanRegistrationExpiresAt)
	if err != nil {
		return false
	}
	return !expiresAt.After(now.UTC())
}

func (app *App) closeClassRegistration(classID int64) error {
	_, err := app.db.Exec(`update classes set join_code = '', registration_enabled = 0, registration_expires_at = '' where id = ?`, classID)
	return err
}

func (app *App) closeExpiredClassRegistrations(now time.Time) error {
	_, err := app.db.Exec(
		`update classes set join_code = '', registration_enabled = 0, registration_expires_at = '' where registration_enabled = 1 and registration_expires_at <> '' and registration_expires_at <= ?`,
		now.UTC().Format(time.RFC3339),
	)
	return err
}

func buildJoinCodeHint(joinCode string) string {
	return strings.TrimSpace(joinCode)
}

func isUniqueConstraintError(err error, key string) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed: "+key)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (app *App) findTeacherByUsername(username string) (*teacherRecord, error) {
	row := app.db.QueryRow(`select id, username, display_name, password_hash, role, disabled from teachers where username = ?`, strings.TrimSpace(username))
	var teacher teacherRecord
	if err := row.Scan(&teacher.ID, &teacher.Username, &teacher.DisplayName, &teacher.PasswordHash, &teacher.Role, &teacher.Disabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &teacher, nil
}

func (app *App) findTeacherByID(teacherID int64) (*teacherRecord, error) {
	row := app.db.QueryRow(`select id, username, display_name, password_hash, role, disabled from teachers where id = ?`, teacherID)
	var teacher teacherRecord
	if err := row.Scan(&teacher.ID, &teacher.Username, &teacher.DisplayName, &teacher.PasswordHash, &teacher.Role, &teacher.Disabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &teacher, nil
}

func (app *App) countActiveOwnersExcluding(excludedTeacherID int64) (int, error) {
	row := app.db.QueryRow(`select count(*) from teachers where id <> ? and role = ? and disabled = 0`, excludedTeacherID, teacherRoleOwner)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (app *App) createSession(token string, teacherID int64) error {
	_, err := app.db.Exec(`insert into sessions (token, teacher_id, expires_at) values (?, ?, ?)`, token, teacherID, time.Now().Add(sessionDuration).UTC().Format(time.RFC3339))
	return err
}

func (app *App) createStudentSession(token string, studentID int64) error {
	_, err := app.db.Exec(`insert into student_sessions (token, student_id, expires_at) values (?, ?, ?)`, token, studentID, time.Now().Add(sessionDuration).UTC().Format(time.RFC3339))
	return err
}

func (app *App) deleteSessionsForTeacher(teacherID int64) (int64, error) {
	result, err := app.db.Exec(`delete from sessions where teacher_id = ?`, teacherID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (app *App) deleteSessionsForStudent(studentID int64) (int64, error) {
	result, err := app.db.Exec(`delete from student_sessions where student_id = ?`, studentID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (app *App) deleteSession(token string) error {
	_, err := app.db.Exec(`delete from sessions where token = ?`, token)
	return err
}

func (app *App) deleteStudentSession(token string) error {
	_, err := app.db.Exec(`delete from student_sessions where token = ?`, token)
	return err
}

func (app *App) findTeacherBySession(token string) (*teacherRecord, error) {
	row := app.db.QueryRow(`
select t.id, t.username, t.display_name, t.password_hash, t.role, t.disabled
from sessions s
join teachers t on t.id = s.teacher_id
where s.token = ? and s.expires_at > ?`,
		token,
		time.Now().UTC().Format(time.RFC3339),
	)
	var teacher teacherRecord
	if err := row.Scan(&teacher.ID, &teacher.Username, &teacher.DisplayName, &teacher.PasswordHash, &teacher.Role, &teacher.Disabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("session not found")
		}
		return nil, err
	}
	return &teacher, nil
}

func (app *App) findStudentBySession(token string) (*studentRecord, error) {
	row := app.db.QueryRow(`
select s.id, s.class_id, s.student_no, s.display_name, s.password_hash, s.activated_at, c.name, s.must_change_password
from student_sessions ss
join students s on s.id = ss.student_id
join classes c on c.id = s.class_id
where ss.token = ? and ss.expires_at > ?`,
		token,
		time.Now().UTC().Format(time.RFC3339),
	)
	var student studentRecord
	if err := row.Scan(
		&student.ID,
		&student.ClassID,
		&student.StudentNo,
		&student.DisplayName,
		&student.PasswordHash,
		&student.ActivatedAt,
		&student.ClassName,
		&student.MustChangePassword,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("session not found")
		}
		return nil, err
	}
	return &student, nil
}

func (app *App) recordLoginLog(request *http.Request, actorType string, actorID int64, username string, actorName string, status string, message string) {
	if _, err := app.db.Exec(`
insert into login_logs (occurred_at, actor_type, actor_id, actor_name, username, status, ip_address, user_agent, message)
values (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339),
		actorType,
		nullablePositiveInt64(actorID),
		actorName,
		username,
		status,
		clientIPAddress(request),
		request.UserAgent(),
		message,
	); err != nil {
		log.Printf("record login log: %v", err)
	}
}

func (app *App) recordOperationLog(request *http.Request, actorType string, actorID int64, actorName string, statusCode int) {
	requestPath := request.URL.Path
	if !shouldRecordOperationLog(request.Method, requestPath) {
		return
	}
	if _, err := app.db.Exec(`
insert into operation_logs (occurred_at, actor_type, actor_id, actor_name, method, path, status_code, ip_address, user_agent, summary)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339),
		actorType,
		nullablePositiveInt64(actorID),
		actorName,
		request.Method,
		requestPath,
		statusCode,
		clientIPAddress(request),
		request.UserAgent(),
		app.operationLogSummaryForRequest(request, actorType, actorID, statusCode),
	); err != nil {
		log.Printf("record operation log: %v", err)
	}
}

func (app *App) operationLogSummaryForRequest(request *http.Request, actorType string, actorID int64, statusCode int) string {
	fallback := operationLogSummary(request.Method, request.URL.Path)
	if statusCode < http.StatusOK || statusCode >= http.StatusBadRequest {
		return fallback
	}
	summary, ok := app.enrichedOperationLogSummary(request, actorType, actorID)
	if !ok {
		return fallback
	}
	return summary
}

func (app *App) enrichedOperationLogSummary(request *http.Request, actorType string, actorID int64) (string, bool) {
	segments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(segments) < 2 || segments[0] != "api" {
		return "", false
	}
	switch segments[1] {
	case "files":
		return app.materialOperationLogSummary(request.Method, segments, actorType, actorID)
	case "student":
		return app.studentOperationLogSummaryForRequest(request, segments, actorID)
	case "assignments":
		return app.assignmentOperationLogSummaryForRequest(request, segments)
	case "library-shares":
		return app.libraryShareOperationLogSummary(request.Method, segments, actorID)
	case "share":
		return app.shareAccessOperationLogSummary(request.Method, segments)
	default:
		return "", false
	}
}

func (app *App) materialOperationLogSummary(method string, segments []string, actorType string, actorID int64) (string, bool) {
	if method != http.MethodGet || len(segments) != 4 {
		return "", false
	}
	action := segments[3]
	if action != "download" && action != "archive" {
		return "", false
	}
	entryID, ok := positivePathID(segments, 2)
	if !ok {
		return "", false
	}
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		return "", false
	}
	if actorType == "student" {
		student, err := app.findStudentRecordByID(actorID)
		if err != nil {
			return "", false
		}
		if _, err := app.getStudentVisibleFileEntry(entryID, *student); err != nil {
			return "", false
		}
		prefix := "学生下载资料"
		if action == "archive" {
			prefix = "学生下载资料压缩包"
		}
		return prefix + "：" + materialScopeLabel(entry) + "/" + auditEntryName(entry), true
	}
	prefix := "老师下载资料"
	if action == "archive" {
		prefix = "老师下载资料压缩包"
	}
	return prefix + "：" + auditEntryName(entry), true
}

func (app *App) studentOperationLogSummaryForRequest(request *http.Request, segments []string, actorID int64) (string, bool) {
	if len(segments) >= 5 && segments[2] == "assignments" && segments[4] == "submission" {
		return app.studentAssignmentSubmissionOperationLogSummary(request.Method, segments, actorID)
	}
	if len(segments) == 7 && segments[2] == "assignments" && segments[4] == "attachments" && segments[6] == "download" {
		return app.studentAssignmentAttachmentOperationLogSummary(request.Method, segments, actorID)
	}
	if len(segments) == 5 && segments[2] == "files" {
		studentFileSegments := []string{"api", "files", segments[3], segments[4]}
		return app.materialOperationLogSummary(request.Method, studentFileSegments, "student", actorID)
	}
	return "", false
}

func (app *App) studentAssignmentAttachmentOperationLogSummary(method string, segments []string, actorID int64) (string, bool) {
	if method != http.MethodGet {
		return "", false
	}
	assignmentID, ok := positivePathID(segments, 3)
	if !ok {
		return "", false
	}
	fileID, ok := positivePathID(segments, 5)
	if !ok {
		return "", false
	}
	student, err := app.findStudentRecordByID(actorID)
	if err != nil {
		return "", false
	}
	assignment, err := app.findVisibleStudentAssignmentByID(assignmentID, student.ClassID)
	if err != nil {
		return "", false
	}
	entry, err := app.findStudentAssignmentAttachmentByID(assignmentID, *student, fileID)
	if err != nil {
		return "", false
	}
	return "学生下载作业附件：" + auditAssignmentLabel(assignment.Title) + "/" + auditEntryName(entry), true
}

func (app *App) studentAssignmentSubmissionOperationLogSummary(method string, segments []string, actorID int64) (string, bool) {
	assignmentID, ok := positivePathID(segments, 3)
	if !ok {
		return "", false
	}
	student, err := app.findStudentRecordByID(actorID)
	if err != nil {
		return "", false
	}
	assignment, err := app.findVisibleStudentAssignmentByID(assignmentID, student.ClassID)
	if err != nil {
		return "", false
	}
	assignmentLabel := auditAssignmentLabel(assignment.Title)
	if len(segments) == 5 && segments[4] == "submission" && method == http.MethodPost {
		return "学生提交作业：" + assignmentLabel, true
	}
	if len(segments) == 8 && segments[4] == "submission" && segments[5] == "files" && segments[7] == "download" && method == http.MethodGet {
		fileID, ok := positivePathID(segments, 6)
		if !ok {
			return "", false
		}
		entry, err := app.findStudentSubmissionFileByID(assignmentID, *student, fileID)
		if err != nil {
			return "", false
		}
		return "学生下载本人提交文件：" + assignmentLabel + "/" + auditEntryName(entry), true
	}
	if len(segments) == 7 && segments[4] == "submission" && segments[5] == "files" && method == http.MethodDelete {
		fileID, ok := positivePathID(segments, 6)
		if !ok {
			return "", false
		}
		entry, err := app.findStudentSubmissionFileByID(assignmentID, *student, fileID)
		if err != nil {
			return "", false
		}
		return "学生删除提交文件：" + assignmentLabel + "/" + auditEntryName(entry), true
	}
	return "", false
}

func (app *App) assignmentOperationLogSummaryForRequest(request *http.Request, segments []string) (string, bool) {
	if len(segments) == 4 && segments[2] == "submissions" && segments[3] == "archive" && request.Method == http.MethodGet {
		return app.assignmentSubmissionsArchiveOperationLogSummary(request.URL.Query())
	}
	assignmentID, ok := positivePathID(segments, 2)
	if !ok {
		return "", false
	}
	classID, err := parseOptionalInt64(request.URL.Query().Get("classId"))
	if err != nil || classID == nil {
		return "", false
	}
	assignment, err := app.findAssignmentByID(assignmentID, *classID)
	if err != nil {
		return "", false
	}
	assignmentLabel := auditAssignmentLabel(assignment.Title)
	if len(segments) == 6 && segments[3] == "attachments" && segments[5] == "download" && request.Method == http.MethodGet {
		fileID, ok := positivePathID(segments, 4)
		if !ok {
			return "", false
		}
		entry, err := app.findAssignmentAttachmentByID(assignmentID, *classID, fileID)
		if err != nil {
			return "", false
		}
		return "老师下载作业附件：" + assignmentLabel + "/" + auditEntryName(entry), true
	}
	if len(segments) == 7 && segments[3] == "submissions" && segments[4] == "files" && segments[6] == "download" && request.Method == http.MethodGet {
		fileID, ok := positivePathID(segments, 5)
		if !ok {
			return "", false
		}
		entry, err := app.findAssignmentSubmissionFileByID(assignmentID, *classID, fileID)
		if err != nil {
			return "", false
		}
		submissionID, ok := submissionIDFromEntryPath(entry)
		if !ok {
			return "", false
		}
		studentName, ok := app.assignmentSubmissionStudentName(assignmentID, *classID, submissionID)
		if !ok {
			return "", false
		}
		return "老师下载学生提交文件：" + assignmentLabel + "/" + studentName + "/" + auditEntryName(entry), true
	}
	return "", false
}

func (app *App) libraryShareOperationLogSummary(method string, segments []string, actorID int64) (string, bool) {
	// POST /api/library-shares - 创建分享
	if method == http.MethodPost && len(segments) == 2 {
		return "老师创建资料分享", true
	}
	// GET /api/library-shares - 查看分享列表（不记录）
	if method == http.MethodGet && len(segments) == 2 {
		return "", false
	}
	// PATCH /api/library-shares/:id - 修改分享
	if method == http.MethodPatch && len(segments) == 3 {
		shareID, ok := positivePathID(segments, 2)
		if !ok {
			return "", false
		}
		share, err := app.findLibraryShareByID(shareID, actorID)
		if err != nil {
			return "", false
		}
		return "老师修改资料分享：" + share.entryName, true
	}
	// DELETE /api/library-shares/:id - 删除分享
	if method == http.MethodDelete && len(segments) == 3 {
		shareID, ok := positivePathID(segments, 2)
		if !ok {
			return "", false
		}
		share, err := app.findLibraryShareByID(shareID, actorID)
		if err != nil {
			return "", false
		}
		return "老师删除资料分享：" + share.entryName, true
	}
	// POST /api/library-shares/:id/reset-code - 重置安全码
	if method == http.MethodPost && len(segments) == 4 && segments[3] == "reset-code" {
		shareID, ok := positivePathID(segments, 2)
		if !ok {
			return "", false
		}
		share, err := app.findLibraryShareByID(shareID, actorID)
		if err != nil {
			return "", false
		}
		return "老师重置分享安全码：" + share.entryName, true
	}
	return "", false
}

func (app *App) shareAccessOperationLogSummary(method string, segments []string) (string, bool) {
	// POST /api/share/:token/verify - 验证安全码（不记录）
	if method == http.MethodPost && len(segments) == 4 && segments[3] == "verify" {
		return "", false
	}
	// GET /api/share/:token - 查看分享根信息（不记录）
	if method == http.MethodGet && len(segments) == 3 {
		return "", false
	}
	// GET /api/share/:token/items - 浏览目录（记录）
	if method == http.MethodGet && len(segments) == 4 && segments[3] == "items" {
		token := segments[2]
		share, err := app.findShareByToken(token)
		if err != nil {
			return "", false
		}
		return "访客查看分享：" + share.entryName, true
	}
	// GET /api/share/:token/files/:id/preview - 预览文件
	if method == http.MethodGet && len(segments) == 6 && segments[3] == "files" && segments[5] == "preview" {
		token := segments[2]
		share, err := app.findShareByToken(token)
		if err != nil {
			return "", false
		}
		return "访客预览分享文件：" + share.entryName, true
	}
	// GET /api/share/:token/files/:id/download - 下载文件
	if method == http.MethodGet && len(segments) == 6 && segments[3] == "files" && segments[5] == "download" {
		token := segments[2]
		share, err := app.findShareByToken(token)
		if err != nil {
			return "", false
		}
		return "访客下载分享文件：" + share.entryName, true
	}
	// GET /api/share/:token/files/:id/archive - 下载文件夹压缩包
	if method == http.MethodGet && len(segments) == 6 && segments[3] == "files" && segments[5] == "archive" {
		token := segments[2]
		share, err := app.findShareByToken(token)
		if err != nil {
			return "", false
		}
		return "访客下载分享文件夹：" + share.entryName, true
	}
	return "", false
}

func (app *App) assignmentSubmissionsArchiveOperationLogSummary(query url.Values) (string, bool) {
	classID, err := parseOptionalInt64(query.Get("classId"))
	if err != nil || classID == nil {
		return "", false
	}
	assignmentIDs, err := parseAssignmentArchiveIDs(query.Get("assignmentIds"))
	if err != nil {
		return "", false
	}
	if len(assignmentIDs) == 1 {
		assignment, err := app.findAssignmentByID(assignmentIDs[0], *classID)
		if err != nil {
			return "", false
		}
		return "老师下载作业提交包：" + auditAssignmentLabel(assignment.Title), true
	}
	if len(assignmentIDs) > 1 {
		return fmt.Sprintf("老师下载作业提交包：指定 %d 个作业", len(assignmentIDs)), true
	}
	return "老师下载作业提交包：全部作业", true
}

func (app *App) assignmentSubmissionStudentName(assignmentID, classID, submissionID int64) (string, bool) {
	var displayName string
	err := app.db.QueryRow(`
select st.display_name
from assignment_submissions s
join students st on st.id = s.student_id
where s.id = ? and s.assignment_id = ? and st.class_id = ?`,
		submissionID,
		assignmentID,
		classID,
	).Scan(&displayName)
	if err != nil {
		return "", false
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "未命名学生", true
	}
	return displayName, true
}

func (app *App) findStudentRecordByID(studentID int64) (*studentRecord, error) {
	row := app.db.QueryRow(`
select s.id, s.class_id, s.student_no, s.display_name, s.password_hash, s.activated_at, c.name, s.must_change_password
from students s
join classes c on c.id = s.class_id
where s.id = ?`, studentID)
	var student studentRecord
	if err := row.Scan(
		&student.ID,
		&student.ClassID,
		&student.StudentNo,
		&student.DisplayName,
		&student.PasswordHash,
		&student.ActivatedAt,
		&student.ClassName,
		&student.MustChangePassword,
	); err != nil {
		return nil, err
	}
	return &student, nil
}

func positivePathID(segments []string, index int) (int64, bool) {
	if index < 0 || index >= len(segments) {
		return 0, false
	}
	id, err := strconv.ParseInt(segments[index], 10, 64)
	return id, err == nil && id > 0
}

func auditAssignmentLabel(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "作业《未命名》"
	}
	return "作业《" + trimmed + "》"
}

func auditEntryName(entry *fileEntry) string {
	if entry == nil {
		return "文件"
	}
	trimmed := strings.TrimSpace(entry.Name)
	if trimmed == "" {
		return "文件"
	}
	return trimmed
}

func materialScopeLabel(entry *fileEntry) string {
	if entry == nil {
		return "资料"
	}
	switch entry.Space {
	case "public":
		return "公共资料"
	case "class":
		return "班级资料"
	default:
		return "资料"
	}
}

func shouldRecordOperationLog(method, requestPath string) bool {
	switch method {
	case http.MethodHead, http.MethodOptions:
		return false
	case http.MethodGet:
		return isDownloadOperationPath(requestPath)
	default:
		return true
	}
}

func isDownloadOperationPath(requestPath string) bool {
	return strings.HasSuffix(requestPath, "/download") || strings.HasSuffix(requestPath, "/archive")
}

func operationLogSummary(method, requestPath string) string {
	if method == http.MethodGet {
		return downloadOperationSummary(requestPath)
	}
	segments := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(segments) < 2 || segments[0] != "api" {
		return method + " " + requestPath
	}
	switch segments[1] {
	case "auth":
		if len(segments) >= 3 && segments[2] == "logout" && method == http.MethodPost {
			return "老师退出登录"
		}
		return methodOperationSummary(method, "老师登录操作", "更新老师认证", "删除老师认证")
	case "student":
		return studentOperationSummary(method, segments)
	case "classes":
		if len(segments) >= 4 && segments[3] == "exam-lock" && method == http.MethodPatch {
			return "切换班级考试锁定"
		}
		if len(segments) >= 4 && segments[3] == "join-code" {
			return methodOperationSummary(method, "刷新班级注册码", "切换班级注册状态", "关闭班级注册")
		}
		return methodOperationSummary(method, "创建班级", "修改班级", "删除班级")
	case "students":
		return methodOperationSummary(method, "新增学生", "修改学生", "删除学生")
	case "teachers":
		return methodOperationSummary(method, "新增老师", "修改老师", "删除老师")
	case "assignments":
		if len(segments) >= 4 && segments[3] == "submissions" {
			if method == http.MethodPatch {
				return "批改作业提交"
			}
			return methodOperationSummary(method, "查看作业提交", "修改作业提交", "删除作业提交")
		}
		if len(segments) >= 4 && segments[3] == "attachments" {
			return methodOperationSummary(method, "上传作业附件", "修改作业附件", "删除作业附件")
		}
		return methodOperationSummary(method, "创建作业", "修改作业", "删除作业")
	case "files":
		return fileOperationSummary(method, segments)
	case "settings":
		return "修改系统设置"
	case "audit":
		if len(segments) >= 3 && segments[2] == "logs" && method == http.MethodDelete {
			return "清理日志"
		}
		return methodOperationSummary(method, "新增日志设置", "修改日志设置", "删除日志设置")
	case "preferences":
		return "修改偏好设置"
	default:
		return methodOperationSummary(method, "执行操作", "更新操作", "删除操作")
	}
}

func studentOperationSummary(method string, segments []string) string {
	if len(segments) >= 3 && segments[2] == "auth" {
		if len(segments) >= 4 && segments[3] == "logout" && method == http.MethodPost {
			return "学生退出登录"
		}
		return methodOperationSummary(method, "学生登录操作", "更新学生认证", "删除学生认证")
	}
	if len(segments) >= 5 && segments[2] == "assignments" && segments[4] == "submission" {
		return methodOperationSummary(method, "提交作业", "更新作业提交", "撤回作业提交")
	}
	if len(segments) >= 4 && segments[2] == "files" {
		return fileOperationSummary(method, segments[1:])
	}
	return methodOperationSummary(method, "学生端操作", "更新学生端操作", "删除学生端操作")
}

func methodOperationSummary(method, createText, updateText, deleteText string) string {
	switch method {
	case http.MethodPost:
		return createText
	case http.MethodPut, http.MethodPatch:
		return updateText
	case http.MethodDelete:
		return deleteText
	default:
		return method + " 操作"
	}
}

func fileOperationSummary(method string, segments []string) string {
	if len(segments) >= 3 {
		switch segments[2] {
		case "upload":
			return "上传文件"
		case "folder":
			return "新建文件夹"
		case "copy":
			return "复制文件"
		case "move":
			return "移动文件"
		}
	}
	return methodOperationSummary(method, "新增文件", "修改文件", "删除文件")
}

func downloadOperationSummary(requestPath string) string {
	segments := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(segments) >= 4 && segments[len(segments)-1] == "archive" {
		if len(segments) >= 3 && segments[1] == "assignments" && segments[2] == "submissions" {
			return "下载作业提交包"
		}
		return "下载压缩包"
	}
	if len(segments) >= 4 && segments[len(segments)-1] == "download" {
		if len(segments) >= 5 && segments[1] == "assignments" && segments[3] == "attachments" {
			return "下载作业附件"
		}
		if len(segments) >= 6 && segments[1] == "assignments" && segments[3] == "submissions" {
			return "下载学生提交文件"
		}
		if len(segments) >= 6 && segments[1] == "student" && segments[2] == "assignments" && segments[4] == "attachments" {
			return "下载作业附件"
		}
		if len(segments) >= 7 && segments[1] == "student" && segments[2] == "assignments" && segments[4] == "submission" {
			return "下载提交文件"
		}
		return "下载文件"
	}
	return "下载文件"
}

func (app *App) listLoginLogs(filters loginLogFilters) ([]loginLogPayload, error) {
	logs, _, err := app.listLoginLogsPage(filters)
	return logs, err
}

func (app *App) listLoginLogsPage(filters loginLogFilters) ([]loginLogPayload, paginationPayload, error) {
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PageSize <= 0 {
		filters.PageSize = filters.Limit
		if filters.PageSize <= 0 {
			filters.PageSize = 500
		}
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}
	where, args := buildLoginLogFilterWhere(filters)
	var total int
	if err := app.db.QueryRow(`select count(*) from login_logs where `+where, args...).Scan(&total); err != nil {
		return nil, paginationPayload{}, err
	}
	args = append(args, filters.PageSize, filters.Offset)
	rows, err := app.db.Query(`
select id, occurred_at, actor_type, actor_name, username, status, ip_address, message
from login_logs
where `+where+`
order by id desc
limit ? offset ?`, args...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()

	logs := make([]loginLogPayload, 0)
	for rows.Next() {
		var item loginLogPayload
		if err := rows.Scan(&item.ID, &item.OccurredAt, &item.ActorType, &item.ActorName, &item.Username, &item.Status, &item.IPAddress, &item.Message); err != nil {
			return nil, paginationPayload{}, err
		}
		logs = append(logs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return logs, buildAuditLogPagination(total, filters.Page, filters.PageSize), nil
}

func (app *App) listOperationLogs(filters operationLogFilters) ([]operationLogPayload, error) {
	logs, _, err := app.listOperationLogsPage(filters)
	return logs, err
}

func (app *App) listOperationLogsPage(filters operationLogFilters) ([]operationLogPayload, paginationPayload, error) {
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PageSize <= 0 {
		filters.PageSize = filters.Limit
		if filters.PageSize <= 0 {
			filters.PageSize = 500
		}
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}
	where, args := buildOperationLogFilterWhere(filters)
	var total int
	if err := app.db.QueryRow(`select count(*) from operation_logs where `+where, args...).Scan(&total); err != nil {
		return nil, paginationPayload{}, err
	}
	args = append(args, filters.PageSize, filters.Offset)
	rows, err := app.db.Query(`
select id, occurred_at, actor_type, actor_name, method, path, status_code, ip_address, summary
from operation_logs
where `+where+`
order by id desc
limit ? offset ?`, args...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()

	logs := make([]operationLogPayload, 0)
	for rows.Next() {
		var item operationLogPayload
		if err := rows.Scan(&item.ID, &item.OccurredAt, &item.ActorType, &item.ActorName, &item.Method, &item.Path, &item.StatusCode, &item.IPAddress, &item.Summary); err != nil {
			return nil, paginationPayload{}, err
		}
		item.Summary = normalizeOperationLogSummary(item.Method, item.Path, item.Summary)
		logs = append(logs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return logs, buildAuditLogPagination(total, filters.Page, filters.PageSize), nil
}

func (app *App) listAuditLogsPage(filters auditLogFilters) ([]auditLogPayload, paginationPayload, error) {
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PageSize <= 0 {
		filters.PageSize = 8
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}
	loginWhere, loginArgs := buildLoginLogFilterWhere(loginLogFilters{
		ActorType: filters.ActorType,
		Status:    auditLoginStatusFilter(filters),
		Query:     filters.Query,
		IP:        filters.IP,
		From:      filters.From,
		To:        filters.To,
	})
	operationWhere, operationArgs := buildOperationLogFilterWhere(operationLogFilters{
		ActorType: filters.ActorType,
		Method:    filters.Method,
		Query:     filters.Query,
		IP:        filters.IP,
		From:      filters.From,
		To:        filters.To,
	})
	if filters.Result == "success" {
		operationWhere += " and status_code >= 200 and status_code < 300"
	} else if filters.Result == "failure" {
		operationWhere += " and status_code >= 400"
	}

	total := 0
	queryParts := make([]string, 0, 2)
	args := make([]any, 0, len(loginArgs)+len(operationArgs)+2)
	if filters.LogType == "" || filters.LogType == "login" {
		loginCount, err := app.countAuditLogRows("login_logs", loginWhere, loginArgs)
		if err != nil {
			return nil, paginationPayload{}, err
		}
		total += loginCount
		queryParts = append(queryParts, `
select id, 'login' as log_type, occurred_at, actor_type, case when trim(actor_name) <> '' then actor_name else username end as account, actor_name, message as action, status as result, ip_address, '' as method, '' as path
from login_logs
where `+loginWhere)
		args = append(args, loginArgs...)
	}
	if filters.LogType == "" || filters.LogType == "operation" {
		operationCount, err := app.countAuditLogRows("operation_logs", operationWhere, operationArgs)
		if err != nil {
			return nil, paginationPayload{}, err
		}
		total += operationCount
		queryParts = append(queryParts, `
select id, 'operation' as log_type, occurred_at, actor_type, actor_name as account, actor_name, summary as action, case when status_code >= 200 and status_code < 300 then 'success' else 'failure' end as result, ip_address, method, path
from operation_logs
where `+operationWhere)
		args = append(args, operationArgs...)
	}
	if len(queryParts) == 0 {
		return []auditLogPayload{}, buildAuditLogPagination(0, filters.Page, filters.PageSize), nil
	}
	args = append(args, filters.PageSize, filters.Offset)
	rows, err := app.db.Query(`
select id, log_type, occurred_at, actor_type, account, actor_name, action, result, ip_address, method, path
from (`+strings.Join(queryParts, "\nunion all\n")+`)
order by occurred_at desc, id desc
limit ? offset ?`, args...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()

	logs := make([]auditLogPayload, 0)
	for rows.Next() {
		var item auditLogPayload
		var method string
		var requestPath string
		if err := rows.Scan(&item.ID, &item.LogType, &item.OccurredAt, &item.ActorType, &item.Account, &item.ActorName, &item.Action, &item.Result, &item.IPAddress, &method, &requestPath); err != nil {
			return nil, paginationPayload{}, err
		}
		if item.LogType == "operation" {
			item.Action = normalizeOperationLogSummary(method, requestPath, item.Action)
		}
		item.Result = auditLogResultLabel(item.Result)
		logs = append(logs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return logs, buildAuditLogPagination(total, filters.Page, filters.PageSize), nil
}

func (app *App) countAuditLogRows(tableName string, where string, args []any) (int, error) {
	var total int
	if err := app.db.QueryRow(`select count(*) from `+tableName+` where `+where, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func auditLoginStatusFilter(filters auditLogFilters) string {
	if filters.Result == "success" || filters.Result == "failure" {
		return filters.Result
	}
	return ""
}

func auditLogResultLabel(result string) string {
	if result == "success" {
		return "成功"
	}
	if result == "failure" {
		return "失败"
	}
	return result
}

func normalizeOperationLogSummary(method, requestPath, summary string) string {
	trimmed := strings.TrimSpace(summary)
	pathSummary := strings.TrimSpace(method + " " + requestPath)
	if trimmed == "" || trimmed == pathSummary || strings.Contains(trimmed, " /api/") {
		return operationLogSummary(method, requestPath)
	}
	return trimmed
}

func (app *App) clearAuditLogsBefore(before string) (clearAuditLogsResult, error) {
	tx, err := app.db.Begin()
	if err != nil {
		return clearAuditLogsResult{}, err
	}
	defer tx.Rollback()

	loginResult, err := tx.Exec(`delete from login_logs where occurred_at < ?`, before)
	if err != nil {
		return clearAuditLogsResult{}, err
	}
	operationResult, err := tx.Exec(`delete from operation_logs where occurred_at < ?`, before)
	if err != nil {
		return clearAuditLogsResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return clearAuditLogsResult{}, err
	}
	deletedLoginLogs, err := loginResult.RowsAffected()
	if err != nil {
		return clearAuditLogsResult{}, err
	}
	deletedOperationLogs, err := operationResult.RowsAffected()
	if err != nil {
		return clearAuditLogsResult{}, err
	}
	return clearAuditLogsResult{
		DeletedLoginLogs:     deletedLoginLogs,
		DeletedOperationLogs: deletedOperationLogs,
	}, nil
}

func parseLoginLogFilters(values url.Values) loginLogFilters {
	page, pageSize, offset := parseAuditPagination(values)
	limit := parseAuditLimit(values.Get("limit"))
	if values.Get("pageSize") == "" && values.Get("page") == "" {
		pageSize = limit
		offset = 0
	}
	return loginLogFilters{
		ActorType: normalizeAuditEnum(values.Get("actorType"), []string{"teacher", "student"}),
		Status:    normalizeAuditEnum(values.Get("status"), []string{"success", "failure"}),
		Query:     strings.TrimSpace(values.Get("q")),
		IP:        strings.TrimSpace(values.Get("ip")),
		From:      normalizeAuditDateBound(values.Get("from"), false),
		To:        normalizeAuditDateBound(values.Get("to"), true),
		Limit:     limit,
		Page:      page,
		PageSize:  pageSize,
		Offset:    offset,
	}
}

func parseOperationLogFilters(values url.Values) operationLogFilters {
	page, pageSize, offset := parseAuditPagination(values)
	limit := parseAuditLimit(values.Get("limit"))
	if values.Get("pageSize") == "" && values.Get("page") == "" {
		pageSize = limit
		offset = 0
	}
	return operationLogFilters{
		ActorType: normalizeAuditEnum(values.Get("actorType"), []string{"teacher", "student"}),
		Method:    normalizeAuditEnum(strings.ToUpper(values.Get("method")), []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}),
		Query:     strings.TrimSpace(values.Get("q")),
		IP:        strings.TrimSpace(values.Get("ip")),
		From:      normalizeAuditDateBound(values.Get("from"), false),
		To:        normalizeAuditDateBound(values.Get("to"), true),
		Limit:     limit,
		Page:      page,
		PageSize:  pageSize,
		Offset:    offset,
	}
}

func parseAuditLogFilters(values url.Values) auditLogFilters {
	page, pageSize, offset := parseAuditPagination(values)
	return auditLogFilters{
		LogType:   normalizeAuditEnum(values.Get("logType"), []string{"login", "operation"}),
		ActorType: normalizeAuditEnum(values.Get("actorType"), []string{"teacher", "student"}),
		Result:    normalizeAuditEnum(values.Get("result"), []string{"success", "failure"}),
		Method:    normalizeAuditEnum(strings.ToUpper(values.Get("method")), []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}),
		Query:     strings.TrimSpace(values.Get("q")),
		IP:        strings.TrimSpace(values.Get("ip")),
		From:      normalizeAuditDateBound(values.Get("from"), false),
		To:        normalizeAuditDateBound(values.Get("to"), true),
		Page:      page,
		PageSize:  pageSize,
		Offset:    offset,
	}
}

func normalizeAuditClearBefore(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("请选择要清理的日期")
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return "", errors.New("清理日期格式必须为 YYYY-MM-DD")
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func normalizeAuditEnum(raw string, allowed []string) string {
	trimmed := strings.TrimSpace(raw)
	for _, item := range allowed {
		if trimmed == item {
			return trimmed
		}
	}
	return ""
}

func normalizeAuditDateBound(raw string, endOfDay bool) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		if endOfDay {
			parsed = parsed.Add(24*time.Hour - time.Second)
		}
		return parsed.UTC().Format(time.RFC3339)
	}
	return ""
}

func parseAuditLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 500
	}
	if limit > 2000 {
		return 2000
	}
	return limit
}

func parseAuditPagination(values url.Values) (int, int, int) {
	page := 1
	pageSize := 8
	if parsedPage, err := strconv.Atoi(strings.TrimSpace(values.Get("page"))); err == nil && parsedPage > 0 {
		page = parsedPage
	}
	if parsedPageSize, err := strconv.Atoi(strings.TrimSpace(values.Get("pageSize"))); err == nil && parsedPageSize > 0 {
		pageSize = parsedPageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, (page - 1) * pageSize
}

func buildAuditLogPagination(total int, page int, pageSize int) paginationPayload {
	if total < 0 {
		total = 0
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 8
	}
	totalPages := 1
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return paginationPayload{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}

func buildLoginLogFilterWhere(filters loginLogFilters) (string, []any) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 8)
	if filters.ActorType != "" {
		conditions = append(conditions, "actor_type = ?")
		args = append(args, filters.ActorType)
	}
	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	}
	if filters.From != "" {
		conditions = append(conditions, "occurred_at >= ?")
		args = append(args, filters.From)
	}
	if filters.To != "" {
		conditions = append(conditions, "occurred_at <= ?")
		args = append(args, filters.To)
	}
	if filters.IP != "" {
		conditions = append(conditions, "ip_address like ?")
		args = append(args, "%"+filters.IP+"%")
	}
	if filters.Query != "" {
		like := "%" + filters.Query + "%"
		conditions = append(conditions, "(username like ? or actor_name like ? or ip_address like ? or message like ?)")
		args = append(args, like, like, like, like)
	}
	return strings.Join(conditions, " and "), args
}

func buildOperationLogFilterWhere(filters operationLogFilters) (string, []any) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 8)
	if filters.ActorType != "" {
		conditions = append(conditions, "actor_type = ?")
		args = append(args, filters.ActorType)
	}
	if filters.Method != "" {
		conditions = append(conditions, "method = ?")
		args = append(args, filters.Method)
	}
	if filters.From != "" {
		conditions = append(conditions, "occurred_at >= ?")
		args = append(args, filters.From)
	}
	if filters.To != "" {
		conditions = append(conditions, "occurred_at <= ?")
		args = append(args, filters.To)
	}
	if filters.IP != "" {
		conditions = append(conditions, "ip_address like ?")
		args = append(args, "%"+filters.IP+"%")
	}
	if filters.Query != "" {
		like := "%" + filters.Query + "%"
		pathLikes := operationLogQueryPathLikes(filters.Query)
		pathConditions := make([]string, 0, len(pathLikes))
		queryArgs := []any{like, like, like}
		for _, pathLike := range pathLikes {
			pathConditions = append(pathConditions, "path like ?")
			queryArgs = append(queryArgs, pathLike)
		}
		queryConditions := []string{"actor_name like ?", "path like ?", "summary like ?"}
		queryConditions = append(queryConditions, pathConditions...)
		conditions = append(conditions, "("+strings.Join(queryConditions, " or ")+")")
		args = append(args, queryArgs...)
	}
	return strings.Join(conditions, " and "), args
}

func operationLogQueryPathLikes(query string) []any {
	trimmed := strings.TrimSpace(query)
	pathLikes := make([]any, 0, 2)
	if strings.Contains(trimmed, "退出") || strings.Contains(strings.ToLower(trimmed), "logout") {
		pathLikes = append(pathLikes, "%/logout%")
	}
	if strings.Contains(trimmed, "提交") || strings.Contains(strings.ToLower(trimmed), "submission") {
		pathLikes = append(pathLikes, "%/submission%")
	}
	return pathLikes
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func teacherLoginLogID(teacher *teacherRecord) int64 {
	if teacher == nil {
		return 0
	}
	return teacher.ID
}

func teacherLoginLogName(teacher *teacherRecord) string {
	if teacher == nil {
		return ""
	}
	return teacher.DisplayName
}

func clientIPAddress(request *http.Request) string {
	remoteAddress := remoteIPAddress(request)
	if trustedProxyAddress(remoteAddress) {
		forwardedFor := strings.TrimSpace(request.Header.Get("X-Forwarded-For"))
		if forwardedFor == "" {
			return remoteAddress
		}
		return strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
	}
	return remoteAddress
}

func remoteIPAddress(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return strings.TrimSpace(host)
	}
	return strings.TrimSpace(request.RemoteAddr)
}

func trustedProxyAddress(address string) bool {
	parsedAddress := net.ParseIP(strings.TrimSpace(address))
	if parsedAddress == nil {
		return false
	}
	for _, rawTrusted := range strings.Split(os.Getenv("CLASSDRIVE_TRUSTED_PROXIES"), ",") {
		trusted := strings.TrimSpace(rawTrusted)
		if trusted == "" {
			continue
		}
		if trustedIP := net.ParseIP(trusted); trustedIP != nil {
			if trustedIP.Equal(parsedAddress) {
				return true
			}
			continue
		}
		_, trustedNetwork, err := net.ParseCIDR(trusted)
		if err == nil && trustedNetwork.Contains(parsedAddress) {
			return true
		}
	}
	return false
}

func (app *App) listFileEntries(space string, classID *int64, currentPath string) ([]fileEntry, error) {
	if _, _, err := app.storageDir(space, classID); err != nil {
		return nil, err
	}
	query := `select id, space, class_id, parent_path, item_path, name, kind, mime_type, size, disk_path, created_at, updated_at from file_entries where space = ? and parent_path = ?`
	args := []any{space, currentPath}
	if classID == nil {
		query += ` and class_id is null`
	} else {
		query += ` and class_id = ?`
		args = append(args, *classID)
	}
	query += ` order by kind desc, name asc`
	rows, err := app.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []fileEntry
	for rows.Next() {
		var entry fileEntry
		if err := rows.Scan(&entry.ID, &entry.Space, &entry.ClassID, &entry.ParentPath, &entry.ItemPath, &entry.Name, &entry.Kind, &entry.MimeType, &entry.Size, &entry.DiskPath, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, entry)
	}
	return items, rows.Err()
}

const fileEntryEffectiveSizeExpression = `case when e.kind = 'dir' then coalesce((select sum(child.size) from file_entries child where child.space = e.space and ifnull(child.class_id, 0) = ifnull(e.class_id, 0) and child.kind = 'file' and child.item_path like e.item_path || '/%'), 0) else e.size end`

func fileEntryListOrderBy(sort string) string {
	switch sort {
	case "name-desc":
		return ` order by e.name desc`
	case "updated-desc":
		return ` order by e.updated_at desc, e.name asc`
	case "updated-asc":
		return ` order by e.updated_at asc, e.name asc`
	case "size-desc":
		return ` order by ` + fileEntryEffectiveSizeExpression + ` desc, e.name asc`
	case "size-asc":
		return ` order by ` + fileEntryEffectiveSizeExpression + ` asc, e.name asc`
	default:
		return ` order by e.name asc`
	}
}

func (app *App) listFileEntriesPage(space string, classID *int64, currentPath string, listQuery teacherListQuery) ([]fileEntry, paginationPayload, error) {
	if _, _, err := app.storageDir(space, classID); err != nil {
		return nil, paginationPayload{}, err
	}
	whereClause := ` where e.space = ? and e.parent_path = ?`
	args := []any{space, currentPath}
	if classID == nil {
		whereClause += ` and e.class_id is null`
	} else {
		whereClause += ` and e.class_id = ?`
		args = append(args, *classID)
	}

	var total int
	if err := app.db.QueryRow(`select count(*) from file_entries e`+whereClause, args...).Scan(&total); err != nil {
		return nil, paginationPayload{}, err
	}

	query := `select e.id, e.space, e.class_id, e.parent_path, e.item_path, e.name, e.kind, e.mime_type, ` +
		fileEntryEffectiveSizeExpression +
		`, e.disk_path, e.created_at, e.updated_at from file_entries e` +
		whereClause +
		fileEntryListOrderBy(listQuery.Sort)
	queryArgs := append([]any(nil), args...)
	if listQuery.Paged {
		query += ` limit ? offset ?`
		queryArgs = append(queryArgs, listQuery.PageSize, listQuery.Offset)
	}

	rows, err := app.db.Query(query, queryArgs...)
	if err != nil {
		return nil, paginationPayload{}, err
	}
	defer rows.Close()

	items := make([]fileEntry, 0, listQuery.PageSize)
	for rows.Next() {
		var entry fileEntry
		if err := rows.Scan(&entry.ID, &entry.Space, &entry.ClassID, &entry.ParentPath, &entry.ItemPath, &entry.Name, &entry.Kind, &entry.MimeType, &entry.Size, &entry.DiskPath, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, paginationPayload{}, err
		}
		items = append(items, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, paginationPayload{}, err
	}
	return items, buildTeacherListPagination(total, listQuery, len(items)), nil
}

func (app *App) searchFileEntries(space string, classID *int64, currentPath, query string) ([]fileEntry, error) {
	if _, _, err := app.storageDir(space, classID); err != nil {
		return nil, err
	}
	parsedQuery, err := parseFileSearchQuery(query)
	if err != nil {
		return nil, err
	}
	if parsedQuery.Keyword == "" && parsedQuery.TypeFilter == "" {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "q 不能为空"}
	}

	sqlQuery := `select id, space, class_id, parent_path, item_path, name, kind, mime_type, size, disk_path, created_at, updated_at from file_entries where space = ?`
	args := []any{space}
	if classID == nil {
		sqlQuery += ` and class_id is null`
	} else {
		sqlQuery += ` and class_id = ?`
		args = append(args, *classID)
	}
	if currentPath == "/" {
		sqlQuery += ` and item_path like ?`
		args = append(args, "/%")
	} else {
		sqlQuery += ` and item_path like ?`
		args = append(args, currentPath+"/%")
	}
	sqlQuery += ` order by kind desc, item_path asc`

	rows, err := app.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]fileEntry, 0)
	for rows.Next() {
		var entry fileEntry
		if err := rows.Scan(&entry.ID, &entry.Space, &entry.ClassID, &entry.ParentPath, &entry.ItemPath, &entry.Name, &entry.Kind, &entry.MimeType, &entry.Size, &entry.DiskPath, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, err
		}
		if matchesFileSearchQuery(entry, parsedQuery) {
			items = append(items, entry)
		}
	}
	return items, rows.Err()
}

func parseFileSearchQuery(query string) (fileSearchQuery, error) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return fileSearchQuery{}, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "q 不能为空"}
	}

	parsed := fileSearchQuery{
		CaseSensitive: false,
	}
	keywordParts := make([]string, 0)
	for _, token := range strings.Fields(trimmedQuery) {
		lowerToken := strings.ToLower(token)
		if strings.HasPrefix(lowerToken, "type:") {
			typeValue, err := parseFileSearchTypeToken(token)
			if err != nil {
				return fileSearchQuery{}, err
			}
			parsed.TypeFilter = typeValue
			continue
		}
		if strings.HasPrefix(lowerToken, "case:") {
			caseSensitive, err := parseFileSearchCaseToken(token)
			if err != nil {
				return fileSearchQuery{}, err
			}
			parsed.CaseSensitive = caseSensitive
			continue
		}
		keywordParts = append(keywordParts, token)
	}
	parsed.Keyword = strings.Join(keywordParts, " ")
	return parsed, nil
}

func parseFileSearchTypeToken(token string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(token[len("type:"):]))
	switch value {
	case "image", "audio", "video", "pdf", "dir", "file":
		return value, nil
	case "":
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "type 过滤条件不能为空"}
	default:
		return "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "type 过滤条件非法"}
	}
}

func parseFileSearchCaseToken(token string) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(token[len("case:"):]))
	switch value {
	case "sensitive":
		return true, nil
	case "insensitive":
		return false, nil
	case "":
		return false, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "case 过滤条件不能为空"}
	default:
		return false, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "case 过滤条件非法"}
	}
}

func matchesFileSearchQuery(entry fileEntry, query fileSearchQuery) bool {
	if !matchesFileSearchType(entry, query.TypeFilter) {
		return false
	}
	if query.Keyword == "" {
		return true
	}
	if query.CaseSensitive {
		return strings.Contains(entry.Name, query.Keyword)
	}
	return strings.Contains(strings.ToLower(entry.Name), strings.ToLower(query.Keyword))
}

func matchesFileSearchType(entry fileEntry, filter string) bool {
	switch filter {
	case "":
		return true
	case "dir":
		return entry.Kind == "dir"
	case "file":
		return entry.Kind == "file"
	}

	if entry.Kind != "file" {
		return false
	}

	normalizedMimeType := strings.ToLower(strings.TrimSpace(strings.Split(entry.MimeType, ";")[0]))
	switch filter {
	case "pdf":
		return strings.EqualFold(path.Ext(entry.Name), ".pdf") || normalizedMimeType == "application/pdf"
	case "image":
		return hasFileSearchExtension(entry.Name, filter) || strings.HasPrefix(normalizedMimeType, "image/")
	case "audio":
		return hasFileSearchExtension(entry.Name, filter) || strings.HasPrefix(normalizedMimeType, "audio/")
	case "video":
		return hasFileSearchExtension(entry.Name, filter) || strings.HasPrefix(normalizedMimeType, "video/")
	default:
		return false
	}
}

func hasFileSearchExtension(name, filter string) bool {
	extensions, ok := fileSearchTypeExtensions[filter]
	if !ok {
		return false
	}
	_, matched := extensions[strings.ToLower(path.Ext(strings.TrimSpace(name)))]
	return matched
}

func (app *App) createFolder(space string, classID *int64, parentPath, name string) (*fileEntry, error) {
	parent, err := normalizeRelativePath(parentPath)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	cleanName, err := sanitizeName(name)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	baseDir, relativeRoot, err := app.storageDir(space, classID)
	if err != nil {
		return nil, err
	}
	itemPath := joinChild(parent, cleanName)
	diskRelative := path.Join(relativeRoot, strings.TrimPrefix(itemPath, "/"))
	if err := app.ensurePathAvailable(space, classID, itemPath); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(baseDir, filepath.FromSlash(strings.TrimPrefix(itemPath, "/"))), 0o755); err != nil {
		return nil, err
	}
	entry := &fileEntry{
		Space:      space,
		ParentPath: parent,
		ItemPath:   itemPath,
		Name:       cleanName,
		Kind:       "dir",
		MimeType:   "inode/directory",
		Size:       0,
		DiskPath:   diskRelative,
	}
	if classID != nil {
		entry.ClassID = sql.NullInt64{Int64: *classID, Valid: true}
	}
	if err := app.insertEntry(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (app *App) createFile(space string, classID *int64, parentPath, name string, reader io.Reader) (*fileEntry, error) {
	parent, err := normalizeRelativePath(parentPath)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	cleanName, err := sanitizeName(name)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	baseDir, relativeRoot, err := app.storageDir(space, classID)
	if err != nil {
		return nil, err
	}
	itemPath := joinChild(parent, cleanName)
	if err := app.ensurePathAvailable(space, classID, itemPath); err != nil {
		return nil, err
	}
	absolutePath := filepath.Join(baseDir, filepath.FromSlash(strings.TrimPrefix(itemPath, "/")))
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(absolutePath, data, 0o644); err != nil {
		return nil, err
	}

	entry := &fileEntry{
		Space:      space,
		ParentPath: parent,
		ItemPath:   itemPath,
		Name:       cleanName,
		Kind:       "file",
		MimeType:   contentTypeForFile(cleanName, data),
		Size:       int64(len(data)),
		DiskPath:   path.Join(relativeRoot, strings.TrimPrefix(itemPath, "/")),
	}
	if classID != nil {
		entry.ClassID = sql.NullInt64{Int64: *classID, Valid: true}
	}
	if err := app.insertEntry(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (app *App) createFileWithConflict(space string, classID *int64, parentPath, name string, reader io.Reader, mode uploadConflictMode) (*fileEntry, uploadAction, error) {
	normalizedParentPath, err := normalizeRelativePath(parentPath)
	if err != nil {
		return nil, "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	cleanName, err := sanitizeName(name)
	if err != nil {
		return nil, "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	resolvedName, action, err := app.resolveConflictName(space, classID, normalizedParentPath, cleanName, mode)
	if err != nil {
		return nil, "", err
	}
	if action == uploadActionSkipped {
		return nil, action, nil
	}
	entry, err := app.createFile(space, classID, normalizedParentPath, resolvedName, reader)
	if err != nil {
		return nil, "", err
	}
	if action == uploadActionCreated && resolvedName != cleanName {
		action = uploadActionRenamed
	}
	return entry, action, nil
}

func (app *App) insertEntry(entry *fileEntry) error {
	now := time.Now().UTC().Format(time.RFC3339)
	entry.CreatedAt = now
	entry.UpdatedAt = now
	result, err := app.db.Exec(`
insert into file_entries (space, class_id, parent_path, item_path, name, kind, mime_type, size, disk_path, created_at, updated_at)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Space, nullableInt(entry.ClassID), entry.ParentPath, entry.ItemPath, entry.Name, entry.Kind, entry.MimeType, entry.Size, entry.DiskPath, entry.CreatedAt, entry.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	entry.ID = id
	return nil
}

func nullableInt(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func (app *App) ensurePathAvailable(space string, classID *int64, itemPath string) error {
	query := `select count(*) from file_entries where space = ? and item_path = ?`
	args := []any{space, itemPath}
	if classID == nil {
		query += ` and class_id is null`
	} else {
		query += ` and class_id = ?`
		args = append(args, *classID)
	}
	var count int
	if err := app.db.QueryRow(query, args...).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return domainError{Status: http.StatusConflict, Code: "conflict", Message: "同名文件或文件夹已存在"}
	}
	return nil
}

func (app *App) findEntryByPath(space string, classID *int64, itemPath string) (*fileEntry, error) {
	query := `select id, space, class_id, parent_path, item_path, name, kind, mime_type, size, disk_path, created_at, updated_at from file_entries where space = ? and item_path = ?`
	args := []any{space, itemPath}
	if classID == nil {
		query += ` and class_id is null`
	} else {
		query += ` and class_id = ?`
		args = append(args, *classID)
	}

	row := app.db.QueryRow(query, args...)
	var entry fileEntry
	if err := row.Scan(&entry.ID, &entry.Space, &entry.ClassID, &entry.ParentPath, &entry.ItemPath, &entry.Name, &entry.Kind, &entry.MimeType, &entry.Size, &entry.DiskPath, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}

func (app *App) resolveConflictName(space string, classID *int64, parentPath, name string, mode uploadConflictMode) (string, uploadAction, error) {
	itemPath := joinChild(parentPath, name)
	existing, err := app.findEntryByPath(space, classID, itemPath)
	if err != nil {
		return "", "", err
	}
	if existing == nil {
		return name, uploadActionCreated, nil
	}

	switch mode {
	case uploadConflictModeReject:
		return "", "", domainError{Status: http.StatusConflict, Code: "conflict", Message: "同名文件或文件夹已存在"}
	case uploadConflictModeSkip:
		return name, uploadActionSkipped, nil
	case uploadConflictModeReplace:
		if err := app.deleteEntry(existing.ID); err != nil {
			return "", "", err
		}
		return name, uploadActionReplaced, nil
	case uploadConflictModeRename:
		nextName, err := app.nextAvailableName(space, classID, parentPath, name)
		if err != nil {
			return "", "", err
		}
		return nextName, uploadActionRenamed, nil
	default:
		return "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "conflictMode 非法"}
	}
}

func (app *App) nextAvailableName(space string, classID *int64, parentPath, name string) (string, error) {
	extension := path.Ext(name)
	baseName := strings.TrimSuffix(name, extension)
	if baseName == "" {
		baseName = name
		extension = ""
	}
	for index := 1; index < 10_000; index++ {
		candidate := fmt.Sprintf("%s (%d)%s", baseName, index, extension)
		existing, err := app.findEntryByPath(space, classID, joinChild(parentPath, candidate))
		if err != nil {
			return "", err
		}
		if existing == nil {
			return candidate, nil
		}
	}
	return "", domainError{Status: http.StatusConflict, Code: "conflict", Message: "无法生成可用名称"}
}

func normalizeUploadRelativePath(relativePath string) ([]string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/")
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return nil, nil
	}
	parts := strings.Split(normalized, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "relativePaths 非法"}
		}
		cleanName, err := sanitizeName(part)
		if err != nil {
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
		}
		segments = append(segments, cleanName)
	}
	return segments, nil
}

func (app *App) ensureDirectoryHierarchy(space string, classID *int64, parentPath string) error {
	normalizedParentPath, err := normalizeRelativePath(parentPath)
	if err != nil {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	if normalizedParentPath == "/" {
		return nil
	}
	segments := strings.Split(strings.TrimPrefix(normalizedParentPath, "/"), "/")
	currentPath := "/"
	for _, segment := range segments {
		itemPath := joinChild(currentPath, segment)
		existing, err := app.findEntryByPath(space, classID, itemPath)
		if err != nil {
			return err
		}
		if existing == nil {
			if _, err := app.createFolder(space, classID, currentPath, segment); err != nil {
				return err
			}
		} else if existing.Kind != "dir" {
			return domainError{Status: http.StatusConflict, Code: "conflict", Message: "目录结构与现有文件冲突"}
		}
		currentPath = itemPath
	}
	return nil
}

func (app *App) resolveUploadTarget(
	space string,
	classID *int64,
	parentPath string,
	fallbackName string,
	relativePath string,
	mode uploadConflictMode,
	rootReplacements map[string]*string,
) (string, string, uploadAction, error) {
	normalizedParentPath, err := normalizeRelativePath(parentPath)
	if err != nil {
		return "", "", "", domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	segments, err := normalizeUploadRelativePath(relativePath)
	if err != nil {
		return "", "", "", err
	}
	if len(segments) == 0 {
		return normalizedParentPath, fallbackName, uploadActionCreated, app.ensureDirectoryHierarchy(space, classID, normalizedParentPath)
	}

	rootName := segments[0]
	if replacement, exists := rootReplacements[rootName]; !exists {
		resolvedName, action, err := app.resolveConflictName(space, classID, normalizedParentPath, rootName, mode)
		if err != nil {
			return "", "", "", err
		}
		switch action {
		case uploadActionSkipped:
			rootReplacements[rootName] = nil
		default:
			rootReplacements[rootName] = &resolvedName
		}
	} else if replacement == nil {
		return "", "", uploadActionSkipped, nil
	}

	replacement := rootReplacements[rootName]
	if replacement == nil {
		return "", "", uploadActionSkipped, nil
	}

	segments[0] = *replacement
	targetName := segments[len(segments)-1]
	targetParentPath := normalizedParentPath
	if len(segments) > 1 {
		targetParentPath = joinChild(normalizedParentPath, path.Join(segments[:len(segments)-1]...))
	}
	if err := app.ensureDirectoryHierarchy(space, classID, targetParentPath); err != nil {
		return "", "", "", err
	}
	return targetParentPath, targetName, uploadActionCreated, nil
}

func (app *App) getEntryByID(id int64) (*fileEntry, error) {
	row := app.db.QueryRow(`select id, space, class_id, parent_path, item_path, name, kind, mime_type, size, disk_path, created_at, updated_at from file_entries where id = ?`, id)
	var entry fileEntry
	if err := row.Scan(&entry.ID, &entry.Space, &entry.ClassID, &entry.ParentPath, &entry.ItemPath, &entry.Name, &entry.Kind, &entry.MimeType, &entry.Size, &entry.DiskPath, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainError{Status: http.StatusNotFound, Code: "not_found", Message: "资源不存在"}
		}
		return nil, err
	}
	return &entry, nil
}

func (app *App) copyEntry(entryID int64, destinationSpace string, destinationClassID *int64, destinationParentPath string) (*fileEntry, error) {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		return nil, err
	}
	switch entry.Kind {
	case "file":
		return app.copyFileEntry(entry, destinationSpace, destinationClassID, destinationParentPath)
	case "dir":
		return app.copyDirectoryEntry(entry, destinationSpace, destinationClassID, destinationParentPath)
	default:
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "不支持复制当前资源"}
	}
}

func (app *App) moveEntry(entryID int64, destinationSpace string, destinationClassID *int64, destinationParentPath string) (*fileEntry, error) {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		return nil, err
	}
	normalizedParentPath, err := normalizeRelativePath(destinationParentPath)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	if err := app.validateMoveDestination(entry, destinationSpace, destinationClassID, normalizedParentPath); err != nil {
		return nil, err
	}
	movedEntry, err := app.copyEntry(entryID, destinationSpace, destinationClassID, normalizedParentPath)
	if err != nil {
		return nil, err
	}
	if err := app.deleteEntry(entryID); err != nil {
		return nil, err
	}
	return movedEntry, nil
}

func (app *App) validateMoveDestination(entry *fileEntry, destinationSpace string, destinationClassID *int64, destinationParentPath string) error {
	if _, _, err := app.storageDir(destinationSpace, destinationClassID); err != nil {
		return err
	}
	if entry.Kind != "dir" {
		return nil
	}
	if entry.Space != destinationSpace {
		return nil
	}
	switch {
	case entry.ClassID.Valid && destinationClassID == nil:
		return nil
	case !entry.ClassID.Valid && destinationClassID != nil:
		return nil
	case entry.ClassID.Valid && destinationClassID != nil && entry.ClassID.Int64 != *destinationClassID:
		return nil
	}
	if destinationParentPath == entry.ItemPath || strings.HasPrefix(destinationParentPath, entry.ItemPath+"/") {
		return domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "目录不能移动到自身或子目录"}
	}
	return nil
}

func (app *App) copyFileEntry(entry *fileEntry, destinationSpace string, destinationClassID *int64, destinationParentPath string) (*fileEntry, error) {
	sourcePath := filepath.Join(app.storageRoot, filepath.FromSlash(entry.DiskPath))
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return app.createFile(destinationSpace, destinationClassID, destinationParentPath, entry.Name, file)
}

func (app *App) copyDirectoryEntry(entry *fileEntry, destinationSpace string, destinationClassID *int64, destinationParentPath string) (*fileEntry, error) {
	descendants, err := app.listDescendants(entry)
	if err != nil {
		return nil, err
	}

	rootCopy, err := app.createFolder(destinationSpace, destinationClassID, destinationParentPath, entry.Name)
	if err != nil {
		return nil, err
	}

	for _, descendant := range descendants {
		relativePath := strings.TrimPrefix(descendant.ItemPath, entry.ItemPath+"/")
		targetParent := rootCopy.ItemPath
		relativeParent := path.Dir(relativePath)
		if relativeParent != "." {
			targetParent = joinChild(rootCopy.ItemPath, relativeParent)
		}

		switch descendant.Kind {
		case "dir":
			if _, err := app.createFolder(destinationSpace, destinationClassID, targetParent, descendant.Name); err != nil {
				return nil, err
			}
		case "file":
			sourcePath := filepath.Join(app.storageRoot, filepath.FromSlash(descendant.DiskPath))
			file, openErr := os.Open(sourcePath)
			if openErr != nil {
				return nil, openErr
			}
			_, createErr := app.createFile(destinationSpace, destinationClassID, targetParent, descendant.Name, file)
			_ = file.Close()
			if createErr != nil {
				return nil, createErr
			}
		default:
			return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: "目录复制包含未知资源类型"}
		}
	}

	return rootCopy, nil
}

func (app *App) listDescendants(entry *fileEntry) ([]fileEntry, error) {
	rows, err := app.db.Query(`
select id, space, class_id, parent_path, item_path, name, kind, mime_type, size, disk_path, created_at, updated_at
from file_entries
where space = ? and ifnull(class_id, 0) = ? and item_path like ?
order by length(item_path) asc, item_path asc`,
		entry.Space,
		classIDKey(entry.ClassID),
		entry.ItemPath+"/%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var descendants []fileEntry
	for rows.Next() {
		var item fileEntry
		if err := rows.Scan(&item.ID, &item.Space, &item.ClassID, &item.ParentPath, &item.ItemPath, &item.Name, &item.Kind, &item.MimeType, &item.Size, &item.DiskPath, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		descendants = append(descendants, item)
	}
	return descendants, rows.Err()
}

func (app *App) renameEntry(entryID int64, newName string) (*fileEntry, error) {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		return nil, err
	}
	cleanName, err := sanitizeName(newName)
	if err != nil {
		return nil, domainError{Status: http.StatusUnprocessableEntity, Code: "invalid_request", Message: err.Error()}
	}
	newPath := joinChild(entry.ParentPath, cleanName)
	if newPath == entry.ItemPath {
		return entry, nil
	}
	classID := nullableClassID(entry.ClassID)
	if err := app.ensurePathAvailable(entry.Space, classID, newPath); err != nil {
		return nil, err
	}
	baseDir, relativeRoot, err := app.storageDir(entry.Space, classID)
	if err != nil {
		return nil, err
	}
	oldAbsolute := filepath.Join(app.storageRoot, filepath.FromSlash(entry.DiskPath))
	newAbsolute := filepath.Join(baseDir, filepath.FromSlash(strings.TrimPrefix(newPath, "/")))
	if err := os.MkdirAll(filepath.Dir(newAbsolute), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(oldAbsolute, newAbsolute); err != nil {
		return nil, err
	}
	newDiskPath := path.Join(relativeRoot, strings.TrimPrefix(newPath, "/"))
	if entry.Kind == "dir" {
		if err := app.renameDescendants(entry, newPath, newDiskPath); err != nil {
			return nil, err
		}
	}
	entry.Name = cleanName
	entry.ItemPath = newPath
	entry.DiskPath = newDiskPath
	entry.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err = app.db.Exec(`update file_entries set name = ?, item_path = ?, disk_path = ?, updated_at = ? where id = ?`, entry.Name, entry.ItemPath, entry.DiskPath, entry.UpdatedAt, entry.ID)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (app *App) renameDescendants(entry *fileEntry, newPath, newDiskPath string) error {
	oldPrefix := entry.ItemPath + "/"
	newPrefix := newPath + "/"
	oldDiskPrefix := entry.DiskPath + "/"
	newDiskPrefix := newDiskPath + "/"
	rows, err := app.db.Query(`select id, item_path, disk_path from file_entries where item_path like ? and space = ? and ifnull(class_id, 0) = ?`, oldPrefix+"%", entry.Space, classIDKey(entry.ClassID))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var itemPath string
		var diskPath string
		if err := rows.Scan(&id, &itemPath, &diskPath); err != nil {
			return err
		}
		updatedPath := strings.Replace(itemPath, oldPrefix, newPrefix, 1)
		updatedDisk := strings.Replace(diskPath, oldDiskPrefix, newDiskPrefix, 1)
		parent := path.Dir(updatedPath)
		if parent == "." {
			parent = "/"
		}
		if !strings.HasPrefix(parent, "/") {
			parent = "/" + parent
		}
		if _, err := app.db.Exec(`update file_entries set parent_path = ?, item_path = ?, disk_path = ?, updated_at = ? where id = ?`, parent, updatedPath, updatedDisk, time.Now().UTC().Format(time.RFC3339), id); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (app *App) deleteEntry(entryID int64) error {
	entry, err := app.getEntryByID(entryID)
	if err != nil {
		return err
	}
	absolutePath := filepath.Join(app.storageRoot, filepath.FromSlash(entry.DiskPath))
	if err := removeFileTree(absolutePath); err != nil {
		return err
	}
	if entry.Kind == "dir" {
		_, err = app.db.Exec(`delete from file_entries where id = ? or (space = ? and ifnull(class_id, 0) = ? and item_path like ?)`, entry.ID, entry.Space, classIDKey(entry.ClassID), entry.ItemPath+"/%")
		return err
	}
	_, err = app.db.Exec(`delete from file_entries where id = ?`, entry.ID)
	return err
}

var removeAllPath = os.RemoveAll

func removeFileTree(absolutePath string) error {
	if err := removeAllPath(absolutePath); err == nil || os.IsNotExist(err) {
		return nil
	}
	return removeFileTreeByWalking(absolutePath)
}

func removeFileTreeByWalking(absolutePath string) error {
	info, err := os.Lstat(absolutePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if err := os.Remove(absolutePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	entries, err := os.ReadDir(absolutePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := removeFileTreeByWalking(filepath.Join(absolutePath, entry.Name())); err != nil {
			return err
		}
	}
	if err := os.Remove(absolutePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func buildFileSummary(entry fileEntry, downloadURL, previewURL string) fileSummary {
	return fileSummary{
		ID:          entry.ID,
		Name:        entry.Name,
		Path:        entry.ItemPath,
		Kind:        entry.Kind,
		MimeType:    entry.MimeType,
		Size:        entry.Size,
		UpdatedAt:   entry.UpdatedAt,
		DownloadURL: downloadURL,
		ArchiveURL:  "/api/files/" + strconv.FormatInt(entry.ID, 10) + "/archive",
		PreviewURL:  previewURL,
	}
}

func buildDefaultFileSummary(entry fileEntry) fileSummary {
	return buildFileSummary(
		entry,
		"/api/files/"+strconv.FormatInt(entry.ID, 10)+"/download",
		"/api/files/"+strconv.FormatInt(entry.ID, 10)+"/preview",
	)
}

func buildStudentFileSummary(entry fileEntry) fileSummary {
	entryID := strconv.FormatInt(entry.ID, 10)
	summary := buildFileSummary(
		entry,
		"/api/student/files/"+entryID+"/download",
		"/api/student/files/"+entryID+"/preview",
	)
	summary.ArchiveURL = "/api/student/files/" + entryID + "/archive"
	return summary
}

func classIDKey(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

func nullableClassID(value sql.NullInt64) *int64 {
	if value.Valid {
		return &value.Int64
	}
	return nil
}
