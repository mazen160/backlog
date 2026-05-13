package models

// ---- Enums ----

type TaskType string

const (
	TaskTypeTask          TaskType = "task"
	TaskTypeBug           TaskType = "bug"
	TaskTypeIssue         TaskType = "issue"
	TaskTypeImprovement   TaskType = "improvement"
	TaskTypeFeature       TaskType = "feature"
	TaskTypeVulnerability TaskType = "vulnerability"
	TaskTypeChore         TaskType = "chore"
	TaskTypeSpike         TaskType = "spike"
	TaskTypeBucketList    TaskType = "bucket-list"
)

var allTaskTypes = [...]TaskType{
	TaskTypeTask, TaskTypeBug, TaskTypeIssue, TaskTypeImprovement,
	TaskTypeFeature, TaskTypeVulnerability, TaskTypeChore, TaskTypeSpike, TaskTypeBucketList,
}

// AllTaskTypes returns a fresh slice copy of every valid task type. The
// underlying source is an array so callers cannot mutate the canonical list.
func AllTaskTypes() []TaskType {
	out := make([]TaskType, len(allTaskTypes))
	copy(out, allTaskTypes[:])
	return out
}

func (t TaskType) Valid() bool {
	for _, v := range allTaskTypes {
		if t == v {
			return true
		}
	}
	return false
}

type TaskStatus string

const (
	TaskStatusTodo  TaskStatus = "todo"
	TaskStatusDoing TaskStatus = "doing"
	TaskStatusDone  TaskStatus = "done"
)

var allTaskStatuses = [...]TaskStatus{TaskStatusTodo, TaskStatusDoing, TaskStatusDone}

// AllTaskStatuses returns a fresh slice copy of every valid task status.
func AllTaskStatuses() []TaskStatus {
	out := make([]TaskStatus, len(allTaskStatuses))
	copy(out, allTaskStatuses[:])
	return out
}

func (s TaskStatus) Valid() bool {
	for _, v := range allTaskStatuses {
		if s == v {
			return true
		}
	}
	return false
}

type ActorKind string

const (
	ActorKindHuman ActorKind = "human"
	ActorKindAI    ActorKind = "ai"
)

// ---- Actor ----

type Actor struct {
	Kind ActorKind `json:"kind"`
	Name string    `json:"name"`
}

// ---- Project ----

type Project struct {
	ID          string `json:"id"`
	Alias       string `json:"alias"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RepoPath    string `json:"repo_path"`
	ArchivedAt  *int64 `json:"archived_at,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type CreateProjectInput struct {
	Alias       string
	Name        string
	Description string
	RepoPath    string
	Actor       Actor
}

type UpdateProjectInput struct {
	Name        *string
	Description *string
	RepoPath    *string
}

// ---- Task ----

