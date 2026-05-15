package web

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mazen160/backlog/internal/models"
	"github.com/mazen160/backlog/internal/repo"
	"github.com/mazen160/backlog/internal/service"
)

//go:embed static
var staticFiles embed.FS

// Server is the backlog web UI HTTP server.
type Server struct {
	db    *sql.DB
	actor models.Actor
	mux   *http.ServeMux
}

func New(db *sql.DB, actor models.Actor) *Server {
	s := &Server{db: db, actor: actor, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Static assets
	static, _ := fs.Sub(staticFiles, "static")
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	// SPA index. Serve index.html for "/" and any other path that isn't
	// an API or static asset, so client-side deep links like /tasks/35 work.
	// ServeMux routes /api/ and /static/ to their handlers first because
	// their prefixes are more specific than "/".
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || (!strings.HasPrefix(r.URL.Path, "/api/") && !strings.HasPrefix(r.URL.Path, "/static/")) {
			data, _ := staticFiles.ReadFile("static/index.html")
			w.Header().Set("Content-Type", "text/html")
			w.Write(data)
			return
		}
		http.NotFound(w, r)
	})

	// API
	s.mux.HandleFunc("/api/projects", s.handleProjects)
	s.mux.HandleFunc("/api/projects/", s.handleProjectByAlias)
	s.mux.HandleFunc("/api/tasks", s.handleTasks)
	s.mux.HandleFunc("/api/tasks/", s.handleTaskByID)
	s.mux.HandleFunc("/api/labels", s.handleLabels)
	s.mux.HandleFunc("/api/plans", s.handlePlans)
	s.mux.HandleFunc("/api/plans/", s.handlePlanByID)
	s.mux.HandleFunc("/api/docs", s.handleDocs)
	s.mux.HandleFunc("/api/docs/", s.handleDocByID)
	s.mux.HandleFunc("/api/memory", s.handleMemory)
	s.mux.HandleFunc("/api/memory/", s.handleMemoryByID)
	s.mux.HandleFunc("/api/attachments", s.handleAttachments)
	s.mux.HandleFunc("/api/attachments/", s.handleAttachmentByID)
	s.mux.HandleFunc("/api/activity", s.handleActivity)
}