type Task struct {
	ID          string     `json:"id"`
	Seq         int        `json:"seq"`
	ProjectID   string     `json:"project_id"`
	Project     *Project   `json:"project,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Type        TaskType   `json:"type"`
	Status      TaskStatus `json:"status"`
	Priority    int        `json:"priority"`
	Assignee    string     `json:"assignee,omitempty"`
	DueAt       *int64     `json:"due_at,omitempty"`
	Actor       Actor      `json:"actor"`
	Source      string     `json:"source,omitempty"`
	ExternalRef string     `json:"external_ref,omitempty"`
	CompletedAt *int64     `json:"completed_at,omitempty"`
	ArchivedAt  *int64     `json:"archived_at,omitempty"`
	CreatedAt   int64      `json:"created_at"`
	UpdatedAt   int64      `json:"updated_at"`
	ProjectPath string     `json:"project_path,omitempty"`
	Labels      []Label    `json:"labels,omitempty"`
	Plans       []Plan     `json:"plans,omitempty"`
	Comments    []Comment  `json:"comments,omitempty"`
}

type TaskFilter struct {
	ProjectAlias    string
	Status          TaskStatus
	Type            TaskType
	Priority        int // 0 = unset
	Assignee        string
	Labels          []string
	ActorKind       ActorKind
	ActorName       string
	Source          string
	Search          string
	IncludeArchived bool
	Limit           int
	Offset          int
	Sort            string // "priority", "created", "updated", "seq" (default: priority)
}

type CreateTaskInput struct {
	ProjectID   string
	Title       string
	Description string
	Type        TaskType
	Status      TaskStatus
	Priority    int
	Assignee    string
	DueAt       *int64
	Actor       Actor
	Source      string
	ExternalRef string
	ProjectPath string
	Labels      []string
	Plans       []CreatePlanInput
}

type UpdateTaskInput struct {
	Title       *string
	Description *string
	Type        *TaskType
	Status      *TaskStatus
	Priority    *int
	Assignee    *string
	DueAt       *int64
	ClearDueAt  bool
	Source      *string
	ExternalRef *string
	ProjectPath *string
}

// ---- Plan ----

type Plan struct {
	ID             string       `json:"id"`
	TaskID         string       `json:"task_id"`
	CurrentVersion int          `json:"current_version"`
	Source         string       `json:"source,omitempty"`
	ArchivedAt     *int64       `json:"archived_at,omitempty"`
	CreatedAt      int64        `json:"created_at"`
	UpdatedAt      int64        `json:"updated_at"`
	Version        *PlanVersion `json:"version,omitempty"`
}

type PlanVersion struct {
	ID         string `json:"id"`
	PlanID     string `json:"plan_id"`
	Version    int    `json:"version"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Actor      Actor  `json:"actor"`
	ChangeNote string `json:"change_note,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

type CreatePlanInput struct {
	TaskID     string
	Title      string
	Body       string
	Source     string
	ChangeNote string
	Actor      Actor
}

type UpdatePlanInput struct {
	Title      string
	Body       string
	ChangeNote string
	Actor      Actor
}

// ---- Comment ----

type Comment struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Body      string `json:"body"`
	Actor     Actor  `json:"actor"`
	CreatedAt int64  `json:"created_at"`
}

type CreateCommentInput struct {
	TaskID string
	Body   string
	Actor  Actor
}

// ---- Label ----

type Label struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
}

type CreateLabelInput struct {
	ProjectID string
	Name      string
	Color     string
	Actor     Actor
}

// ---- Activity ----

type Activity struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id,omitempty"`
	Entity    string `json:"entity"`
	EntityID  string `json:"entity_id"`
	Action    string `json:"action"`
	Summary   string `json:"summary"`
	Actor     Actor  `json:"actor"`
	Payload   string `json:"payload"`
	CreatedAt int64  `json:"created_at"`
}

// ---- Memory ----

type Memory struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"project_id"`
	Project   *Project `json:"project,omitempty"`
	Body      string   `json:"body"`
	Tags      string   `json:"tags,omitempty"`
	Actor     Actor    `json:"actor"`
	CreatedAt int64    `json:"created_at"`
}

type CreateMemoryInput struct {
	ProjectID string
	Body      string
	Tags      string
	Actor     Actor
}

// ---- Doc ----

type Doc struct {
	ID             string      `json:"id"`
	ProjectID      string      `json:"project_id"`
	Project        *Project    `json:"project,omitempty"`
	Title          string      `json:"title"`
	CurrentVersion int         `json:"current_version"`
	Actor          Actor       `json:"actor"`
	ArchivedAt     *int64      `json:"archived_at,omitempty"`
	CreatedAt      int64       `json:"created_at"`
	UpdatedAt      int64       `json:"updated_at"`
	Version        *DocVersion `json:"version,omitempty"`
}

type DocVersion struct {
	ID         string `json:"id"`
	DocID      string `json:"doc_id"`
	Version    int    `json:"version"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Actor      Actor  `json:"actor"`
	ChangeNote string `json:"change_note,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

type CreateDocInput struct {
	ProjectID string
	Title     string
	Body      string
	Actor     Actor
}

type UpdateDocInput struct {
	Title      string
	Body       string
	ChangeNote string
	Actor      Actor
}

// ---- Attachment ----

type Attachment struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	MimeType   string `json:"mime_type"`
	Size       int64  `json:"size"`
	LinkedType string `json:"linked_type"`
	LinkedID   string `json:"linked_id"`
	Actor      Actor  `json:"actor"`
	CreatedAt  int64  `json:"created_at"`
}

type AttachmentWithData struct {
	Attachment
	Data []byte `json:"-"`
}

// AttachmentMeta is like Attachment but includes display info about the linked entity.
// When the listing spans multiple projects, ProjectAlias / ProjectName are populated
// so the client can render a project chip per row.
type AttachmentMeta struct {
	Attachment
	TaskSeq      int    `json:"task_seq"`
	TaskTitle    string `json:"task_title"`
	DocTitle     string `json:"doc_title"`
	ProjectAlias string `json:"project_alias,omitempty"`
	ProjectName  string `json:"project_name,omitempty"`
}