// ─── Projects ─────────────────────────────────────────────────────────────────

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	svc := service.NewProjectService(s.db)
	switch r.Method {
	case http.MethodGet:
		includeArchived := r.URL.Query().Get("include_archived") == "true" || r.URL.Query().Get("include_archived") == "1"
		projects, err := svc.List(r.Context(), includeArchived)
		if err != nil {
			jsonError(w, err, 500)
			return
		}
		jsonOK(w, map[string]interface{}{"projects": projects})

	case http.MethodPost:
		var in struct {
			Alias       string `json:"alias"`
			Name        string `json:"name"`
			Description string `json:"description"`
			RepoPath    string `json:"repo_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonError(w, err, 400)
			return
		}
		p, err := svc.Create(r.Context(), models.CreateProjectInput{
			Alias:       in.Alias,
			Name:        in.Name,
			Description: in.Description,
			RepoPath:    in.RepoPath,
			Actor:       s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, p)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleProjectByAlias(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.SplitN(path, "/", 2)
	alias, err := url.PathUnescape(parts[0])
	if err != nil || alias == "" {
		http.NotFound(w, r)
		return
	}
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}

	svc := service.NewProjectService(s.db)
	switch {
	case r.Method == http.MethodPatch && sub == "archive":
		p, err := svc.Archive(r.Context(), alias, s.actor)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, p)
	case r.Method == http.MethodPatch && sub == "unarchive":
		p, err := svc.Unarchive(r.Context(), alias, s.actor)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, p)
	case r.Method == http.MethodDelete && sub == "":
		p, err := svc.GetByAlias(r.Context(), alias)
		if err != nil {
			jsonError(w, err, 404)
			return
		}
		if p.ArchivedAt == nil {
			jsonError(w, fmt.Errorf("project must be archived before deletion"), 400)
			return
		}
		if err := svc.Delete(r.Context(), alias, s.actor); err != nil {
			jsonError(w, err, 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "not found", 404)
	}
}

// ─── Tasks ────────────────────────────────────────────────────────────────────

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	planSvc := service.NewPlanService(s.db)
	labelSvc := service.NewLabelService(s.db)
	taskSvc := service.NewTaskService(s.db, planSvc, labelSvc)

	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		f := models.TaskFilter{
			ProjectAlias:    q.Get("project"),
			Status:          models.TaskStatus(q.Get("status")),
			Type:            models.TaskType(q.Get("type")),
			Labels:          q["label"],
			Search:          q.Get("search"),
			Sort:            q.Get("sort"),
			IncludeArchived: q.Get("include_archived") == "true" || q.Get("include_archived") == "1",
			Limit:           50,
			Offset:          0,
		}
		if p := q.Get("priority"); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n >= 1 && n <= 5 {
				f.Priority = n
			}
		}
		if l := q.Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				f.Limit = n
			}
		}
		if o := q.Get("offset"); o != "" {
			if n, err := strconv.Atoi(o); err == nil && n >= 0 {
				f.Offset = n
			}
		}
		tasks, total, err := taskSvc.List(r.Context(), f)
		if err != nil {
			if isFTSSyntaxError(err) {
				jsonError(w, fmt.Errorf("invalid search syntax"), 400)
				return
			}
			jsonError(w, err, 500)
			return
		}
		jsonOK(w, map[string]interface{}{"tasks": tasks, "page": map[string]int{"total": total}})

	case http.MethodPost:
		var in struct {
			Project     string   `json:"project"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Type        string   `json:"type"`
			Status      string   `json:"status"`
			Priority    int      `json:"priority"`
			Assignee    string   `json:"assignee"`
			DueAt       *int64   `json:"due_at"`
			DueDate     string   `json:"due_date"`
			Source      string   `json:"source"`
			ExternalRef string   `json:"external_ref"`
			ProjectPath string   `json:"project_path"`
			Labels      []string `json:"labels"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonError(w, err, 400)
			return
		}
		projSvc := service.NewProjectService(s.db)
		p, err := projSvc.GetByAlias(r.Context(), in.Project)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		if in.Priority == 0 {
			in.Priority = 3
		}
		dueAt, err := webDueAt(in.DueAt, in.DueDate)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		t, err := taskSvc.Create(r.Context(), models.CreateTaskInput{
			ProjectID:   p.ID,
			Title:       in.Title,
			Description: in.Description,
			Type:        models.TaskType(in.Type),
			Status:      models.TaskStatus(in.Status),
			Priority:    in.Priority,
			Assignee:    in.Assignee,
			DueAt:       dueAt,
			Source:      in.Source,
			ExternalRef: in.ExternalRef,
			ProjectPath: in.ProjectPath,
			Labels:      in.Labels,
			Actor:       s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, t)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	planSvc := service.NewPlanService(s.db)
	labelSvc := service.NewLabelService(s.db)
	taskSvc := service.NewTaskService(s.db, planSvc, labelSvc)

	// /api/tasks/{id}[/comments|/plans|/status]
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}

	ctx := r.Context()

	switch {
	case r.Method == http.MethodGet && sub == "":
		t, err := taskSvc.Get(ctx, id, true, true)
		if err != nil {
			jsonError(w, err, 404)
			return
		}
		jsonOK(w, t)

	case r.Method == http.MethodPatch && sub == "":
		var in struct {
			Title       *string `json:"title"`
			Description *string `json:"description"`
			Type        *string `json:"type"`
			Priority    *int    `json:"priority"`
			Assignee    *string `json:"assignee"`
			DueAt       *int64  `json:"due_at"`
			DueDate     *string `json:"due_date"`
			ClearDueAt  bool    `json:"clear_due_at"`
			Source      *string `json:"source"`
			ExternalRef *string `json:"external_ref"`
			ProjectPath *string `json:"project_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonError(w, err, 400)
			return
		}
		inp := models.UpdateTaskInput{}
		if in.Title != nil {
			inp.Title = in.Title
		}
		if in.Description != nil {
			inp.Description = in.Description
		}
		if in.Type != nil {
			tt := models.TaskType(*in.Type)
			inp.Type = &tt
		}
		if in.Priority != nil {
			inp.Priority = in.Priority
		}
		if in.Assignee != nil {
			inp.Assignee = in.Assignee
		}
		if in.ClearDueAt {
			inp.ClearDueAt = true
		} else if in.DueAt != nil || in.DueDate != nil {
			dueDate := ""
			if in.DueDate != nil {
				dueDate = *in.DueDate
			}
			dueAt, err := webDueAt(in.DueAt, dueDate)
			if err != nil {
				jsonError(w, err, 400)
				return
			}
			inp.DueAt = dueAt
		}
		if in.Source != nil {
			inp.Source = in.Source
		}
		if in.ExternalRef != nil {
			inp.ExternalRef = in.ExternalRef
		}
		if in.ProjectPath != nil {
			inp.ProjectPath = in.ProjectPath
		}
		t, err := taskSvc.Update(ctx, id, inp, s.actor)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, t)

	case r.Method == http.MethodPatch && sub == "status":
		var in struct {
			Status string `json:"status"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		t, err := taskSvc.Move(ctx, id, models.TaskStatus(in.Status), s.actor)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, t)

	case r.Method == http.MethodPatch && sub == "archive":
		t, err := taskSvc.Archive(ctx, id, s.actor)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, t)

	case r.Method == http.MethodPatch && sub == "unarchive":
		t, err := taskSvc.Unarchive(ctx, id, s.actor)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, t)

	case r.Method == http.MethodPost && sub == "comments":
		var in struct {
			Body string `json:"body"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		taskID, err := taskSvc.ResolveRef(ctx, id)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		c, err := service.NewCommentService(s.db).Create(ctx, models.CreateCommentInput{
			TaskID: taskID,
			Body:   in.Body,
			Actor:  s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, c)

	case r.Method == http.MethodPost && sub == "plans":
		var in struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		taskID, err := taskSvc.ResolveRef(ctx, id)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		pl, err := planSvc.Create(ctx, models.CreatePlanInput{
			TaskID: taskID,
			Title:  in.Title,
			Body:   in.Body,
			Actor:  s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, pl)

	case r.Method == http.MethodPost && sub == "labels":
		var in struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonError(w, err, 400)
			return
		}
		if strings.TrimSpace(in.Name) == "" {
			jsonError(w, fmt.Errorf("name is required"), 400)
			return
		}
		t, err := taskSvc.Get(ctx, id, false, false)
		if err != nil {
			jsonError(w, err, 404)
			return
		}
		if err := labelSvc.AttachByName(ctx, t.ProjectID, t.ID, strings.TrimSpace(in.Name), s.actor); err != nil {
			jsonError(w, err, 400)
			return
		}
		updated, err := taskSvc.Get(ctx, t.ID, true, true)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, updated)

	case r.Method == http.MethodDelete && strings.HasPrefix(sub, "labels/"):
		name, err := url.PathUnescape(strings.TrimPrefix(sub, "labels/"))
		if err != nil || strings.TrimSpace(name) == "" {
			jsonError(w, fmt.Errorf("label name is required"), 400)
			return
		}
		t, err := taskSvc.Get(ctx, id, false, false)
		if err != nil {
			jsonError(w, err, 404)
			return
		}
		if err := labelSvc.Detach(ctx, t.ID, t.ProjectID, strings.TrimSpace(name), s.actor); err != nil {
			jsonError(w, err, 400)
			return
		}
		updated, err := taskSvc.Get(ctx, t.ID, true, true)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, updated)

	default:
		http.Error(w, "not found", 404)
	}
}

// ─── Labels ──────────────────────────────────────────────────────────────────

func (s *Server) handleLabels(w http.ResponseWriter, r *http.Request) {
	labelSvc := service.NewLabelService(s.db)
	projSvc := service.NewProjectService(s.db)

	switch r.Method {
	case http.MethodGet:
		project := r.URL.Query().Get("project")
		if project == "" {
			jsonError(w, fmt.Errorf("project is required"), 400)
			return
		}
		p, err := projSvc.GetByAlias(r.Context(), project)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		labels, err := labelSvc.ListForProject(r.Context(), p.ID)
		if err != nil {
			jsonError(w, err, 500)
			return
		}
		if labels == nil {
			labels = []*models.Label{}
		}
		jsonOK(w, map[string]interface{}{"labels": labels})

	case http.MethodPost:
		var in struct {
			Project string `json:"project"`
			Name    string `json:"name"`
			Color   string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonError(w, err, 400)
			return
		}
		if strings.TrimSpace(in.Project) == "" || strings.TrimSpace(in.Name) == "" {
			jsonError(w, fmt.Errorf("project and name are required"), 400)
			return
		}
		p, err := projSvc.GetByAlias(r.Context(), strings.TrimSpace(in.Project))
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		label, err := labelSvc.Create(r.Context(), models.CreateLabelInput{
			ProjectID: p.ID,
			Name:      strings.TrimSpace(in.Name),
			Color:     strings.TrimSpace(in.Color),
			Actor:     s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, label)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

// ─── Plans ────────────────────────────────────────────────────────────────────

func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	// Empty project means "all projects" — the service/repo aggregate across
	// every (non-archived) project and populate project_alias / project_name
	// on each row so the client can render a project chip.
	project := r.URL.Query().Get("project")
	planSvc := service.NewPlanService(s.db)
	plans, err := planSvc.ListForProject(r.Context(), project)
	if err != nil {
		jsonError(w, err, 500)
		return
	}
	if plans == nil {
		jsonOK(w, map[string]interface{}{"plans": []interface{}{}})
		return
	}
	jsonOK(w, map[string]interface{}{"plans": plans})
}

func (s *Server) handlePlanByID(w http.ResponseWriter, r *http.Request) {
	planSvc := service.NewPlanService(s.db)

	// /api/plans/{id}[/history]
	path := strings.TrimPrefix(r.URL.Path, "/api/plans/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}

	ctx := r.Context()

	switch {
	case r.Method == http.MethodGet && sub == "":
		p, err := planSvc.Get(ctx, id, 0)
		if err != nil {
			jsonError(w, err, 404)
			return
		}
		jsonOK(w, p)

	case r.Method == http.MethodPatch && sub == "":
		var in struct {
			Title      string `json:"title"`
			Body       string `json:"body"`
			ChangeNote string `json:"change_note"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		p, err := planSvc.Update(ctx, id, models.UpdatePlanInput{
			Title:      in.Title,
			Body:       in.Body,
			ChangeNote: in.ChangeNote,
			Actor:      s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, p)

	case r.Method == http.MethodGet && sub == "history":
		versions, err := planSvc.History(ctx, id)
		if err != nil {
			jsonError(w, err, 404)
			return
		}
		jsonOK(w, map[string]interface{}{"versions": versions})

	default:
		http.Error(w, "not found", 404)
	}
}

// ─── Docs ─────────────────────────────────────────────────────────────────────

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	svc := service.NewDocService(s.db)
	switch r.Method {
	case http.MethodGet:
		project := r.URL.Query().Get("project")
		docs, err := svc.List(r.Context(), project)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		if docs == nil {
			docs = []*models.Doc{}
		}
		jsonOK(w, map[string]interface{}{"docs": docs})

	case http.MethodPost:
		var in struct {
			Project string `json:"project"`
			Title   string `json:"title"`
			Body    string `json:"body"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		d, err := svc.Create(r.Context(), in.Project, models.CreateDocInput{
			Title: in.Title, Body: in.Body, Actor: s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, d)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleDocByID(w http.ResponseWriter, r *http.Request) {
	svc := service.NewDocService(s.db)

	// /api/docs/{id}[/history]
	path := strings.TrimPrefix(r.URL.Path, "/api/docs/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}

	ctx := r.Context()

	if r.Method == http.MethodGet && sub == "history" {
		versions, err := svc.History(ctx, id)
		if err != nil {
			jsonError(w, err, 404)
			return
		}
		jsonOK(w, map[string]interface{}{"versions": versions})
		return
	}

	switch r.Method {
	case http.MethodGet:
		d, err := svc.Get(ctx, id)
		if err != nil {
			jsonError(w, err, 404)
			return
		}
		jsonOK(w, d)

	case http.MethodPatch:
		var in struct {
			Title      string `json:"title"`
			Body       string `json:"body"`
			ChangeNote string `json:"change_note"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		d, err := svc.Update(ctx, id, models.UpdateDocInput{
			Title: in.Title, Body: in.Body, ChangeNote: in.ChangeNote, Actor: s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, d)

	case http.MethodDelete:
		if err := svc.Delete(ctx, id, s.actor); err != nil {
			jsonError(w, err, 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

// ─── Memory ───────────────────────────────────────────────────────────────────

func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	svc := service.NewMemoryService(s.db)
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		entries, err := svc.List(r.Context(), q.Get("project"), q.Get("tag"))
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		if entries == nil {
			entries = []*models.Memory{}
		}
		jsonOK(w, map[string]interface{}{"entries": entries})

	case http.MethodPost:
		var in struct {
			Project string `json:"project"`
			Body    string `json:"body"`
			Tags    string `json:"tags"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		projSvc := service.NewProjectService(s.db)
		p, err := projSvc.GetByAlias(r.Context(), in.Project)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		m, err := svc.Add(r.Context(), models.CreateMemoryInput{
			ProjectID: p.ID, Body: in.Body, Tags: in.Tags, Actor: s.actor,
		})
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, m)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleMemoryByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/memory/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}

	svc := service.NewMemoryService(s.db)

	switch {
	case r.Method == http.MethodDelete && sub == "":
		if err := svc.Delete(r.Context(), id); err != nil {
			jsonError(w, err, 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodPatch && sub == "":
		var in struct {
			Body string `json:"body"`
			Tags string `json:"tags"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		m, err := svc.Update(r.Context(), id, in.Body, in.Tags)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, m)

	case r.Method == http.MethodPatch && sub == "append":
		var in struct {
			Value string `json:"value"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		m, err := svc.Append(r.Context(), id, in.Value, s.actor)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, m)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

// ─── Attachments ──────────────────────────────────────────────────────────────

func (s *Server) handleAttachments(w http.ResponseWriter, r *http.Request) {
	svc := service.NewAttachmentService(s.db)
	switch r.Method {
	case http.MethodGet:
		project := r.URL.Query().Get("project")
		taskID := r.URL.Query().Get("task_id")

		if taskID != "" {
			// List attachments for a specific task
			attachments, err := svc.List(r.Context(), "task", taskID)
			if err != nil {
				jsonError(w, err, 500)
				return
			}
			if attachments == nil {
				attachments = []*models.Attachment{}
			}
			jsonOK(w, map[string]interface{}{"attachments": attachments})
			return
		}

		// Empty project means "all projects" — the service/repo aggregate
		// across every (non-archived) project and populate project_alias /
		// project_name on each row so the client can render a project chip.
		attachments, err := svc.ListForProject(r.Context(), project)
		if err != nil {
			jsonError(w, err, 500)
			return
		}
		if attachments == nil {
			jsonOK(w, map[string]interface{}{"attachments": []interface{}{}})
			return
		}
		jsonOK(w, map[string]interface{}{"attachments": attachments})

	case http.MethodPost:
		// 32 MiB max in-memory cap for the multipart form; the service still
		// enforces the per-file 10 MiB limit.
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			jsonError(w, err, 400)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, fmt.Errorf("file field is required: %w", err), 400)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		linkedType := r.FormValue("linked_type")
		linkedID := r.FormValue("linked_id")
		if linkedType == "" || linkedID == "" {
			jsonError(w, fmt.Errorf("linked_type and linked_id are required"), 400)
			return
		}
		if linkedType != "task" && linkedType != "doc" {
			jsonError(w, fmt.Errorf("linked_type must be \"task\" or \"doc\""), 400)
			return
		}
		name := r.FormValue("name")
		if name == "" {
			name = header.Filename
		}
		a, err := svc.Add(r.Context(), name, data, linkedType, linkedID, s.actor)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		jsonOK(w, a)

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleAttachmentByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/attachments/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	svc := service.NewAttachmentService(s.db)
	switch r.Method {
	case http.MethodGet:
		a, err := svc.Get(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", a.MimeType)
		w.Header().Set("Content-Disposition", contentDispositionAttachment(a.Name))
		w.Write(a.Data)
	case http.MethodDelete:
		if err := svc.Delete(r.Context(), id, s.actor); err != nil {
			jsonError(w, err, 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// ─── Activity ─────────────────────────────────────────────────────────────────

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	q := r.URL.Query()
	projectAlias := q.Get("project")
	entityKind := q.Get("kind")
	actorKind := q.Get("actor_kind")
	actorName := q.Get("actor_name")
	if actor := strings.TrimSpace(q.Get("actor")); actor != "" {
		if parts := strings.SplitN(actor, ":", 2); len(parts) == 2 {
			actorKind = parts[0]
			actorName = parts[1]
		} else {
			actorName = actor
		}
	}
	limit := 10000
	offset := 0
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := q.Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
	if offset < 0 {
		offset = 0
	}

	entityID := q.Get("entity_id")
	if entityID != "" && entityKind == "" {
		entityKind = "task"
	}

	// Resolve project alias to ID
	projectID := ""
	if projectAlias != "" {
		projSvc := service.NewProjectService(s.db)
		p, err := projSvc.GetByAlias(r.Context(), projectAlias)
		if err != nil {
			jsonError(w, err, 400)
			return
		}
		projectID = p.ID
	}

	actRepo := repo.NewActivityRepo(s.db)
	var events []*models.Activity
	var err error
	if entityID != "" {
		// Direct per-entity lookup — ignores other filters for simplicity
		events, err = actRepo.ListForEntity(r.Context(), entityKind, entityID)
	} else {
		events, err = actRepo.List(r.Context(), projectID, entityKind, actorKind, actorName, limit, offset)
	}
	if err != nil {
		jsonError(w, err, 500)
		return
	}
	if events == nil {
		events = []*models.Activity{}
	}
	jsonOK(w, map[string]interface{}{"events": events, "total": len(events)})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func webDueAt(dueAt *int64, dueDate string) (*int64, error) {
	if dueAt != nil {
		return dueAt, nil
	}
	dueDate = strings.TrimSpace(dueDate)
	if dueDate == "" {
		return nil, nil
	}
	formats := []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"}
	for _, f := range formats {
		if t, err := time.Parse(f, dueDate); err == nil {
			ns := t.UnixNano()
			return &ns, nil
		}
	}
	if n, err := strconv.ParseInt(dueDate, 10, 64); err == nil {
		return &n, nil
	}
	return nil, fmt.Errorf("invalid due date %q (use YYYY-MM-DD or RFC3339)", dueDate)
}

// isFTSSyntaxError reports whether err is a SQLite FTS5 query syntax error.
// Such errors come from user-supplied search text, so the web layer maps them
// to a clean 400 instead of leaking the raw SQL engine message.
func isFTSSyntaxError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "fts5: syntax error") ||
		strings.Contains(msg, "unterminated string")
}

// contentDispositionAttachment builds a safe Content-Disposition header from a
// user-supplied filename. The ASCII filename has CR/LF/quote/backslash stripped
// to block header injection; the original filename is also emitted as a
// percent-encoded RFC 5987 filename* parameter so clients that support UTF-8
// can still see the real name.
func contentDispositionAttachment(name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '"' || r == '\\' || r < 0x20 || r > 0x7e {
			return '_'
		}
		return r
	}, name)
	if ascii == "" {
		ascii = "download"
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(name))
}
